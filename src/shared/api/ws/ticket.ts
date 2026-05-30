import { httpRequest } from "@/shared/api/http";

type IssueWsTicketResponse = {
  ticket?: string;
  expiresInSec?: number;
};

export async function issueWsTicket(roomId: string): Promise<string> {
  const response = await httpRequest<IssueWsTicketResponse>("/api/ws-ticket", {
    method: "POST",
    body: { roomId },
  });

  if (!response.ticket) {
    throw new Error("WS ticket is missing in response");
  }

  return response.ticket;
}
