import { httpRequest } from "@/shared/api/http";

export type HistoryItem = {
  matchId: string;
  stake: number;
  /** Gross payout credited in settlement, if any. */
  payout: number;
  /** Net result after stake and commission; preferred for UI. */
  profit: number;
  result: "win" | "loss";
  createdAt: string;
};

type HistoryResponse = {
  items: HistoryItem[];
  total: number;
};

export type HistoryCalendarDay = {
  date: string;
  games: number;
  profit: number;
};

type HistoryCalendarResponse = {
  items: HistoryCalendarDay[];
};

type HistoryQuery = {
  limit?: number;
  offset?: number;
  from?: string;
  to?: string;
};

export async function getHistory(query: HistoryQuery = {}): Promise<HistoryResponse> {
  const params = new URLSearchParams();
  if (query.limit != null) params.set("limit", String(query.limit));
  if (query.offset != null) params.set("offset", String(query.offset));
  if (query.from) params.set("from", query.from);
  if (query.to) params.set("to", query.to);
  const suffix = params.toString() ? `?${params.toString()}` : "";
  const response = await httpRequest<HistoryResponse>(`/api/history${suffix}`);
  return {
    items: Array.isArray(response.items) ? response.items : [],
    total: typeof response.total === "number" ? response.total : 0,
  };
}

export async function getHistoryCalendar(month: string): Promise<HistoryCalendarDay[]> {
  const params = new URLSearchParams({ month });
  const response = await httpRequest<HistoryCalendarResponse>(`/api/history/calendar?${params.toString()}`);
  return Array.isArray(response.items) ? response.items : [];
}
