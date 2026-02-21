import { useState } from "react";
import { Link } from "react-router-dom";
import { BackIcon, TrashIcon, UsersIcon } from "@/shared/ui/Icons";
import { ConfirmModal } from "@/shared/ui/StateBlocks";

export function FriendsPage() {
  const [friends, setFriends] = useState([
    { id: "1", name: "Anastasia", online: false },
    { id: "2", name: "Anastasia", online: true },
    { id: "3", name: "Anastasia", online: false },
    { id: "4", name: "Anastasia", online: true },
    { id: "5", name: "Anastasia", online: false },
    { id: "6", name: "Anastasia", online: true },
  ]);
  const [selectedId, setSelectedId] = useState("3");
  const [friendToDelete, setFriendToDelete] = useState<string | null>(null);

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

      <div className="list">
        {friends.map((friend) => (
          <div className="friend-row-wrap" key={friend.id}>
            <button
              className={`friend-row ${friend.online ? "friend-row--online" : ""} ${
                selectedId === friend.id ? "friend-row--selected" : ""
              }`}
              type="button"
              onClick={() => setSelectedId(friend.id)}
            >
              <div className="friend-row__avatar">{friend.name.slice(0, 1)}</div>
              <span>{friend.name}</span>
              {friend.online && <span className="friend-row__dot" />}
            </button>
            {selectedId === friend.id && (
              <button
                className="friend-row__delete"
                type="button"
                onClick={() => setFriendToDelete(friend.id)}
              >
                <TrashIcon size={18} />
              </button>
            )}
          </div>
        ))}
      </div>

      <div className="card card--compact">
        <div className="card__label">Реферальная ссылка</div>
        <div className="card__hint card__hint--mono">https://t.me/your_bot?start=ref_123</div>
      </div>

      <ConfirmModal
        isOpen={Boolean(friendToDelete)}
        title="Удалить друга?"
        message="Вы уверены, что хотите удалить пользователя из друзей?"
        confirmLabel="Да"
        cancelLabel="Нет"
        onConfirm={() => {
          if (friendToDelete) {
            setFriends((prev) => prev.filter((item) => item.id !== friendToDelete));
            setSelectedId((current) => (current === friendToDelete ? "" : current));
          }
          setFriendToDelete(null);
        }}
        onCancel={() => setFriendToDelete(null)}
      />
    </section>
  );
}
