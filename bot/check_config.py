#!/usr/bin/env python3
"""Проверка загрузки конфига бота. Запуск: из папки bot — python check_config.py"""

import os
import sys
from pathlib import Path

bot_dir = Path(__file__).resolve().parent
os.chdir(bot_dir)
if str(bot_dir) not in sys.path:
    sys.path.insert(0, str(bot_dir))

try:
    from dotenv import load_dotenv

    loaded1 = load_dotenv(bot_dir / ".env")
    loaded2 = load_dotenv(bot_dir.parent / ".env")
    print("Загрузка .env: bot/.env=%s, корень=%s" % (loaded1, loaded2))
except ImportError:
    print("python-dotenv не установлен — переменные только из окружения")

try:
    import config as cfg
except Exception as exc:
    print()
    print("CONFIG ERROR: %s" % exc)
    sys.exit(1)

print()
print("BOT_ENV: %s" % cfg.BOT_ENV)
print("BOT_TOKEN: %s" % ("задан (%s...)" % cfg.BOT_TOKEN[:20] if cfg.BOT_TOKEN else "НЕ ЗАДАН"))
print("ADMIN_IDS: %s" % (cfg.ADMIN_IDS if cfg.ADMIN_IDS else "ПУСТО (админка не будет работать)"))
print("API_BASE_URL: %s" % (cfg.API_BASE_URL or "(не задан)"))
print("ADMIN_SECRET: %s" % ("задан" if cfg.ADMIN_SECRET else "не задан"))
print("REDIS_URL: %s" % (cfg.REDIS_URL or "(не задан)"))
print("BOT_ENABLE_REDIS_FSM: %s" % cfg.BOT_ENABLE_REDIS_FSM)
print("SUBSCRIBERS_DB_PATH: %s" % cfg.SUBSCRIBERS_DB_PATH)
print()

if not cfg.ADMIN_IDS:
    print("Чтобы включить админку: в bot/.env добавьте ADMIN_IDS=ВАШ_TELEGRAM_ID")
    print("Узнать ID: @userinfobot. Затем перезапустите бота.")
else:
    print("Админка включена для ID: %s" % cfg.ADMIN_IDS)

if cfg.REDIS_URL:
    print("Subscriber/FSM production path: Redis")
else:
    print("Subscriber fallback: SQLite, FSM fallback: MemoryStorage")
