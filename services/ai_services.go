package services

import (
	"context"
	"fmt"
	"os"

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

func (a *AIService) GenerateDSL(description string) (string, error) {
	resp, err := a.client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Model: "llama-3.3-70b-versatile",
		Messages: []openai.ChatCompletionMessageParamUnion{
			{OfUser: &openai.ChatCompletionUserMessageParam{
				Content: openai.ChatCompletionUserMessageParamContentUnion{
					OfString: openai.String(fmt.Sprintf("Generate a DSL for this workflow: %s", description)),
				},
			}},
		},
	})
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}