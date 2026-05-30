import { Suspense, lazy, type ComponentType } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { GameLayout } from "@/layouts/GameLayout";
import { MainLayout } from "@/layouts/MainLayout";
import { useLanguage } from "@/shared/providers/LanguageProvider";

type RoutePage = ComponentType<Record<string, never>>;
type RouteModule = Record<string, RoutePage>;

function lazyPage(loader: () => Promise<RouteModule>, exportName: string) {
  return lazy(async () => {
    const module = await loader();
    const component = module[exportName];
    if (!component) {
      throw new Error(`Route export "${exportName}" was not found`);
    }
    return { default: component };
  });
}

function RouteLoader() {
  const { t } = useLanguage();

  return (
    <div
      aria-busy="true"
      style={{
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        minHeight: "40dvh",
        color: "var(--color-text-secondary, #8d8d93)",
      }}
    >
      {t("app.loading")}
    </div>
  );
}

function renderLazyRoute(Component: RoutePage) {
  return (
    <Suspense fallback={<RouteLoader />}>
      <Component />
    </Suspense>
  );
}

const LobbyPage = lazyPage(() => import("@/pages/lobby/LobbyPage"), "LobbyPage");
const PlayPage = lazyPage(() => import("@/pages/Play/PlayPage"), "PlayPage");
const CreateGamePage = lazyPage(() => import("@/pages/create-game/CreateGamePage"), "CreateGamePage");
const ProfilePage = lazyPage(() => import("@/pages/Profile/ProfilePage"), "ProfilePage");
const DepositPage = lazyPage(() => import("@/pages/Profile/DepositPage"), "DepositPage");
const WithdrawPage = lazyPage(() => import("@/pages/Profile/WithdrawPage"), "WithdrawPage");
const GameRoomPage = lazyPage(() => import("@/pages/GameRoom/GameRoomPage"), "GameRoomPage");
const GameTablePage = lazyPage(() => import("@/pages/game-table/GameTablePage"), "GameTablePage");
const GameAddFriendsPage = lazyPage(() => import("@/pages/GameAddFriends/GameAddFriendsPage"), "GameAddFriendsPage");
const FinishWinPage = lazyPage(() => import("@/pages/FinishWin/FinishWinPage"), "FinishWinPage");
const FinishLosePage = lazyPage(() => import("@/pages/FinishLose/FinishLosePage"), "FinishLosePage");
const SettingsPage = lazyPage(() => import("@/pages/Settings/SettingsPage"), "SettingsPage");
const NamePage = lazyPage(() => import("@/pages/Name/NamePage"), "NamePage");
const CurrencyPage = lazyPage(() => import("@/pages/Currency/CurrencyPage"), "CurrencyPage");
const LanguagePage = lazyPage(() => import("@/pages/Language/LanguagePage"), "LanguagePage");
const FriendsPage = lazyPage(() => import("@/pages/Friends/FriendsPage"), "FriendsPage");
const FriendsAddPage = lazyPage(() => import("@/pages/FriendsAdd/FriendsAddPage"), "FriendsAddPage");
const HistoryGamesPage = lazyPage(() => import("@/pages/HistoryGames/HistoryGamesPage"), "HistoryGamesPage");
const HistoryDatePage = lazyPage(() => import("@/pages/HistoryDate/HistoryDatePage"), "HistoryDatePage");
const HistoryCalendarPage = lazyPage(() => import("@/pages/HistoryCalendar/HistoryCalendarPage"), "HistoryCalendarPage");
const TransactionsPage = lazyPage(() => import("@/pages/Transactions/TransactionsPage"), "TransactionsPage");

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<MainLayout />}>
        <Route index element={<Navigate to="/play" replace />} />
        <Route path="/home" element={<Navigate to="/play" replace />} />
        <Route path="/lobby" element={renderLazyRoute(LobbyPage)} />
        <Route path="/play" element={renderLazyRoute(PlayPage)} />
        <Route path="/play/create" element={renderLazyRoute(CreateGamePage)} />
        <Route path="/profile" element={renderLazyRoute(ProfilePage)} />
        <Route path="/profile/deposit" element={renderLazyRoute(DepositPage)} />
        <Route path="/profile/withdraw" element={renderLazyRoute(WithdrawPage)} />
      </Route>

      <Route path="/create" element={<Navigate to="/play/create" replace />} />

      <Route element={<GameLayout />}>
        <Route path="/room/:id" element={renderLazyRoute(GameRoomPage)} />
        <Route path="/game/:id" element={renderLazyRoute(GameTablePage)} />
        <Route path="/game/:id/friends" element={renderLazyRoute(GameAddFriendsPage)} />
        <Route path="/finish/win" element={renderLazyRoute(FinishWinPage)} />
        <Route path="/finish/lose" element={renderLazyRoute(FinishLosePage)} />
        <Route path="/profile/settings" element={renderLazyRoute(SettingsPage)} />
        <Route path="/profile/settings/name" element={renderLazyRoute(NamePage)} />
        <Route path="/profile/settings/currency" element={renderLazyRoute(CurrencyPage)} />
        <Route path="/profile/settings/language" element={renderLazyRoute(LanguagePage)} />
        <Route path="/profile/friends" element={renderLazyRoute(FriendsPage)} />
        <Route path="/profile/friends/add" element={renderLazyRoute(FriendsAddPage)} />
        <Route path="/profile/history/games" element={renderLazyRoute(HistoryGamesPage)} />
        <Route path="/profile/history/date" element={renderLazyRoute(HistoryDatePage)} />
        <Route path="/profile/history/calendar" element={renderLazyRoute(HistoryCalendarPage)} />
        <Route path="/profile/history/transactions" element={renderLazyRoute(TransactionsPage)} />
      </Route>

      <Route path="*" element={<Navigate to="/play" replace />} />
    </Routes>
  );
}
