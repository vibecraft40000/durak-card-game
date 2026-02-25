import asyncio
import logging
import sys

from aiogram import Bot, Dispatcher
from aiogram.client.default import DefaultBotProperties
from aiogram.enums import ParseMode
from aiogram.webhook.aiohttp_server import SimpleRequestHandler, setup_application
from aiohttp import web

from config import BOT_TOKEN, WEBHOOK_PATH, WEBHOOK_URL, HOST, PORT, ADMIN_IDS

if not BOT_TOKEN:
    raise SystemExit(
        "BOT_TOKEN не задан. Добавьте в .env или запустите:\n"
        "  $env:BOT_TOKEN=\"ваш_токен\"; python main.py"
    )
from handlers import start_router, admin_router

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    stream=sys.stdout,
)
logger = logging.getLogger(__name__)
if ADMIN_IDS:
    logger.info("Admin panel enabled for Telegram IDs: %s", ADMIN_IDS)
else:
    logger.warning("ADMIN_IDS not set — /admin will not work. Set ADMIN_IDS in bot/.env")


async def on_startup(bot: Bot) -> None:
    if WEBHOOK_URL:
        await bot.set_webhook(
            f"{WEBHOOK_URL.rstrip('/')}{WEBHOOK_PATH}",
            drop_pending_updates=True,
        )
        logger.info("Webhook set: %s%s", WEBHOOK_URL, WEBHOOK_PATH)
    else:
        logger.warning("WEBHOOK_URL not set — running in long polling mode")


async def on_shutdown(bot: Bot) -> None:
    if WEBHOOK_URL:
        await bot.delete_webhook()
    logger.info("Bot shutdown")


def main() -> None:
    bot = Bot(
        token=BOT_TOKEN,
        default=DefaultBotProperties(parse_mode=ParseMode.HTML),
    )
    dp = Dispatcher()
    dp.include_router(start_router)
    dp.include_router(admin_router)
    dp.startup.register(on_startup)
    dp.shutdown.register(on_shutdown)

    if WEBHOOK_URL:
        app = web.Application()
        handler = SimpleRequestHandler(
            dispatcher=dp,
            bot=bot,
            handle_in_background=True,
        )
        handler.register(app, path=WEBHOOK_PATH)
        setup_application(app, dp, bot=bot)
        web.run_app(app, host=HOST, port=PORT)
    else:
        async def run_polling() -> None:
            await dp.start_polling(bot)

        asyncio.run(run_polling())


if __name__ == "__main__":
    main()
