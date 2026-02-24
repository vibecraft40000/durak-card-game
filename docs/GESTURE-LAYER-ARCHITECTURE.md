# Gesture Layer Architecture

4-layer model: Gesture → Interaction Engine → Animation → Network.

## Flow

```
Card.tsx (PlayerHandFan)
   ↓ onDragEnd
validateCardDrop({ card, position, tableRect, matchState, currentUserId, interactionLocked })
   ↓
DropResult: accept | reject | reject_silent
   ↓ ACCEPT
onPlayCard(cardId, action) → wsClient.send(make_move)
   ↓ REJECT
Shake animation + haptic
   ↓ REJECT_SILENT
No feedback (interactionLocked)
```

## Files

| File | Responsibility |
|------|----------------|
| `zones.ts` | detectZone(position, tableRect) — spatial mapping |
| `validation.ts` | isValidMove(card, zone, matchState, currentUserId) — mirrors server rules |
| `interaction.engine.ts` | validateCardDrop(ctx) — central decision |

## Input Lock

`interactionLocked` from game store. When true → REJECT_SILENT (no shake, no dispatch).
