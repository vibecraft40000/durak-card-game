import asyncio
import logging
import sys

from aiogram import Bot, Dispatcher
from aiogram.client.default import DefaultBotProperties
from aiogram.enums import ParseMode
from aiogram.fsm.storage.memory import MemoryStorage
from aiogram.webhook.aiohttp_server import SimpleRequestHandler, setup_application
from aiohttp import web

from config import (
    ADMIN_IDS,
    BOT_ENABLE_REDIS_FSM,
    BOT_TOKEN,
    HOST,
    PORT,
    REDIS_URL,
    WEBHOOK_PATH,
    WEBHOOK_URL,
)
from handlers import admin_router, start_router
from storage import close_storage, init_storage

if not BOT_TOKEN:
    raise SystemExit(
        "BOT_TOKEN РЅРµ Р·Р°РґР°РЅ. Р”РѕР±Р°РІСЊС‚Рµ РІ .env РёР»Рё Р·Р°РїСѓСЃС‚РёС‚Рµ:\n"
        '  $env:BOT_TOKEN="РІР°С€_С‚РѕРєРµРЅ"; python main.py'
    )

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    stream=sys.stdout,
)
logger = logging.getLogger(__name__)
if ADMIN_IDS:
    logger.info("Admin panel enabled for Telegram IDs: %s", ADMIN_IDS)
else:
    logger.warning("ADMIN_IDS not set вЂ” /admin will not work. Set ADMIN_IDS in bot/.env")


def _build_fsm_storage():
    if BOT_ENABLE_REDIS_FSM and REDIS_URL:
        try:
            from aiogram.fsm.storage.redis import RedisStorage
        except ImportError as exc:
            raise SystemExit(
                "BOT_ENABLE_REDIS_FSM is enabled, but aiogram Redis storage dependencies are missing. "
                "Install bot/requirements.txt."
            ) from exc

        return RedisStorage.from_url(REDIS_URL)

    if REDIS_URL and not BOT_ENABLE_REDIS_FSM:
        logger.warning("REDIS_URL is set but BOT_ENABLE_REDIS_FSM=0, so FSM remains in memory.")
    else:
        logger.warning("REDIS_URL not set вЂ” using in-memory FSM storage. Recovery after restart is limited.")
    return MemoryStorage()


async def on_startup(bot: Bot) -> None:
    await init_storage()
    if WEBHOOK_URL:
        await bot.set_webhook(
            f"{WEBHOOK_URL.rstrip('/')}{WEBHOOK_PATH}",
            drop_pending_updates=True,
        )
        logger.info("Webhook set: %s%s", WEBHOOK_URL, WEBHOOK_PATH)
    else:
        logger.warning("WEBHOOK_URL not set вЂ” running in long polling mode")


def main() -> None:
    bot = Bot(
        token=BOT_TOKEN,
        default=DefaultBotProperties(parse_mode=ParseMode.HTML),
    )
    fsm_storage = _build_fsm_storage()
    dp = Dispatcher(storage=fsm_storage)
    dp.include_router(start_router)
    dp.include_router(admin_router)
    dp.startup.register(on_startup)

    async def on_shutdown(bot: Bot) -> None:
        if WEBHOOK_URL:
            await bot.delete_webhook()
        await close_storage()
        await fsm_storage.close()
        logger.info("Bot shutdown")

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
