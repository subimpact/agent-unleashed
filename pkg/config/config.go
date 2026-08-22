package config

import (
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

type SystemConfig struct {
	AgentName    string `yaml:"agent_name"`
	WorkspaceDir string `yaml:"workspace_dir"`
	DataDir      string `yaml:"data_dir"`
	SkillsDir    string `yaml:"skills_dir"`
	LogLevel     string `yaml:"log_level"`
}

type ModelConfig struct {
	Driver           string  `yaml:"driver"`             // 'auto', 'agy', 'claude', 'aider', 'hermes', 'ollama', 'api'
	AgyBinaryPath    string  `yaml:"agy_binary_path"`    // Optional explicit path to agy
	ClaudeBinaryPath string  `yaml:"claude_binary_path"` // Optional explicit path to claude
	CodexBinaryPath  string  `yaml:"codex_binary_path"`  // Optional explicit path to codex
	AiderBinaryPath  string  `yaml:"aider_binary_path"`  // Optional explicit path to aider
	HermesBinaryPath string  `yaml:"hermes_binary_path"` // Optional explicit path to hermes
	Effort           string  `yaml:"effort"`             // 'low', 'medium', 'high'
	AutoApproveTools bool    `yaml:"auto_approve_tools"` // Auto-approve permissions where supported
	ModelName        string  `yaml:"model_name"`         // Model override (e.g., 'gemini-2.5-pro', 'claude-3-7-sonnet', 'hermes3')
	OllamaEndpoint   string  `yaml:"ollama_endpoint"`    // Default: 'http://localhost:11434'
	GeminiAPIKey     string  `yaml:"gemini_api_key"`
	OpenRouterAPIKey string  `yaml:"openrouter_api_key"`
	AnthropicAPIKey  string  `yaml:"anthropic_api_key"`
	OpenAIAPIKey     string  `yaml:"openai_api_key"`
	Temperature      float64 `yaml:"temperature"`
	MaxOutputTokens  int     `yaml:"max_output_tokens"`
}

type PalaceMemoryConfig struct {
	Enabled                 bool    `yaml:"enabled"`
	DBPath                  string  `yaml:"db_path"`
	VectorDimension         int     `yaml:"vector_dimension"`
	SimilarityThreshold     float64 `yaml:"similarity_threshold"`
	MaxContextMemories      int     `yaml:"max_context_memories"`
	DecayHalfLifeDays       float64 `yaml:"decay_half_life_days"` // Half-life for temporal decay (default: 30 days)
	AutoIngestConversations bool    `yaml:"auto_ingest_conversations"`
}

type ReflectionConfig struct {
	Enabled                   bool `yaml:"enabled"`
	AutoGenerateSkills        bool `yaml:"auto_generate_skills"`
	AutoExtractFacts          bool `yaml:"auto_extract_facts"`
	MinToolStepsForReflection int  `yaml:"min_tool_steps_for_reflection"`
}

type CLIGatewayConfig struct {
	Enabled bool `yaml:"enabled"`
}

type TelegramGatewayConfig struct {
	Enabled        bool    `yaml:"enabled"`
	BotToken       string  `yaml:"bot_token"`
	AllowedChatIDs []int64 `yaml:"allowed_chat_ids"`
	AdminChatID    int64   `yaml:"admin_chat_id"`
}

type DiscordGatewayConfig struct {
	Enabled           bool    `yaml:"enabled"`
	BotToken          string  `yaml:"bot_token"`
	GuildIDs          []int64 `yaml:"guild_ids"`
	AllowedChannelIDs []int64 `yaml:"allowed_channel_ids"`
}

type RESTAPIGatewayConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	WebhookSecret string `yaml:"webhook_secret"`
}

type WikiConfig struct {
	Enabled   bool   `yaml:"enabled"`
	WikiDir   string `yaml:"wiki_dir"`
	AutoBuild bool   `yaml:"auto_build"`
}

type GatewaysConfig struct {
	CLI      CLIGatewayConfig      `yaml:"cli"`
	Telegram TelegramGatewayConfig `yaml:"telegram"`
	Discord  DiscordGatewayConfig  `yaml:"discord"`
	RESTAPI  RESTAPIGatewayConfig  `yaml:"rest_api"`
}

type AppConfig struct {
	System     SystemConfig       `yaml:"system"`
	Model      ModelConfig        `yaml:"model"`
	Memory     PalaceMemoryConfig `yaml:"memory"`
	Reflection ReflectionConfig   `yaml:"reflection"`
	Wiki       WikiConfig         `yaml:"wiki"`
	Gateways   GatewaysConfig     `yaml:"gateways"`
}

func expandEnv(content string) string {
	re := regexp.MustCompile(`\$\{([A-Za-z0-9_]+)\}`)
	return re.ReplaceAllStringFunc(content, func(m string) string {
		varName := m[2 : len(m)-1]
		return os.Getenv(varName)
	})
}

func LoadConfig(configPath string) (*AppConfig, error) {
	cfg := &AppConfig{
		System: SystemConfig{
			AgentName:    "Agent-Unleashed",
			WorkspaceDir: ".",
			DataDir:      "./data",
			SkillsDir:    "./.agents/skills",
			LogLevel:     "INFO",
		},
		Model: ModelConfig{
			Driver:           "auto",
			Effort:           "high",
			AutoApproveTools: true,
			ModelName:        "auto",
			OllamaEndpoint:   "http://localhost:11434",
		},
		Memory: PalaceMemoryConfig{
			Enabled:             true,
			DBPath:              "./data/memory.sqlite",
			VectorDimension:     384,
			SimilarityThreshold: 0.40,
			MaxContextMemories:  5,
			DecayHalfLifeDays:   30.0,
		},
		Reflection: ReflectionConfig{
			Enabled:                   true,
			AutoGenerateSkills:        true,
			AutoExtractFacts:          true,
			MinToolStepsForReflection: 2,
		},
		Wiki: WikiConfig{
			Enabled:   true,
			WikiDir:   "./.agents/wiki",
			AutoBuild: true,
		},
		Gateways: GatewaysConfig{
			CLI:     CLIGatewayConfig{Enabled: true},
			RESTAPI: RESTAPIGatewayConfig{Enabled: true, Host: "127.0.0.1", Port: 8080},
		},
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		examplePath := "config.yaml.example"
		if _, err := os.Stat(examplePath); err == nil {
			configPath = examplePath
		} else {
			return cfg, nil
		}
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	expanded := expandEnv(string(raw))
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, err
	}

	if cfg.System.WorkspaceDir == "" {
		cfg.System.WorkspaceDir = "."
	}
	cfg.System.WorkspaceDir, _ = filepath.Abs(cfg.System.WorkspaceDir)

	return cfg, nil
}

func SaveConfig(configPath string, cfg *AppConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}
