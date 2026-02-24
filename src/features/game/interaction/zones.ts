/**
 * Drop zone detection — spatial mapping of screen coordinates to game zones.
 * Used by Interaction Engine to determine intended action from drag end position.
 */

export type DropZone = "table" | "avatar_take" | "hand";

export type Point = { x: number; y: number };

export type TableRect = DOMRect | { left: number; top: number; right: number; bottom: number };

/**
 * Detects which drop zone the given point lies within.
 * Snap radius: ~30px — points within this distance of table edge count as table.
 */
export function detectZone(point: Point, tableRect: TableRect | null): DropZone | null {
  if (!tableRect) return null;

  const left = tableRect.left;
  const right = tableRect.right;
  const top = tableRect.top;
  const bottom = tableRect.bottom;

  const snapRadius = 30;
  const expandedLeft = left - snapRadius;
  const expandedRight = right + snapRadius;
  const expandedTop = top - snapRadius;
  const expandedBottom = bottom + snapRadius;

  if (
    point.x >= expandedLeft &&
    point.x <= expandedRight &&
    point.y >= expandedTop &&
    point.y <= expandedBottom
  ) {
    return "table";
  }

  return null;
}
