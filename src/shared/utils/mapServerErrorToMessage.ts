export type ServerErrorLike = {
  code?: string;
  reason?: string;
  message?: string;
};

export function mapServerErrorToMessage(err?: ServerErrorLike): { text: string; code?: string } {
  if (!err) {
    return { text: "Неизвестная ошибка сервера" };
  }

  const code = err.code || err.reason || (err as any).type;

  switch (code) {
    case "expectedVersion":
      return {
        text: "Состояние устарело. Обновите состояние комнаты.",
        code: "expectedVersion",
      };
    case "kicked":
      return {
        text: "Вы были исключены из комнаты.",
        code: "kicked",
      };
    case "notAllowed":
    case "forbidden":
      return {
        text: "Действие запрещено сервером.",
        code: "notAllowed",
      };
    case "insufficientFunds":
      return {
        text: "Недостаточно средств.",
        code: "insufficientFunds",
      };
    case "internalError":
      return {
        text: "Внутренняя ошибка сервера, попробуйте позже.",
        code: "internalError",
      };
    default:
      return {
        text: err.message || "Ошибка сервера.",
        code,
      };
  }
}

