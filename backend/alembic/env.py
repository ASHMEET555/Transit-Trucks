from __future__ import annotations

import os
from logging.config import fileConfig

from alembic import context
from sqlalchemy import engine_from_config, pool

config = context.config

if config.config_file_name is not None:
    fileConfig(config.config_file_name)

target_metadata = None


def _resolved_sqlalchemy_url() -> str:
    backend = os.getenv("DB_BACKEND", "sqlite").lower()
    if backend == "postgres":
        dsn = os.getenv("POSTGRES_DSN")
        if not dsn:
            raise RuntimeError("POSTGRES_DSN must be set when DB_BACKEND=postgres")
        return dsn

    db_path = os.getenv("DB_PATH")
    if not db_path:
        return "sqlite:///database/transitops.db"
    return f"sqlite:///{db_path}"


config.set_main_option("sqlalchemy.url", _resolved_sqlalchemy_url())


def run_migrations_offline() -> None:
    url = config.get_main_option("sqlalchemy.url")
    context.configure(url=url, target_metadata=target_metadata, literal_binds=True)

    with context.begin_transaction():
        context.run_migrations()


def run_migrations_online() -> None:
    connectable = engine_from_config(
        config.get_section(config.config_ini_section, {}),
        prefix="sqlalchemy.",
        poolclass=pool.NullPool,
    )

    with connectable.connect() as connection:
        context.configure(connection=connection, target_metadata=target_metadata)

        with context.begin_transaction():
            context.run_migrations()


if context.is_offline_mode():
    run_migrations_offline()
else:
    run_migrations_online()
