import { httpRequest } from "./http";

export type CreatePaymentResponse = {
  directPayLink: string;
  externalId: string;
  amount: number;
};

export async function createPayment(amount: number): Promise<CreatePaymentResponse> {
  return httpRequest<CreatePaymentResponse>("/api/payments/create", {
    method: "POST",
    body: { amount },
  });
}
