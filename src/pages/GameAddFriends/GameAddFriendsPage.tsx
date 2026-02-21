import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { BackIcon } from "@/shared/ui/Icons";
import { AppCard } from "@/shared/ui/Card";
import { AppButton } from "@/shared/ui/Button";
import { hapticNotification } from "@/shared/lib/telegram";

const BOT_USERNAME = import.meta.env.VITE_TELEGRAM_BOT_USERNAME ?? "DurakOnlineBot";

export function GameAddFriendsPage() {
  const { id } = useParams<{ id: string }>();
  const [copied, setCopied] = useState(false);
  const shareUrl = `https://t.me/${BOT_USERNAME}?start=room_${id ?? "unknown"}`;

  async function handleShare() {
    const text = `Присоединяйся к игре в дурака!`;
    if (navigator.share) {
      try {
        await navigator.share({ title: "Дурак Онлайн", text, url: shareUrl });
        hapticNotification("success");
        setCopied(true);
      } catch {
        await navigator.clipboard.writeText(shareUrl);
        hapticNotification("success");
        setCopied(true);
      }
    } else {
      await navigator.clipboard.writeText(shareUrl);
      hapticNotification("success");
      setCopied(true);
    }
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <section className="screen">
      <div className="page-header">
        <Link className="icon-button" to={`/game/${id}`}>
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">Пригласить друзей</h1>
        <div className="page-header__spacer" />
      </div>

      <AppCard>
        <div className="card__label">Ссылка для приглашения</div>
        <label className="field">
          <input value={shareUrl} readOnly />
        </label>
        <AppButton variant="primary" type="button" onClick={() => void handleShare()}>
          {copied ? "Скопировано!" : "Поделиться"}
        </AppButton>
      </AppCard>
    </section>
  );
}
