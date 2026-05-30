import { trRuntime } from "@/shared/i18n/runtime";

export type ServerErrorLike = {
  code?: string;
  reason?: string;
  message?: string;
};

export function mapServerErrorToMessage(err?: ServerErrorLike): { text: string; code?: string } {
  if (!err) {
    return { text: trRuntime("Неизвестная ошибка сервера", "Невідома помилка сервера") };
  }

  const code = err.code || err.reason || (err as any).type;
  const message = err.message ?? "";
  const messageLower = message.toLowerCase();

  if (messageLower.includes("cannot report shuler")) {
    return {
      text: trRuntime("Окно жалобы на шулера уже закрыто.", "Вікно скарги на шулера вже закрите."),
      code: "shulerReportWindowClosed",
    };
  }

  if (messageLower.includes("shuler play is not allowed")) {
    return {
      text: trRuntime("Шулер-ход сейчас недоступен.", "Шулер-хід зараз недоступний."),
      code: "shulerPlayDenied",
    };
  }

  switch (code) {
    case "expectedVersion":
    case "VERSION_MISMATCH":
      return {
        text: trRuntime("Состояние устарело. Обновите состояние комнаты.", "Стан застарів. Оновіть стан кімнати."),
        code: "expectedVersion",
      };
    case "kicked":
      return {
        text: trRuntime("Вы были исключены из комнаты.", "Вас було виключено з кімнати."),
        code: "kicked",
      };
    case "notAllowed":
    case "forbidden":
      return {
        text: trRuntime("Действие запрещено сервером.", "Дію заборонено сервером."),
        code: "notAllowed",
      };
    case "insufficientFunds":
      return {
        text: trRuntime("Недостаточно средств.", "Недостатньо коштів."),
        code: "insufficientFunds",
      };
    case "internalError":
      return {
        text: trRuntime("Внутренняя ошибка сервера, попробуйте позже.", "Внутрішня помилка сервера, спробуйте пізніше."),
        code: "internalError",
      };
    case "INVALID_TURN":
      return {
        text: trRuntime("Сейчас не ваш ход.", "Зараз не ваш хід."),
        code: "invalidTurn",
      };
    case "INVALID_CARD":
      return {
        text: trRuntime("Эта карта сейчас не подходит.", "Ця карта зараз не підходить."),
        code: "invalidCard",
      };
    case "INVALID_ACTION":
      return {
        text: trRuntime("Это действие сейчас недоступно.", "Ця дія зараз недоступна."),
        code: "invalidAction",
      };
    case "RATE_LIMIT":
      return {
        text: trRuntime("Слишком много действий. Попробуйте чуть позже.", "Забагато дій. Спробуйте трохи пізніше."),
        code: "rateLimit",
      };
    default:
      return {
        text: message || trRuntime("Ошибка сервера.", "Помилка сервера."),
        code,
      };
  }
}
