from __future__ import annotations

from flask import Blueprint, render_template, request

from api_client import get_logs, get_withdrawals
from auth_utils import admin_required
from config import ADMIN_SECRET, API_BASE_URL

logs_bp = Blueprint("logs", __name__)


@logs_bp.route("/")
@admin_required
def index():
    if not API_BASE_URL or not ADMIN_SECRET:
        return render_template(
            "logs.html",
            logs=[],
            withdrawals=[],
            error="Set API_BASE_URL and ADMIN_SECRET in .env",
            limit=100,
        )

    try:
        limit = int(request.args.get("limit", 100))
    except ValueError:
        limit = 100

    if limit <= 0 or limit > 200:
        limit = 100

    logs_response = get_logs(API_BASE_URL, ADMIN_SECRET, limit=limit)
    withdrawals_response = get_withdrawals(API_BASE_URL, ADMIN_SECRET, limit=30)

    return render_template(
        "logs.html",
        logs=(logs_response or {}).get("logs", []),
        withdrawals=(withdrawals_response or {}).get("withdrawals", []),
        error=None if logs_response is not None else "Failed to load operation logs",
        limit=limit,
    )
