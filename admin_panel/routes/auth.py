from flask import Blueprint, render_template, redirect, url_for, request, flash
from flask_login import login_user, logout_user, login_required

from models import db, Admin
from auth_utils import admin_required

auth_bp = Blueprint("auth", __name__)


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
    next_url = request.args.get("next") or url_for("dashboard.index")
    return redirect(next_url)


@auth_bp.route("/logout")
@login_required
def logout():
    logout_user()
    flash("Вы вышли из системы.", "info")
    return redirect(url_for("auth.login"))
