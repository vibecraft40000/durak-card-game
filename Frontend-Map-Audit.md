## Frontend Map & Audit (Durak Online Mini App)

### 1. Общая карта фронтенда
- **Точка входа**
  - `src/main.tsx`: монтирование React‑приложения, обёртки провайдеров (Theme, Router).
  - `src/app/App.tsx`: инициализация Telegram WebApp (`initTelegramWebApp`), bootstrap Telegram‑авторизации, обработка `start_param` (deep‑link в комнату), рендер `AppRoutes`.
- **Маршрутизация**
  - `src/app/routes.tsx`: `react-router-dom` c двумя layout‑ами:
    - `MainLayout`: маршруты `/play`, `/create`, `/profile`, `/profile/deposit`, `/profile/withdraw` и т.п.
    - `GameLayout`: маршруты `/room/:id`, `/game/:id`, `/game/:id/friends`, `/finish/win`, `/finish/lose`, профильные настройки/история.

### 2. Страницы и layout‑ы
- **Layouts**
  - `MainLayout.tsx`: основной shell Mini App c нижней навигацией (`BottomNav`), контейнером контента и общими стилями.
  - `GameLayout.tsx`: отдельный layout под игровой контекст (стол, комната, финишные экраны).
- **Ключевые страницы**
  - Игровой поток:
    - `PlayPage.tsx`: список доступных комнат/режимов.
    - `CreateGamePage.tsx`: создание стола (настройки колоды/ставки/режима).
    - `GameRoomPage.tsx`: лобби комнаты до старта.
    - `GameTablePage.tsx`: основной игровой стол (руки игроков, стол, действия, таймер).
    - `FinishWinPage.tsx` / `FinishLosePage.tsx`: итоговые экраны.
  - Профиль и финансы:
    - `ProfilePage.tsx`, `DepositPage.tsx`, `WithdrawPage.tsx`, `TransactionsPage.tsx`.
  - Социалка и история:
    - `FriendsPage.tsx`, `FriendsAddPage.tsx`, `GameAddFriendsPage.tsx`, `History*Page.tsx`.
  - Настройки:
    - `SettingsPage.tsx`, `NamePage.tsx`, `CurrencyPage.tsx`, `LanguagePage.tsx`.

### 3. Сетевой слой и WebSocket
- **HTTP API**
  - `shared/api/http.ts`: базовый HTTP‑клиент с авторизацией.
  - `shared/api/auth.ts`: Telegram‑bootstrap, получение access‑token, dev‑режим.
  - Специализированные клиенты: `rooms.ts`, `user.ts`, `deposit.ts`, `withdraw.ts`, `payments.ts`, `config.ts`.
- **WebSocket**
  - `shared/api/ws/types.ts`: строгие типы `ClientWsEvent`, `ServerWsEvent`, `MatchStatePayload`.
  - `shared/api/ws/socket.ts`: один `WsClient` с:
    - подключением по `VITE_WS_URL` (fallback `/ws`);
    - автоматическим reconnect с экспоненциальной паузой;
    - авторизацией через `token` в query;
    - событиями `join_room` / `reconnect` / `sync_request`.
  - `shared/api/ws/events.ts`: глобальный event‑bus для подписки разных частей UI на сообщения WS.

### 4. Состояние игры и игровые сущности
- **Game store**
  - `store/game.store.ts`:
    - хранит `GameState` (roomId, статус `idle/connecting/ready/error`, `matchState`, ошибки, лог активности, reconnect‑флаги, `matchResult`, `interactionLocked`);
    - функции `setGameConnecting`, `setGameReady`, `setGameError/clearGameError`, `setMatchStateIfNewer`, `setReconnectingPlayer`, `setMatchResult`, `setDisplayBalance`;
    - listeners через `subscribeGameStore` (простая самописная шина).
- **Селекторы и типы**
  - `entities/game/model/selectors.ts`: `selectIsMyTurn`, `selectCanAttack/Defend/Take/Pass`, `selectCurrentPhase` — UI зависит от абстракции, не от сырых флагов.
  - `entities/card/types.ts`, `entities/player/types.ts`, `entities/match/types.ts`: модели карт, игроков, матч‑действий.
- **Игровой UI**
  - `features/game/PlayerHandFan.tsx`: визуал и интерактив для руки игрока (фан, drag‑зоны, взаимодействие с `interaction.engine.ts`).
  - `features/game/interaction/*`: hit‑zones, валидация и координатная логика для drag‑drop.
  - `shared/ui/PlayingCard.tsx`: отдельный компонент карты (hand/table/mini/back/placeholder).

### 5. UI‑кит, тема, shell
- **UI компоненты**
  - `shared/ui/Button.tsx`, `Card.tsx`, `Avatar.tsx`, `StateBlocks.tsx`, `ReconnectOverlay.tsx`, `ThemeToggle.tsx`, `BottomNav.tsx`.
  - `shared/ui/Icons.tsx`: SVG‑иконки иконок навигации/действий.
- **Тема и стили**
  - `shared/providers/ThemeProvider.tsx`, `shared/lib/theme.ts`: переключение темы, работа с CSS‑переменными.
  - `styles/tokens.css`, `styles/global.css`: дизайн‑система и глобальные стили.
- **Telegram интеграция**
  - `shared/lib/telegram.ts`: обёртки над WebApp API + haptics (`hapticImpact`, `hapticSelection`, `hapticNotification`), работа со `start_param`.

### 6. Аудит: что хорошо
- **Архитектура**
  - FSD‑подход с разделением `pages / layouts / entities / features / shared / processes / store`.
  - Чёткая граница: стейт игры в `game.store.ts`, селекторы отдельно, UI‑кит изолирован в `shared/ui`.
  - Один WS‑клиент и единый типизированный протокол (`ClientWsEvent` / `ServerWsEvent`).
- **Интеграция с Telegram Mini App**
  - Корректный bootstrap initData + плавный fallback в dev‑режим.
  - Haptic‑обратная связь и обработка `start_param` (deeplink в комнату) — удобно для реального использования.
- **UX игрового стола**
  - Таймер хода (`turnEndsAt` → `secondsLeft`), визуальные подсказки, лог событий, отображение баланса, reconnect‑оверлей.
  - Разделение «игрового» UI (`GameTablePage`, `PlayerHandFan`) и “служебных” экранах (лобби, профиль, история, настройки).

### 7. Аудит: зоны для улучшения
- **Согласованность с новой TMA‑спецификацией**
  - Фронт сейчас ориентирован на более простой матч: одиночный победитель, без явной поддержки 3–4 мест, ничьих и режима “Шулер”.
  - Для поддержки режимов (подкидной/переводной, “Шулер”) потребуется:
    - расширить `MatchStatePayload` (режим, лимиты стола, статус игрока, флаги “шулер активен” и окно репорта);
    - добавить новые типы WS‑событий (`shulerPlay`, `shulerReport`, доп. фазы/этапы);
    - доработать селекторы и UI‑кнопки (кнопка «Замечено», отображение статуса “Шулер”).
- **Управление состоянием**
  - Собственный стор прост и читабелен, но по мере роста функционала (несколько режимов, доп. стейты/таймеры) стоит подумать о:
    - более строгой машине состояний на клиенте (например, xstate/zustand‑машина или хотя бы явный enum экранных состояний);
    - вынесении части логики из `GameTablePage` (она уже довольно длинная) в отдельные хуки/модули.
- **Диагностика и ошибки**
  - Сейчас ошибки в матче отображаются как `gameState.error` + текст в хинте; при расширении протокола стоит унифицировать error‑коды (на бэке уже есть `errorCode`), и на фронте завести отображение по мапе код→локализованное сообщение.
- **Тестирование фронта**
  - Структура кода хорошо подходит под unit/E2E тесты, но в репо пока нет явной фронтовой тест‑обвязки (jest/vitest + Playwright/Cypress). Для продукта уровня TMA имеет смысл добавить хотя бы smoke‑набор:
    - базовый рендер страниц, корректное поведение при моке WS, сценарий входа/создания комнаты/ходов.

