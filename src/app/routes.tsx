import { Navigate, Route, Routes } from "react-router-dom";
import { GameLayout } from "@/layouts/GameLayout";
import { MainLayout } from "@/layouts/MainLayout";
import { CreateGamePage } from "@/pages/create-game/CreateGamePage";
import { HomePage } from "@/pages/home/HomePage";
import { LobbyPage } from "@/pages/lobby/LobbyPage";
import { CurrencyPage } from "@/pages/Currency/CurrencyPage";
import { FinishLosePage } from "@/pages/FinishLose/FinishLosePage";
import { FinishWinPage } from "@/pages/FinishWin/FinishWinPage";
import { FriendsPage } from "@/pages/Friends/FriendsPage";
import { FriendsAddPage } from "@/pages/FriendsAdd/FriendsAddPage";
import { GameAddFriendsPage } from "@/pages/GameAddFriends/GameAddFriendsPage";
import { GameTablePage } from "@/pages/game-table/GameTablePage";
import { GameRoomPage } from "@/pages/GameRoom/GameRoomPage";
import { HistoryCalendarPage } from "@/pages/HistoryCalendar/HistoryCalendarPage";
import { HistoryDatePage } from "@/pages/HistoryDate/HistoryDatePage";
import { HistoryGamesPage } from "@/pages/HistoryGames/HistoryGamesPage";
import { LanguagePage } from "@/pages/Language/LanguagePage";
import { NamePage } from "@/pages/Name/NamePage";
import { PlayPage } from "@/pages/Play/PlayPage";
import { DepositPage } from "@/pages/Profile/DepositPage";
import { WithdrawPage } from "@/pages/Profile/WithdrawPage";
import { ProfilePage } from "@/pages/Profile/ProfilePage";
import { SettingsPage } from "@/pages/Settings/SettingsPage";
import { TransactionsPage } from "@/pages/Transactions/TransactionsPage";

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<MainLayout />}>
        <Route index element={<Navigate to="/play" replace />} />
        <Route path="/home" element={<HomePage />} />
        <Route path="/lobby" element={<LobbyPage />} />
        <Route path="/play" element={<PlayPage />} />
        <Route path="/play/create" element={<CreateGamePage />} />
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="/profile/deposit" element={<DepositPage />} />
        <Route path="/profile/withdraw" element={<WithdrawPage />} />
      </Route>

      <Route path="/create" element={<Navigate to="/play/create" replace />} />

      <Route element={<GameLayout />}>
        <Route path="/room/:id" element={<GameRoomPage />} />
        <Route path="/game/:id" element={<GameTablePage />} />
        <Route path="/game/:id/friends" element={<GameAddFriendsPage />} />
        <Route path="/finish/win" element={<FinishWinPage />} />
        <Route path="/finish/lose" element={<FinishLosePage />} />
        <Route path="/profile/settings" element={<SettingsPage />} />
        <Route path="/profile/settings/name" element={<NamePage />} />
        <Route path="/profile/settings/currency" element={<CurrencyPage />} />
        <Route path="/profile/settings/language" element={<LanguagePage />} />
        <Route path="/profile/friends" element={<FriendsPage />} />
        <Route path="/profile/friends/add" element={<FriendsAddPage />} />
        <Route path="/profile/history/games" element={<HistoryGamesPage />} />
        <Route path="/profile/history/date" element={<HistoryDatePage />} />
        <Route path="/profile/history/calendar" element={<HistoryCalendarPage />} />
        <Route path="/profile/history/transactions" element={<TransactionsPage />} />
      </Route>

      <Route path="*" element={<Navigate to="/play" replace />} />
    </Routes>
  );
}
