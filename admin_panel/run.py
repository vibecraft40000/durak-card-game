#!/usr/bin/env python3
from app import create_app
from config import DEBUG, HOST, PORT, get_settings

app = create_app()


if __name__ == "__main__":
    settings = get_settings()
    if settings.is_production:
        raise SystemExit(
            "Production mode requires a WSGI server. Start admin_panel/wsgi.py with waitress/gunicorn and do not use Flask's built-in server."
        )
    else:
        app.run(host=HOST, port=PORT, debug=DEBUG)
