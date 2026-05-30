import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { acceptFriendRequest, getFriendRequests, sendFriendRequest, type FriendEntry } from "@/shared/api/friends";
import { HttpError } from "@/shared/api/http";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { BackIcon } from "@/shared/ui/Icons";

function requesterLabel(item: FriendEntry, fallback: string) {
  const profile = item.friend;
  if (!profile) {
    return item.userId;
  }
  return profile.display_name || (profile.username ? `@${profile.username}` : profile.first_name || fallback);
}

export function FriendsAddPage() {
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  const [friendId, setFriendId] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [requests, setRequests] = useState<FriendEntry[]>([]);
  const [isLoadingRequests, setIsLoadingRequests] = useState(true);
  const [acceptingRequestId, setAcceptingRequestId] = useState<string | null>(null);

  useEffect(() => {
    void loadRequests();
  }, []);

  async function loadRequests() {
    setIsLoadingRequests(true);
    try {
      const items = await getFriendRequests();
      setRequests(items);
    } catch {
      setRequests([]);
    } finally {
      setIsLoadingRequests(false);
    }
  }

  async function handleSendRequest() {
    const trimmed = friendId.trim();
    if (!trimmed || isSubmitting) {
      return;
    }
    setError(null);
    setSuccess(null);
    setIsSubmitting(true);
    try {
      await sendFriendRequest(trimmed);
      setSuccess(tr("Запрос отправлен.", "Запит надіслано."));
      setFriendId("");
    } catch (err) {
      setError(formatFriendRequestError(err, tr));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleAccept(requestUserId: string) {
    if (!requestUserId || acceptingRequestId) {
      return;
    }
    setAcceptingRequestId(requestUserId);
    try {
      await acceptFriendRequest(requestUserId);
      setRequests((prev) => prev.filter((item) => item.userId !== requestUserId));
    } finally {
      setAcceptingRequestId(null);
    }
  }

  return (
    <section className="screen friends-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile/friends">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">{tr("Добавить друга", "Додати друга")}</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="card form-grid">
        <div className="card__hint">{tr("Введите username (@name) или user id", "Введіть username (@name) або user id")}</div>
        <input value={friendId} onChange={(event) => setFriendId(event.target.value)} placeholder="@username" />
        {error && <div className="card__hint card__hint--error">{error}</div>}
        {success && <div className="card__hint">{success}</div>}
        <button
          className="button button--primary"
          type="button"
          onClick={() => void handleSendRequest()}
          disabled={isSubmitting}
        >
          {isSubmitting ? tr("Отправка...", "Надсилання...") : tr("Отправить запрос", "Надіслати запит")}
        </button>
      </div>

      <div className="card form-grid">
        <div className="card__label">{tr("Входящие заявки", "Вхідні запити")}</div>
        {isLoadingRequests && <div className="card__hint">{tr("Загрузка...", "Завантаження...")}</div>}
        {!isLoadingRequests && requests.length === 0 && (
          <div className="card__hint">{tr("Новых заявок нет.", "Нових запитів немає.")}</div>
        )}
        {!isLoadingRequests &&
          requests.map((item) => (
            <div className="card__row" key={item.id}>
              <span>{requesterLabel(item, tr("Игрок", "Гравець"))}</span>
              <button
                type="button"
                className="button"
                disabled={acceptingRequestId !== null}
                onClick={() => void handleAccept(item.userId)}
              >
                {acceptingRequestId === item.userId ? "..." : tr("Принять", "Прийняти")}
              </button>
            </div>
          ))}
      </div>
    </section>
  );
}

function formatFriendRequestError(
  err: unknown,
  tr: (ru: string, uk: string) => string,
) {
  if (err instanceof HttpError) {
    const message = String(err.responseBody ?? err.message ?? "")
      .trim()
      .toLowerCase();

    if (message.includes("cannot add yourself")) {
      return tr("Нельзя добавить себя в друзья.", "Не можна додати себе в друзі.");
    }
    if (message.includes("user not found")) {
      return tr("Пользователь не найден.", "Користувача не знайдено.");
    }
    if (message.includes("already friends")) {
      return tr("Этот пользователь уже у вас в друзьях.", "Цей користувач уже у вас у друзях.");
    }
    if (message.includes("request already exists")) {
      return tr("Запрос уже отправлен.", "Запит уже надіслано.");
    }
    if (typeof err.responseBody === "string" && err.responseBody.trim()) {
      return err.responseBody.trim();
    }
  }

  return tr("Не удалось отправить запрос в друзья.", "Не вдалося надіслати запит у друзі.");
}
