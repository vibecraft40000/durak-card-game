import { httpRequest } from "./http";

export type WithdrawCreateResponse = {
  transferId: number;
  amount: number;
  asset: string;
  status: string;
};

export async function createWithdraw(amount: number): Promise<WithdrawCreateResponse> {
  return httpRequest<WithdrawCreateResponse>("/api/withdraw/create", {
    method: "POST",
    body: { amount },
  });
}
