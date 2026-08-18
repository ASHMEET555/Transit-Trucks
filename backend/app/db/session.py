# backend/app/db/session.py
import os

from dotenv import load_dotenv

from app.db.postgres_repo import PostgresRepository
from app.db.sqlite_repo import SQLiteRepository

load_dotenv()


def _resolve_backend() -> str:
    explicit = os.getenv("DB_BACKEND")
    if explicit:
        return explicit.lower()
    # Auto-switch to postgres when DSN is present.
    if os.getenv("POSTGRES_DSN"):
        return "postgres"
    return "sqlite"


backend = _resolve_backend()
if backend == "postgres":
    db = PostgresRepository()
else:
    db = SQLiteRepository()