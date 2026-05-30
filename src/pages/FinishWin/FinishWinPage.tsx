import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { motion } from "framer-motion";
import { getProfile } from "@/shared/api/user";
import { useLanguage } from "@/shared/providers/LanguageProvider";
import { hapticNotification } from "@/shared/lib/telegram";
import { useCountUp } from "@/shared/hooks/useCountUp";
import { getGameState, subscribeGameStore } from "@/store/game.store";

export function FinishWinPage() {
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
    currentUserId && matchResult?.netResults?.length
      ? matchResult.netResults.find((p) => p.userId === currentUserId)?.amount ?? 0
      : 0;

  const displayAmount = useCountUp(myPayout, 800, true);
  const amountText = myPayout >= 0 ? `+${displayAmount.toFixed(2)} ${currency}` : `${displayAmount.toFixed(2)} ${currency}`;

  return (
    <section className="screen finish-screen finish-screen--blur">
      <div className="finish-confetti" aria-hidden>
        {Array.from({ length: 12 }).map((_, i) => (
          <motion.div
            key={i}
            className="finish-confetti__particle"
            initial={{ opacity: 0, scale: 0, x: 0, y: 0 }}
            animate={{
              opacity: [0, 1, 0.8],
              scale: [0, 1],
              x: Math.cos((i / 12) * Math.PI * 2) * 80,
              y: Math.sin((i / 12) * Math.PI * 2) * 80 - 20,
            }}
            transition={{
              duration: 1.2,
              delay: i * 0.04,
              opacity: { times: [0, 0.3, 1] },
            }}
          />
        ))}
      </div>
      <motion.div
        className="result-card result-card--win"
        initial={{ opacity: 0, scale: 0.9 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ type: "spring", stiffness: 300, damping: 24 }}
      >
        <div className="result-card__title">{tr("Игра завершена!", "Гру завершено!")}</div>
        <div className="result-card__message">{tr("Победа", "Перемога")}</div>
        <div className="card__hint">{tr("Чистый результат матча", "Чистий результат матчу")}</div>
        <div className="result-card__amount result-card__amount--plus">{amountText}</div>
        <Link className="button button--primary" to="/play" onClick={() => hapticNotification("success")}>
          {tr("Продолжить", "Продовжити")}
        </Link>
      </motion.div>
    </section>
  );
}
