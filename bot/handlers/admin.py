# Admin handlers for @durakton777_bot — only ADMIN_IDS can use.

import logging
import inspect
from aiogram import Bot, F, Router
from aiogram.filters import Command
from aiogram.types import CallbackQuery, InlineKeyboardButton, InlineKeyboardMarkup, Message
from aiogram.fsm.context import FSMContext
from aiogram.fsm.state import State, StatesGroup

from config import ADMIN_IDS, API_BASE_URL, ADMIN_SECRET
from storage import add_subscriber, get_subscriber_chat_ids

logger = logging.getLogger(__name__)
router = Router(name="admin")


def admin_only(func):
    """
    Декоратор для callback/message-хендлеров админки.
    - Проверяет, что from_user.id есть в ADMIN_IDS.
    - Оставляет только те kwargs, которые реально есть в сигнатуре func
      (Aiogram передаёт много служебных аргументов: dispatcher, event_from_user и т.д.).
    """

    sig = inspect.signature(func)
    accepted_kwargs = set(sig.parameters.keys())

    async def wrapper(event, *args, **kwargs):
        user_id = getattr(event.from_user, "id", None) if getattr(event, "from_user", None) else None
        if not ADMIN_IDS:
            logger.warning("ADMIN_IDS пустой — задайте в .env (например ADMIN_IDS=5521738246)")
            if hasattr(event, "answer"):
                await event.answer("Админка отключена: не задан ADMIN_IDS в .env")
            return
        if user_id not in ADMIN_IDS:
            logger.info("Отказ в админ-хендлере для user_id=%s (ожидаются %s)", user_id, ADMIN_IDS)
            if hasattr(event, "answer"):
                if isinstance(event, CallbackQuery):
                    await event.answer("Доступ запрещён.", show_alert=True)
                else:
                    await event.answer("Доступ запрещён.")
            return

        # Оставляем только те kwargs, которые есть в сигнатуре целевой функции.
        filtered_kwargs = {k: v for k, v in kwargs.items() if k in accepted_kwargs}
        return await func(event, *args, **filtered_kwargs)

    return wrapper


class BroadcastStates(StatesGroup):
    waiting_text = State()
    confirm = State()


@router.message(Command("admin"))
async def cmd_admin(message: Message, state: FSMContext) -> None:
    """Всегда отвечаем на /admin: либо панель, либо причина отказа + как исправить."""
    await state.clear()
    if not message.from_user:
        await message.answer("Не удалось определить отправителя. Пишите /admin в личку боту.")
        return
    user_id = message.from_user.id

    # 1) ADMIN_IDS не задан — показываем как включить админку
    if not ADMIN_IDS:
        logger.warning("ADMIN_IDS пустой — пользователь %s не видит админку", user_id)
        help_text = (
            "⚠️ <b>Админка не включена</b>\n\n"
            "В папке <code>bot</code> создайте или откройте файл <code>.env</code> и добавьте строку:\n"
            "<code>ADMIN_IDS=ВАШ_TELEGRAM_ID</code>\n\n"
            f"Ваш Telegram ID: <b>{user_id}</b>\n"
            "Значит нужно: <code>ADMIN_IDS=" + str(user_id) + "</code>\n\n"
            "Узнать свой ID можно у бота @userinfobot. После правки перезапустите бота."
        )
        await message.answer(help_text)
        return

    # 2) Пользователь не в списке админов — показываем его ID и как добавить
    if user_id not in ADMIN_IDS:
        logger.info("Отказ в /admin: user_id=%s не в %s", user_id, ADMIN_IDS)
        help_text = (
            "🚫 <b>Доступ запрещён</b>\n\n"
            f"Ваш Telegram ID: <b>{user_id}</b>\n"
            "Сейчас в админах только: " + ", ".join(str(x) for x in ADMIN_IDS) + "\n\n"
            "Чтобы войти: в <code>bot/.env</code> измените строку на:\n"
            "<code>ADMIN_IDS=" + str(user_id) + "</code>\n"
            "или через запятую: <code>ADMIN_IDS=123,456," + str(user_id) + "</code>\n\n"
            "После сохранения файла перезапустите бота."
        )
        await message.answer(help_text)
        return

    # 3) Доступ есть — показываем панель (с защитой от ошибок)
    try:
        count = len(get_subscriber_chat_ids())
    except Exception as e:
        logger.exception("get_subscriber_chat_ids: %s", e)
        count = 0
    text = (
        "<b>Панель администратора</b>\n\n"
        f"Подписчиков (чат-ов с /start): <b>{count}</b>\n\n"
        "Выберите действие:"
    )
    keyboard = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="📊 Статистика", callback_data="admin:stats")],
        [InlineKeyboardButton(text="👥 Пользователи", callback_data="admin:users")],
        [InlineKeyboardButton(text="💰 Заявки на вывод", callback_data="admin:withdrawals")],
        [InlineKeyboardButton(text="📢 Рассылка", callback_data="admin:broadcast")],
        [InlineKeyboardButton(text="❌ Закрыть", callback_data="admin:close")],
    ])
    await message.answer(text, reply_markup=keyboard)


@router.callback_query(F.data == "admin:close")
@admin_only
async def admin_close(cb: CallbackQuery, state: FSMContext) -> None:
    await state.clear()
    await cb.message.delete()
    await cb.answer()


@router.callback_query(F.data == "admin:withdrawals")
@admin_only
async def admin_withdrawals(cb: CallbackQuery, bot: Bot) -> None:
    """Показать последние заявки на вывод из API бэкенда."""
    if not API_BASE_URL or not ADMIN_SECRET:
        await cb.message.edit_text(
            "Задайте API_BASE_URL и ADMIN_SECRET в .env для просмотра заявок на вывод."
        )
        await cb.answer()
        return
    try:
        import aiohttp
        async with aiohttp.ClientSession() as session:
            async with session.get(
                f"{API_BASE_URL.rstrip('/')}/admin/withdrawals",
                params={"limit": 20},
                headers={"X-Admin-Secret": ADMIN_SECRET},
                timeout=aiohttp.ClientTimeout(total=8),
            ) as resp:
                if resp.status != 200:
                    await cb.message.edit_text("Не удалось загрузить заявки (ошибка API).")
                    await cb.answer()
                    return
                data = await resp.json()
    except Exception as e:
        logger.warning("Admin withdrawals API error: %s", e)
        await cb.message.edit_text("API бэкенда недоступен.")
        await cb.answer()
        return
    withdrawals = data.get("withdrawals") or []
    if not withdrawals:
        text = "Заявок на вывод пока нет."
    else:
        lines = ["<b>Последние выводы средств</b>\n"]
        for w in withdrawals[:15]:
            name = (w.get("display_name") or w.get("username") or "—").strip() or "—"
            username = (w.get("username") or "").strip()
            uname = f"@{username}" if username else "—"
            amount = w.get("amount", 0)
            created = (w.get("created_at") or "")[:16].replace("T", " ")
            lines.append(f"• {name} {uname} — <b>{amount:.2f} USD</b> ({created})")
        text = "\n".join(lines)
        if len(withdrawals) > 15:
            text += f"\n\n… и ещё {len(withdrawals) - 15}"
    keyboard = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="◀️ Назад", callback_data="admin:back")],
    ])
    await cb.message.edit_text(text, reply_markup=keyboard)
    await cb.answer()


@router.callback_query(F.data == "admin:users")
@admin_only
async def admin_users(cb: CallbackQuery, bot: Bot) -> None:
    """Список пользователей из бэкенда (последние N)."""
    if not API_BASE_URL or not ADMIN_SECRET:
        await cb.message.edit_text(
            "Задайте API_BASE_URL и ADMIN_SECRET в .env, чтобы смотреть список пользователей."
        )
        await cb.answer()
        return
    try:
        import aiohttp

        async with aiohttp.ClientSession() as session:
            async with session.get(
                f"{API_BASE_URL.rstrip('/')}/admin/users",
                params={"offset": 0, "limit": 50},
                headers={"X-Admin-Secret": ADMIN_SECRET},
                timeout=aiohttp.ClientTimeout(total=8),
            ) as resp:
                if resp.status != 200:
                    await cb.message.edit_text("Не удалось загрузить пользователей (ошибка API).")
                    await cb.answer()
                    return
                data = await resp.json()
    except Exception as e:
        logger.warning("Admin users API error: %s", e)
        await cb.message.edit_text("API бэкенда недоступен.")
        await cb.answer()
        return

    users = data.get("users") or []
    total = data.get("total") or len(users)
    if not users:
        text = "Пользователей пока нет."
    else:
        lines = [f"<b>Пользователи</b> (показаны последние {len(users)} из {total})\n"]
        for u in users:
            first = (u.get("first_name") or "").strip()
            last = (u.get("last_name") or "").strip()
            full = (first + " " + last).strip()
            display_name = (u.get("display_name") or "").strip()
            username = (u.get("username") or "").strip()
            uname = f"@{username}" if username else ""

            # Если в display_name сгенерированный Dev xxxx — предпочитаем реальное имя/username.
            if not full:
                if display_name and not display_name.lower().startswith("dev "):
                    full = display_name
                elif username:
                    full = ""

            if uname and full:
                name = f"{full} {uname}"
            elif uname:
                name = uname
            else:
                name = full or "Без имени"

            username = (u.get("username") or "").strip()
            lang = (u.get("language") or "ru").upper()
            created = (u.get("created_at") or "")[:10]
            line = f"• {name} — язык {lang}, с {created or 'неизв.'}"
            lines.append(line)
        text = "\n".join(lines)

    keyboard = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="◀️ Назад", callback_data="admin:back")],
    ])
    await cb.message.edit_text(text, reply_markup=keyboard)
    await cb.answer()


@router.callback_query(F.data == "admin:back")
@admin_only
async def admin_back(cb: CallbackQuery, state: FSMContext) -> None:
    """Вернуться в главное меню админки."""
    await state.clear()
    count = len(get_subscriber_chat_ids())
    text = (
        "<b>Панель администратора</b>\n\n"
        f"Подписчиков (чат-ов с /start): <b>{count}</b>\n\n"
        "Выберите действие:"
    )
    keyboard = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="📊 Статистика", callback_data="admin:stats")],
        [InlineKeyboardButton(text="👥 Пользователи", callback_data="admin:users")],
        [InlineKeyboardButton(text="💰 Заявки на вывод", callback_data="admin:withdrawals")],
        [InlineKeyboardButton(text="📢 Рассылка", callback_data="admin:broadcast")],
        [InlineKeyboardButton(text="❌ Закрыть", callback_data="admin:close")],
    ])
    await cb.message.edit_text(text, reply_markup=keyboard)
    await cb.answer()


@router.callback_query(F.data == "admin:stats")
@admin_only
async def admin_stats(cb: CallbackQuery, bot: Bot) -> None:
    count = len(get_subscriber_chat_ids())
    text = f"<b>Статистика</b>\n\nПодписчиков: <b>{count}</b>"
    if API_BASE_URL and ADMIN_SECRET:
        try:
            import aiohttp
            async with aiohttp.ClientSession() as session:
                async with session.get(
                    f"{API_BASE_URL.rstrip('/')}/admin/stats",
                    headers={"X-Admin-Secret": ADMIN_SECRET},
                    timeout=aiohttp.ClientTimeout(total=5),
                ) as resp:
                    if resp.status == 200:
                        data = await resp.json()
                        text += f"\n\nПользователей в БД: <b>{data.get('users_count', '—')}</b>"
                        if "games_today" in data:
                            text += f"\nИгр за сегодня: <b>{data.get('games_today')}</b>"
        except Exception as e:
            logger.warning("Admin stats API error: %s", e)
            text += "\n\n(API бэкенда недоступен)"
    else:
        text += "\n\n(Для данных с бэкенда задайте API_BASE_URL и ADMIN_SECRET в .env)"
    keyboard = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="◀️ Назад", callback_data="admin:back")],
    ])
    await cb.message.edit_text(text, reply_markup=keyboard)
    await cb.answer()


@router.callback_query(F.data == "admin:broadcast")
@admin_only
async def admin_broadcast_start(cb: CallbackQuery, state: FSMContext) -> None:
    await state.set_state(BroadcastStates.waiting_text)
    await cb.message.edit_text(
        "Введите текст рассылки (поддерживается HTML). Отправьте /cancel для отмены."
    )
    await cb.answer()


@router.message(Command("cancel"), BroadcastStates.waiting_text)
@admin_only
async def broadcast_cancel(message: Message, state: FSMContext) -> None:
    await state.clear()
    await message.answer("Рассылка отменена.")
    await cmd_admin(message, state)


@router.message(BroadcastStates.waiting_text, F.text)
@admin_only
async def broadcast_got_text(message: Message, state: FSMContext) -> None:
    text = message.text
    if not text or len(text) > 4000:
        await message.answer("Текст должен быть от 1 до 4000 символов.")
        return
    await state.update_data(broadcast_text=text)
    await state.set_state(BroadcastStates.confirm)
    count = len(get_subscriber_chat_ids())
    keyboard = InlineKeyboardMarkup(inline_keyboard=[
        [InlineKeyboardButton(text="✅ Отправить", callback_data="admin:send_broadcast")],
        [InlineKeyboardButton(text="❌ Отмена", callback_data="admin:close")],
    ])
    await message.answer(
        f"Отправить это сообщение <b>{count}</b> подписчикам?\n\n<blockquote>{text[:200]}{'…' if len(text) > 200 else ''}</blockquote>",
        reply_markup=keyboard,
    )


@router.callback_query(F.data == "admin:send_broadcast", BroadcastStates.confirm)
@admin_only
async def broadcast_confirm(cb: CallbackQuery, state: FSMContext, bot: Bot) -> None:
    data = await state.get_data()
    text = data.get("broadcast_text") or ""
    await state.clear()
    chat_ids = get_subscriber_chat_ids()
    ok, fail = 0, 0
    for cid in chat_ids:
        try:
            await bot.send_message(cid, text)
            ok += 1
        except Exception as e:
            logger.warning("Broadcast to %s failed: %s", cid, e)
            fail += 1
    await cb.message.edit_text(f"Рассылка завершена: доставлено {ok}, ошибок {fail}.")
    await cb.answer()


# Register subscriber on /start (so we can broadcast)
async def register_subscriber_from_message(message: Message) -> None:
    if not message.from_user:
        return
    chat_id = message.chat.id if message.chat else message.from_user.id
    add_subscriber(chat_id, message.from_user.id, message.from_user.username)
