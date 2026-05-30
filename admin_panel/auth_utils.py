import hmac
import secrets
from functools import wraps

from flask import abort, current_app, flash, redirect, request, session, url_for
from flask_login import current_user

CSRF_FIELD_NAME = "csrf_token"
_CSRF_SESSION_KEY = "_admin_panel_csrf_token"
_UNSAFE_METHODS = {"POST", "PUT", "PATCH", "DELETE"}


def generate_csrf_token() -> str:
    token = session.get(_CSRF_SESSION_KEY)
    if not token:
        token = secrets.token_urlsafe(32)
        session[_CSRF_SESSION_KEY] = token
    return token


def _is_valid_csrf_token(submitted_token: str | None) -> bool:
    expected_token = session.get(_CSRF_SESSION_KEY)
    if not expected_token or not submitted_token:
        return False
    return hmac.compare_digest(expected_token, submitted_token)


def register_security_baseline(app) -> None:
    @app.before_request
    def enforce_csrf():
        if request.method not in _UNSAFE_METHODS:
            return None

        submitted_token = request.form.get(CSRF_FIELD_NAME) or request.headers.get("X-CSRF-Token")
        if _is_valid_csrf_token(submitted_token):
            return None

        current_app.logger.warning("CSRF validation failed for %s %s", request.method, request.path)
        abort(400, description="CSRF token missing or invalid.")

    @app.context_processor
    def inject_csrf_helpers():
        return {
            "csrf_token": generate_csrf_token,
            "csrf_field_name": CSRF_FIELD_NAME,
        }


def admin_required(f):
    """Р”РµРєРѕСЂР°С‚РѕСЂ: С‚РѕР»СЊРєРѕ РґР»СЏ Р°РІС‚РѕРёР·РѕРІР°РЅРЅС‹С… Р°РґРјРёРЅРѕРІ."""

    @wraps(f)
    def decorated_view(*args, **kwargs):
        if not current_user.is_authenticated:
            flash("Р’РѕР№РґРёС‚Рµ РІ СЃРёСЃС‚РµРјСѓ.", "warning")
            return redirect(url_for("auth.login", next=request.url))
        if not getattr(current_user, "is_active", True):
            flash("Р”РѕСЃС‚СѓРї Р·Р°РїСЂРµС‰С‘РЅ.", "danger")
            return redirect(url_for("auth.login"))
        return f(*args, **kwargs)

    return decorated_view
