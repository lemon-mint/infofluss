package main

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/google/go-jsonnet"
	"gopkg.eu.org/envloader"
)

func LoadConfig(file string) (*Config, error) {
	envloader.LoadEnvFile(".env")
	vm := jsonnet.MakeVM()

	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			vm.ExtVar("ENV_"+k, v)
		}
	}

	jsondata, err := vm.EvaluateFile(file)
	if err != nil {
		return nil, err
	}

	var c Config
	err = json.Unmarshal([]byte(jsondata), &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

type Config struct {
	ModelConfigs    ModelConfigs   `json:"model_configs"`
	CrawlerConfigs  CrawlerConfigs `json:"crawler_configs"`
	Providers       []Providers    `json:"providers"`
	SearchEngines   []string       `json:"search_engines"`
	SearchEndpoints []string       `json:"search_endpoints"`
}

type Parameters struct {
	Temperature float32 `json:"temperature"`
	TopP        float32 `json:"top_p"`
	TopK        int     `json:"top_k"`
	MaxTokens   int     `json:"max_tokens"`
}

type ModelConfig struct {
	Model      string     `json:"model"`
	Parameters Parameters `json:"parameters"`
	Provider   string     `json:"provider"`
}

type ModelConfigs struct {
	QueryPlanner      ModelConfig `json:"query_planner"`
	ResponseGenerator ModelConfig `json:"response_generator"`
	SearchReranker    ModelConfig `json:"search_reranker"`
	Chat              ModelConfig `json:"chat"`
}

type Providers struct {
	APIKey    string `json:"api_key"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Baseurl   string `json:"baseurl,omitempty"`
	Location  string `json:"location,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
}

type CrawlerConfigs struct {
	Mode string `json:"mode"`
}
