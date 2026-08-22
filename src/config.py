"""
Antigravity-Unleashed Configuration Loader
Supports environment variable expansion and default fallbacks.
"""

import os
import re
import yaml
from pathlib import Path
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field


class SystemConfig(BaseModel):
    agent_name: str = "Antigravity-Unleashed"
    workspace_dir: str = "."
    data_dir: str = "./data"
    skills_dir: str = "./.agents/skills"
    log_level: str = "INFO"


class ModelConfig(BaseModel):
    provider: str = "gemini"
    model_name: str = "gemini-2.5-pro"
    api_key: Optional[str] = None
    temperature: float = 0.2
    max_output_tokens: int = 8192


class MemoryConfig(BaseModel):
    enabled: bool = True
    db_path: str = "./data/memory.sqlite"
    vector_dimension: int = 384
    similarity_threshold: float = 0.65
    max_context_memories: int = 5
    auto_ingest_conversations: bool = True


class ReflectionConfig(BaseModel):
    enabled: bool = True
    auto_generate_skills: bool = True
    auto_extract_facts: bool = True
    min_tool_steps_for_reflection: int = 2


class CliGatewayConfig(BaseModel):
    enabled: bool = True


class TelegramGatewayConfig(BaseModel):
    enabled: bool = False
    bot_token: Optional[str] = None
    allowed_chat_ids: List[int] = Field(default_factory=list)
    admin_chat_id: Optional[int] = None


class DiscordGatewayConfig(BaseModel):
    enabled: bool = False
    bot_token: Optional[str] = None
    guild_ids: List[int] = Field(default_factory=list)
    allowed_channel_ids: List[int] = Field(default_factory=list)


class RestApiGatewayConfig(BaseModel):
    enabled: bool = True
    host: str = "127.0.0.1"
    port: int = 8080
    webhook_secret: Optional[str] = None


class GatewaysConfig(BaseModel):
    cli: CliGatewayConfig = Field(default_factory=CliGatewayConfig)
    telegram: TelegramGatewayConfig = Field(default_factory=TelegramGatewayConfig)
    discord: DiscordGatewayConfig = Field(default_factory=DiscordGatewayConfig)
    rest_api: RestApiGatewayConfig = Field(default_factory=RestApiGatewayConfig)


class AppConfig(BaseModel):
    system: SystemConfig = Field(default_factory=SystemConfig)
    model: ModelConfig = Field(default_factory=ModelConfig)
    memory: MemoryConfig = Field(default_factory=MemoryConfig)
    reflection: ReflectionConfig = Field(default_factory=ReflectionConfig)
    gateways: GatewaysConfig = Field(default_factory=GatewaysConfig)


def _expand_env_vars(data: Any) -> Any:
    """Recursively expand ${VAR} or $VAR in strings."""
    if isinstance(data, dict):
        return {k: _expand_env_vars(v) for k, v in data.items()}
    elif isinstance(data, list):
        return [_expand_env_vars(item) for item in data]
    elif isinstance(data, str):
        pattern = re.compile(r"\$\{([A-Za-z0-9_]+)\}")
        matches = pattern.findall(data)
        for var in matches:
            val = os.getenv(var, "")
            data = data.replace(f"${{{var}}}", val)
        return data
    return data


def load_config(config_path: str = "config.yaml") -> AppConfig:
    path = Path(config_path)
    if not path.exists():
        example_path = Path("config.yaml.example")
        if example_path.exists():
            path = example_path
        else:
            return AppConfig()

    with open(path, "r", encoding="utf-8") as f:
        raw_data = yaml.safe_load(f) or {}

    expanded_data = _expand_env_vars(raw_data)
    return AppConfig(**expanded_data)
