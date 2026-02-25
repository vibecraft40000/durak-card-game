import logging
from aiogram import Router
from aiogram.types import Message
from aiogram.filters import CommandStart

from storage import add_subscriber

router = Router(name="start")
logger = logging.getLogger(__name__)


@router.message(CommandStart())
async def cmd_start(message: Message):
    """При /start: считываем данные пользователя, логируем, приветствие без кнопки."""
    user = message.from_user
    if not user:
        return

    chat_id = message.chat.id if message.chat else user.id
    add_subscriber(chat_id, user.id, user.username)

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
