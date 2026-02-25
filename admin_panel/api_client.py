"""REST-клиент к Go бэкенду для админки."""
import logging
from typing import Any

import requests

logger = logging.getLogger(__name__)


def _headers(admin_secret: str) -> dict:
    return {"X-Admin-Secret": admin_secret, "Accept": "application/json"}


def get_stats(api_base_url: str, admin_secret: str) -> dict[str, Any] | None:
    """GET /admin/stats — число пользователей и т.д."""
    if not api_base_url or not admin_secret:
        return None
    url = f"{api_base_url.rstrip('/')}/admin/stats"
    try:
        r = requests.get(url, headers=_headers(admin_secret), timeout=5)
        if r.status_code == 200:
            return r.json()
    except Exception as e:
        logger.warning("Admin API stats: %s", e)
    return None


def get_users(
    api_base_url: str, admin_secret: str, offset: int = 0, limit: int = 20
) -> dict[str, Any] | None:
    """GET /admin/users — список пользователей с пагинацией."""
    if not api_base_url or not admin_secret:
        return None
    url = f"{api_base_url.rstrip('/')}/admin/users"
    try:
        r = requests.get(
            url,
            params={"offset": offset, "limit": limit},
            headers=_headers(admin_secret),
            timeout=10,
        )
        if r.status_code == 200:
            return r.json()
    except Exception as e:
        logger.warning("Admin API users: %s", e)
    return None
