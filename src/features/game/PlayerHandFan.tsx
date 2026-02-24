import { memo, useCallback, useRef, useState } from "react";
import { motion } from "framer-motion";
import type { Card } from "@/entities/card/types";
import type { MatchStatePayload } from "@/shared/api/ws/types";
import { validateCardDrop } from "@/features/game/interaction/interaction.engine";
import { canPlayCard } from "@/entities/game/lib/canPlayCard";
import { PlayingCard } from "@/shared/ui/PlayingCard";
import { hapticImpact, hapticSelection } from "@/shared/lib/telegram";

/** ANIMATION-SPEC: maxSpread 60°, arc layout */
const FAN_SPREAD = 60;
const SPRING = { type: "spring" as const, stiffness: 280, damping: 22 };
/** Invalid drop: shake 8px, 140ms */
const REJECT_SHAKE_PX = 8;
const REJECT_DURATION_MS = 140;

type PlayerHandFanProps = {
  cards: Card[];
  matchState: MatchStatePayload | null;
  currentUserId: string | null;
  canAct: boolean;
  interactionLocked?: boolean;
  tableRectRef: React.RefObject<HTMLDivElement | null>;
  onPlayCard: (cardId: string, action: "attack" | "defend") => void;
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

  const getFanStyle = useCallback((index: number) => {
    const n = Math.max(1, cards.length);
    const step = n > 1 ? FAN_SPREAD / (n - 1) : 0;
    const rotation = -FAN_SPREAD / 2 + index * step;
    return {
      transformOrigin: "bottom center",
      rotate: rotation,
    };
  }, [cards.length]);

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
    [canAct, matchState, currentUserId, interactionLocked, onPlayCard, onReject, tableRectRef],
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

        return (
          <motion.div
            key={card.id}
            layout
            className="hand-fan__card-wrap"
            style={{
              ...getFanStyle(index),
              zIndex: isDragging ? 100 : index,
            }}
            initial={false}
            animate={{
              scale: isDragging ? 1.12 : 1,
              y: isDragging ? 0 : 0,
              rotate: isDragging ? 0 : getFanStyle(index).rotate,
              x: 0,
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
                    : "0 6px 20px rgba(0,0,0,0.35)",
                x: isReject ? [0, -REJECT_SHAKE_PX, REJECT_SHAKE_PX, -REJECT_SHAKE_PX, REJECT_SHAKE_PX, 0] : 0,
              }}
              transition={
                isReject
                  ? { duration: REJECT_DURATION_MS / 1000 }
                  : { type: "spring", stiffness: 340, damping: 24 }
              }
            >
              <PlayingCard rank={card.rank} suit={card.suit} variant="hand" />
            </motion.div>
          </motion.div>
        );
      })}
    </div>
  );
});
