from __future__ import annotations

import os
from dataclasses import dataclass
from functools import lru_cache
from pathlib import Path

# Load .env from bot/ or project root.
try:
    from dotenv import load_dotenv

    bot_dir = Path(__file__).resolve().parent
    load_dotenv(bot_dir / ".env")
    load_dotenv(bot_dir.parent / ".env")
except ImportError:
    bot_dir = Path(__file__).resolve().parent

_TRUE_VALUES = {"1", "true", "yes", "on"}
_FALSE_VALUES = {"0", "false", "no", "off"}
_VALID_ENVS = {"development", "testing", "production"}


class ConfigError(RuntimeError):
    """Raised when bot configuration is incomplete or unsafe."""


def _read_env(name: str, default: str = "") -> str:
    return (os.environ.get(name) or default).strip()


def _read_bool(name: str, default: bool) -> bool:
    raw = os.environ.get(name)
    if raw is None:
        return default

    normalized = raw.strip().lower()
    if normalized in _TRUE_VALUES:
        return True
    if normalized in _FALSE_VALUES:
        return False
    raise ConfigError(f"{name} must be one of: 1/0, true/false, yes/no, on/off.")


def _read_int(name: str, default: int) -> int:
    raw = os.environ.get(name)
    if raw is None or not raw.strip():
        return default
    try:
        return int(raw)
    except ValueError as exc:
        raise ConfigError(f"{name} must be an integer.") from exc


def _parse_admin_ids() -> list[int]:
    raw = _read_env("ADMIN_IDS")
    if not raw:
        return []

    result: list[int] = []
    for value in raw.split(","):
        normalized = value.strip()
        if not normalized:
            continue
        if normalized.isdigit():
            result.append(int(normalized))
    return result


@dataclass(frozen=True, slots=True)
class Settings:
    environment: str
    bot_token: str
    admin_ids: list[int]
    webapp_url: str
    webhook_path: str
    webhook_url: str
    host: str
    port: int
    api_base_url: str
    admin_secret: str
    redis_url: str
    subscribers_db_path: str
    fsm_redis_enabled: bool
    required_channel_id: str  # Telegram channel ID for subscription gate (e.g. -1001234567890); bot must be admin
    channel_invite_link: str  # Invite link for "Subscribe" button (e.g. https://t.me/+xxx)

    @property
    def is_production(self) -> bool:
        return self.environment == "production"


def _build_settings() -> Settings:
    environment = _read_env("BOT_ENV", "development").lower()
    if environment not in _VALID_ENVS:
        raise ConfigError(f"BOT_ENV must be one of: {', '.join(sorted(_VALID_ENVS))}.")

    webhook_path = _read_env("WEBHOOK_PATH", "/webhook")
    if webhook_path and not webhook_path.startswith("/"):
        raise ConfigError("WEBHOOK_PATH must start with '/'.")

    redis_url = _read_env("REDIS_URL")
    if environment == "production" and not redis_url:
        raise ConfigError("REDIS_URL must be set in production for durable subscriber/FSM storage.")

    bot_data_dir = bot_dir / "data"
    subscribers_db_path = _read_env("BOT_SUBSCRIBERS_DB_PATH", str(bot_data_dir / "bot_storage.sqlite3"))

    return Settings(
        environment=environment,
        bot_token=(_read_env("BOT_TOKEN") or _read_env("TELEGRAM_BOT_TOKEN")).strip(),
        admin_ids=_parse_admin_ids(),
        webapp_url=_read_env("WEBAPP_URL", "https://your-domain.example"),
        webhook_path=webhook_path,
        webhook_url=_read_env("WEBHOOK_URL"),
        host=_read_env("HOST", "0.0.0.0"),
        port=_read_int("PORT", 8081),
        api_base_url=_read_env("API_BASE_URL"),
        admin_secret=_read_env("ADMIN_SECRET"),
        redis_url=redis_url,
        subscribers_db_path=subscribers_db_path,
        fsm_redis_enabled=_read_bool("BOT_ENABLE_REDIS_FSM", default=bool(redis_url)),
        required_channel_id=_read_env("REQUIRED_CHANNEL_ID", ""),
        channel_invite_link=_read_env("CHANNEL_INVITE_LINK", "https://t.me/+P_rbSS0y5N9jM2Qy"),
    )


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return _build_settings()


_SETTINGS = get_settings()

BOT_ENV = _SETTINGS.environment
BOT_TOKEN = _SETTINGS.bot_token
ADMIN_IDS = _SETTINGS.admin_ids
WEBAPP_URL = _SETTINGS.webapp_url
WEBHOOK_PATH = _SETTINGS.webhook_path
WEBHOOK_URL = _SETTINGS.webhook_url
HOST = _SETTINGS.host
PORT = _SETTINGS.port
API_BASE_URL = _SETTINGS.api_base_url
ADMIN_SECRET = _SETTINGS.admin_secret
REDIS_URL = _SETTINGS.redis_url
SUBSCRIBERS_DB_PATH = _SETTINGS.subscribers_db_path
BOT_ENABLE_REDIS_FSM = _SETTINGS.fsm_redis_enabled
REQUIRED_CHANNEL_ID = _SETTINGS.required_channel_id
CHANNEL_INVITE_LINK = _SETTINGS.channel_invite_link
