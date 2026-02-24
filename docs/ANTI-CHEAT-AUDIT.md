# Anti-Cheat / Trust Boundary Audit

## Принцип

**Сервер — единственный источник истины.** Клиент может только предложить action; сервер полностью валидирует и применяет или отвергает.

## Что проверяется на сервере

| Проверка | Где | Описание |
|----------|-----|----------|
| TurnPlayerID | engine/rules.go | Действие разрешено только игроку на очереди |
| TurnState | engine/rules.go | Phase must match action (attack → play_card, defend → beat/take) |
| Card in hand | engine/rules.go `popCard` | Карта должна быть в руке; popCard убирает её |
| Attack rank | `canAttackCard` | Первая атака: любая; подкидывание: rank из table |
| Defense beats | `beats()` | Отбивка: та же масть + выше, или козырь |
| Round limit | `roundLimit(state)` | Не больше карт в руке защитника на столе |

## Что НЕ доверяем клиенту

- `action` — нормализуем и валидируем
- `cardId` — проверяем наличие в руке
- `expectedVersion` — проверяем против state.Version

## Граница доверия

```
Client                    Server
  |                          |
  | make_move(action, cardId)|
  | ------------------------>|
  |                          | validate turn
  |                          | validate phase
  |                          | popCard(cardId)  <- card MUST be in hand
  |                          | apply action
  |<-------------------------|
  | game_state / error       |
```

## Rate Limiting

- `make_move`: 30 запросов / 10 сек на пользователя (redis sliding window)

## Exploit Vectors (закрыты)

1. **Карта "из воздуха"** — popCard возвращает false → ErrCardMissing
2. **Чужой ход** — TurnPlayerID check → ErrInvalidTurn
3. **Неверная фаза** — TurnState check → ErrInvalidTurn
4. **Дублирование хода** — actionId idempotency (Redis processed_actions)
5. **Version rollback** — expectedVersion check → ErrVersionMismatch
