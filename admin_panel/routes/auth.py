from flask import Blueprint, render_template, redirect, url_for, request, flash
from flask_login import login_user, logout_user, login_required
from urllib.parse import urljoin, urlparse

from models import db, Admin

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
        flash("Введите логин и пароль.", "danger")
        return render_template("login.html")
    admin = Admin.query.filter_by(username=username).first()
    if not admin or not admin.check_password(password):
        flash("Неверный логин или пароль.", "danger")
        return render_template("login.html")
    if not admin.is_active:
        flash("Доступ отключён.", "danger")
        return render_template("login.html")
    login_user(admin, remember=bool(request.form.get("remember")))
    next_url_raw = request.args.get("next") or ""
    next_url = next_url_raw if _is_safe_redirect(next_url_raw) else url_for("dashboard.index")
    return redirect(next_url)


@auth_bp.route("/logout")
@login_required
def logout():
    logout_user()
    flash("Вы вышли из системы.", "info")
    return redirect(url_for("auth.login"))
