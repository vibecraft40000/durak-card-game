import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { getFriendRequests, getFriends, removeFriend, type FriendEntry } from "@/shared/api/friends";
import { getReferralSummary, type ReferralStats } from "@/shared/api/referrals";
import { getProfile } from "@/shared/api/user";
import { buildTelegramMiniAppLink } from "@/shared/lib/telegram";
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
  const [incomingRequestsCount, setIncomingRequestsCount] = useState<number | null>(null);

  const [refLink, setRefLink] = useState(buildTelegramMiniAppLink("ref_default"));
  const [refStats, setRefStats] = useState<ReferralStats | null>(null);
  const [referralError, setReferralError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    void loadFriends();
    void loadReferralSummary();
    void loadIncomingRequests();
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

  async function loadIncomingRequests() {
    try {
      const items = await getFriendRequests();
      setIncomingRequestsCount(items.length);
    } catch {
      setIncomingRequestsCount(null);
    }
  }

  async function loadReferralSummary() {
    setReferralError(null);
    try {
      const [profileResponse, referralSummary] = await Promise.all([getProfile(), getReferralSummary(20)]);
      const referralCode = (referralSummary.referralCode || profileResponse.user.referral_code) ?? "ref";
      setRefLink(buildTelegramMiniAppLink(`ref_${referralCode}`));
      setRefStats(referralSummary.stats);
    } catch {
      void getProfile()
        .then((response) => {
          const referralCode = response.user.referral_code ?? "ref";
          setRefLink(buildTelegramMiniAppLink(`ref_${referralCode}`));
        })
        .catch(() => undefined);
      setReferralError(tr("Не удалось загрузить статистику рефералов.", "Не вдалося завантажити статистику рефералів."));
    }
  }

  const selectedFriend = useMemo(
    () => friends.find((friend) => otherUserId(friend) === selectedFriendId) ?? null,
    [friends, selectedFriendId],
  );

  async function copyReferralLink() {
    try {
      await navigator.clipboard.writeText(refLink);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      setReferralError(tr("Не удалось скопировать ссылку.", "Не вдалося скопіювати посилання."));
    }
  }

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

      <div className="card card--compact">
        <div className="card__row">
          <div>
            <div className="card__label">
              {incomingRequestsCount && incomingRequestsCount > 0
                ? tr("Входящие заявки", "Вхідні запити")
                : tr("Добавить друга", "Додати друга")}
            </div>
            <div className="card__hint">
              {incomingRequestsCount && incomingRequestsCount > 0
                ? tr(
                    `Новых заявок: ${incomingRequestsCount}. Откройте этот раздел, чтобы принять их.`,
                    `Нових запитів: ${incomingRequestsCount}. Відкрийте цей розділ, щоб прийняти їх.`,
                  )
                : tr(
                    "Поиск друзей и входящие заявки находятся на следующем экране.",
                    "Пошук друзів та вхідні запити знаходяться на наступному екрані.",
                  )}
            </div>
          </div>
          <Link className="button button--ghost" to="/profile/friends/add">
            {incomingRequestsCount && incomingRequestsCount > 0 ? tr("Открыть", "Відкрити") : tr("Перейти", "Перейти")}
          </Link>
        </div>
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
        <button type="button" className="button" onClick={() => void copyReferralLink()}>
          {copied ? tr("Скопировано", "Скопійовано") : tr("Скопировать ссылку", "Скопіювати посилання")}
        </button>
      </div>

      <div className="card card--compact">
        <div className="card__label">{tr("Реферальная статистика", "Реферальна статистика")}</div>
        {refStats ? (
          <>
            <div className="card__row">
              <span>{tr("Приглашено", "Запрошено")}</span>
              <strong>{refStats.total_invited}</strong>
            </div>
            <div className="card__row">
              <span>{tr("Сыграли хотя бы 1 игру", "Зіграли хоча б 1 гру")}</span>
              <strong>{refStats.active_invited}</strong>
            </div>
            <div className="card__row">
              <span>{tr("Всего игр приглашенных", "Усього ігор запрошених")}</span>
              <strong>{refStats.total_games}</strong>
            </div>
            <div className="card__row">
              <span>{tr("Всего депозитов", "Усього депозитів")}</span>
              <strong>{refStats.total_deposits_usd.toFixed(2)} USD</strong>
            </div>

            {refStats.recent_invites.length > 0 ? (
              <div className="list">
                {refStats.recent_invites.slice(0, 8).map((invite) => {
                  const name =
                    invite.display_name || (invite.username ? `@${invite.username}` : invite.user_id.slice(0, 8));
                  return (
                    <div key={invite.user_id} className="card__row">
                      <span>{name}</span>
                      <span className="card__hint">
                        {invite.games_played} {tr("игр", "ігор")} · {invite.deposits_usd.toFixed(2)} USD
                      </span>
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="card__hint">
                {tr("Пока нет приглашенных пользователей.", "Поки немає запрошених користувачів.")}
              </div>
            )}
          </>
        ) : (
          <div className="card__hint">{tr("Загрузка...", "Завантаження...")}</div>
        )}
        {referralError && <div className="card__hint card__hint--error">{referralError}</div>}
      </div>

      <ConfirmModal
        isOpen={Boolean(friendToDelete)}
        title={tr("Удалить друга?", "Видалити друга?")}
        message={tr(
          "Вы уверены, что хотите удалить пользователя из друзей?",
          "Ви впевнені, що хочете видалити користувача з друзів?",
        )}
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
