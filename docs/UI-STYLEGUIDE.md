# UI Style Guide (Telegram Mini App)

## Core Typography
- Primary: `SF Pro Display`
- Fallback: `Inter, system-ui, -apple-system, Segoe UI`
- Base body size: `16px` minimum on interactive text.

## Color System
- Background gradient + radial overlay for depth.
- Surface cards: semi-opaque dark blue layers.
- Accent: high-visibility blue for primary actions.
- Semantic states:
  - Success: green (`win`, active status)
  - Danger: red (`error`, shuler badge)
  - Muted: low-contrast hints

## Spacing and Grid
- Mobile-first width: up to 390px container.
- Vertical rhythm via tokenized spacing (`--space-*`).
- Lists and cards use consistent 8/10/12px gaps.

## Component States
- Buttons: default, active, disabled.
- Bottom nav: active item highlight.
- Game actions: disabled when intent locked / action unavailable.
- Form inputs: default + error hint.

## Motion
- Turn urgency pulse animation for active player.
- Card hover/focus transitions.
- Reconnect overlay and error banners use smooth opacity transitions.

## Accessibility
- Touch targets are >= 40px where possible.
- Text contrast preserved for critical controls.
- Distinct color + text labels for states (not color-only).

## New Screens Updated in This Iteration
- Play flow supports:
  - quick game
  - join by room ID
- Create flow is split into 3 steps:
  - stake
  - game settings
  - shuler mode
- Game table includes:
  - quick chat reactions
  - text chat composer
  - shuler indicator and report action
