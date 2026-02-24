import { httpRequest } from "./http";

export type DepositCreateResponse = {
  invoiceId: number;
  invoiceUrl: string;
  amount: number;
  status: string;
};

export async function createDepositInvoice(amount: number): Promise<DepositCreateResponse> {
  return httpRequest<DepositCreateResponse>("/api/deposit/create", {
    method: "POST",
    body: { amount },
  });
}
