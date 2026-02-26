import { httpRequest } from "@/shared/api/http";

export type ProfileTransactionItem = {
  id: string;
  type: string;
  amount: number;
  status: string;
  match_id?: string;
  created_at: string;
};

type TransactionsResponse = {
  items: ProfileTransactionItem[];
};

export async function getProfileTransactions(limit = 50, offset = 0): Promise<ProfileTransactionItem[]> {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  const response = await httpRequest<TransactionsResponse>(`/api/profile/transactions?${params.toString()}`);
  return Array.isArray(response.items) ? response.items : [];
}
