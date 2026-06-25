package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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
	)
	return &AIService{client: client}
}

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
- action fields: action (send_email/approve/reject), params (optional key-value pairs)
- Return ONLY the JSON, no explanation, no markdown`

func (a *AIService) GenerateDSL(description string) (*models.DSL, error) {
	resp, err := a.client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Model: "llama-3.3-70b-versatile",
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &openai.ResponseFormatJSONObjectParam{},
		},
		Messages: []openai.ChatCompletionMessageParamUnion{
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
		},
	})
	if err != nil {
		return nil, err
	}

	var dsl models.DSL
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &dsl); err != nil {
		return nil, fmt.Errorf("AI returned invalid DSL: %w", err)
	}

	return &dsl, nil
}