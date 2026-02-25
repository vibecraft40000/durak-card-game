import os
from pathlib import Path

# Load .env from bot/ or project root
try:
    from dotenv import load_dotenv
    bot_dir = Path(__file__).resolve().parent
    load_dotenv(bot_dir / ".env")
    load_dotenv(bot_dir.parent / ".env")
except ImportError:
    pass

BOT_TOKEN: str = (
    os.environ.get("BOT_TOKEN")
    or os.environ.get("TELEGRAM_BOT_TOKEN")
    or ""
).strip()

def _parse_admin_ids() -> list[int]:
    raw = os.environ.get("ADMIN_IDS", "").strip()
    if not raw:
        return []
    result = []
    for x in raw.split(","):
        s = x.strip()
        if not s:
            continue
        try:
            if s.isdigit():
                result.append(int(s))
        except ValueError:
            continue
    return result

ADMIN_IDS: list[int] = _parse_admin_ids()
"""Telegram user IDs that can use /admin. Get your ID from @userinfobot."""

WEBAPP_URL: str = os.environ.get("WEBAPP_URL", "https://durakonline.duckdns.org")
WEBHOOK_PATH: str = os.environ.get("WEBHOOK_PATH", "/webhook")
WEBHOOK_URL: str = os.environ.get("WEBHOOK_URL", "")
HOST: str = os.environ.get("HOST", "0.0.0.0")
PORT: int = int(os.environ.get("PORT", "8081"))

# Optional: backend API for admin stats (e.g. https://api.your-domain.com)
API_BASE_URL: str = (os.environ.get("API_BASE_URL") or "").strip()
ADMIN_SECRET: str = (os.environ.get("ADMIN_SECRET") or "").strip()
