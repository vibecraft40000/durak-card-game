import logging

from flask import Flask, redirect
from flask_login import LoginManager, current_user
from werkzeug.middleware.proxy_fix import ProxyFix

from auth_utils import register_security_baseline
from config import ADMIN_PASSWORD, ADMIN_USERNAME, get_settings
from models import Admin, db

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def create_app() -> Flask:
    settings = get_settings()
    app = Flask(__name__)
    app.config["SECRET_KEY"] = settings.secret_key
    app.config["SQLALCHEMY_DATABASE_URI"] = settings.database_uri
    app.config["SQLALCHEMY_TRACK_MODIFICATIONS"] = settings.sqlalchemy_track_modifications
    app.config["SESSION_COOKIE_SECURE"] = settings.session_cookie_secure
    app.config["SESSION_COOKIE_HTTPONLY"] = True
    app.config["SESSION_COOKIE_SAMESITE"] = settings.session_cookie_samesite
    app.config["REMEMBER_COOKIE_SECURE"] = settings.remember_cookie_secure
    app.config["REMEMBER_COOKIE_HTTPONLY"] = True
    app.config["REMEMBER_COOKIE_SAMESITE"] = settings.remember_cookie_samesite
    app.config["PERMANENT_SESSION_LIFETIME"] = settings.session_lifetime
    app.config["APP_SETTINGS"] = settings

    if settings.trust_proxy_headers:
        app.wsgi_app = ProxyFix(app.wsgi_app, x_for=1, x_proto=1, x_host=1)

    db.init_app(app)
    register_security_baseline(app)

    login_manager = LoginManager()
    login_manager.init_app(app)
    login_manager.login_view = "auth.login"
    login_manager.login_message = "Р’РѕР№РґРёС‚Рµ РґР»СЏ РґРѕСЃС‚СѓРїР° РІ Р°РґРјРёРЅРєСѓ."

    @login_manager.user_loader
    def load_user(user_id):
        return db.session.get(Admin, int(user_id))

    with app.app_context():
        db.create_all()
        has_admin = Admin.query.first() is not None
        if not has_admin and not ADMIN_PASSWORD:
            raise RuntimeError(
                "ADMIN_PASSWORD must be set before first admin_panel startup so the initial admin can be created."
            )
        if ADMIN_PASSWORD and Admin.query.filter_by(username=ADMIN_USERNAME).first() is None:
            admin = Admin(username=ADMIN_USERNAME)
            admin.set_password(ADMIN_PASSWORD)
            db.session.add(admin)
            db.session.commit()
            logger.info("РЎРѕР·РґР°РЅ РїРµСЂРІС‹Р№ Р°РґРјРёРЅ: %s", ADMIN_USERNAME)

    from routes.auth import auth_bp
    from routes.dashboard import dashboard_bp
    from routes.logs import logs_bp
    from routes.settings import settings_bp
    from routes.users import users_bp

    app.register_blueprint(auth_bp, url_prefix="/")
    app.register_blueprint(dashboard_bp, url_prefix="/dashboard")
    app.register_blueprint(users_bp, url_prefix="/users")
    app.register_blueprint(logs_bp, url_prefix="/logs")
    app.register_blueprint(settings_bp, url_prefix="/settings")

    @app.route("/")
    def index():
        return redirect("/dashboard" if current_user.is_authenticated else "/login")

    app.jinja_env.globals["current_user"] = current_user
    return app
