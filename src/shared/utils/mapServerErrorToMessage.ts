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

  switch (code) {
    case "expectedVersion":
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
    default:
      return {
        text: err.message || trRuntime("Ошибка сервера.", "Помилка сервера."),
        code,
      };
  }
}
