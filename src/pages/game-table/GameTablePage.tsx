import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { applyMockMatchAction, isMockApiEnabled } from "@/mocks/mockApi";
import type { MatchActionType, Room } from "@/entities/match/types";
import { joinGameRoom } from "@/processes/joinGame.process";
import { getRoom } from "@/shared/api/rooms";
import {
  getTelegramUser,
  hapticImpact,
  hapticNotification,
  hapticSelection,
} from "@/shared/lib/telegram";
import { wsClient } from "@/shared/api/ws/socket";
import { AppAvatar } from "@/shared/ui/Avatar";
import { BackIcon } from "@/shared/ui/Icons";
import { AppCard } from "@/shared/ui/Card";
import { AppButton } from "@/shared/ui/Button";
import { CardSkeleton, ConfirmModal, EmptyStateBlock } from "@/shared/ui/StateBlocks";
import {
  addActivity,
  clearGameError,
  getGameState,
  setGameConnecting,
  setMatchState,
  subscribeGameStore,
} from "@/store/game.store";

export function GameTablePage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [room, setRoom] = useState<Room | null>(null);
  const [gameState, setGameState] = useState(getGameState());
  const [isRoomLoading, setIsRoomLoading] = useState(true);
  const [isExitModalOpen, setIsExitModalOpen] = useState(false);
  const [selectedCardId, setSelectedCardId] = useState<string | null>(null);
  const [secondsLeft, setSecondsLeft] = useState<number | null>(null);
  const telegramUser = getTelegramUser();
  const currentUserId = telegramUser?.id ? String(telegramUser.id) : null;
  const matchState = gameState.matchState;
  const players = matchState?.players ?? [];
  const currentPlayer = useMemo(
    () => players.find((player) => player.id === currentUserId) ?? players[0],
    [currentUserId, players],
  );
  const opponents = useMemo(
    () => players.filter((player) => player.id !== currentPlayer?.id),
    [currentPlayer?.id, players],
  );
  const hasDetailedHand = Boolean(currentPlayer?.hand?.length);
  const isMyTurn = useMemo(() => {
    if (!matchState || !currentPlayer) {
      return false;
    }
    if (matchState.turnPlayerId) {
      return matchState.turnPlayerId === currentPlayer.id;
    }
    return currentPlayer.isCurrentTurn;
  }, [currentPlayer, matchState]);
  const canAct = gameState.status === "ready" && matchState?.status === "playing" && isMyTurn;
  const tablePairs = useMemo(() => buildTablePairs(matchState?.tableCards ?? []), [matchState?.tableCards]);
  const winnerName = useMemo(() => {
    if (!matchState?.winnerPlayerId) {
      return null;
    }
    return players.find((player) => player.id === matchState.winnerPlayerId)?.username ?? "Игрок";
  }, [matchState?.winnerPlayerId, players]);
  const isWinner = matchState?.winnerPlayerId && currentPlayer
    ? matchState.winnerPlayerId === currentPlayer.id
    : false;

  useEffect(() => {
    if (!id) {
      setIsRoomLoading(false);
      return;
    }
    void getRoom(id)
      .then((data) => setRoom(data))
      .catch(() => setRoom(null))
      .finally(() => setIsRoomLoading(false));
  }, [id]);

  useEffect(() => {
    return subscribeGameStore(setGameState);
  }, []);

  useEffect(() => {
    if (!id) {
      return;
    }

    let cleanup: (() => void) | undefined;
    void joinGameRoom(id).then((dispose) => {
      cleanup = dispose;
    });

    return () => cleanup?.();
  }, [id]);

  useEffect(() => {
    if (!matchState?.turnEndsAt || matchState.status !== "playing") {
      setSecondsLeft(null);
      return;
    }

    const tick = () => {
      const diffMs = matchState.turnEndsAt! - Date.now();
      setSecondsLeft(Math.max(0, Math.ceil(diffMs / 1000)));
    };

    tick();
    const interval = window.setInterval(tick, 1000);
    return () => window.clearInterval(interval);
  }, [matchState?.status, matchState?.turnEndsAt, matchState?.turnPlayerId]);

  function sendAction(action: MatchActionType) {
    if (!id) {
      return;
    }
    const cardId = selectedCardId ?? undefined;
    if (isMockApiEnabled()) {
      const next = applyMockMatchAction({ roomId: id, action, cardId });
      setMatchState(next);
      addActivity(`Mock action: ${action}`);
      if (action === "attack" || action === "defend") {
        setSelectedCardId(null);
      }
      return;
    }
    const selectedCard =
      action === "attack" || action === "defend" ? { cardId } : {};

    hapticImpact("medium");
    wsClient.send({
      type: "make_move",
      payload: { roomId: id, action: toWireAction(action), ...selectedCard },
    });
    if (action === "attack" || action === "defend") {
      setSelectedCardId(null);
    }
  }

  return (
    <section className="screen game-table-screen">
      <div className="page-header">
        <Link className="icon-button" to="/play">
          <BackIcon size={17} />
        </Link>
        <h1 className="page-header__title">Игровой стол</h1>
        <div className="page-header__spacer" />
      </div>

      {isRoomLoading && <CardSkeleton rows={4} />}

      {gameState.status === "connecting" && room && (
        <AppCard>
          <div className="card__hint">Подключение к игре...</div>
        </AppCard>
      )}

      {gameState.status === "error" && room && (
        <AppCard>
          <div className="card__hint card__hint--error">{gameState.error}</div>
          <AppButton
            variant="primary"
            type="button"
            onClick={() => {
              if (!id) return;
              clearGameError();
              setGameConnecting(id);
              wsClient.disconnect();
              wsClient.connect(id);
            }}
          >
            Повторить подключение
          </AppButton>
        </AppCard>
      )}

      {!isRoomLoading && !room && (
        <EmptyStateBlock
          title="Комната не найдена"
          message="Стол недоступен. Вернитесь в список игр и выберите другую комнату."
          actionLabel="К списку игр"
          onAction={() => navigate("/play")}
        />
      )}

      {room && (
        <>
          <AppCard className="game-board">
            <div className="game-board__top">
              <div className="game-board__room">
                <strong>{room.title}</strong>
                <span>${room.stakeUsd}</span>
              </div>
              <div className="game-board__meta">
                <span className="game-board__trump">
                  {matchState?.trumpCard ? (
                    <span className={`playing-card playing-card--mini ${getSuitClass(matchState.trumpCard.suit)}`}>
                      {matchState.trumpCard.rank}{getSuitSymbol(matchState.trumpCard.suit)}
                    </span>
                  ) : (
                    formatSuit(matchState?.trumpSuit)
                  )}
                </span>
                <span>{secondsLeft !== null ? `${secondsLeft}с` : "..."}</span>
              </div>
            </div>

            <div className="game-board__opponents">
              {opponents.slice(0, 3).map((player, index) => (
                <div className="game-opponent" key={player.id}>
                  <AppAvatar
                    name={player.username || `player-${index + 1}`}
                    photoUrl={(player as { photo_url?: string }).photo_url}
                    className="game-opponent__avatar"
                  />
                  <div className="game-opponent__name">@{player.username || `player-${index + 1}`}</div>
                  <div className="game-opponent__cards">{player.handCount}</div>
                </div>
              ))}
            </div>

            <div className="game-board__table">
              {tablePairs.length ? (
                <div className="table-pairs">
                  {tablePairs.map((pair, index) => (
                    <div className="table-pairs__item" key={`pair-${index}`}>
                      <CardFace suit={pair.attack.suit} rank={pair.attack.rank} />
                      {pair.defense ? (
                        <CardFace suit={pair.defense.suit} rank={pair.defense.rank} />
                      ) : (
                        <div className="playing-card playing-card--placeholder" />
                      )}
                    </div>
                  ))}
                </div>
              ) : (
                <div className="card__hint card__hint--center">Стол пока пуст</div>
              )}
            </div>

            <div className="game-board__turn">
              {isMyTurn ? "Ваш ход" : "Ход соперника"}
              {gameState.error && <span className="game-board__error"> · {gameState.error}</span>}
            </div>
            {gameState.reconnectingPlayerId && gameState.reconnectingPlayerId !== currentUserId && (
              <div className="game-board__reconnect-hint card__hint card__hint--center">
                Соперник отключился. Ожидание 60 сек...
              </div>
            )}
          </AppCard>

          <AppCard className="hand-zone">
            <div className="card__label">Ваши карты</div>
            {currentPlayer?.handCount ? (
              <div className="cards-grid cards-grid--hand">
                {hasDetailedHand
                  ? currentPlayer.hand?.map((card) => (
                      <button
                        className={`playing-card ${getSuitClass(card.suit)} ${
                          selectedCardId === card.id ? "playing-card--selected" : ""
                        }`}
                        key={card.id}
                        type="button"
                        onClick={() => {
                          hapticSelection();
                          setSelectedCardId(card.id);
                        }}
                      >
                        <span>{card.rank}</span>
                        <span>{getSuitSymbol(card.suit)}</span>
                      </button>
                    ))
                  : Array.from({ length: currentPlayer.handCount }).map((_, index) => (
                      <div className="playing-card playing-card--back" key={`back-${index}`} />
                    ))}
              </div>
            ) : (
              <div className="card__hint">Карт на руке нет</div>
            )}
          </AppCard>

          <AppCard className="game-actions">
            <div className="action-list action-list--inline">
              <AppButton
                variant="primary"
                type="button"
                onClick={() => sendAction("take")}
                disabled={!canAct}
              >
                Беру
              </AppButton>
              <AppButton
                type="button"
                onClick={() => sendAction("defend")}
                disabled={!canAct || (hasDetailedHand && !selectedCardId)}
              >
                Бью
              </AppButton>
              <AppButton
                type="button"
                onClick={() => sendAction("attack")}
                disabled={!canAct || (hasDetailedHand && !selectedCardId)}
              >
                Подкинуть
              </AppButton>
            </div>
            <div className="game-actions__secondary">
              <AppButton type="button" onClick={() => sendAction("pass")} disabled={!canAct}>
                Пас
              </AppButton>
              <Link className="button" to={`/game/${id}/friends`}>
                Друзья
              </Link>
              <AppButton type="button" onClick={() => setIsExitModalOpen(true)}>
                Выйти
              </AppButton>
            </div>
            <div className="game-actions__balance">Ваш баланс: ${room.stakeUsd * 3}</div>
          </AppCard>

          <AppCard>
            <div className="card__label">Лог событий</div>
            {gameState.activity.length ? (
              <div className="list">
                {gameState.activity.slice(-5).map((item, index) => (
                  <div className="card__hint" key={`${item}-${index}`}>
                    {item}
                  </div>
                ))}
              </div>
            ) : (
              <div className="card__hint">Событий пока нет</div>
            )}
          </AppCard>

          {matchState?.status === "finished" && (
            <div className="result-card">
              <div className="result-card__title">{isWinner ? "Победа!" : "Игра завершена"}</div>
              <div className="result-card__message">
                {isWinner && gameState.matchFinishedAbandoned
                  ? "Соперник не вернулся. Победа."
                  : winnerName
                    ? `Победитель: ${winnerName}`
                    : "Ожидание итогового расчета."}
              </div>
              <button
                className="button button--primary"
                type="button"
                onClick={() => {
                  hapticNotification(isWinner ? "success" : "warning");
                  navigate(isWinner ? "/finish/win" : "/finish/lose");
                }}
              >
                Продолжить
              </button>
            </div>
          )}
        </>
      )}

      <ConfirmModal
        isOpen={isExitModalOpen}
        title="Покинуть текущую игру?"
        message="Если выйти сейчас, вы можете потерять ставку и место за столом."
        confirmLabel="Выйти"
        onConfirm={() => navigate("/play")}
        onCancel={() => setIsExitModalOpen(false)}
      />
    </section>
  );
}

function CardFace({ suit, rank }: { suit: string; rank: string }) {
  return (
    <button className={`playing-card ${getSuitClass(suit)}`} type="button">
      <span>{rank}</span>
      <span>{getSuitSymbol(suit)}</span>
    </button>
  );
}

function getSuitSymbol(suit: string) {
  switch (suit) {
    case "hearts":
      return "♥";
    case "diamonds":
      return "♦";
    case "clubs":
      return "♣";
    case "spades":
      return "♠";
    default:
      return "?";
  }
}

function getSuitClass(suit: string) {
  return suit === "hearts" || suit === "diamonds" ? "playing-card--red" : "playing-card--dark";
}

function formatSuit(suit?: string) {
  if (!suit) {
    return "-";
  }
  return `${getSuitSymbol(suit)} ${suit}`;
}

function buildTablePairs(cards: Array<{ id: string; suit: string; rank: string }>) {
  const pairs: Array<{
    attack: { id: string; suit: string; rank: string };
    defense?: { id: string; suit: string; rank: string };
  }> = [];

  for (let index = 0; index < cards.length; index += 2) {
    pairs.push({
      attack: cards[index],
      defense: cards[index + 1],
    });
  }

  return pairs;
}

function toWireAction(action: MatchActionType) {
  switch (action) {
    case "attack":
      return "attack_card";
    case "defend":
      return "defend_card";
    case "take":
      return "take_cards";
    case "pass":
      return "pass_turn";
    default:
      return action;
  }
}
