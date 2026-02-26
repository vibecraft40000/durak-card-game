import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { motion } from "framer-motion";
import { getProfile } from "@/shared/api/user";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { hapticNotification } from "@/shared/lib/telegram";
import { useCountUp } from "@/shared/hooks/useCountUp";
import { getGameState, subscribeGameStore } from "@/store/game.store";

export function FinishLosePage() {
  const [currentUserId, setCurrentUserId] = useState<string | null>(null);
  const currency = "USD";
  const { language } = useLanguage();
  const tr = (ru: string, uk: string) => (language === "uk" ? uk : ru);

  const [matchResult, setMatchResult] = useState(() => getGameState().matchResult ?? null);

  useEffect(() => {
    void getProfile().then((r) => setCurrentUserId(r.user.id));
  }, []);

  useEffect(() => {
    return subscribeGameStore((state) => setMatchResult(state.matchResult));
  }, []);

  const myPayout =
    currentUserId && matchResult?.payouts?.length
      ? matchResult.payouts.find((p) => p.userId === currentUserId)?.amount ?? 0
      : 0;

  const displayAmount = useCountUp(myPayout, 800, true);
  const amountText = myPayout >= 0 ? `+${displayAmount.toFixed(2)} ${currency}` : `${displayAmount.toFixed(2)} ${currency}`;

  return (
    <section className="screen finish-screen finish-screen--blur finish-screen--lose">
      <motion.div
        className="result-card result-card--lose"
        initial={{ opacity: 0, scale: 0.9 }}
        animate={{
          opacity: 1,
          scale: 1,
          x: [0, -4, 4, -2, 2, 0],
        }}
        transition={{
          opacity: { duration: 0.3 },
          scale: { type: "spring", stiffness: 300, damping: 24 },
          x: { duration: 0.4, delay: 0.2 },
        }}
      >
        <div className="result-card__title">{tr("Игра завершена!", "Гру завершено!")}</div>
        <div className="result-card__message">{tr("Поражение", "Поразка")}</div>
        <div className="result-card__amount result-card__amount--minus">{amountText}</div>
        <Link className="button button--primary" to="/play" onClick={() => hapticNotification("warning")}>
          {tr("Продолжить", "Продовжити")}
        </Link>
      </motion.div>
    </section>
  );
}
