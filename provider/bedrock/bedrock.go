// Package bedrock implements provider.Provider on top of the AWS
// Bedrock Converse API, giving hermind access to every model family
// Bedrock hosts (Claude, Llama, Mistral, Titan, etc.) through a single
// adapter.
//
// Configuration: the Bedrock provider intentionally does not add fields
// to config.ProviderConfig. Instead, it relies on the standard AWS
// credential chain:
//
//   - Region: AWS_REGION env var, or the active shared-config profile.
//   - Credentials: AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY env vars,
//     AWS_PROFILE (for shared ~/.aws/credentials), IAM roles, SSO,
//     instance metadata — whatever awsconfig.LoadDefaultConfig finds.
//
// cfg.APIKey is ignored; cfg.BaseURL is passed through as a custom
// Bedrock endpoint URL if set (useful for VPC endpoints or test
// doubles). cfg.Model supplies the Bedrock model ID.
package bedrock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
)

// converseAPI is the subset of bedrockruntime.Client that this package
// uses. Splitting it out lets tests inject a fake without touching AWS.
type converseAPI interface {
	Converse(ctx context.Context, in *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error)
	ConverseStream(ctx context.Context, in *bedrockruntime.ConverseStreamInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamOutput, error)
}

// Bedrock is the provider.Provider implementation for AWS Bedrock models.
type Bedrock struct {
	client converseAPI
	model  string
	region string
}

// New constructs a Bedrock provider from config.
//
// Region resolution order:
//  1. AWS_REGION environment variable.
//  2. The active shared-config profile's region.
//
// Credentials come from the standard AWS default chain (env vars,
// shared config file, IAM roles, SSO, instance metadata).
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	region := os.Getenv("AWS_REGION")

	loadOpts := []func(*awsconfig.LoadOptions) error{}
	if region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(region))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("bedrock: load aws config: %w", err)
	}
	if region == "" {
		region = awsCfg.Region
	}
	if region == "" {
		return nil, errors.New("bedrock: region is required (set AWS_REGION or configure an AWS profile)")
	}

	clientOpts := []func(*bedrockruntime.Options){}
	if cfg.BaseURL != "" {
		url := cfg.BaseURL
		clientOpts = append(clientOpts, func(o *bedrockruntime.Options) {
			o.BaseEndpoint = aws.String(url)
		})
	}

	return &Bedrock{
		client: bedrockruntime.NewFromConfig(awsCfg, clientOpts...),
		model:  cfg.Model,
		region: region,
	}, nil
}

// Name returns "bedrock".
func (b *Bedrock) Name() string { return "bedrock" }

// Available returns true when the underlying client is constructed.
// The AWS SDK defers credential resolution until the first request, so
// this optimistically reports true and surfaces credential failures as
// provider.Error on the first call.
func (b *Bedrock) Available() bool { return b != nil && b.client != nil }

// ModelInfo returns capabilities for known Bedrock-hosted models.
// Unknown models fall back to a conservative default that still
// enables tools and streaming — Converse supports both universally.
func (b *Bedrock) ModelInfo(model string) *provider.ModelInfo {
	switch {
	case hasAnyPrefix(model,
		"anthropic.claude-opus-4", "anthropic.claude-sonnet-4",
		"us.anthropic.claude-opus-4", "us.anthropic.claude-sonnet-4",
		"global.anthropic.claude-opus-4", "global.anthropic.claude-sonnet-4"):
		return &provider.ModelInfo{
			ContextLength:     200_000,
			MaxOutputTokens:   8_192,
			SupportsVision:    true,
			SupportsTools:     true,
			SupportsStreaming: true,
			SupportsCaching:   true,
			SupportsReasoning: false,
		}
	case hasAnyPrefix(model,
		"anthropic.claude-3", "us.anthropic.claude-3", "eu.anthropic.claude-3"):
		return &provider.ModelInfo{
			ContextLength:     200_000,
			MaxOutputTokens:   4_096,
			SupportsVision:    true,
			SupportsTools:     true,
			SupportsStreaming: true,
			SupportsCaching:   false,
			SupportsReasoning: false,
		}
	default:
		return &provider.ModelInfo{
			ContextLength:     128_000,
			MaxOutputTokens:   4_096,
			SupportsVision:    false,
			SupportsTools:     true,
			SupportsStreaming: true,
			SupportsCaching:   false,
			SupportsReasoning: false,
		}
	}
}

// EstimateTokens uses a ~4-chars-per-token heuristic, matching the
// other provider bootstrap implementations. A future change can swap
// in a per-family tokenizer.
func (b *Bedrock) EstimateTokens(model, text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	return (len(text) + 3) / 4, nil
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
