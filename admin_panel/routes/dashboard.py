from flask import Blueprint, render_template
from auth_utils import admin_required
from api_client import get_stats
from config import API_BASE_URL, ADMIN_SECRET

dashboard_bp = Blueprint("dashboard", __name__)


@dashboard_bp.route("/")
@admin_required
def index():
    stats = get_stats(API_BASE_URL, ADMIN_SECRET) if API_BASE_URL and ADMIN_SECRET else None
    users_count = (stats.get("users_count") if stats else None) or "—"
    api_ok = stats is not None
    return render_template(
        "dashboard.html",
        users_count=users_count,
        api_ok=api_ok,
        api_configured=bool(API_BASE_URL and ADMIN_SECRET),
    )
