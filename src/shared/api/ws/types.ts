import type { Card } from "@/entities/card/types";
import type { MatchActionType } from "@/entities/match/types";
import type { Player } from "@/entities/player/types";

export type ClientWsEvent =
  | {
      type: "join_room";
      payload: { roomId: string };
    }
  | {
      type: "create_room";
      payload: { title: string; stake: number; mode: string; deck: number };
    }
  | {
      type: "make_move";
      payload: {
        roomId: string;
        action: MatchActionType | "attack_card" | "defend_card" | "take_cards" | "pass_turn" | "end_round";
        cardId?: string;
      };
    }
  | {
      type: "ready";
      payload: { roomId: string };
    }
  | {
      type: "confirm_join";
      payload: { roomId: string };
    }
  | {
      type: "send_message";
      payload: { roomId: string; message: string };
    }
  | {
      type: "reconnect";
      payload: { roomId: string };
    };

export type MatchStatePayload = {
  roomId: string;
  matchId?: string;
  players?: Player[];
  tableCards: Card[];
  trumpSuit: string;
  trumpCard?: { id: string; suit: string; rank: string };
  turnPlayerId?: string;
  turnEndsAt?: number;
  winnerPlayerId?: string;
  status: "waiting" | "playing" | "finished";
};

export type ServerWsEvent =
  | { type: "room_update"; payload: Record<string, unknown> }
  | { type: "move_applied"; payload: { roomId: string; matchId: string; playerId: string; action: string; cardId?: string } }
  | { type: "game_state"; payload: MatchStatePayload }
  | { type: "timer_update"; payload: { roomId: string; turnPlayerId: string; turnEndsAt: number } }
  | { type: "match_finished"; payload: { roomId: string; winnerPlayerId?: string; abandoned?: boolean } }
  | { type: "player_disconnected"; payload: { roomId: string; playerId: string } }
  | { type: "player_reconnected"; payload: { roomId: string; playerId: string } }
  | { type: "chat_message"; payload: { userId: string; message: string } }
  | { type: "error"; payload: { message: string } };
