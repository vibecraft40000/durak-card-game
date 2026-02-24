# Animation Spec (Production-Level)

Референсы: Clash Royale (responsiveness), Hearthstone (drag), Marvel Snap (snap clarity).

## Principles

- 60 FPS, GPU: only `transform` + `opacity`
- Never teleport — use FLIP/layout animations on reconcile

---

## Card Idle State

| Param | Value |
|-------|-------|
| Floating | translateY: -2px ↔ 0px |
| Loop | 2–3 sec ease-in-out |
| Shadow | soft, subtle |

---

## Tap (выбор карты)

| Param | Value |
|-------|-------|
| Duration | 120ms |
| Easing | cubic-bezier(0.2, 0.8, 0.2, 1) |
| scale | 1 → 1.06 |
| translateY | 0 → -12px |
| haptic | light impact |

---

## Drag Start

| Param | Value |
|-------|-------|
| Duration | 80ms |
| scale | 1.06 → 1.12 |
| z-index | top layer |
| rotation | follows finger (max ±8°) |
| other cards | opacity 0.7 |

---

## Invalid Drop

| Param | Value |
|-------|-------|
| shake | x: 8px |
| Duration | 140ms |
| haptic | error |

---

## Throw (бросок на стол)

| Param | Value |
|-------|-------|
| Duration | 250ms |
| Easing | cubic-bezier(0.25, 1, 0.5, 1) |
| scale | 1.12 → 1, overshoot 1.02 |
| rotation | → 0° |

---

## CardHand Arc Layout

- maxSpread = 60°
- angleStep = maxSpread / (n - 1)
- For 6 cards: -30°, -18°, -6°, 6°, 18°, 30°
- transform: rotate(angle), translateY(sin(angle) * 20px)
- Z-index: center higher; drag = max

---

## Reconcile (version_mismatch)

- FLIP: record first position → apply new state → invert → animate to final
- framer-motion: `layout` prop on card containers
- Never instant re-mount
