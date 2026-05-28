package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ServerPort      string  `env:"SERVER_PORT" envDefault:"3001"`
	StorageDir      string  `env:"STORAGE_DIR" envDefault:"./storage"`
	JWTSecret       string  `env:"JWT_SECRET" envDefault:"dev-secret-change-me"`
	SigKey          string  `env:"SIG_KEY" envDefault:"dev-sig-key"`
	SigSalt         string  `env:"SIG_SALT" envDefault:"dev-sig-salt"`
	AuthToken       string  `env:"AUTH_TOKEN"`
	VectorDB        string  `env:"VECTOR_DB" envDefault:"lancedb"`
	LLMProvider     string  `env:"LLM_PROVIDER" envDefault:"openai"`
	LLMModel        string  `env:"LLM_MODEL" envDefault:"gpt-4o-mini"`
	EmbeddingEngine string  `env:"EMBEDDING_ENGINE" envDefault:"openai"`
	EmbeddingModel  string  `env:"EMBEDDING_MODEL" envDefault:"text-embedding-3-small"`
	TTSProvider     string  `env:"TTS_PROVIDER" envDefault:"native"`
	EnableHTTPS     bool    `env:"ENABLE_HTTPS" envDefault:"false"`
	MultiUserMode   bool    `env:"MULTI_USER_MODE" envDefault:"false"`
	DebugMode       bool    `env:"DEBUG_MODE" envDefault:"false"`
	OpenAiKey       string  `env:"OPEN_AI_KEY"`
	LLMApiKey       string  `env:"LLM_API_KEY"`
	LLMTemperature  float64 `env:"LLM_TEMPERATURE" envDefault:"0.7"`
	LLMMaxTokens    int     `env:"LLM_MAX_TOKENS" envDefault:"4096"`

	// === OpenAI ===
	OpenAiModelPref string `env:"OPEN_MODEL_PREF"`

	// === Azure ===
	AzureOpenAiEndpoint     string `env:"AZURE_OPENAI_ENDPOINT"`
	AzureOpenAiKey          string `env:"AZURE_OPENAI_KEY"`
	AzureOpenAiModelPref    string `env:"AZURE_OPENAI_MODEL_PREF"`
	AzureOpenAiModelType    string `env:"AZURE_OPENAI_MODEL_TYPE" envDefault:"default"`
	AzureOpenAiTokenLimit   int    `env:"AZURE_OPENAI_TOKEN_LIMIT"`
	AzureOpenAiResourceName string `env:"AZURE_OPENAI_RESOURCE_NAME"`
	AzureOpenAiDeployment   string `env:"AZURE_OPENAI_DEPLOYMENT"`

	// === Anthropic ===
	AnthropicApiKey       string `env:"ANTHROPIC_API_KEY"`
	AnthropicModelPref    string `env:"ANTHROPIC_MODEL_PREF"`
	AnthropicCacheControl string `env:"ANTHROPIC_CACHE_CONTROL" envDefault:"none"`

	// === Gemini (Google) ===
	GeminiApiKey        string `env:"GEMINI_API_KEY"`
	GeminiModelPref     string `env:"GEMINI_LLM_MODEL_PREF"`
	GeminiSafetySetting string `env:"GEMINI_SAFETY_SETTING"`

	// === LMStudio ===
	LMStudioBasePath   string `env:"LMSTUDIO_BASE_PATH"`
	LMStudioModelPref  string `env:"LMSTUDIO_MODEL_PREF"`
	LMStudioTokenLimit int    `env:"LMSTUDIO_MODEL_TOKEN_LIMIT"`
	LMStudioAuthToken  string `env:"LMSTUDIO_AUTH_TOKEN"`

	// === LocalAI ===
	LocalAiBasePath   string `env:"LOCAL_AI_BASE_PATH"`
	LocalAiModelPref  string `env:"LOCAL_AI_MODEL_PREF"`
	LocalAiTokenLimit int    `env:"LOCAL_AI_MODEL_TOKEN_LIMIT"`
	LocalAiApiKey     string `env:"LOCAL_AI_API_KEY"`

	// === Ollama ===
	OllamaBasePath     string `env:"OLLAMA_BASE_PATH"`
	OllamaModelPref    string `env:"OLLAMA_MODEL_PREF"`
	OllamaTokenLimit   int    `env:"OLLAMA_MODEL_TOKEN_LIMIT"`
	OllamaKeepAliveSec int    `env:"OLLAMA_KEEP_ALIVE_TIMEOUT"`
	OllamaAuthToken    string `env:"OLLAMA_AUTH_TOKEN"`

	// === TogetherAI ===
	TogetherAiApiKey    string `env:"TOGETHER_AI_API_KEY"`
	TogetherAiModelPref string `env:"TOGETHER_AI_MODEL_PREF"`

	// === FireworksAI ===
	FireworksApiKey    string `env:"FIREWORKS_API_KEY"`
	FireworksModelPref string `env:"FIREWORKS_MODEL_PREF"`

	// === Mistral ===
	MistralApiKey    string `env:"MISTRAL_API_KEY"`
	MistralModelPref string `env:"MISTRAL_MODEL_PREF"`

	// === HuggingFace ===
	HuggingFaceEndpoint   string `env:"HUGGING_FACE_LLM_ENDPOINT"`
	HuggingFaceApiKey     string `env:"HUGGING_FACE_LLM_API_KEY"`
	HuggingFaceTokenLimit int    `env:"HUGGING_FACE_LLM_TOKEN_LIMIT"`

	// === Perplexity ===
	PerplexityApiKey    string `env:"PERPLEXITY_API_KEY"`
	PerplexityModelPref string `env:"PERPLEXITY_MODEL_PREF"`

	// === OpenRouter ===
	OpenRouterApiKey    string `env:"OPENROUTER_API_KEY"`
	OpenRouterModelPref string `env:"OPENROUTER_MODEL_PREF"`

	// === Novita ===
	NovitaLLMApiKey    string `env:"NOVITA_LLM_API_KEY"`
	NovitaLLMModelPref string `env:"NOVITA_LLM_MODEL_PREF"`

	// === Groq ===
	GroqApiKey    string `env:"GROQ_API_KEY"`
	GroqModelPref string `env:"GROQ_MODEL_PREF"`

	// === KoboldCPP ===
	KoboldBasePath   string `env:"KOBOLD_CPP_BASE_PATH"`
	KoboldModelPref  string `env:"KOBOLD_CPP_MODEL_PREF"`
	KoboldTokenLimit int    `env:"KOBOLD_CPP_MODEL_TOKEN_LIMIT"`
	KoboldMaxTokens  int    `env:"KOBOLD_CPP_MAX_TOKENS"`

	// === TextGenWebUI ===
	TextGenBasePath   string `env:"TEXT_GEN_WEB_UI_BASE_PATH"`
	TextGenTokenLimit int    `env:"TEXT_GEN_WEB_UI_MODEL_TOKEN_LIMIT"`
	TextGenApiKey     string `env:"TEXT_GEN_WEB_UI_API_KEY"`

	// === Cohere ===
	CohereApiKey    string `env:"COHERE_API_KEY"`
	CohereModelPref string `env:"COHERE_MODEL_PREF"`

	// === LiteLLM ===
	LiteLLMModelPref string `env:"LITE_LLM_MODEL_PREF"`
	LiteLLMBasePath  string `env:"LITE_LLM_BASE_PATH"`
	LiteLLMApiKey    string `env:"LITE_LLM_API_KEY"`

	// === GenericOpenAI ===
	GenericOpenAiBasePath  string `env:"GENERIC_OPEN_AI_BASE_PATH"`
	GenericOpenAiModelPref string `env:"GENERIC_OPEN_AI_MODEL_PREF"`
	GenericOpenAiKey       string `env:"GENERIC_OPEN_AI_API_KEY"`
	GenericOpenAiMaxTokens int    `env:"GENERIC_OPEN_AI_MAX_TOKENS"`

	// === Bedrock ===
	AwsBedrockLLMAccessKeyId  string `env:"AWS_BEDROCK_LLM_ACCESS_KEY_ID"`
	AwsBedrockLLMAccessKey    string `env:"AWS_BEDROCK_LLM_ACCESS_KEY"`
	AwsBedrockLLMRegion       string `env:"AWS_BEDROCK_LLM_REGION"`
	AwsBedrockLLMSessionToken string `env:"AWS_BEDROCK_LLM_SESSION_TOKEN"`
	AwsBedrockLLMModel        string `env:"AWS_BEDROCK_LLM_MODEL_PREFERENCE"`

	// === DeepSeek ===
	DeepSeekApiKey    string `env:"DEEPSEEK_API_KEY"`
	DeepSeekModelPref string `env:"DEEPSEEK_MODEL_PREF"`

	// === ApiPie ===
	ApiPieApiKey    string `env:"APIPIE_API_KEY"`
	ApiPieModelPref string `env:"APIPIE_MODEL_PREF"`

	// === XAI ===
	XAIApiKey    string `env:"XAI_LLM_API_KEY"`
	XAIModelPref string `env:"XAI_LLM_MODEL_PREF"`

	// === NVIDIA NIM ===
	NvidiaNimLLMBasePath  string `env:"NVIDIA_NIM_LLM_BASE_PATH"`
	NvidiaNimLLMModelPref string `env:"NVIDIA_NIM_LLM_MODEL_PREF"`

	// === PPIO ===
	PpioApiKey    string `env:"PPIO_API_KEY"`
	PpioModelPref string `env:"PPIO_MODEL_PREF"`

	// === Dell Pro AI Studio ===
	DellProAiStudioBasePath   string `env:"DPAIS_LLM_BASE_PATH"`
	DellProAiStudioModelPref  string `env:"DPAIS_LLM_MODEL_PREF"`
	DellProAiStudioTokenLimit int    `env:"DELL_PRO_AI_STUDIO_MODEL_TOKEN_LIMIT"`

	// === MoonshotAI (Kimi) ===
	MoonshotAiApiKey    string `env:"MOONSHOT_AI_API_KEY"`
	MoonshotAiModelPref string `env:"MOONSHOT_AI_MODEL_PREF"`

	// === CometAPI ===
	CometApiLLMApiKey    string `env:"COMETAPI_LLM_API_KEY"`
	CometApiLLMModelPref string `env:"COMETAPI_LLM_MODEL_PREF"`

	// === Foundry ===
	FoundryBasePath  string `env:"FOUNDRY_BASE_PATH"`
	FoundryModelPref string `env:"FOUNDRY_MODEL_PREF"`

	// === ZAI ===
	ZAiApiKey    string `env:"ZAI_API_KEY"`
	ZAiModelPref string `env:"ZAI_MODEL_PREF"`

	// === GiteeAI ===
	GiteeAIApiKey    string `env:"GITEE_AI_API_KEY"`
	GiteeAIModelPref string `env:"GITEE_AI_MODEL_PREF"`

	// === Docker Model Runner ===
	DockerModelRunnerBasePath  string `env:"DOCKER_MODEL_RUNNER_BASE_PATH"`
	DockerModelRunnerModelPref string `env:"DOCKER_MODEL_RUNNER_LLM_MODEL_PREF"`

	// === PrivateMode ===
	PrivateModeBasePath  string `env:"PRIVATEMODE_LLM_BASE_PATH"`
	PrivateModeModelPref string `env:"PRIVATEMODE_LLM_MODEL_PREF"`

	// === SambaNova ===
	SambaNovaLLMApiKey    string `env:"SAMBANOVA_LLM_API_KEY"`
	SambaNovaLLMModelPref string `env:"SAMBANOVA_LLM_MODEL_PREF"`

	// === Lemonade ===
	LemonadeLLMBasePath  string `env:"LEMONADE_LLM_BASE_PATH"`
	LemonadeLLMApiKey    string `env:"LEMONADE_LLM_API_KEY"`
	LemonadeLLMModelPref string `env:"LEMONADE_LLM_MODEL_PREF"`

	EmbeddingApiKey        string `env:"EMBEDDING_API_KEY"`
	DatabaseURL            string `env:"DATABASE_URL"`
	CollectorURL           string `env:"COLLECTOR_URL" envDefault:"http://localhost:8888"`
	CommunicationKey       string `env:"COMMUNICATION_KEY" envDefault:"hermind"`
	DisableViewChatHistory bool   `env:"DISABLE_VIEW_CHAT_HISTORY" envDefault:"false"`
	SimpleSSOEnabled       bool   `env:"SIMPLE_SSO_ENABLED" envDefault:"false"`
	SimpleSSONoLogin       bool   `env:"SIMPLE_SSO_NO_LOGIN" envDefault:"false"`

	// === Pinecone ===
	PineconeAPIKey string `env:"PINECONE_API_KEY"`
	PineconeIndex  string `env:"PINECONE_INDEX"`

	// === Qdrant ===
	QdrantEndpoint string `env:"QDRANT_ENDPOINT"`
	QdrantAPIKey   string `env:"QDRANT_API_KEY"`

	// === Chroma / ChromaCloud ===
	ChromaEndpoint  string `env:"CHROMA_ENDPOINT"`
	ChromaAPIHeader string `env:"CHROMA_API_HEADER"`
	ChromaAPIKey    string `env:"CHROMA_API_KEY"`

	// === Weaviate ===
	WeaviateEndpoint string `env:"WEAVIATE_ENDPOINT"`
	WeaviateAPIKey   string `env:"WEAVIATE_API_KEY"`

	// === Milvus ===
	MilvusAddress  string `env:"MILVUS_ADDRESS"`
	MilvusUsername string `env:"MILVUS_USERNAME"`
	MilvusPassword string `env:"MILVUS_PASSWORD"`

	// === Zilliz ===
	ZillizEndpoint string `env:"ZILLIZ_ENDPOINT"`
	ZillizAPIToken string `env:"ZILLIZ_API_TOKEN"`

	// === Astra DB ===
	AstraDBApplicationToken string `env:"ASTRA_DB_APPLICATION_TOKEN"`
	AstraDBEndpoint         string `env:"ASTRA_DB_ENDPOINT"`

	// === MCP ===
	MCPCallTimeoutDefault       time.Duration `env:"MCP_CALL_TIMEOUT_DEFAULT" envDefault:"30s"`
	MCPCallConcurrencyPerServer int           `env:"MCP_CALL_CONCURRENCY_PER_SERVER" envDefault:"4"`

	// === Agent Runtime ===
	AgentSessionMaxDuration  time.Duration `env:"AGENT_SESSION_MAX_DURATION" envDefault:"30m"`
	AgentAllowedOrigins      string        `env:"AGENT_ALLOWED_ORIGINS" envDefault:""` // CSV; "" = same-host; "*" = any
	AgentToolApprovalTimeout time.Duration `env:"AGENT_TOOL_APPROVAL_TIMEOUT" envDefault:"2m"`

	// === Agent Flow ===
	AgentFlowAllowPrivateIPs bool `env:"AGENT_FLOW_ALLOW_PRIVATE_IPS" envDefault:"false"`

	// === MiniMax (pantheon-only) ===
	MinimaxAPIKey    string `env:"MINIMAX_API_KEY"`
	MinimaxModelPref string `env:"MINIMAX_MODEL_PREF"`

	// === Qwen (pantheon-only) ===
	QwenAPIKey    string `env:"QWEN_API_KEY"`
	QwenModelPref string `env:"QWEN_MODEL_PREF"`

	// === Wenxin (pantheon-only) ===
	WenxinAPIKey    string `env:"WENXIN_API_KEY"`
	WenxinModelPref string `env:"WENXIN_MODEL_PREF"`

	// === Zhipu (pantheon-only) ===
	ZhipuAPIKey    string `env:"ZHIPU_API_KEY"`
	ZhipuModelPref string `env:"ZHIPU_MODEL_PREF"`

	// === Embedding ===
	EmbeddingBasePath string `env:"EMBEDDING_BASE_PATH"`

	// === TTS ===
	ElevenLabsAPIKey        string `env:"ELEVENLABS_API_KEY"`
	ElevenLabsVoiceID       string `env:"ELEVENLABS_VOICE_ID" envDefault:"21m00Tcm4TlvDq8ikWAM"`
	ElevenLabsModel         string `env:"ELEVENLABS_MODEL" envDefault:"eleven_monolingual_v1"`
	OpenAITTSModel          string `env:"OPEN_AI_TTS_MODEL" envDefault:"tts-1"`
	OpenAITTSVoice          string `env:"OPEN_AI_TTS_VOICE" envDefault:"alloy"`
	TTSOpenAICompatKey      string `env:"TTS_OPEN_AI_COMPATIBLE_KEY"`
	TTSOpenAICompatEndpoint string `env:"TTS_OPEN_AI_COMPATIBLE_ENDPOINT"`
	TTSOpenAICompatModel    string `env:"TTS_OPEN_AI_COMPATIBLE_MODEL" envDefault:"tts-1"`
	TTSOpenAICompatVoice    string `env:"TTS_OPEN_AI_COMPATIBLE_VOICE" envDefault:"alloy"`

	// === Reranker ===
	RerankProvider  string `env:"RERANK_PROVIDER" envDefault:""` // "" = noop
	RerankAPIKey    string `env:"RERANK_API_KEY"`
	RerankModelPref string `env:"RERANK_MODEL" envDefault:"rerank-english-v3.0"`

	// === Agent Skills ===
	AgentFilesystemEnabled  bool   `env:"AGENT_FILESYSTEM_ENABLED" envDefault:"true"`
	AgentFilesystemRoot     string `env:"AGENT_FILESYSTEM_ROOT"` // empty → <StorageDir>/hermind-fs
	AgentCreateFilesEnabled bool   `env:"AGENT_CREATE_FILES_ENABLED" envDefault:"true"`
	AgentCreateFilesDir     string `env:"AGENT_CREATE_FILES_DIR"` // empty → <StorageDir>/generated-files

	// === unioffice license (pptx generation) ===
	UnidocMeteredKey string `env:"UNIDOC_METERED_KEY"`

	// === OAuth / Public Base URL ===
	PublicBaseURL    string `env:"PUBLIC_BASE_URL"`                       // e.g. "https://anything.example.com"
	OutlookAuthority string `env:"OUTLOOK_AUTHORITY" envDefault:"common"` // common | consumers | <tenant-id>

	// === Background Workers ===
	WorkerCleanupOrphanInterval string `env:"WORKER_CLEANUP_ORPHAN_INTERVAL"    envDefault:"0 */12 * * *"`
	WorkerCleanupOrphanTimeout  string `env:"WORKER_CLEANUP_ORPHAN_TIMEOUT"     envDefault:"5m"`
	WorkerCleanupOrphanEnabled  bool   `env:"WORKER_CLEANUP_ORPHAN_ENABLED"     envDefault:"true"`

	WorkerCleanupGeneratedInterval string `env:"WORKER_CLEANUP_GENERATED_INTERVAL" envDefault:"0 */8 * * *"`
	WorkerCleanupGeneratedTimeout  string `env:"WORKER_CLEANUP_GENERATED_TIMEOUT"  envDefault:"5m"`
	WorkerCleanupGeneratedEnabled  bool   `env:"WORKER_CLEANUP_GENERATED_ENABLED"  envDefault:"true"`

	WorkerSyncWatchedInterval string `env:"WORKER_SYNC_WATCHED_INTERVAL"      envDefault:"0 * * * *"`
	WorkerSyncWatchedTimeout  string `env:"WORKER_SYNC_WATCHED_TIMEOUT"       envDefault:"10m"`
	WorkerSyncWatchedEnabled  bool   `env:"WORKER_SYNC_WATCHED_ENABLED"       envDefault:"true"`
}

func Load() (*Config, error) {
	var cfg Config

	// Load YAML config file if present (env vars take precedence).
	configFile := os.Getenv("CONFIG_FILE")
	if configFile != "" {
		_ = loadYAMLConfig(configFile)
	} else {
		_ = loadYAMLConfig("config.yaml")
		_ = loadYAMLConfig("../config.yaml")
	}

	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.StorageDir == "" {
		cfg.StorageDir = "./storage"
	}
	if err := os.MkdirAll(cfg.StorageDir, 0755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	if cfg.AgentFilesystemRoot == "" {
		cfg.AgentFilesystemRoot = filepath.Join(cfg.StorageDir, "hermind-fs")
	}
	if cfg.AgentCreateFilesDir == "" {
		cfg.AgentCreateFilesDir = filepath.Join(cfg.StorageDir, "generated-files")
	}
	if cfg.PublicBaseURL == "" {
		cfg.PublicBaseURL = "http://localhost:" + cfg.ServerPort
	}
	cfg.PublicBaseURL = strings.TrimRight(cfg.PublicBaseURL, "/")
	_ = os.MkdirAll(cfg.AgentFilesystemRoot, 0755)
	_ = os.MkdirAll(cfg.AgentCreateFilesDir, 0755)
	return &cfg, nil
}

// loadYAMLConfig reads a flat YAML file and injects its keys into the
// process environment.  Existing environment variables are never overwritten,
// so env vars always take precedence over the config file.
func loadYAMLConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return err
	}

	for key, value := range values {
		if os.Getenv(key) != "" {
			continue
		}
		switch v := value.(type) {
		case string:
			os.Setenv(key, v)
		case bool:
			os.Setenv(key, fmt.Sprintf("%t", v))
		case int:
			os.Setenv(key, fmt.Sprintf("%d", v))
		case int64:
			os.Setenv(key, fmt.Sprintf("%d", v))
		case float64:
			os.Setenv(key, fmt.Sprintf("%v", v))
		default:
			os.Setenv(key, fmt.Sprintf("%v", v))
		}
	}
	return nil
}
