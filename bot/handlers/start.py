import logging
from aiogram import Router
from aiogram.types import Message, WebAppInfo, KeyboardButton, ReplyKeyboardMarkup
from aiogram.filters import CommandStart

router = Router(name="start")
logger = logging.getLogger(__name__)


def get_play_keyboard() -> ReplyKeyboardMarkup:
    from config import WEBAPP_URL

    return ReplyKeyboardMarkup(
        keyboard=[
            [
                KeyboardButton(
                    text="Играть",
                    web_app=WebAppInfo(url=WEBAPP_URL),
                )
            ]
        ],
        resize_keyboard=True,
        is_persistent=True,
    )


@router.message(CommandStart())
async def cmd_start(message: Message):
    """При /start: считываем данные пользователя, логируем, показываем кнопку Играть."""
    user = message.from_user
    if not user:
        return

    user_info = {
        "id": user.id,
        "username": user.username,
        "first_name": user.first_name,
        "last_name": user.last_name,
        "language_code": user.language_code,
    }
    logger.info("User /start: %s", user_info)

    await message.answer(
        "Нажмите «Играть», чтобы открыть игру. Ваши данные будут переданы в мини-приложение.",
        reply_markup=get_play_keyboard(),
    )
