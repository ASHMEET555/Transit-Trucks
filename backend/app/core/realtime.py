import json
from typing import Any

from fastapi import WebSocket


class FleetEventBroker:
    def __init__(self) -> None:
        self._clients: set[WebSocket] = set()

    async def connect(self, websocket: WebSocket) -> None:
        await websocket.accept()
        self._clients.add(websocket)

    def disconnect(self, websocket: WebSocket) -> None:
        self._clients.discard(websocket)

    async def broadcast(self, event: str, payload: dict[str, Any]) -> None:
        if not self._clients:
            return
        message = json.dumps({"event": event, "payload": payload})
        dead: list[WebSocket] = []
        for client in self._clients:
            try:
                await client.send_text(message)
            except Exception:
                dead.append(client)
        for client in dead:
            self.disconnect(client)


fleet_events = FleetEventBroker()
