"""REST client for Go backend admin endpoints."""

from __future__ import annotations

import logging
from typing import Any

import requests
from flask_login import current_user

logger = logging.getLogger(__name__)


def _current_admin_actor() -> str:
    try:
        if not getattr(current_user, "is_authenticated", False):
            return ""
        username = str(getattr(current_user, "username", "") or "").strip()
        admin_id = ""
        get_id = getattr(current_user, "get_id", None)
        if callable(get_id):
            admin_id = str(get_id() or "").strip()
        if username and admin_id:
            return f"{username}#{admin_id}"
        return username or admin_id
    except RuntimeError:
        return ""


def _headers(admin_secret: str) -> dict[str, str]:
    headers = {
        "X-Admin-Secret": admin_secret,
        "Accept": "application/json",
        "Content-Type": "application/json",
    }
    actor = _current_admin_actor()
    if actor:
        headers["X-Admin-Actor"] = actor
    return headers


def _request(
    method: str,
    api_base_url: str,
    admin_secret: str,
    path: str,
    *,
    params: dict[str, Any] | None = None,
    json_body: dict[str, Any] | None = None,
    timeout: int = 10,
) -> dict[str, Any] | None:
    if not api_base_url or not admin_secret:
        return None

    url = f"{api_base_url.rstrip('/')}{path}"
    try:
        response = requests.request(
            method,
            url,
            params=params,
            json=json_body,
            headers=_headers(admin_secret),
            timeout=timeout,
        )
        if response.status_code == 200:
            return response.json()
        logger.warning("Admin API %s %s failed: %s %s", method, path, response.status_code, response.text)
    except Exception as exc:  # pragma: no cover - network errors are expected runtime conditions
        logger.warning("Admin API %s %s error: %s", method, path, exc)

    return None


def get_stats(api_base_url: str, admin_secret: str) -> dict[str, Any] | None:
    return _request("GET", api_base_url, admin_secret, "/admin/stats", timeout=5)


def get_users(api_base_url: str, admin_secret: str, offset: int = 0, limit: int = 20) -> dict[str, Any] | None:
    return _request(
        "GET",
        api_base_url,
        admin_secret,
        "/admin/users",
        params={"offset": offset, "limit": limit},
    )


def ban_user(api_base_url: str, admin_secret: str, user_id: str) -> dict[str, Any] | None:
    return _request("POST", api_base_url, admin_secret, f"/admin/users/{user_id}/ban")


def unban_user(api_base_url: str, admin_secret: str, user_id: str) -> dict[str, Any] | None:
    return _request("POST", api_base_url, admin_secret, f"/admin/users/{user_id}/unban")


def adjust_balance(
    api_base_url: str,
    admin_secret: str,
    user_id: str,
    amount: float,
    reason: str,
) -> dict[str, Any] | None:
    return _request(
        "POST",
        api_base_url,
        admin_secret,
        f"/admin/users/{user_id}/balance-adjust",
        json_body={"amount": amount, "reason": reason},
    )


def get_logs(api_base_url: str, admin_secret: str, limit: int = 100) -> dict[str, Any] | None:
    return _request(
        "GET",
        api_base_url,
        admin_secret,
        "/admin/logs",
        params={"limit": limit},
    )


def get_withdrawals(api_base_url: str, admin_secret: str, limit: int = 30) -> dict[str, Any] | None:
    return _request(
        "GET",
        api_base_url,
        admin_secret,
        "/admin/withdrawals",
        params={"limit": limit},
    )
