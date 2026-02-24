# Game State Machine Spec

Формальная спецификация переходов фаз и разрешённых действий для Durak game engine.

## Фазы (TurnState)

| Phase | Описание |
|-------|----------|
| `attack` | Атакующий ходит картой (или подкидывает) |
| `defend` | Защитник отбивается или забирает |

## Переходы

| Current Phase | Action | Next Phase | Side Effects |
|---------------|--------|------------|--------------|
| attack | `play_card` | defend | TurnPlayerID → defender, set TurnEndsAt |
| defend | `beat_card` | attack | TurnPlayerID → attacker, set TurnEndsAt |
| defend | `take` | attack | transfer table to defender, swap roles, refill hands, set TurnEndsAt |
| attack | `pass` | attack | clear table, refill, swap roles, set TurnEndsAt |

## Условия

### play_card (attack)
- `TurnState == attack`
- `TurnPlayerID == AttackerID`
- Card in hand
- Round limit not exceeded (len(table)/2 < defender hand size)
- Attack rank allowed (first attack: any; подкидывание: rank on table)

### beat_card (defend)
- `TurnState == defend`
- `TurnPlayerID == DefenderID`
- Card in hand
- len(TableCards) odd (есть неприкрытая атака)
- Defense card beats attack card (same suit + higher rank, or trump)

### take (defend)
- `TurnState == defend`
- `TurnPlayerID == DefenderID`

### pass (attack)
- `TurnState == attack`
- `TurnPlayerID == AttackerID`
- len(TableCards) > 0 and even (все пары отбиты)

## Throwing (подкидывание) — placeholder

Будущая фаза: атакующий и «подкидчики» могут подкидывать карты того же ранга, что на столе.

| Current | Action | Next | Side Effects |
|---------|--------|------|--------------|
| attack | `throw_card` | attack/defend | add to table, round limit check |

## Timeout Actions

При `now > TurnEndsAt`:

| Phase | Auto-action |
|-------|-------------|
| defend | `take` |
| attack, table has pairs | `pass` |
| attack, empty table or no cards | `attack` first card / `pass` |

## Version

Версия инкрементируется при каждом успешном действии. Клиент обязан отправлять `expectedVersion` для optimistic locking.
