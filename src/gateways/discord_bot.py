"""
Antigravity-Unleashed Discord Gateway
Enables 24/7 interaction via Discord bot mentions, DMs, or dedicated server channels.
"""

import asyncio
import logging
from typing import Optional, List

logger = logging.getLogger("antigravity_unleashed.gateway.discord")


class DiscordGateway:
    def __init__(self, engine, bot_token: str, guild_ids: Optional[List[int]] = None, allowed_channel_ids: Optional[List[int]] = None):
        self.engine = engine
        self.bot_token = bot_token
        self.guild_ids = guild_ids or []
        self.allowed_channel_ids = allowed_channel_ids or []
        self._client = None

    async def start(self):
        if not self.bot_token:
            logger.warning("[Discord Gateway] No bot token provided. Gateway disabled.")
            return

        try:
            import discord
        except ImportError:
            logger.error("[Discord Gateway] discord.py is not installed. Run: pip install discord.py")
            return

        logger.info("[Discord Gateway] Initializing Discord Bot listener...")
        intents = discord.Intents.default()
        intents.message_content = True
        self._client = discord.Client(intents=intents)

        @self._client.event
        async def on_ready():
            logger.info(f"[Discord Gateway] Logged in as {self._client.user} (ID: {self._client.user.id})")

        @self._client.event
        async def on_message(message):
            # Ignore own messages
            if message.author == self._client.user:
                return

            # Check if mentioned or in DM
            is_dm = isinstance(message.channel, discord.DMChannel)
            is_mentioned = self._client.user in message.mentions

            if not (is_dm or is_mentioned):
                return

            if self.allowed_channel_ids and message.channel.id not in self.allowed_channel_ids:
                return

            clean_prompt = message.clean_content.replace(f"@{self._client.user.name}", "").strip()
            session_id = f"discord_{message.channel.id}"

            async with message.channel.typing():
                accumulated = ""
                async for event in self.engine.chat(session_id=session_id, user_message=clean_prompt, channel_name="discord"):
                    if event["type"] == "text":
                        accumulated += event["content"]

                response_text = accumulated or "Task executed."
                if len(response_text) > 1950:
                    response_text = response_text[:1940] + "\n...(truncated)"
                
                await message.reply(response_text)

        # Run client asynchronously
        asyncio.create_task(self._client.start(self.bot_token))

    async def stop(self):
        if self._client:
            await self._client.close()
