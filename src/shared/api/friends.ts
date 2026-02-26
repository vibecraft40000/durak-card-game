import { httpRequest } from "@/shared/api/http";

export type FriendUser = {
  id: string;
  username?: string;
  display_name?: string;
  first_name?: string;
  last_name?: string;
  photo_url?: string;
};

export type FriendEntry = {
  id: string;
  userId: string;
  friendId: string;
  status: "pending" | "accepted" | "blocked";
  isOnline?: boolean;
  createdAt: string;
  friend?: FriendUser;
};

type FriendsResponse = {
  friends: FriendEntry[];
};

type RequestsResponse = {
  requests: FriendEntry[];
};

export async function getFriends(): Promise<FriendEntry[]> {
  const response = await httpRequest<FriendsResponse>("/api/friends");
  return Array.isArray(response.friends) ? response.friends : [];
}

export async function getFriendRequests(): Promise<FriendEntry[]> {
  const response = await httpRequest<RequestsResponse>("/api/friends/requests");
  return Array.isArray(response.requests) ? response.requests : [];
}

export async function sendFriendRequest(friendId: string): Promise<void> {
  await httpRequest<void>("/api/friends/request", {
    method: "POST",
    body: { friendId },
  });
}

export async function acceptFriendRequest(requestId: string): Promise<void> {
  await httpRequest<void>("/api/friends/accept", {
    method: "POST",
    body: { requestId },
  });
}

export async function removeFriend(friendId: string): Promise<void> {
  await httpRequest<void>("/api/friends/remove", {
    method: "POST",
    body: { friendId },
  });
}
