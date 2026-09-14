//go:build ignore

package main

import (
	"context"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"mebot/config"
	"log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if len(cfg.NvidiaAPIKeys) == 0 {
		log.Fatal("no nvidia keys")
	}

	key := cfg.NvidiaAPIKeys[0]

	ocfg := openai.DefaultConfig(key)
	ocfg.BaseURL = "https://integrate.api.nvidia.com/v1"

	c := openai.NewClientWithConfig(ocfg)

	// test with user's model input
	req := openai.ChatCompletionRequest{
		Model: "llama-3.3-70b-instruct",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Hello world",
			},
		},
	}

	res, err := c.CreateChatCompletion(context.Background(), req)
	if err != nil {
		fmt.Printf("Error with llama-3.3-70b-instruct: %v\n", err)
	} else {
		fmt.Printf("Success: %v\n", res.Choices[0].Message.Content)
	}

	// test with meta/ prefix
	req.Model = "meta/llama-3.3-70b-instruct"
	res2, err := c.CreateChatCompletion(context.Background(), req)
	if err != nil {
		fmt.Printf("Error with meta/: %v\n", err)
	} else {
		fmt.Printf("Success: %v\n", res2.Choices[0].Message.Content)
	}
}
