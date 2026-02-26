from flask import Blueprint, render_template
from auth_utils import admin_required
from api_client import get_stats
from config import API_BASE_URL, ADMIN_SECRET

dashboard_bp = Blueprint("dashboard", __name__)


@dashboard_bp.route("/")
@admin_required
def index():
    stats = get_stats(API_BASE_URL, ADMIN_SECRET) if API_BASE_URL and ADMIN_SECRET else None
    api_ok = stats is not None

    metrics = {
        "users_count": (stats.get("users_count") if stats else None) or "—",
        "games_total": (stats.get("games_total") if stats else None) or 0,
        "games_active": (stats.get("games_active") if stats else None) or 0,
        "games_finished": (stats.get("games_finished") if stats else None) or 0,
        "deposits_count": (stats.get("deposits_count") if stats else None) or 0,
        "deposits_amount": (stats.get("deposits_amount") if stats else None) or 0,
        "withdrawals_count": (stats.get("withdrawals_count") if stats else None) or 0,
        "withdrawals_amount": (stats.get("withdrawals_amount") if stats else None) or 0,
        "admin_adjust_count": (stats.get("admin_adjust_count") if stats else None) or 0,
    }

    return render_template(
        "dashboard.html",
        metrics=metrics,
        api_ok=api_ok,
        api_configured=bool(API_BASE_URL and ADMIN_SECRET),
    )
