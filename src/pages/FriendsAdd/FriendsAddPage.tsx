import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { acceptFriendRequest, getFriendRequests, sendFriendRequest, type FriendEntry } from "@/shared/api/friends";
import { BackIcon } from "@/shared/ui/Icons";

function requesterLabel(item: FriendEntry) {
  const profile = item.friend;
  if (!profile) {
    return item.userId;
  }
  return profile.display_name || (profile.username ? `@${profile.username}` : profile.first_name || item.userId);
}

export function FriendsAddPage() {
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
      setSuccess("Запрос отправлен.");
      setFriendId("");
    } catch {
      setError("Не удалось отправить запрос в друзья.");
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
        <h1 className="page-header__title">Добавить друга</h1>
        <div className="page-header__spacer" />
      </div>

      <div className="card form-grid">
        <div className="card__hint">Введите username (@name) или user id</div>
        <input
          value={friendId}
          onChange={(event) => setFriendId(event.target.value)}
          placeholder="@username"
        />
        {error && <div className="card__hint card__hint--error">{error}</div>}
        {success && <div className="card__hint">{success}</div>}
        <button className="button button--primary" type="button" onClick={() => void handleSendRequest()} disabled={isSubmitting}>
          {isSubmitting ? "Отправка..." : "Отправить запрос"}
        </button>
      </div>

      <div className="card form-grid">
        <div className="card__label">Входящие заявки</div>
        {isLoadingRequests && <div className="card__hint">Загрузка...</div>}
        {!isLoadingRequests && requests.length === 0 && <div className="card__hint">Новых заявок нет.</div>}
        {!isLoadingRequests &&
          requests.map((item) => (
            <div className="card__row" key={item.id}>
              <span>{requesterLabel(item)}</span>
              <button
                type="button"
                className="button"
                disabled={acceptingRequestId !== null}
                onClick={() => void handleAccept(item.userId)}
              >
                {acceptingRequestId === item.userId ? "..." : "Принять"}
              </button>
            </div>
          ))}
      </div>
    </section>
  );
}
