package main

import (
	"context"

	"github.com/lemon-mint/coord"
	"github.com/lemon-mint/coord/llm"
	"github.com/lemon-mint/coord/pconf"
	"github.com/lemon-mint/coord/provider"

	_ "github.com/lemon-mint/coord/provider/aistudio"
	_ "github.com/lemon-mint/coord/provider/anthropic"
	_ "github.com/lemon-mint/coord/provider/openai"
	_ "github.com/lemon-mint/coord/provider/vertexai"
)

func Connect(m Providers) (c provider.LLMClient, err error) {
	switch m.Type {
	case "aistudio":
		c, err = coord.NewLLMClient(context.Background(), "aistudio", pconf.WithAPIKey(m.APIKey))
	case "anthropic":
		c, err = coord.NewLLMClient(context.Background(), "anthropic", pconf.WithAPIKey(m.APIKey))
	case "openai":
		if m.Baseurl != "" {
			c, err = coord.NewLLMClient(context.Background(), "openai", pconf.WithAPIKey(m.APIKey), pconf.WithBaseURL(m.Baseurl))
			break
		}
		c, err = coord.NewLLMClient(context.Background(), "openai", pconf.WithAPIKey(m.APIKey))
	case "vertexai":
		c, err = coord.NewLLMClient(context.Background(), "vertexai", pconf.WithLocation(m.Location), pconf.WithProjectID(m.ProjectID))
	}
	return
}

func GetModel(c provider.LLMClient, name string, params Parameters) (m llm.Model, err error) {
	config := new(llm.Config)
	config.Temperature = &params.Temperature
	config.SafetyFilterThreshold = llm.BlockOnlyHigh
	if params.TopP != 0 {
		config.TopP = &params.TopP
	}
	if params.TopK != 0 {
		config.TopK = &params.TopK
	}
	if params.MaxTokens != 0 {
		config.MaxOutputTokens = &params.MaxTokens
	}

	return c.NewLLM(name, config)
}
