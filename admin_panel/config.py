import os
import warnings
from pathlib import Path

from dotenv import load_dotenv

load_dotenv(Path(__file__).resolve().parent / ".env")
load_dotenv(Path(__file__).resolve().parent.parent / ".env")

SECRET_KEY = os.environ.get("FLASK_SECRET_KEY") or os.environ.get("SECRET_KEY")
if not SECRET_KEY:
    SECRET_KEY = os.urandom(32).hex()
    warnings.warn("FLASK_SECRET_KEY/SECRET_KEY is not set; using ephemeral generated key.", RuntimeWarning)
SQLALCHEMY_DATABASE_URI = os.environ.get("ADMIN_DATABASE_URI") or "sqlite:///admin_data.db"
SQLALCHEMY_TRACK_MODIFICATIONS = False

# API бэкенда (Go) для статистики и списка пользователей
API_BASE_URL = (os.environ.get("API_BASE_URL") or "").strip()
ADMIN_SECRET = (os.environ.get("ADMIN_SECRET") or "").strip()

# Первый админ (создаётся при первом запуске, если таблица пуста)
ADMIN_USERNAME = os.environ.get("ADMIN_USERNAME", "admin")
ADMIN_PASSWORD = os.environ.get("ADMIN_PASSWORD", "")
