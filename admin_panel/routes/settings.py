import os
from flask import Blueprint, render_template
from auth_utils import admin_required
from config import API_BASE_URL, ADMIN_SECRET

settings_bp = Blueprint("settings", __name__)


@settings_bp.route("/")
@admin_required
def index():
    return render_template(
        "settings.html",
        api_base_url=API_BASE_URL or "не задан",
        admin_secret_set=bool(ADMIN_SECRET),
    )
