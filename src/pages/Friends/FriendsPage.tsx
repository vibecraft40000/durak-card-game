import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { getFriends, removeFriend, type FriendEntry } from "@/shared/api/friends";
import { getProfile } from "@/shared/api/user";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { BackIcon, TrashIcon, UsersIcon } from "@/shared/ui/Icons";
import { ConfirmModal } from "@/shared/ui/StateBlocks";

function friendName(friend: FriendEntry, fallback: string) {
  const profile = friend.friend;
  if (!profile) {
    return fallback;
  }
  return (
    profile.display_name ||
    (profile.username ? `@${profile.username}` : [profile.first_name, profile.last_name].filter(Boolean).join(" ")) ||
    fallback
  );
}

function friendAvatarLetter(label: string) {
  return label.slice(0, 1).toUpperCase();
}

function otherUserId(friend: FriendEntry) {
  return friend.friend?.id ?? friend.friendId ?? friend.userId;
}

export function FriendsPage() {
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  const [friends, setFriends] = useState<FriendEntry[]>([]);
  const [selectedFriendId, setSelectedFriendId] = useState("");
  const [friendToDelete, setFriendToDelete] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isRemoving, setIsRemoving] = useState(false);
  const [refLink, setRefLink] = useState("https://t.me/your_bot?start=ref");

  useEffect(() => {
    void loadFriends();
    void getProfile()
      .then((response) => {
        const bot = import.meta.env.VITE_TELEGRAM_BOT_USERNAME ?? "your_bot";
        const referralCode = response.user.referral_code ?? "ref";
        setRefLink(`https://t.me/${bot}?start=ref_${referralCode}`);
      })
      .catch(() => undefined);
  }, []);

  async function loadFriends() {
    setIsLoading(true);
    setError(null);
    try {
      const list = await getFriends();
      setFriends(list);
      if (list.length > 0) {
        setSelectedFriendId(otherUserId(list[0]));
      }
    } catch {
      setError(tr("Не удалось загрузить друзей.", "Не вдалося завантажити друзів."));
    } finally {
      setIsLoading(false);
    }
  }

  const selectedFriend = useMemo(
    () => friends.find((friend) => otherUserId(friend) === selectedFriendId) ?? null,
    [friends, selectedFriendId],
  );

  return (
    <section className="screen friends-screen">
      <div className="page-header">
        <Link className="icon-button" to="/profile">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">{tr("Друзья", "Друзі")}</h1>
        <Link className="icon-button" to="/profile/friends/add">
          <UsersIcon size={17} />
        </Link>
      </div>

      {isLoading && <div className="card__hint">{tr("Загрузка друзей...", "Завантаження друзів...")}</div>}
      {error && <div className="card__hint card__hint--error">{error}</div>}

      {!isLoading && !error && (
        <div className="list">
          {friends.length === 0 && <div className="card__hint">{tr("Список друзей пуст.", "Список друзів порожній.")}</div>}
          {friends.map((friend) => {
            const id = otherUserId(friend);
            const isSelected = selectedFriendId === id;
            const name = friendName(friend, tr("Игрок", "Гравець"));
            const isOnline = Boolean(friend.isOnline);

            return (
              <div className="friend-row-wrap" key={friend.id}>
                <button
                  className={`friend-row ${isOnline ? "friend-row--online" : ""} ${
                    isSelected ? "friend-row--selected" : ""
                  }`}
                  type="button"
                  onClick={() => setSelectedFriendId(id)}
                >
                  <div className="friend-row__avatar">{friendAvatarLetter(name)}</div>
                  <span>{name}</span>
                  <span className={`friend-row__dot ${isOnline ? "friend-row__dot--online" : "friend-row__dot--offline"}`} />
                </button>
                {isSelected && (
                  <button className="friend-row__delete" type="button" onClick={() => setFriendToDelete(id)}>
                    <TrashIcon size={18} />
                  </button>
                )}
              </div>
            );
          })}
        </div>
      )}

      <div className="card card--compact">
        <div className="card__label">{tr("Реферальная ссылка", "Реферальне посилання")}</div>
        <div className="card__hint card__hint--mono">{refLink}</div>
      </div>

      <ConfirmModal
        isOpen={Boolean(friendToDelete)}
        title={tr("Удалить друга?", "Видалити друга?")}
        message={tr("Вы уверены, что хотите удалить пользователя из друзей?", "Ви впевнені, що хочете видалити користувача з друзів?")}
        confirmLabel={isRemoving ? tr("Удаление...", "Видалення...") : tr("Да", "Так")}
        cancelLabel={tr("Нет", "Ні")}
        onConfirm={() => {
          if (!friendToDelete || isRemoving) {
            return;
          }
          setIsRemoving(true);
          void removeFriend(friendToDelete)
            .then(() => {
              setFriends((prev) => prev.filter((item) => otherUserId(item) !== friendToDelete));
              if (selectedFriend && otherUserId(selectedFriend) === friendToDelete) {
                setSelectedFriendId("");
              }
            })
            .catch(() => {
              setError(tr("Не удалось удалить друга.", "Не вдалося видалити друга."));
            })
            .finally(() => {
              setIsRemoving(false);
              setFriendToDelete(null);
            });
        }}
        onCancel={() => {
          if (!isRemoving) {
            setFriendToDelete(null);
          }
        }}
      />
    </section>
  );
}
