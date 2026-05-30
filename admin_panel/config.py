from __future__ import annotations

import os
from dataclasses import dataclass
from datetime import timedelta
from functools import lru_cache
from pathlib import Path

from dotenv import load_dotenv

load_dotenv(Path(__file__).resolve().parent / ".env")
load_dotenv(Path(__file__).resolve().parent.parent / ".env")

_TRUE_VALUES = {"1", "true", "yes", "on"}
_FALSE_VALUES = {"0", "false", "no", "off"}
_VALID_ENVS = {"development", "testing", "production"}


class ConfigError(RuntimeError):
    """Raised when admin_panel configuration is incomplete or unsafe."""


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


@dataclass(frozen=True, slots=True)
class Settings:
    environment: str
    secret_key: str
    database_uri: str
    sqlalchemy_track_modifications: bool
    api_base_url: str
    admin_secret: str
    admin_username: str
    admin_password: str
    host: str
    port: int
    debug: bool
    trust_proxy_headers: bool
    session_cookie_secure: bool
    session_cookie_samesite: str
    remember_cookie_secure: bool
    remember_cookie_samesite: str
    session_lifetime: timedelta

    @property
    def is_production(self) -> bool:
        return self.environment == "production"


def _build_settings() -> Settings:
    environment = _read_env("ADMIN_PANEL_ENV", _read_env("FLASK_ENV", "development")).lower()
    if environment not in _VALID_ENVS:
        raise ConfigError(
            f"ADMIN_PANEL_ENV/FLASK_ENV must be one of: {', '.join(sorted(_VALID_ENVS))}."
        )

    secret_key = _read_env("FLASK_SECRET_KEY") or _read_env("SECRET_KEY")
    if not secret_key:
        raise ConfigError("FLASK_SECRET_KEY or SECRET_KEY must be set.")
    if environment == "production" and len(secret_key) < 32:
        raise ConfigError("FLASK_SECRET_KEY must be at least 32 characters in production.")

    database_uri = _read_env("ADMIN_DATABASE_URI", "sqlite:///admin_data.db")
    if not database_uri:
        raise ConfigError("ADMIN_DATABASE_URI must not be empty.")

    api_base_url = _read_env("API_BASE_URL")
    admin_secret = _read_env("ADMIN_SECRET")

    debug = _read_bool("FLASK_DEBUG", default=environment == "development")
    if environment == "production" and debug:
        raise ConfigError("FLASK_DEBUG must be disabled in production.")

    return Settings(
        environment=environment,
        secret_key=secret_key,
        database_uri=database_uri,
        sqlalchemy_track_modifications=False,
        api_base_url=api_base_url,
        admin_secret=admin_secret,
        admin_username=_read_env("ADMIN_USERNAME", "admin"),
        admin_password=os.environ.get("ADMIN_PASSWORD", ""),
        host=_read_env("HOST", "0.0.0.0"),
        port=_read_int("PORT", 5000),
        debug=debug,
        trust_proxy_headers=_read_bool("ADMIN_PANEL_TRUST_PROXY_HEADERS", default=False),
        session_cookie_secure=environment == "production",
        session_cookie_samesite="Lax",
        remember_cookie_secure=environment == "production",
        remember_cookie_samesite="Lax",
        session_lifetime=timedelta(hours=12),
    )


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return _build_settings()


_SETTINGS = get_settings()

SECRET_KEY = _SETTINGS.secret_key
SQLALCHEMY_DATABASE_URI = _SETTINGS.database_uri
SQLALCHEMY_TRACK_MODIFICATIONS = _SETTINGS.sqlalchemy_track_modifications
API_BASE_URL = _SETTINGS.api_base_url
ADMIN_SECRET = _SETTINGS.admin_secret
ADMIN_USERNAME = _SETTINGS.admin_username
ADMIN_PASSWORD = _SETTINGS.admin_password
ADMIN_PANEL_ENV = _SETTINGS.environment
HOST = _SETTINGS.host
PORT = _SETTINGS.port
DEBUG = _SETTINGS.debug
TRUST_PROXY_HEADERS = _SETTINGS.trust_proxy_headers
SESSION_COOKIE_SECURE = _SETTINGS.session_cookie_secure
SESSION_COOKIE_SAMESITE = _SETTINGS.session_cookie_samesite
REMEMBER_COOKIE_SECURE = _SETTINGS.remember_cookie_secure
REMEMBER_COOKIE_SAMESITE = _SETTINGS.remember_cookie_samesite
PERMANENT_SESSION_LIFETIME = _SETTINGS.session_lifetime
