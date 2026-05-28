package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
	pantheonAnthropic "github.com/odysseythink/pantheon/providers/anthropic"
	pantheonApiPie "github.com/odysseythink/pantheon/providers/apipie"
	pantheonAzure "github.com/odysseythink/pantheon/providers/azure"
	pantheonBedrock "github.com/odysseythink/pantheon/providers/bedrock"
	pantheonCohere "github.com/odysseythink/pantheon/providers/cohere"
	pantheonCometAPI "github.com/odysseythink/pantheon/providers/cometapi"
	pantheonDeepSeek "github.com/odysseythink/pantheon/providers/deepseek"
	pantheonDellPro "github.com/odysseythink/pantheon/providers/dellproaistudio"
	pantheonDockerModelRunner "github.com/odysseythink/pantheon/providers/dockermodelrunner"
	pantheonFireworks "github.com/odysseythink/pantheon/providers/fireworks"
	pantheonFoundry "github.com/odysseythink/pantheon/providers/foundry"
	pantheonGenericOpenAI "github.com/odysseythink/pantheon/providers/genericopenai"
	pantheonGiteeAI "github.com/odysseythink/pantheon/providers/giteeai"
	pantheonGoogle "github.com/odysseythink/pantheon/providers/google"
	pantheonGroq "github.com/odysseythink/pantheon/providers/groq"
	pantheonHuggingFace "github.com/odysseythink/pantheon/providers/huggingface"
	pantheonMoonshot "github.com/odysseythink/pantheon/providers/kimi"
	pantheonKoboldCPP "github.com/odysseythink/pantheon/providers/koboldcpp"
	pantheonLemonade "github.com/odysseythink/pantheon/providers/lemonade"
	pantheonLiteLLM "github.com/odysseythink/pantheon/providers/litellm"
	pantheonLMStudio "github.com/odysseythink/pantheon/providers/lmstudio"
	pantheonLocalAI "github.com/odysseythink/pantheon/providers/localai"
	pantheonMistral "github.com/odysseythink/pantheon/providers/mistral"
	pantheonNovita "github.com/odysseythink/pantheon/providers/novita"
	pantheonNvidiaNIM "github.com/odysseythink/pantheon/providers/nvidianim"
	pantheonOllama "github.com/odysseythink/pantheon/providers/ollama"
	pantheonOpenAI "github.com/odysseythink/pantheon/providers/openai"
	pantheonOpenRouter "github.com/odysseythink/pantheon/providers/openrouter"
	pantheonPerplexity "github.com/odysseythink/pantheon/providers/perplexity"
	pantheonPPIO "github.com/odysseythink/pantheon/providers/ppio"
	pantheonPrivateMode "github.com/odysseythink/pantheon/providers/privatemode"
	pantheonSambaNova "github.com/odysseythink/pantheon/providers/sambanova"
	pantheonTextGenWebUI "github.com/odysseythink/pantheon/providers/textgenwebui"
	pantheonTogether "github.com/odysseythink/pantheon/providers/together"
	pantheonMinimax "github.com/odysseythink/pantheon/providers/minimax"
	pantheonQwen "github.com/odysseythink/pantheon/providers/qwen"
	pantheonWenxin "github.com/odysseythink/pantheon/providers/wenxin"
	pantheonXAI "github.com/odysseythink/pantheon/providers/xai"
	pantheonZAI "github.com/odysseythink/pantheon/providers/zai"
	pantheonZhipu "github.com/odysseythink/pantheon/providers/zhipu"
)

func init() {
	providerRegistry["openai"] = buildOpenAI
	providerRegistry["azure"] = buildAzure
	providerRegistry["anthropic"] = buildAnthropic
	providerRegistry["gemini"] = buildGoogle
	providerRegistry["lmstudio"] = buildLMStudio
	providerRegistry["localai"] = buildLocalAI
	providerRegistry["ollama"] = buildOllama
	providerRegistry["togetherai"] = buildTogether
	providerRegistry["fireworksai"] = buildFireworks
	providerRegistry["mistral"] = buildMistral
	providerRegistry["huggingface"] = buildHuggingFace
	providerRegistry["perplexity"] = buildPerplexity
	providerRegistry["openrouter"] = buildOpenRouter
	providerRegistry["novita"] = buildNovita
	providerRegistry["groq"] = buildGroq
	providerRegistry["koboldcpp"] = buildKoboldCPP
	providerRegistry["textgenwebui"] = buildTextGenWebUI
	providerRegistry["cohere"] = buildCohere
	providerRegistry["litellm"] = buildLiteLLM
	providerRegistry["generic-openai"] = buildGenericOpenAI
	providerRegistry["bedrock"] = buildBedrock
	providerRegistry["deepseek"] = buildDeepSeek
	providerRegistry["apipie"] = buildApiPie
	providerRegistry["xai"] = buildXAI
	providerRegistry["nvidia-nim"] = buildNvidiaNIM
	providerRegistry["ppio"] = buildPPIO
	providerRegistry["dpaiStudio"] = buildDellPro
	providerRegistry["moonshotai"] = buildMoonshot
	providerRegistry["cometapi"] = buildCometAPI
	providerRegistry["foundry"] = buildFoundry
	providerRegistry["zai"] = buildZAI
	providerRegistry["giteeai"] = buildGiteeAI
	providerRegistry["docker-model-runner"] = buildDockerModelRunner
	providerRegistry["privatemode"] = buildPrivateMode
	providerRegistry["sambanova"] = buildSambaNova
	providerRegistry["lemonade"] = buildLemonade
	providerRegistry["minimax"] = buildMinimax
	providerRegistry["qwen"] = buildQwen
	providerRegistry["wenxin"] = buildWenxin
	providerRegistry["zhipu"] = buildZhipu
}

func buildOpenAI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["OpenAiKey"],
		cfg.OpenAiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no OpenAI API key configured")
	}
	p, err := pantheonOpenAI.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildAzure(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["AzureOpenAiKey"],
		cfg.AzureOpenAiKey,
		cfg.LLMApiKey,
	)
	resourceName := firstNonEmpty(
		settings["AzureOpenAiResourceName"],
		cfg.AzureOpenAiResourceName,
	)
	deployment := firstNonEmpty(
		settings["AzureOpenAiDeployment"],
		cfg.AzureOpenAiDeployment,
	)

	if resourceName == "" || deployment == "" {
		endpoint := firstNonEmpty(
			settings["AzureOpenAiEndpoint"],
			cfg.AzureOpenAiEndpoint,
		)
		if endpoint != "" {
			rn, dep, err := parseAzureEndpoint(endpoint)
			if err == nil {
				if resourceName == "" {
					resourceName = rn
				}
				if deployment == "" {
					deployment = dep
				}
			} else {
				mlog.Warning("buildAzure: failed to parse endpoint: ", err)
			}
		}
	}

	if apiKey == "" || resourceName == "" || deployment == "" {
		return nil, fmt.Errorf("azure: apiKey, resourceName, and deployment are required")
	}

	opts := []pantheonAzure.Option{}
	if endpoint := cfg.AzureOpenAiEndpoint; endpoint != "" {
		opts = append(opts, pantheonAzure.WithBaseURL(endpoint))
	}

	p, err := pantheonAzure.New(apiKey, resourceName, deployment, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildAnthropic(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["AnthropicApiKey"],
		cfg.AnthropicApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Anthropic API key configured")
	}
	p, err := pantheonAnthropic.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildGoogle(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["GeminiLLMApiKey"],
		cfg.GeminiApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Gemini API key configured")
	}
	p, err := pantheonGoogle.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildLMStudio(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["LMStudioBasePath"],
		cfg.LMStudioBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no LMStudio base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	p, err := pantheonLMStudio.New("", pantheonLMStudio.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildLocalAI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["LocalAiBasePath"],
		cfg.LocalAiBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no LocalAI base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["LocalAiApiKey"],
		cfg.LocalAiApiKey,
	)

	opts := []pantheonLocalAI.Option{pantheonLocalAI.WithBaseURL(baseURL)}
	p, err := pantheonLocalAI.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildOllama(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["OllamaLLMBasePath"],
		cfg.OllamaBasePath,
	)
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")

	p, err := pantheonOllama.New("", pantheonOllama.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildTogether(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["TogetherAiApiKey"],
		cfg.TogetherAiApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no TogetherAI API key configured")
	}
	p, err := pantheonTogether.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildFireworks(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["FireworksApiKey"],
		cfg.FireworksApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Fireworks API key configured")
	}
	p, err := pantheonFireworks.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildMistral(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["MistralApiKey"],
		cfg.MistralApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Mistral API key configured")
	}
	p, err := pantheonMistral.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildHuggingFace(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	endpoint := firstNonEmpty(
		settings["HuggingFaceLLMEndpoint"],
		cfg.HuggingFaceEndpoint,
	)
	if endpoint == "" {
		return nil, fmt.Errorf("no HuggingFace endpoint configured")
	}
	apiKey := firstNonEmpty(
		settings["HuggingFaceLLMAccessToken"],
		cfg.HuggingFaceApiKey,
		cfg.LLMApiKey,
	)

	opts := []pantheonHuggingFace.Option{pantheonHuggingFace.WithBaseURL(endpoint)}
	p, err := pantheonHuggingFace.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildPerplexity(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["PerplexityApiKey"],
		cfg.PerplexityApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Perplexity API key configured")
	}
	p, err := pantheonPerplexity.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildOpenRouter(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["OpenRouterApiKey"],
		cfg.OpenRouterApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no OpenRouter API key configured")
	}
	p, err := pantheonOpenRouter.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildNovita(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["NovitaLLMApiKey"],
		cfg.NovitaLLMApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Novita API key configured")
	}
	p, err := pantheonNovita.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildGroq(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["GroqApiKey"],
		cfg.GroqApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Groq API key configured")
	}
	p, err := pantheonGroq.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildKoboldCPP(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["KoboldCPPBasePath"],
		cfg.KoboldBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no KoboldCPP base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["KoboldCPPApiKey"],
		cfg.LLMApiKey,
	)

	opts := []pantheonKoboldCPP.Option{pantheonKoboldCPP.WithBaseURL(baseURL)}
	p, err := pantheonKoboldCPP.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildTextGenWebUI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["TextGenWebUIBasePath"],
		cfg.TextGenBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no TextGenWebUI base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["TextGenWebUIAPIKey"],
		cfg.TextGenApiKey,
		cfg.LLMApiKey,
	)

	opts := []pantheonTextGenWebUI.Option{pantheonTextGenWebUI.WithBaseURL(baseURL)}
	p, err := pantheonTextGenWebUI.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildCohere(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["CohereApiKey"],
		cfg.CohereApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Cohere API key configured")
	}
	p, err := pantheonCohere.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildLiteLLM(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["LiteLLMBasePath"],
		cfg.LiteLLMBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no LiteLLM base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["LiteLLMApiKey"],
		cfg.LiteLLMApiKey,
		cfg.LLMApiKey,
	)

	opts := []pantheonLiteLLM.Option{pantheonLiteLLM.WithBaseURL(baseURL)}
	p, err := pantheonLiteLLM.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildGenericOpenAI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["GenericOpenAiBasePath"],
		cfg.GenericOpenAiBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no GenericOpenAI base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["GenericOpenAiKey"],
		cfg.GenericOpenAiKey,
		cfg.LLMApiKey,
	)

	opts := []pantheonGenericOpenAI.Option{pantheonGenericOpenAI.WithBaseURL(baseURL)}
	p, err := pantheonGenericOpenAI.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildBedrock(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	accessKeyID := firstNonEmpty(
		settings["AwsBedrockLLMAccessKeyId"],
		cfg.AwsBedrockLLMAccessKeyId,
	)
	secretKey := firstNonEmpty(
		settings["AwsBedrockLLMAccessKey"],
		cfg.AwsBedrockLLMAccessKey,
	)
	region := firstNonEmpty(
		settings["AwsBedrockLLMRegion"],
		cfg.AwsBedrockLLMRegion,
	)
	sessionToken := firstNonEmpty(
		settings["AwsBedrockLLMSessionToken"],
		cfg.AwsBedrockLLMSessionToken,
	)

	if accessKeyID == "" || secretKey == "" || region == "" {
		return nil, fmt.Errorf("bedrock: accessKeyID, secretKey, and region are required")
	}

	opts := []pantheonBedrock.Option{}
	if sessionToken != "" {
		opts = append(opts, pantheonBedrock.WithSessionToken(sessionToken))
	}

	p, err := pantheonBedrock.New(accessKeyID, secretKey, region, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildDeepSeek(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["DeepSeekApiKey"],
		cfg.DeepSeekApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no DeepSeek API key configured")
	}
	p, err := pantheonDeepSeek.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildApiPie(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["ApiPieApiKey"],
		cfg.ApiPieApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no ApiPie API key configured")
	}
	p, err := pantheonApiPie.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildXAI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["XAIApiKey"],
		cfg.XAIApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no XAI API key configured")
	}
	p, err := pantheonXAI.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildNvidiaNIM(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["NvidiaNimLLMBasePath"],
		cfg.NvidiaNimLLMBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no NVIDIA NIM base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	p, err := pantheonNvidiaNIM.New("", pantheonNvidiaNIM.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildPPIO(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["PpioApiKey"],
		cfg.PpioApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no PPIO API key configured")
	}
	p, err := pantheonPPIO.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildDellPro(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["DellProAiStudioBasePath"],
		cfg.DellProAiStudioBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no Dell Pro AI Studio base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["DellProApiKey"],
		cfg.LLMApiKey,
	)

	opts := []pantheonDellPro.Option{pantheonDellPro.WithBaseURL(baseURL)}
	p, err := pantheonDellPro.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildMoonshot(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["MoonshotAiApiKey"],
		cfg.MoonshotAiApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no Moonshot API key configured")
	}
	p, err := pantheonMoonshot.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildCometAPI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["CometApiLLMApiKey"],
		cfg.CometApiLLMApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no CometAPI API key configured")
	}
	p, err := pantheonCometAPI.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildFoundry(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["FoundryBasePath"],
		cfg.FoundryBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no Foundry base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	p, err := pantheonFoundry.New("", pantheonFoundry.WithBaseURL(baseURL))
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildZAI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["ZAiApiKey"],
		cfg.ZAiApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no ZAI API key configured")
	}
	p, err := pantheonZAI.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildGiteeAI(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["GiteeAIApiKey"],
		cfg.GiteeAIApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no GiteeAI API key configured")
	}
	p, err := pantheonGiteeAI.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildDockerModelRunner(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["DockerModelRunnerBasePath"],
		cfg.DockerModelRunnerBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no Docker Model Runner base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["DockerModelRunnerApiKey"],
		cfg.LLMApiKey,
	)

	opts := []pantheonDockerModelRunner.Option{pantheonDockerModelRunner.WithBaseURL(baseURL)}
	p, err := pantheonDockerModelRunner.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildPrivateMode(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["PrivateModeBasePath"],
		cfg.PrivateModeBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no PrivateMode base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["PrivateModeApiKey"],
		cfg.LLMApiKey,
	)

	opts := []pantheonPrivateMode.Option{pantheonPrivateMode.WithBaseURL(baseURL)}
	p, err := pantheonPrivateMode.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildSambaNova(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["SambaNovaLLMApiKey"],
		cfg.SambaNovaLLMApiKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("no SambaNova API key configured")
	}
	p, err := pantheonSambaNova.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildLemonade(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	baseURL := firstNonEmpty(
		settings["LemonadeLLMBasePath"],
		cfg.LemonadeLLMBasePath,
	)
	if baseURL == "" {
		return nil, fmt.Errorf("no Lemonade base path configured")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	apiKey := firstNonEmpty(
		settings["LemonadeLLMApiKey"],
		cfg.LemonadeLLMApiKey,
		cfg.LLMApiKey,
	)

	opts := []pantheonLemonade.Option{pantheonLemonade.WithBaseURL(baseURL)}
	p, err := pantheonLemonade.New(apiKey, opts...)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildMinimax(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["MinimaxApiKey"],
		cfg.MinimaxAPIKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("minimax: no API key configured")
	}
	p, err := pantheonMinimax.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildQwen(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["QwenApiKey"],
		cfg.QwenAPIKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("qwen: no API key configured")
	}
	p, err := pantheonQwen.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildWenxin(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["WenxinApiKey"],
		cfg.WenxinAPIKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("wenxin: no API key configured")
	}
	p, err := pantheonWenxin.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}

func buildZhipu(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
	apiKey := firstNonEmpty(
		settings["ZhipuApiKey"],
		cfg.ZhipuAPIKey,
		cfg.LLMApiKey,
	)
	if apiKey == "" {
		return nil, fmt.Errorf("zhipu: no API key configured")
	}
	p, err := pantheonZhipu.New(apiKey)
	if err != nil {
		return nil, err
	}
	return p.LanguageModel(ctx, modelID)
}
