"""
Antigravity-Unleashed REST & Webhook Gateway
FastAPI server enabling remote webhook triggers and programmatic agent execution.
"""

import asyncio
import logging
from typing import Optional, Dict, Any
from pydantic import BaseModel

logger = logging.getLogger("antigravity_unleashed.gateway.rest_api")


class WebhookPayload(BaseModel):
    prompt: str
    session_id: Optional[str] = "webhook_default"
    webhook_secret: Optional[str] = None


class RestApiGateway:
    def __init__(self, engine, host: str = "127.0.0.1", port: int = 8080, webhook_secret: Optional[str] = None):
        self.engine = engine
        self.host = host
        self.port = port
        self.webhook_secret = webhook_secret
        self._server = None

    async def start(self):
        try:
            from fastapi import FastAPI, HTTPException, Header
            import uvicorn
        except ImportError:
            logger.error("[REST Gateway] fastapi/uvicorn not installed. Run: pip install fastapi uvicorn")
            return

        app = FastAPI(title="Antigravity-Unleashed API", version="1.0.0")

        @app.get("/health")
        async def health_check():
            return {"status": "online", "agent": "Antigravity-Unleashed"}

        @app.post("/api/v1/trigger")
        async def trigger_task(payload: WebhookPayload):
            if self.webhook_secret and payload.webhook_secret != self.webhook_secret:
                raise HTTPException(status_code=401, detail="Invalid webhook secret.")

            response_chunks = []
            async for event in self.engine.chat(
                session_id=payload.session_id,
                user_message=payload.prompt,
                channel_name="webhook"
            ):
                if event["type"] == "text":
                    response_chunks.append(event["content"])

            return {
                "session_id": payload.session_id,
                "response": "".join(response_chunks)
            }

        config = uvicorn.Config(app=app, host=self.host, port=self.port, log_level="warning")
        server = uvicorn.Server(config)
        self._server = server
        logger.info(f"[REST Gateway] API server running on http://{self.host}:{self.port}")
        asyncio.create_task(server.serve())

    async def stop(self):
        if self._server:
            self._server.should_exit = True
