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
	Provider         string  `yaml:"provider"`
	AgyBinaryPath    string  `yaml:"agy_binary_path"`
	Effort           string  `yaml:"effort"`
	AutoApproveTools bool    `yaml:"auto_approve_tools"`
	ModelName        string  `yaml:"model_name"`
	APIKey           string  `yaml:"api_key"`
	Temperature      float64 `yaml:"temperature"`
	MaxOutputTokens  int     `yaml:"max_output_tokens"`
}

type MemoryConfig struct {
	Enabled                 bool    `yaml:"enabled"`
	DBPath                  string  `yaml:"db_path"`
	VectorDimension         int     `yaml:"vector_dimension"`
	SimilarityThreshold     float64 `yaml:"similarity_threshold"`
	MaxContextMemories      int     `yaml:"max_context_memories"`
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

type GatewaysConfig struct {
	CLI      CLIGatewayConfig      `yaml:"cli"`
	Telegram TelegramGatewayConfig `yaml:"telegram"`
	Discord  DiscordGatewayConfig  `yaml:"discord"`
	RESTAPI  RESTAPIGatewayConfig  `yaml:"rest_api"`
}

type AppConfig struct {
	System     SystemConfig     `yaml:"system"`
	Model      ModelConfig      `yaml:"model"`
	Memory     MemoryConfig     `yaml:"memory"`
	Reflection ReflectionConfig `yaml:"reflection"`
	Gateways   GatewaysConfig   `yaml:"gateways"`
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
			AgentName:    "Antigravity-Unleashed",
			WorkspaceDir: ".",
			DataDir:      "./data",
			SkillsDir:    "./.agents/skills",
			LogLevel:     "INFO",
		},
		Model: ModelConfig{
			Provider:         "agy",
			Effort:           "high",
			AutoApproveTools: true,
			ModelName:        "auto",
		},
		Memory: MemoryConfig{
			Enabled:             true,
			DBPath:              "./data/memory.sqlite",
			VectorDimension:     384,
			SimilarityThreshold: 0.50,
			MaxContextMemories:  5,
		},
		Reflection: ReflectionConfig{
			Enabled:                   true,
			AutoGenerateSkills:        true,
			AutoExtractFacts:          true,
			MinToolStepsForReflection: 2,
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

	// Ensure absolute or clean paths
	if cfg.System.WorkspaceDir == "" {
		cfg.System.WorkspaceDir = "."
	}
	cfg.System.WorkspaceDir, _ = filepath.Abs(cfg.System.WorkspaceDir)

	return cfg, nil
}
