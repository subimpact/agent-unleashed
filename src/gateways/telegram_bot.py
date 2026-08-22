"""
Antigravity-Unleashed Telegram Gateway
Enables 24/7 interaction via Telegram DMs or group chats with real-time progress updates.
"""

import asyncio
import logging
from typing import Optional

logger = logging.getLogger("antigravity_unleashed.gateway.telegram")


class TelegramGateway:
    def __init__(self, engine, bot_token: str, allowed_chat_ids=None, admin_chat_id=None):
        self.engine = engine
        self.bot_token = bot_token
        self.allowed_chat_ids = allowed_chat_ids or []
        self.admin_chat_id = admin_chat_id
        self._app = None

    async def start(self):
        if not self.bot_token:
            logger.warning("[Telegram Gateway] No bot token provided. Gateway disabled.")
            return

        try:
            from telegram import Update
            from telegram.ext import ApplicationBuilder, CommandHandler, MessageHandler, filters, ContextTypes
        except ImportError:
            logger.error("[Telegram Gateway] python-telegram-bot is not installed. Run: pip install python-telegram-bot")
            return

        logger.info("[Telegram Gateway] Initializing Telegram Bot listener...")
        self._app = ApplicationBuilder().token(self.bot_token).build()

        async def start_cmd(update: Update, context: ContextTypes.DEFAULT_TYPE):
            chat_id = update.effective_chat.id
            if self.allowed_chat_ids and chat_id not in self.allowed_chat_ids:
                await update.message.reply_text("⛔ Unauthorized access.")
                return
            await update.message.reply_text(
                "🚀 *Antigravity-Unleashed Online!*\n\n"
                "I am your 24/7 autonomous pair programmer with persistent memory, tool execution, and self-learning capabilities.\n\n"
                "Send me any task or coding instruction.",
                parse_mode="Markdown"
            )

        async def memory_cmd(update: Update, context: ContextTypes.DEFAULT_TYPE):
            recent = self.engine.memory_store.get_recent_memories(limit=5)
            if not recent:
                await update.message.reply_text("🧠 *Memory Store*: No memories recorded yet.", parse_mode="Markdown")
                return
            text = "🧠 *Recent Persistent Memories:*\n\n"
            for m in recent:
                text += f"• `[{m['category']}]` {m['content']} _({m['source']})_\n"
            await update.message.reply_text(text, parse_mode="Markdown")

        async def handle_message(update: Update, context: ContextTypes.DEFAULT_TYPE):
            if not update.message or not update.message.text:
                return
            chat_id = update.effective_chat.id
            if self.allowed_chat_ids and chat_id not in self.allowed_chat_ids:
                return

            user_text = update.message.text
            session_id = f"telegram_{chat_id}"

            # Send typing action
            await update.message.chat.send_action("typing")

            # Stream processing
            status_msg = await update.message.reply_text("⚡ Thinking...")
            accumulated_response = ""

            async for event in self.engine.chat(session_id=session_id, user_message=user_text, channel_name="telegram"):
                if event["type"] == "text":
                    accumulated_response += event["content"]
                elif event["type"] == "tool_start":
                    await status_msg.edit_text(f"🔧 Executing `{event['tool_name']}`...")
                elif event["type"] == "memory_recall":
                    logger.info(f"[Telegram] Injected {event['count']} recalled memories.")

            final_text = accumulated_response if accumulated_response else "Task processed."
            # Split if exceeds Telegram's 4096 character limit
            if len(final_text) > 4000:
                final_text = final_text[:3990] + "\n...(truncated)"
            
            try:
                await status_msg.edit_text(final_text, parse_mode="Markdown")
            except Exception:
                try:
                    # Fallback to plain text if Markdown syntax fails
                    await status_msg.edit_text(final_text)
                except Exception:
                    await update.message.reply_text(final_text)

        self._app.add_handler(CommandHandler("start", start_cmd))
        self._app.add_handler(CommandHandler("memory", memory_cmd))
        self._app.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_message))

        await self._app.initialize()
        await self._app.start()
        await self._app.updater.start_polling()
        logger.info("[Telegram Gateway] Bot is polling and ready for messages.")

    async def stop(self):
        if self._app:
            await self._app.updater.stop()
            await self._app.stop()
            await self._app.shutdown()
