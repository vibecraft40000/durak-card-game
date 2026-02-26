import { httpRequest } from "@/shared/api/http";

export type ReferralInvite = {
  user_id: string;
  username: string;
  display_name: string;
  joined_at: string;
  games_played: number;
  deposits_usd: number;
};

export type ReferralStats = {
  total_invited: number;
  active_invited: number;
  total_games: number;
  total_deposits_usd: number;
  recent_invites: ReferralInvite[];
};

type ReferralSummaryResponse = {
  referralCode: string;
  stats: ReferralStats;
};

export async function getReferralSummary(limit = 20): Promise<ReferralSummaryResponse> {
  return httpRequest<ReferralSummaryResponse>(`/api/referrals/summary?limit=${Math.max(1, Math.min(100, limit))}`);
}
