package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"workflow-engine/models"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type AIService struct {
	client openai.Client
}

func NewAIService() *AIService {
	client := openai.NewClient(
		option.WithAPIKey(os.Getenv("GROQ_API_KEY")),
		option.WithBaseURL("https://api.groq.com/openai/v1"),
		// The SDK retries 429/5xx internally by default, silently sleeping
		// inside a single call — which would hide rate-limit backoff time
		// inside what's supposed to be a clean "model latency" measurement.
		// We disable that and do our own instrumented retry in generate().
		option.WithMaxRetries(0),
	)
	return &AIService{client: client}
}

// maxRateLimitRetries caps how many times generate() will retry a 429
// before giving up — mirrors the SDK's old default of 2 retries.
const maxRateLimitRetries = 2

var systemPrompt = `You are a workflow DSL generator. Convert natural language workflow descriptions into structured JSON.

You must return ONLY valid JSON with this exact structure:
{
  "steps": [
    {
      "type": "condition",
      "field": "age",
      "operator": "gt",
      "value": 18,
      "on_pass": "next",
      "on_fail": "reject"
    },
    {
      "type": "action",
      "action": "send_email",
      "params": {
        "template": "welcome"
      }
    }
  ]
}

Rules:
- type is either "condition" or "action"
- condition fields: field, operator (gt/lt/eq/gte/lte), value, on_pass, on_fail
- on_pass and on_fail are either "next" or "reject"
- field can be any user attribute: age, email, country, credit_score, role, status, etc.

Available actions:
- approve: mark workflow as approved, no params needed
- reject: mark workflow as rejected, no params needed
- send_email: send an email, params: { "template": "<template_name>", "to": "<email_field>" }
- assign_role: assign a role to the user, params: { "role": "<role_name>" }
- webhook: call an external URL, params: { "url": "<url>", "method": "POST" }

- Return ONLY the JSON, no explanation, no markdown`

// GenerateDSL asks the AI to convert a description into DSL.
// It returns the parsed DSL, the raw text the AI replied with (needed if a
// caller wants to retry with correction), how long was spent waiting out
// rate-limit backoff (separate from genuine call time), and an error if the
// request ultimately failed or the reply wasn't valid JSON.
func (a *AIService) GenerateDSL(description string) (*models.DSL, string, time.Duration, error) {
	return a.generate(description, nil)
}

// RetryDSL re-asks the AI for a corrected DSL, replaying its previous (bad)
// reply plus the validation errors it needs to fix. Intended for a single
// retry after GenerateDSL produces invalid output — not a general retry loop.
func (a *AIService) RetryDSL(description string, previousRaw string, validationErrs []error) (*models.DSL, string, time.Duration, error) {
	reasons := make([]string, len(validationErrs))
	for i, e := range validationErrs {
		reasons[i] = e.Error()
	}

	correction := []openai.ChatCompletionMessageParamUnion{
		{OfAssistant: &openai.ChatCompletionAssistantMessageParam{
			Content: openai.ChatCompletionAssistantMessageParamContentUnion{
				OfString: openai.String(previousRaw),
			},
		}},
		{OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(fmt.Sprintf(
					"That response was invalid for these reasons:\n- %s\n\nFix these issues and return corrected JSON only, following the schema exactly. No explanation, no markdown.",
					strings.Join(reasons, "\n- "),
				)),
			},
		}},
	}

	return a.generate(description, correction)
}

// generate calls the model, retrying on 429s itself (SDK auto-retry is
// disabled in NewAIService) so rate-limit backoff can be timed and reported
// separately from genuine call latency.
func (a *AIService) generate(description string, extra []openai.ChatCompletionMessageParamUnion) (*models.DSL, string, time.Duration, error) {
	messages := []openai.ChatCompletionMessageParamUnion{
		{OfSystem: &openai.ChatCompletionSystemMessageParam{
			Content: openai.ChatCompletionSystemMessageParamContentUnion{
				OfString: openai.String(systemPrompt),
			},
		}},
		{OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(fmt.Sprintf("Generate DSL for: %s", description)),
			},
		}},
	}
	messages = append(messages, extra...)

	var rateLimitWait time.Duration

	for attempt := 0; ; attempt++ {
		resp, err := a.client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
			Model: "openai/gpt-oss-120b",
			ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: &openai.ResponseFormatJSONObjectParam{},
			},
			Messages: messages,
		})
		if err != nil {
			var apiErr *openai.Error
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests && attempt < maxRateLimitRetries {
				wait := rateLimitBackoff(apiErr, attempt)
				rateLimitWait += wait
				time.Sleep(wait)
				continue
			}
			return nil, "", rateLimitWait, err
		}

		raw := resp.Choices[0].Message.Content

		var dsl models.DSL
		if err := json.Unmarshal([]byte(raw), &dsl); err != nil {
			return nil, raw, rateLimitWait, fmt.Errorf("AI returned invalid DSL: %w", err)
		}

		return &dsl, raw, rateLimitWait, nil
	}
}

// rateLimitBackoff honors the Retry-After header Groq sends on a 429 when
// present, otherwise falls back to a short exponential backoff.
func rateLimitBackoff(apiErr *openai.Error, attempt int) time.Duration {
	if apiErr.Response != nil {
		if ra := apiErr.Response.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				return time.Duration(secs) * time.Second
			}
		}
	}
	return time.Duration(1<<attempt) * time.Second
}