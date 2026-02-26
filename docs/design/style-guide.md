# Дурак Онлайн — Style Guide (MVP)

Краткая документация по текущей дизайн-системе и правилам внедрения.

## 1. Типографика

- Основной стек: `SF Pro Display`, fallback: `Inter`, `system-ui`.
- Размеры:
  - Body: `16px`
  - Label: `14px`
  - Hint: `12px`
  - Heading: `24px`
  - Display: `36px`
- Вес:
  - Regular `400`
  - Semibold `600`
  - Bold `700`

## 2. Цветовая система

Источник токенов: `src/styles/tokens.css`.

- Базовые:
  - `--color-bg-primary`
  - `--color-bg-secondary`
  - `--color-surface`
  - `--color-text-primary`
  - `--color-text-secondary`
- Акцент и статусы:
  - `--color-accent`
  - `--color-success`
  - `--color-error`
- Кнопки:
  - `--color-btn-primary`
  - `--color-btn-secondary`

## 3. Сетка и отступы

- Базовая шкала spacing: `4 / 6 / 8 / 10 / 12 / 14 / 16 / 20 / 24`.
- Контентный контейнер экрана: внутренний padding + safe-area.
- Скругления:
  - Card: `16px`
  - Input: `12px`
  - Chip/Button pill: `999px`

## 4. Состояния UI элементов

- Button:
  - `default`: базовый фон/бордер
  - `active`: акцентный градиент
  - `pressed`: уменьшение opacity/контраст
  - `disabled`: пониженная контрастность + блок клика
- Room card:
  - `normal`
  - `busy` (комната заполнена)
- Game status:
  - `turn`
  - `urgent`
  - `shuler`
  - `disconnected`

## 5. Анимация

- Переходы интерфейса: `180-240ms`, easing `ease-out`.
- Интерактивные элементы (hover/press): `120-160ms`.
- Результат матча: плавное появление и масштабирование суммы (`~220ms`).
- В игре избегать тяжелых анимаций, мешающих чтению карт.

## 6. Адаптивность (mobile first)

- Целевой диапазон: `320px` до `430px`.
- Обязательная поддержка Telegram safe-area (`env(safe-area-inset-*)`).
- Нижняя навигация фиксирована и не должна перекрывать CTA.

## 7. Интеграция с кодом

- Все цвета и размеры задаются через CSS custom properties.
- Новые темы добавлять расширением `tokens.css` с `data-theme` селекторами.
- Текстовые строки экрана должны идти через i18n слой (`ru/uk`).

## 8. Проверочный чек-лист перед релизом

- Тексты: RU/UA без смешения языков на одном экране.
- Валюта: одинаковый формат в `Play / Profile / Game`.
- Контраст и читаемость: нет мелкого текста ниже `12px`.
- Кнопки: есть состояния `active/disabled`.
- Игровой экран: индикатор шулера не перекрывает важные элементы.
