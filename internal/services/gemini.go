package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"iter"
	"os"

	"google.golang.org/genai"
)

type GeminiService struct {
	client *genai.Client
}

func NewGeminiService(ctx context.Context, apiKey string) (*GeminiService, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}
	return &GeminiService{client: client}, nil
}

func (s *GeminiService) UploadFile(ctx context.Context, path string) (*genai.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	result, err := s.client.Files.Upload(ctx, f, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}
	return result, nil
}

func (s *GeminiService) GenerateTitle(ctx context.Context, prompt string) (string, error) {
	model := "gemini-2.5-flash"
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: "You are a helpful assistant. you jo is to generate a title for given prompt within 20 CHARACTERS."}},
		},
	}
	resp, err := s.client.Models.GenerateContent(ctx, model, []*genai.Content{{Parts: []*genai.Part{{Text: prompt}}}}, config)
	if err != nil {
		return "", fmt.Errorf("failed to generate title: %w", err)
	}
	return resp.Candidates[0].Content.Parts[0].Text, nil
}

// generateImageTool generates an image using the Gemini model.
// This matches the Python reference's tool_generate_image.
func (s *GeminiService) generateImageTool(ctx context.Context, prompt string) (string, []byte, error) {
	fmt.Printf("Generating image for prompt: %s\n", prompt)
	model := "gemini-2.5-flash-image" // Using flash-exp as it often supports image gen or use specific image model

	resp, err := s.client.Models.GenerateContent(ctx, model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, &genai.GenerateContentConfig{
		ResponseModalities: []string{"IMAGE"},
	})
	if err != nil {
		return "", nil, fmt.Errorf("image generation failed: %w", err)
	}

	for _, cand := range resp.Candidates {
		for _, part := range cand.Content.Parts {
			if part.InlineData != nil {
				return "Image generated successfully.", part.InlineData.Data, nil
			}
		}
	}
	return "Failed to generate image.", nil, nil
}

func (s *GeminiService) ChatStream(ctx context.Context, prompt string, fileURI string, imageCallback func(string)) (iter.Seq2[*genai.GenerateContentResponse, error], error) {
	model := "gemini-2.5-flash"

	var parts []*genai.Part
	if fileURI != "" {
		parts = append(parts, &genai.Part{
			FileData: &genai.FileData{
				FileURI: fileURI,
				MIMEType: "application/pdf",
			},
		})
	}
	parts = append(parts, &genai.Part{Text: prompt})

	contents := []*genai.Content{{Parts: parts}}
	
	toolDecl := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name: "generate_image",
				Description: "Generates an image based on the prompt.",
				Parameters: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"prompt": {
							Type: genai.TypeString,
							Description: "The prompt for the image generation.",
						},
					},
					Required: []string{"prompt"},
				},
			},
		},
	}

	config := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{toolDecl},
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: "You are a helpful assistant. If the user asks for an image, use the generate_image tool. KEEP YOUR RESPONSES IN PURE STYLISH MARKDOWN FORMAT."}},
		},
	}

	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		streamIterator := s.client.Models.GenerateContentStream(ctx, model, contents, config)
		
		var functionCall *genai.FunctionCall
		
		for resp, err := range streamIterator {
			if err != nil {
				yield(nil, err)
				return
			}
			
			for _, cand := range resp.Candidates {
				for _, part := range cand.Content.Parts {
					if part.FunctionCall != nil {
						functionCall = part.FunctionCall
					}
				}
			}
			
			if !yield(resp, nil) {
				return
			}
		}

		if functionCall != nil && functionCall.Name == "generate_image" {
			args := functionCall.Args
			promptVal, ok := args["prompt"].(string)
			if !ok {
				yield(nil, fmt.Errorf("invalid prompt argument"))
				return
			}
			
			msg, imgData, err := s.generateImageTool(ctx, promptVal)
			if err != nil {
				yield(nil, fmt.Errorf("tool error: %v", err))
				return
			}
			
			if imgData != nil {
				imgB64 := base64.StdEncoding.EncodeToString(imgData)
				imageCallback(imgB64)
			}
			
			
			
			history := []*genai.Content{}
			history = append(history, contents...)
			history = append(history, &genai.Content{
				Role: "model",
				Parts: []*genai.Part{{FunctionCall: functionCall}},
			})
			history = append(history, &genai.Content{
				Role: "user",
				Parts: []*genai.Part{{
					FunctionResponse: &genai.FunctionResponse{
						Name: "generate_image",
						Response: map[string]any{"result": msg},
					},
				}},
			})
			
			secondStream := s.client.Models.GenerateContentStream(ctx, model, history, config)
			for resp, err := range secondStream {
				if !yield(resp, err) {
					return
				}
			}
		}
	}, nil
}
