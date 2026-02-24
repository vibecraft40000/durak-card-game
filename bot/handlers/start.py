import logging
from aiogram import Router
from aiogram.types import Message
from aiogram.filters import CommandStart

router = Router(name="start")
logger = logging.getLogger(__name__)


@router.message(CommandStart())
async def cmd_start(message: Message):
    """При /start: считываем данные пользователя, логируем, приветствие без кнопки."""
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
        "Дурак Онлайн. Откройте игру через Menu Button (иконка слева от поля ввода)."
    )
