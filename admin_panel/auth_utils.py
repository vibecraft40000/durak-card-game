from functools import wraps
from flask import redirect, url_for, flash, request
from flask_login import current_user


def admin_required(f):
    """Декоратор: только для авторизованных админов."""
    @wraps(f)
    def decorated_view(*args, **kwargs):
        if not current_user.is_authenticated:
            flash("Войдите в систему.", "warning")
            return redirect(url_for("auth.login", next=request.url))
        if not getattr(current_user, "is_active", True):
            flash("Доступ запрещён.", "danger")
            return redirect(url_for("auth.login"))
        return f(*args, **kwargs)
    return decorated_view
