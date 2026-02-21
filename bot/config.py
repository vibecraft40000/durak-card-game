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
WEBAPP_URL: str = os.environ.get("WEBAPP_URL", "https://your-domain.com")
WEBHOOK_PATH: str = os.environ.get("WEBHOOK_PATH", "/webhook")
WEBHOOK_URL: str = os.environ.get("WEBHOOK_URL", "")
HOST: str = os.environ.get("HOST", "0.0.0.0")
PORT: int = int(os.environ.get("PORT", "8081"))
