import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { getFriends, removeFriend, type FriendEntry } from "@/shared/api/friends";
import { getProfile } from "@/shared/api/user";
import { BackIcon, TrashIcon, UsersIcon } from "@/shared/ui/Icons";
import { ConfirmModal } from "@/shared/ui/StateBlocks";

function friendName(friend: FriendEntry) {
  const profile = friend.friend;
  if (!profile) {
    return "Игрок";
  }
  return (
    profile.display_name ||
    (profile.username ? `@${profile.username}` : [profile.first_name, profile.last_name].filter(Boolean).join(" ")) ||
    "Игрок"
  );
}

function friendAvatarLetter(friend: FriendEntry) {
  const label = friendName(friend);
  return label.slice(0, 1).toUpperCase();
}

function otherUserId(friend: FriendEntry) {
  return friend.friend?.id ?? friend.friendId ?? friend.userId;
}

export function FriendsPage() {
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
      setError("Не удалось загрузить друзей.");
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
        <h1 className="page-header__title">Друзья</h1>
        <Link className="icon-button" to="/profile/friends/add">
          <UsersIcon size={17} />
        </Link>
      </div>

      {isLoading && <div className="card__hint">Загрузка друзей...</div>}
      {error && <div className="card__hint card__hint--error">{error}</div>}

      {!isLoading && !error && (
        <div className="list">
          {friends.length === 0 && <div className="card__hint">Список друзей пуст.</div>}
          {friends.map((friend) => {
            const id = otherUserId(friend);
            const isSelected = selectedFriendId === id;
            const name = friendName(friend);
            const isAccepted = friend.status === "accepted";

            return (
              <div className="friend-row-wrap" key={friend.id}>
                <button
                  className={`friend-row ${isAccepted ? "friend-row--online" : ""} ${
                    isSelected ? "friend-row--selected" : ""
                  }`}
                  type="button"
                  onClick={() => setSelectedFriendId(id)}
                >
                  <div className="friend-row__avatar">{friendAvatarLetter(friend)}</div>
                  <span>{name}</span>
                  {isAccepted && <span className="friend-row__dot" />}
                </button>
                {isSelected && (
                  <button
                    className="friend-row__delete"
                    type="button"
                    onClick={() => setFriendToDelete(id)}
                  >
                    <TrashIcon size={18} />
                  </button>
                )}
              </div>
            );
          })}
        </div>
      )}

      <div className="card card--compact">
        <div className="card__label">Реферальная ссылка</div>
        <div className="card__hint card__hint--mono">{refLink}</div>
      </div>

      <ConfirmModal
        isOpen={Boolean(friendToDelete)}
        title="Удалить друга?"
        message="Вы уверены, что хотите удалить пользователя из друзей?"
        confirmLabel={isRemoving ? "Удаление..." : "Да"}
        cancelLabel="Нет"
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
              setError("Не удалось удалить друга.");
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
