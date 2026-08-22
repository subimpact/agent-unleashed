"""
Antigravity-Unleashed Master Daemon
Runs 24/7 background orchestrator managing all active communication gateways.
"""

import asyncio
import logging
import signal
import sys
from pathlib import Path

from src.config import load_config, AppConfig
from src.core.engine import UnleashedAgentEngine
from src.gateways.cli import CliGateway
from src.gateways.telegram_bot import TelegramGateway
from src.gateways.discord_bot import DiscordGateway
from src.gateways.rest_api import RestApiGateway

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s"
)
logger = logging.getLogger("antigravity_unleashed.daemon")


class UnleashedDaemon:
    def __init__(self, config_path: str = "config.yaml"):
        self.config = load_config(config_path)
        self.engine = UnleashedAgentEngine(self.config)
        self.gateways = []

    async def run(self):
        logger.info("=" * 60)
        logger.info("  ANTIGRAVITY-UNLEASHED DAEMON STARTING")
        logger.info("  Autonomous 24/7 Engine & Multi-Channel Gateway")
        logger.info("=" * 60)

        # 1. REST API Gateway
        if self.config.gateways.rest_api.enabled:
            api_gw = RestApiGateway(
                engine=self.engine,
                host=self.config.gateways.rest_api.host,
                port=self.config.gateways.rest_api.port,
                webhook_secret=self.config.gateways.rest_api.webhook_secret
            )
            await api_gw.start()
            self.gateways.append(api_gw)

        # 2. Telegram Gateway
        if self.config.gateways.telegram.enabled:
            tg_gw = TelegramGateway(
                engine=self.engine,
                bot_token=self.config.gateways.telegram.bot_token or "",
                allowed_chat_ids=self.config.gateways.telegram.allowed_chat_ids,
                admin_chat_id=self.config.gateways.telegram.admin_chat_id
            )
            await tg_gw.start()
            self.gateways.append(tg_gw)

        # 3. Discord Gateway
        if self.config.gateways.discord.enabled:
            dc_gw = DiscordGateway(
                engine=self.engine,
                bot_token=self.config.gateways.discord.bot_token or "",
                guild_ids=self.config.gateways.discord.guild_ids,
                allowed_channel_ids=self.config.gateways.discord.allowed_channel_ids
            )
            await dc_gw.start()
            self.gateways.append(dc_gw)

        # 4. CLI Gateway (Foreground interactive if enabled)
        if self.config.gateways.cli.enabled:
            cli_gw = CliGateway(engine=self.engine)
            await cli_gw.start()
        else:
            # Keep daemon alive in headless mode
            logger.info("[Daemon] Running in headless 24/7 mode. Press Ctrl+C to stop.")
            while True:
                await asyncio.sleep(3600)


def main():
    daemon = UnleashedDaemon()
    try:
        asyncio.run(daemon.run())
    except KeyboardInterrupt:
        logger.info("[Daemon] Received shutdown signal. Exiting cleanly.")


if __name__ == "__main__":
    main()
