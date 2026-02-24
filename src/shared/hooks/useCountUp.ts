import { useEffect, useState } from "react";

/**
 * Animates from 0 to target over durationMs.
 * Used for finish screen amount count-up.
 */
export function useCountUp(
  target: number,
  durationMs: number = 800,
  enabled: boolean = true,
): number {
  const [display, setDisplay] = useState(0);

  useEffect(() => {
    if (!enabled) {
      setDisplay(target);
      return;
    }
    setDisplay(0);
    const start = performance.now();
    const tick = (now: number) => {
      const elapsed = now - start;
      const t = Math.min(1, elapsed / durationMs);
      const eased = 1 - (1 - t) ** 2;
      setDisplay(target * eased);
      if (t < 1) requestAnimationFrame(tick);
    };
    const id = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(id);
  }, [target, durationMs, enabled]);

  return display;
}
