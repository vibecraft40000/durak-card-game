from flask import Blueprint, render_template, request
from auth_utils import admin_required
from api_client import get_users
from config import API_BASE_URL, ADMIN_SECRET

users_bp = Blueprint("users", __name__)
PER_PAGE = 20


@users_bp.route("/")
@admin_required
def index():
    if not API_BASE_URL or not ADMIN_SECRET:
        return render_template(
            "users.html",
            users=[],
            total=0,
            page=1,
            pages=0,
            error="Задайте API_BASE_URL и ADMIN_SECRET в .env",
        )
    page = max(1, int(request.args.get("page", 1)))
    offset = (page - 1) * PER_PAGE
    data = get_users(API_BASE_URL, ADMIN_SECRET, offset=offset, limit=PER_PAGE)
    if data is None:
        return render_template(
            "users.html",
            users=[],
            total=0,
            page=page,
            pages=0,
            error="Не удалось загрузить данные с API.",
        )
    users_list = data.get("users") or []
    total = data.get("total") or 0
    pages = (total + PER_PAGE - 1) // PER_PAGE if total else 0
    return render_template(
        "users.html",
        users=users_list,
        total=total,
        page=page,
        pages=pages,
        error=None,
    )
