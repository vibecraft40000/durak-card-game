"""REST client for Go backend admin endpoints."""

from __future__ import annotations

import logging
from typing import Any

import requests

logger = logging.getLogger(__name__)


def _headers(admin_secret: str) -> dict[str, str]:
    return {
        "X-Admin-Secret": admin_secret,
        "Accept": "application/json",
        "Content-Type": "application/json",
    }


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
