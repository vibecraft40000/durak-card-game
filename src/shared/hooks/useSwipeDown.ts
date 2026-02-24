import { useCallback, useRef } from "react";

const SWIPE_THRESHOLD_PX = 60;

/**
 * Returns onTouchStart/onTouchEnd handlers for swipe-down gesture.
 * Calls onSwipeDown when user swipes down by at least SWIPE_THRESHOLD_PX.
 */
export function useSwipeDown(onSwipeDown: () => void) {
  const startY = useRef(0);

  const onTouchStart = useCallback((e: React.TouchEvent) => {
    startY.current = e.touches[0].clientY;
  }, []);

  const onTouchEnd = useCallback(
    (e: React.TouchEvent) => {
      const endY = e.changedTouches[0].clientY;
      const deltaY = endY - startY.current;
      if (deltaY >= SWIPE_THRESHOLD_PX) {
        onSwipeDown();
      }
    },
    [onSwipeDown],
  );

  return { onTouchStart, onTouchEnd };
}
