import logging

from aiogram import Bot, Router
from aiogram.filters import CommandStart
from aiogram.types import (
    CallbackQuery,
    InlineKeyboardButton,
    InlineKeyboardMarkup,
    Message,
    WebAppInfo,
)

from config import CHANNEL_INVITE_LINK, REQUIRED_CHANNEL_ID, WEBAPP_URL
from storage import add_subscriber

router = Router(name="start")
logger = logging.getLogger(__name__)

# When subscription gate is disabled, show full welcome and "Играть" immediately.
START_MESSAGE = """🃏 <b>Durak Online — новый проект от сообщества @CodePridurka</b>

Добро пожаловать в мир азарта, стратегии и больших побед.

Чтобы получать новости и иметь доступ к игре, подпишись на наш канал:
👉 {channel_link}

После подписки нажми «Проверить подписку», затем откроется кнопка «Играть».

👋 <b>Добро пожаловать в Durak Online!</b>

Готов проверить свою удачу, обыграть соперников и занять место среди лучших игроков?
Твой путь начинается прямо сейчас. Открывай Mini App через кнопку ниже и заходи в игру!

📢 <b>Наш канал:</b> {channel_link}
Там новости, обновления и анонсы.

🚀 <b>ВНИМАНИЕ: Beta-Test</b>
Сейчас игра находится на стадии активного бета-тестирования.

⚠️ <b>Важная информация:</b>
Во время бета-теста возможны изменения баланса, механик и игровых систем.

🐛 <b>Нашли баг или есть идея?</b>
Напиши создателю: @killapanic

Удачи за столом! ♠️♥️♣️♦️"""

NOT_SUBSCRIBED_MESSAGE = """🃏 <b>Durak Online</b>

Чтобы получить доступ к игре и следить за новостями проекта, нужно подписаться на наш канал:

👉 {channel_link}

<b>Что делать:</b>
1. Нажми кнопку «Подписаться» и перейди в канал.
2. Подпишись на канал.
3. Вернись сюда и нажми «Проверить подписку».
4. После успешной проверки появится кнопка «Играть» — откроется Mini App."""


def _is_subscription_required() -> bool:
    return bool(REQUIRED_CHANNEL_ID and REQUIRED_CHANNEL_ID.strip())


async def check_channel_membership(bot: Bot, telegram_user_id: int) -> bool:
    if not _is_subscription_required():
        return True
    try:
        member = await bot.get_chat_member(chat_id=REQUIRED_CHANNEL_ID.strip(), user_id=telegram_user_id)
        return member.status in ("creator", "administrator", "member", "restricted")
    except Exception as e:
        logger.warning("check_channel_membership failed: %s", e)
        return False


def build_play_keyboard() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="Играть", web_app=WebAppInfo(url=WEBAPP_URL))],
        ]
    )


def build_subscribe_keyboard() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="Подписаться", url=CHANNEL_INVITE_LINK)],
            [InlineKeyboardButton(text="Проверить подписку", callback_data="check_subscription")],
        ]
    )


@router.message(CommandStart())
async def cmd_start(message: Message) -> None:
    user = message.from_user
    if not user:
        return

    chat_id = message.chat.id if message.chat else user.id
    await add_subscriber(chat_id, user.id, user.username)

    user_info = {
        "id": user.id,
        "username": user.username,
        "first_name": user.first_name,
        "last_name": user.last_name,
        "language_code": user.language_code,
    }
    logger.info("User /start: %s", user_info)

    if _is_subscription_required():
        is_member = await check_channel_membership(message.bot, user.id)
        if not is_member:
            await message.answer(
                NOT_SUBSCRIBED_MESSAGE.format(channel_link=CHANNEL_INVITE_LINK),
                reply_markup=build_subscribe_keyboard(),
                disable_web_page_preview=True,
            )
            return
        text = START_MESSAGE.format(channel_link=CHANNEL_INVITE_LINK)
        await message.answer(
            text,
            reply_markup=build_play_keyboard(),
            disable_web_page_preview=True,
        )
        return

    await message.answer(
        START_MESSAGE.format(channel_link=CHANNEL_INVITE_LINK),
        reply_markup=build_play_keyboard(),
        disable_web_page_preview=True,
    )


@router.callback_query(lambda c: c.data == "check_subscription")
async def callback_check_subscription(callback: CallbackQuery) -> None:
    user = callback.from_user
    if not user:
        await callback.answer()
        return

    if not _is_subscription_required():
        await callback.answer("Доступ открыт.")
        return

    is_member = await check_channel_membership(callback.bot, user.id)
    if is_member:
        text = START_MESSAGE.format(channel_link=CHANNEL_INVITE_LINK)
        try:
            await callback.message.edit_text(
                text,
                reply_markup=build_play_keyboard(),
            )
        except Exception:
            await callback.message.answer(
                text,
                reply_markup=build_play_keyboard(),
                disable_web_page_preview=True,
            )
        await callback.answer("Подписка подтверждена! Нажимай «Играть».", show_alert=True)
    else:
        await callback.answer("Пока вы не подписаны на канал. Подпишитесь и нажмите снова.", show_alert=True)
