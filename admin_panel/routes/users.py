from __future__ import annotations

from flask import Blueprint, flash, redirect, render_template, request, url_for

from api_client import adjust_balance, ban_user, get_users, unban_user
from auth_utils import admin_required
from config import ADMIN_SECRET, API_BASE_URL

users_bp = Blueprint("users", __name__)
PER_PAGE = 20


@users_bp.route("/", methods=["GET", "POST"])
@admin_required
def index():
    page = max(1, int(request.args.get("page", 1)))

    if request.method == "POST":
        if not API_BASE_URL or not ADMIN_SECRET:
            flash("Set API_BASE_URL and ADMIN_SECRET first", "danger")
            return redirect(url_for("users.index", page=page))

        action = (request.form.get("action") or "").strip()
        user_id = (request.form.get("user_id") or "").strip()
        if not user_id:
            flash("User ID is required", "warning")
            return redirect(url_for("users.index", page=page))

        if action == "ban":
            ok = ban_user(API_BASE_URL, ADMIN_SECRET, user_id)
            flash("User banned" if ok else "Failed to ban user", "success" if ok else "danger")
        elif action == "unban":
            ok = unban_user(API_BASE_URL, ADMIN_SECRET, user_id)
            flash("User unbanned" if ok else "Failed to unban user", "success" if ok else "danger")
        elif action == "adjust_balance":
            amount_raw = (request.form.get("amount") or "0").strip()
            reason = (request.form.get("reason") or "manual adjust").strip()
            try:
                amount = float(amount_raw)
            except ValueError:
                flash("Amount must be a number", "warning")
                return redirect(url_for("users.index", page=page))

            if amount == 0:
                flash("Amount must be non-zero", "warning")
                return redirect(url_for("users.index", page=page))

            ok = adjust_balance(API_BASE_URL, ADMIN_SECRET, user_id, amount, reason)
            flash("Balance updated" if ok else "Failed to update balance", "success" if ok else "danger")
        else:
            flash("Unknown action", "warning")

        return redirect(url_for("users.index", page=page))

    if not API_BASE_URL or not ADMIN_SECRET:
        return render_template(
            "users.html",
            users=[],
            total=0,
            page=1,
            pages=0,
            error="Set API_BASE_URL and ADMIN_SECRET in .env",
        )

    offset = (page - 1) * PER_PAGE
    data = get_users(API_BASE_URL, ADMIN_SECRET, offset=offset, limit=PER_PAGE)
    if data is None:
        return render_template(
            "users.html",
            users=[],
            total=0,
            page=page,
            pages=0,
            error="Failed to load users from API.",
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
