package missioncontrol

import (
	"context"
	"fmt"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// GetLLMProvider creates an LLM provider from the PicoClaw config.
// If modelName is empty, it uses the default model from config.
func GetLLMProvider(configPath, modelName string) (providers.LLMProvider, string, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load config: %w", err)
	}

	if modelName == "" {
		modelName = cfg.Agents.Defaults.GetModelName()
	}

	if modelName == "" {
		return nil, "", fmt.Errorf("no model configured: set a default model in config")
	}

	modelCfg, err := cfg.GetModelConfig(modelName)
	if err != nil || modelCfg == nil {
		return nil, "", fmt.Errorf("model %q not found in config: %w", modelName, err)
	}

	provider, strippedModel, err := providers.CreateProviderFromConfig(modelCfg)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create provider for model %q: %w", modelName, err)
	}

	return provider, strippedModel, nil
}

// CompleteWithConfig is a convenience function that loads the config, creates
// a provider, and calls Complete in one step.
func CompleteWithConfig(ctx context.Context, configPath, modelName string, req LLMRequest) (*LLMResponse, error) {
	provider, model, err := GetLLMProvider(configPath, modelName)
	if err != nil {
		return nil, err
	}
	if req.Model == "" {
		req.Model = model
	}
	return Complete(ctx, req, provider)
}

// CompleteJSONWithConfig is a convenience function that loads the config, creates
// a provider, and calls CompleteJSON in one step.
func CompleteJSONWithConfig(ctx context.Context, configPath, modelName string, req LLMRequest, target any) error {
	provider, model, err := GetLLMProvider(configPath, modelName)
	if err != nil {
		return err
	}
	if req.Model == "" {
		req.Model = model
	}
	return CompleteJSON(ctx, req, provider, target)
}