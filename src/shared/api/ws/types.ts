import type { Card, Rank } from "@/entities/card/types";
import type { MatchActionType } from "@/entities/match/types";
import type { Player } from "@/entities/player/types";

// All possible player intents sent from client to server
export type IntentType =
  | "playAttack"
  | "playDefend"
  | "throwIn"
  | "shulerPlay"
  | "translate"
  | "take"
  | "pass"
  | "endTurn"
  | "confirmStake"
  | "shulerReport";

// Seat (player position) at the table
export interface Seat {
  id: string;
  name: string;
  cardCount: number;
  isReady: boolean;
  isConfirmed: boolean;
  avatarUrl?: string;
}

export interface MatchAffordances {
  canAct: boolean;
  canAttack: boolean;
  canDefend: boolean;
  canTake: boolean;
  canPass: boolean;
  canThrowIn: boolean;
  canTranslate: boolean;
  canShulerPlay: boolean;
  canShulerReport: boolean;
  attackCardIds?: string[];
  defendCardIds?: string[];
  throwInCardIds?: string[];
  translateCardIds?: string[];
  shulerPlayCardIds?: string[];
  defendableTargetCardIds?: string[];
}

// Extended match state payload from backend
export interface MatchStatePayload {
  // Versioning / last action
  version: number;
  lastActionId?: string;

  // High-level phase of the match
  phase: "betting" | "playing" | "attack" | "defend" | "result";

  // Table configuration
  deckType?: 24 | 36 | 52;
  mode?: "podkidnoy" | "perevodnoy";
  stakeUsd?: number;
  trumpSuit: string;
  attackerPlayerId?: string;
  defenderPlayerId?: string;

  // Deck / table state
  stockCount?: number;
  discardCount?: number;
  capacityOnTable?: number;
  allowedRanks?: Rank[];
  /** Absolute timestamp (ms since epoch) when current turn ends */
  turnEndsAt?: number;
  /** Seat index of player whose turn it is (attacker/defender/thrower depending on phase) */
  turnSeatIndex?: number;

  // Cards on the table (attack/defense pairs)
  tablePiles?: {
    attack: Card;
    defend?: Card;
    /** Legacy/client-friendly alias for defend */
    defense?: Card;
  }[];

  // Players and roles
  seats?: Seat[];
  attackerSeat?: number;
  defenderSeat?: number;

  // Current viewer data
  myHand?: string[];
  mySeatIndex?: number;

  // Shuler ability / report window
  shuler?: {
    isWindowOpen: boolean;
    windowEndsAt?: number;
    activePlayers: string[];
  };
  affordances?: MatchAffordances;

  // Finish / payouts
  finish?: {
    bank: number;
    commission: number;
    /** Net profit/loss per player after stake and commission; not gross wallet credits. */
    payouts: Record<string, number>;
    places: string[];
  };

  // Legacy / transitional fields kept for backward compatibility with older client code
  roomId?: string;
  matchId?: string;
  status?: string;
  turnPlayerId?: string;
  tableCards?: Card[];
  players?: Player[];
  winnerPlayerId?: string;
}

type SyncRequestPayload = {
  roomId: string;
  lastKnownVersion?: number;
  lastKnownMatchId?: string;
  supportsStateDiff?: boolean;
};

export type ClientWsEvent =
  | {
      type: "join_room";
      payload: SyncRequestPayload;
    }
  | {
      type: "create_room";
      payload: { title: string; stake: number; mode: string; deck: number };
    }
  | {
      type: "make_move";
      payload: {
        roomId: string;
        action:
          | MatchActionType
          | "attack_card"
          | "defend_card"
          | "throw_card"
          | "translate"
          | "take_cards"
          | "pass_turn"
          | "end_round"
          | "shuler_play"
          | "shuler_report";
        cardId?: string;
        expectedVersion?: number;
        actionId?: string;
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
      type: "start_game";
      payload: { roomId: string };
    }
  | {
      type: "confirm_stake";
      payload: { roomId: string };
    }
  | {
      type: "send_message";
      payload: { roomId: string; message: string };
    }
  | {
      type: "reconnect";
      payload: SyncRequestPayload;
    }
  | {
      type: "sync_request";
      payload: SyncRequestPayload;
    };

type ServerWsEventMeta = {
  correlationId?: string;
  locale?: string;
};

export type ServerWsEvent = ServerWsEventMeta &
  (
    | { type: "room_update"; payload: Record<string, unknown> }
    | {
        type: "state_sync";
        payload: {
          roomId: string;
          matchId: string;
        fromVersion: number;
        toVersion: number;
        mode: "replay" | "snapshot" | "noop";
        replayCount: number;
        replayFromVersion?: number;
      };
    }
    | {
        type: "move_applied";
        payload: {
          roomId: string;
          matchId: string;
          eventId?: string;
          playerId: string;
          action: string;
          cardId?: string;
        };
      }
    | {
        type: "state_diff";
        payload: {
          roomId: string;
          matchId: string;
          fromVersion: number;
          toVersion: number;
          patch: Partial<MatchStatePayload>;
        };
      }
    | { type: "game_state"; payload: MatchStatePayload }
    | { type: "timer_update"; payload: { roomId: string; turnPlayerId: string; turnEndsAt: number } }
    | {
        type: "match_finished";
        payload: {
          roomId: string;
          winnerPlayerId?: string;
          winnerPlayerIds?: string[];
          isDraw?: boolean;
          finishGroups?: string[][];
          abandoned?: boolean;
          settlementId?: string;
          /** Net profit/loss per player after stake and commission; not gross wallet credits. */
          payouts?: { userId: string; amount: number }[];
          commission?: number;
          pot?: number;
          newBalances?: Record<string, number>;
        };
      }
    | { type: "player_disconnected"; payload: { roomId: string; playerId: string } }
    | { type: "player_reconnected"; payload: { roomId: string; playerId: string } }
    | { type: "player_afk_bot_takeover"; payload: { roomId: string; playerId: string } }
    | { type: "chat_message"; payload: { userId: string; message: string } }
    | { type: "error"; payload: { message: string; errorCode?: string } }
    | {
        type: "version_mismatch";
        payload: { roomId: string; action: string; cardId?: string; actionId?: string };
      }
  );
