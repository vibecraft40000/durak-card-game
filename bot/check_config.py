#!/usr/bin/env python3
"""Проверка загрузки конфига бота. Запуск: из папки bot — python check_config.py"""
import os
import sys
from pathlib import Path

# убедиться, что грузим из папки bot
bot_dir = Path(__file__).resolve().parent
os.chdir(bot_dir)
if str(bot_dir) not in sys.path:
    sys.path.insert(0, str(bot_dir))

# загружаем .env вручную, как в config
try:
    from dotenv import load_dotenv
    loaded1 = load_dotenv(bot_dir / ".env")
    loaded2 = load_dotenv(bot_dir.parent / ".env")
    print("Загрузка .env: bot/.env=%s, корень=%s" % (loaded1, loaded2))
except ImportError:
    print("python-dotenv не установлен — переменные только из окружения")

# импортируем config после смены cwd и загрузки .env
from config import BOT_TOKEN, ADMIN_IDS, API_BASE_URL, ADMIN_SECRET

print()
print("BOT_TOKEN: %s" % ("задан (%s...)" % BOT_TOKEN[:20] if BOT_TOKEN else "НЕ ЗАДАН"))
print("ADMIN_IDS: %s" % (ADMIN_IDS if ADMIN_IDS else "ПУСТО (админка не будет работать)"))
print("API_BASE_URL: %s" % (API_BASE_URL or "(не задан)"))
print("ADMIN_SECRET: %s" % ("задан" if ADMIN_SECRET else "не задан"))
print()
if not ADMIN_IDS:
    print("Чтобы включить админку: в bot/.env добавьте ADMIN_IDS=ВАШ_TELEGRAM_ID")
    print("Узнать ID: @userinfobot. Затем перезапустите бота.")
else:
    print("Админка включена для ID: %s" % ADMIN_IDS)
