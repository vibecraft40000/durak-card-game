"""Simple file-based storage for bot subscribers (for admin broadcast)."""
import json
import logging
from pathlib import Path

logger = logging.getLogger(__name__)

_BOT_DIR = Path(__file__).resolve().parent
_SUBSCRIBERS_FILE = _BOT_DIR / "data" / "subscribers.json"


def _ensure_data_dir() -> Path:
    _SUBSCRIBERS_FILE.parent.mkdir(parents=True, exist_ok=True)
    return _SUBSCRIBERS_FILE.parent


def add_subscriber(chat_id: int, user_id: int, username: str | None) -> None:
    """Add or refresh a subscriber (call on /start)."""
    _ensure_data_dir()
    data = _load()
    data["chats"] = data.get("chats") or {}
    entry = data["chats"].get(str(chat_id)) or {}
    entry["user_id"] = user_id
    entry["username"] = username or ""
    entry["ts"] = entry.get("ts", 0)
    from time import time
    entry["ts"] = int(time())
    data["chats"][str(chat_id)] = entry
    _save(data)
    logger.debug("Subscriber added: chat_id=%s", chat_id)


def get_subscriber_chat_ids() -> list[int]:
    """Return list of chat IDs for broadcast."""
    data = _load()
    chats = data.get("chats") or {}
    return [int(c) for c in chats.keys() if str(c).lstrip("-").isdigit()]


def _load() -> dict:
    if not _SUBSCRIBERS_FILE.exists():
        return {}
    try:
        with open(_SUBSCRIBERS_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    except (json.JSONDecodeError, OSError) as e:
        logger.warning("Could not load subscribers: %s", e)
        return {}


def _save(data: dict) -> None:
    try:
        with open(_SUBSCRIBERS_FILE, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
    except OSError as e:
        logger.warning("Could not save subscribers: %s", e)
