import logging
import os
from pathlib import Path

from flask import Flask, redirect
from flask_login import LoginManager, current_user

from config import (
    SECRET_KEY,
    SQLALCHEMY_DATABASE_URI,
    SQLALCHEMY_TRACK_MODIFICATIONS,
    ADMIN_USERNAME,
    ADMIN_PASSWORD,
)
from models import db, Admin

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def create_app() -> Flask:
    app = Flask(__name__)
    app.config["SECRET_KEY"] = SECRET_KEY
    app.config["SQLALCHEMY_DATABASE_URI"] = SQLALCHEMY_DATABASE_URI
    app.config["SQLALCHEMY_TRACK_MODIFICATIONS"] = SQLALCHEMY_TRACK_MODIFICATIONS

    db.init_app(app)
    login_manager = LoginManager()
    login_manager.init_app(app)
    login_manager.login_view = "auth.login"
    login_manager.login_message = "Войдите для доступа в админку."

    @login_manager.user_loader
    def load_user(user_id):
        return db.session.get(Admin, int(user_id))

    with app.app_context():
        db.create_all()
        if ADMIN_PASSWORD and Admin.query.filter_by(username=ADMIN_USERNAME).first() is None:
            admin = Admin(username=ADMIN_USERNAME)
            admin.set_password(ADMIN_PASSWORD)
            db.session.add(admin)
            db.session.commit()
            logger.info("Создан первый админ: %s", ADMIN_USERNAME)

    from routes.auth import auth_bp
    from routes.dashboard import dashboard_bp
    from routes.users import users_bp
    from routes.settings import settings_bp

    app.register_blueprint(auth_bp, url_prefix="/")
    app.register_blueprint(dashboard_bp, url_prefix="/dashboard")
    app.register_blueprint(users_bp, url_prefix="/users")
    app.register_blueprint(settings_bp, url_prefix="/settings")

    @app.route("/")
    def index():
        return redirect("/dashboard" if current_user.is_authenticated else "/login")

    app.jinja_env.globals["current_user"] = current_user

    return app
