# Durak Online Mini App — Visual Directions

## Direction 1: Arcade Felt (implemented baseline)
- Mood: clean game-table look, glowing neon accents, readable dark surfaces.
- Palette:
  - Background: `#010a1b` → `#0f1e4d`
  - Surface: `#1f2d74`, `#2e3f9c`
  - Accent: `#808aff`
  - Success: `#35d884`
  - Danger: `#ff6b8e`
- Typography:
  - Display: `SF Pro Display` (fallback: Inter/system)
  - Body: `SF Pro Display`
- Composition:
  - Rounded cards, compact mobile blocks, fixed bottom nav.
  - Table is central visual anchor.

## Direction 2: Casino Glass
- Mood: premium fintech + game blend.
- Palette:
  - Background: deep graphite `#0d111f`
  - Glass surfaces: `rgba(255,255,255,0.08)`
  - Accent: emerald `#27d29b`
  - Warning: amber `#ffbf5a`
- Typography:
  - Display: `SF Pro Display Semibold`
  - Body: `IBM Plex Sans`
- Composition:
  - Frosted panels, sharper spacing rhythm, denser data cards.
  - Motion: smooth fade/slide between screen groups.

## Direction 3: Paper Cards Modern
- Mood: bright casual card game with strong legibility.
- Palette:
  - Background: warm light `#f3f5fb`
  - Surface: `#ffffff`
  - Accent: royal blue `#3f61ff`
  - Danger: `#e54863`
- Typography:
  - Display: `Manrope`
  - Body: `Manrope`
- Composition:
  - High contrast card faces, thin-stroke icons, clean separators.
  - Better for long sessions (reduced visual fatigue).

## Notes
- Three directions are documented for handoff/selection.
- Current codebase uses Direction 1 with mobile-first constraints.
- Direction switch can be implemented via theme token sets without refactoring page logic.
