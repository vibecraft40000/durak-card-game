from urllib.parse import urljoin, urlparse

from flask import Blueprint, flash, redirect, render_template, request, url_for
from flask_login import login_required, login_user, logout_user

from models import Admin

auth_bp = Blueprint("auth", __name__)


def _is_safe_redirect(target: str) -> bool:
    if not target:
        return False
    ref = urlparse(request.host_url)
    test = urlparse(urljoin(request.host_url, target))
    return test.scheme in {"http", "https"} and ref.netloc == test.netloc


@auth_bp.route("/login", methods=["GET", "POST"])
def login():
    if request.method == "GET":
        return render_template("login.html")

    username = (request.form.get("username") or "").strip()
    password = request.form.get("password") or ""
    if not username or not password:
        flash("Р’РІРµРґРёС‚Рµ Р»РѕРіРёРЅ Рё РїР°СЂРѕР»СЊ.", "danger")
        return render_template("login.html")

    admin = Admin.query.filter_by(username=username).first()
    if not admin or not admin.check_password(password):
        flash("РќРµРІРµСЂРЅС‹Р№ Р»РѕРіРёРЅ РёР»Рё РїР°СЂРѕР»СЊ.", "danger")
        return render_template("login.html")
    if not admin.is_active:
        flash("Р”РѕСЃС‚СѓРї РѕС‚РєР»СЋС‡С‘РЅ.", "danger")
        return render_template("login.html")

    login_user(admin, remember=bool(request.form.get("remember")))
    next_url_raw = (request.form.get("next") or request.args.get("next") or "").strip()
    next_url = next_url_raw if _is_safe_redirect(next_url_raw) else url_for("dashboard.index")
    return redirect(next_url)


@auth_bp.route("/logout", methods=["POST"])
@login_required
def logout():
    logout_user()
    flash("Р’С‹ РІС‹С€Р»Рё РёР· СЃРёСЃС‚РµРјС‹.", "info")
    return redirect(url_for("auth.login"))
