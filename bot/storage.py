"""Durable storage for bot subscribers with Redis production path and SQLite fallback."""

from __future__ import annotations

import json
import logging
import sqlite3
from pathlib import Path
from time import time

from config import REDIS_URL, SUBSCRIBERS_DB_PATH

logger = logging.getLogger(__name__)

_BOT_DIR = Path(__file__).resolve().parent
_LEGACY_SUBSCRIBERS_FILE = _BOT_DIR / "data" / "subscribers.json"
_SUBSCRIBERS_DB_FILE = Path(SUBSCRIBERS_DB_PATH)
_SUBSCRIBERS_SET_KEY = "durak:bot:subscribers"
_SUBSCRIBER_HASH_KEY_PREFIX = "durak:bot:subscriber:"

_sqlite_conn: sqlite3.Connection | None = None
_redis_client = None
_migration_checked = False


def storage_backend_name() -> str:
    return "redis" if REDIS_URL else "sqlite"


async def init_storage() -> None:
    if REDIS_URL:
        client = await _get_redis_client()
        await client.ping()
        logger.info("Subscriber storage backend: redis")
    else:
        _ensure_sqlite()
        logger.warning(
            "REDIS_URL not set; using SQLite for subscribers and leaving FSM in memory. Multi-instance recovery remains limited."
        )

    await _migrate_legacy_json_if_needed()


async def close_storage() -> None:
    global _redis_client, _sqlite_conn

    if _redis_client is not None:
        await _redis_client.aclose()
        _redis_client = None

    if _sqlite_conn is not None:
        _sqlite_conn.close()
        _sqlite_conn = None


async def add_subscriber(chat_id: int, user_id: int, username: str | None) -> None:
    try:
        await _migrate_legacy_json_if_needed()
        timestamp = int(time())

        if REDIS_URL:
            client = await _get_redis_client()
            key = f"{_SUBSCRIBER_HASH_KEY_PREFIX}{chat_id}"
            async with client.pipeline(transaction=True) as pipe:
                pipe.sadd(_SUBSCRIBERS_SET_KEY, str(chat_id))
                pipe.hset(
                    key,
                    mapping={
                        "user_id": user_id,
                        "username": username or "",
                        "ts": timestamp,
                    },
                )
                await pipe.execute()
        else:
            conn = _ensure_sqlite()
            conn.execute(
                """
                INSERT INTO subscribers (chat_id, user_id, username, ts)
                VALUES (?, ?, ?, ?)
                ON CONFLICT(chat_id) DO UPDATE SET
                    user_id = excluded.user_id,
                    username = excluded.username,
                    ts = excluded.ts
                """,
                (chat_id, user_id, username or "", timestamp),
            )
            conn.commit()
    except Exception as exc:
        logger.warning("Could not persist subscriber %s: %s", chat_id, exc)


async def get_subscriber_chat_ids() -> list[int]:
    try:
        await _migrate_legacy_json_if_needed()

        if REDIS_URL:
            client = await _get_redis_client()
            raw_values = await client.smembers(_SUBSCRIBERS_SET_KEY)
            result = [int(value) for value in raw_values if str(value).lstrip("-").isdigit()]
            result.sort()
            return result

        conn = _ensure_sqlite()
        rows = conn.execute("SELECT chat_id FROM subscribers ORDER BY chat_id").fetchall()
        return [int(row[0]) for row in rows]
    except Exception as exc:
        logger.warning("Could not load subscribers: %s", exc)
        return []


async def _get_redis_client():
    global _redis_client

    if _redis_client is None:
        try:
            from redis.asyncio import Redis
        except ImportError as exc:
            raise RuntimeError(
                "redis package is required when REDIS_URL is set. Install bot/requirements.txt."
            ) from exc

        _redis_client = Redis.from_url(REDIS_URL, encoding="utf-8", decode_responses=True)

    return _redis_client


def _ensure_sqlite() -> sqlite3.Connection:
    global _sqlite_conn

    if _sqlite_conn is None:
        _SUBSCRIBERS_DB_FILE.parent.mkdir(parents=True, exist_ok=True)
        _sqlite_conn = sqlite3.connect(_SUBSCRIBERS_DB_FILE)
        _sqlite_conn.execute(
            """
            CREATE TABLE IF NOT EXISTS subscribers (
                chat_id INTEGER PRIMARY KEY,
                user_id INTEGER NOT NULL,
                username TEXT NOT NULL DEFAULT '',
                ts INTEGER NOT NULL
            )
            """
        )
        _sqlite_conn.commit()

    return _sqlite_conn


async def _migrate_legacy_json_if_needed() -> None:
    global _migration_checked

    if _migration_checked:
        return
    _migration_checked = True

    entries = _load_legacy_subscribers()
    if not entries:
        return

    if REDIS_URL:
        client = await _get_redis_client()
        if await client.scard(_SUBSCRIBERS_SET_KEY):
            return

        async with client.pipeline(transaction=True) as pipe:
            for chat_id, entry in entries.items():
                key = f"{_SUBSCRIBER_HASH_KEY_PREFIX}{chat_id}"
                pipe.sadd(_SUBSCRIBERS_SET_KEY, chat_id)
                pipe.hset(key, mapping=entry)
            await pipe.execute()
    else:
        conn = _ensure_sqlite()
        existing = conn.execute("SELECT COUNT(*) FROM subscribers").fetchone()
        if existing and int(existing[0]) > 0:
            return

        conn.executemany(
            """
            INSERT INTO subscribers (chat_id, user_id, username, ts)
            VALUES (?, ?, ?, ?)
            ON CONFLICT(chat_id) DO UPDATE SET
                user_id = excluded.user_id,
                username = excluded.username,
                ts = excluded.ts
            """,
            [
                (int(chat_id), entry["user_id"], entry["username"], entry["ts"])
                for chat_id, entry in entries.items()
            ],
        )
        conn.commit()

    logger.info(
        "Migrated %s legacy subscribers from %s into %s storage.",
        len(entries),
        _LEGACY_SUBSCRIBERS_FILE,
        storage_backend_name(),
    )


def _load_legacy_subscribers() -> dict[str, dict[str, int | str]]:
    if not _LEGACY_SUBSCRIBERS_FILE.exists():
        return {}

    try:
        with open(_LEGACY_SUBSCRIBERS_FILE, "r", encoding="utf-8") as file:
            data = json.load(file)
    except (json.JSONDecodeError, OSError) as exc:
        logger.warning("Could not load legacy subscribers: %s", exc)
        return {}

    chats = data.get("chats") or {}
    result: dict[str, dict[str, int | str]] = {}
    for raw_chat_id, raw_entry in chats.items():
        chat_id = str(raw_chat_id).strip()
        if not chat_id.lstrip("-").isdigit():
            continue

        entry = raw_entry if isinstance(raw_entry, dict) else {}
        user_id = entry.get("user_id")
        if not isinstance(user_id, int):
            try:
                user_id = int(user_id or 0)
            except (TypeError, ValueError):
                user_id = 0

        timestamp = entry.get("ts")
        if not isinstance(timestamp, int):
            try:
                timestamp = int(timestamp or 0)
            except (TypeError, ValueError):
                timestamp = 0

        result[chat_id] = {
            "user_id": user_id,
            "username": str(entry.get("username") or ""),
            "ts": timestamp,
        }

    return result
