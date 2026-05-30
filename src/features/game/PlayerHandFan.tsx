import { memo, useCallback, useRef, useState } from "react";
import { motion } from "framer-motion";
import type { Card } from "@/entities/card/types";
import type { MatchStatePayload } from "@/shared/api/ws/types";
import { validateCardDrop } from "@/features/game/interaction/interaction.engine";
import { canPlayCard } from "@/entities/game/lib/canPlayCard";
import { PlayingCard } from "@/shared/ui/PlayingCard";
import { hapticImpact, hapticSelection } from "@/shared/lib/telegram";

const FAN_SPREAD = 40;
const SPRING = { type: "spring" as const, stiffness: 280, damping: 22 };
const REJECT_SHAKE_PX = 8;
const REJECT_DURATION_MS = 140;

type PlayerHandFanProps = {
  cards: Card[];
  matchState: MatchStatePayload | null;
  currentUserId: string | null;
  canAct: boolean;
  interactionLocked?: boolean;
  tableRectRef: React.RefObject<HTMLDivElement | null>;
  onPlayCard: (cardId: string, action: "attack" | "defend" | "throw" | "shuler_play") => void;
  onReject?: () => void;
};

export const PlayerHandFan = memo(function PlayerHandFan({
  cards,
  matchState,
  currentUserId,
  canAct,
  interactionLocked = false,
  tableRectRef,
  onPlayCard,
  onReject,
}: PlayerHandFanProps) {
  const [draggingId, setDraggingId] = useState<string | null>(null);
  const [rejectId, setRejectId] = useState<string | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const getFanStyle = useCallback(
    (index: number) => {
      const count = Math.max(1, cards.length);
      const centered = index - (count - 1) / 2;
      const spread = count > 1 ? Math.min(58, 22 + count * 4) : 0;
      const step = count > 1 ? spread / (count - 1) : 0;
      return {
        rotate: -spread / 2 + index * step,
        x: centered * (count > 5 ? 50 : 56),
        y: Math.abs(centered) * 8,
      };
    },
    [cards.length],
  );

  const handleDragStart = useCallback(() => {
    hapticSelection();
  }, []);

  const handleDragEnd = useCallback(
    (
      card: Card,
      e: MouseEvent | TouchEvent | PointerEvent,
      info: { point?: { x: number; y: number }; offset: { x: number; y: number } },
    ) => {
      setDraggingId(null);
      const rect = tableRectRef.current?.getBoundingClientRect() ?? null;
      const ev = e as PointerEvent;
      const px = info.point?.x ?? ev.clientX ?? 0;
      const py = info.point?.y ?? ev.clientY ?? 0;
      const result = validateCardDrop({
        card,
        position: { x: px, y: py },
        tableRect: rect,
        matchState,
        currentUserId,
        interactionLocked,
      });

      if (result.outcome === "reject_silent") return;
      if (result.outcome === "reject") {
        setRejectId(result.cardId);
        hapticImpact("light");
        onReject?.();
        setTimeout(() => setRejectId(null), REJECT_DURATION_MS);
        return;
      }

      hapticImpact("medium");
      onPlayCard(result.cardId, result.action);
    },
    [currentUserId, interactionLocked, matchState, onPlayCard, onReject, tableRectRef],
  );

  if (cards.length === 0) {
    return <div className="hand-fan hand-fan--empty" />;
  }

  return (
    <div ref={containerRef} className="hand-fan">
      {cards.map((card, index) => {
        const isDragging = draggingId === card.id;
        const isReject = rejectId === card.id;
        const validation = canPlayCard(card, matchState, currentUserId);
        const canDrag = canAct && !interactionLocked && validation.valid;
        const isPlayable = validation.valid && canAct && !interactionLocked;
        const fanStyle = getFanStyle(index);
        const centeredIndex = index - (cards.length - 1) / 2;

        return (
          <motion.div
            key={card.id}
            layout
            className="hand-fan__card-wrap"
            style={{
              transformOrigin: "bottom center",
              zIndex: isDragging ? 100 : 100 - Math.abs(centeredIndex),
            }}
            initial={false}
            animate={{
              scale: isDragging ? 1.12 : 1,
              y: isDragging ? -24 : fanStyle.y,
              rotate: isDragging ? 0 : fanStyle.rotate,
              x: fanStyle.x,
            }}
            transition={SPRING}
          >
            <motion.div
              drag={canDrag}
              dragConstraints={containerRef}
              dragElastic={0.1}
              onDragStart={() => setDraggingId(card.id)}
              onDragEnd={(e, info) => handleDragEnd(card, e, info)}
              onPointerDown={handleDragStart}
              className="hand-fan__card"
              animate={{
                boxShadow: isDragging
                  ? "0 18px 48px rgba(0,0,0,0.55)"
                  : isReject
                    ? "0 6px 20px rgba(0,0,0,0.35)"
                    : "0 8px 24px rgba(0,0,0,0.42)",
                x: isReject ? [0, -REJECT_SHAKE_PX, REJECT_SHAKE_PX, -REJECT_SHAKE_PX, REJECT_SHAKE_PX, 0] : 0,
              }}
              transition={
                isReject
                  ? { duration: REJECT_DURATION_MS / 1000 }
                  : { type: "spring", stiffness: 340, damping: 24 }
              }
            >
              <PlayingCard rank={card.rank} suit={card.suit} variant="hand" dimmed={!isPlayable} />
            </motion.div>
          </motion.div>
        );
      })}
    </div>
  );
});
