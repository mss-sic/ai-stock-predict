import { createBrowserRouter, Navigate, Outlet, useLocation, useRouteError } from 'react-router-dom';
import { useAuth } from './services/AuthContext';
import AppLayout from './App';
import BoardPage from './pages/BoardPage';
import HistoryBoardPage from './pages/HistoryBoardPage';
import HeatmapPage from './pages/HeatmapPage';
import ConceptBoardPage from './pages/ConceptBoardPage';
import ConceptBoardDetailPage from './pages/ConceptBoardDetailPage';
import StockDetailPage from './pages/StockDetailPage';
import ForecastPage from './pages/ForecastPage';
import AIAnalysisPage from './pages/AIAnalysisPage';
import WatchlistPage from './pages/WatchlistPage';
import StrategyPage from './pages/StrategyPage';
import HoldingsPage from './pages/HoldingsPage';

import RiskDashboard from './pages/RiskDashboard';
import RiskRules from './pages/RiskRules';
import StockListPage from './pages/StockListPage';
import LimitStatsPage from "./pages/LimitStatsPage";
import SentimentDashboard from './pages/SentimentDashboard';
import DataManagementPage from './pages/DataManagementPage';
import TaskLogPage from './pages/TaskLogPage';
import SettingsPage from './pages/SettingsPage';
import PersonalSettingsPage from "./pages/PersonalSettingsPage";
import UserCostPage from "./pages/UserCostPage";
import LoginPage from './pages/LoginPage';
import AdminPage from './pages/AdminPage';
import PkListPage from './pages/PkListPage';
import PkDetailPage from './pages/PkDetailPage';
import PkAdminPage from './pages/PkAdminPage';
import BacktestDetailPage from "./pages/BacktestDetailPage";
import MarketStylePage from "./pages/MarketStylePage";
import DragonTigerPage from "./pages/DragonTigerPage";
import UnlockCalendarPage from "./pages/UnlockCalendarPage";
import ThemeHeatPage from "./pages/ThemeHeatPage";
import MacroNewsPage from "./pages/MacroNewsPage";
import IndustryComparePage from "./pages/IndustryComparePage";
import FearGreedPage from "./pages/FearGreedPage";
import CapitalFlowPage from "./pages/CapitalFlowPage";
import AnnouncementsPage from "./pages/AnnouncementsPage";
import LiveTradingPage from './pages/LiveTradingPage';
import LiveRunDetailPage from './pages/LiveRunDetailPage';

import PkEntryDetailPage from './pages/PkEntryDetailPage';

function ErrorBoundary() {
  const error = useRouteError() as any;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100vh', background: 'var(--color-fill-2)', gap: 12 }}>
      <span style={{ fontSize: 48 }}>😵</span>
      <h2 style={{ color: 'var(--color-text-1)', margin: 0 }}>页面出错了</h2>
      <p style={{ color: 'var(--color-text-3)', fontSize: 13, maxWidth: 400, textAlign: 'center' }}>
        {error?.status === 404 ? '页面不存在' : error?.message || '未知错误'}
      </p>
      <a href="/" style={{ color: 'var(--color-primary)', fontSize: 13 }}>返回首页</a>
    </div>
  );
}

function ProtectedRoute() {
  const { user, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh', background: 'var(--color-fill-2)' }}>
        <span style={{ color: 'var(--color-text-3)', fontSize: 14 }}>加载中...</span>
      </div>
    );
  }

  if (!user) {
    return <Navigate to={`/login?redirect=${encodeURIComponent(location.pathname)}`} replace />;
  }

  return <Outlet />;
}

const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />,
    errorElement: <ErrorBoundary />,
  },
  {
    element: <ProtectedRoute />,
    errorElement: <ErrorBoundary />,
    children: [
      {
        path: '/',
        element: <AppLayout />,
        errorElement: <ErrorBoundary />,
        children: [
          { index: true, element: <BoardPage /> },
          { path: 'board/history', element: <HistoryBoardPage /> },
          { path: 'board/heatmap', element: <HeatmapPage /> },
          { path: 'board/concepts', element: <ConceptBoardPage /> },
          { path: 'concept/:code', element: <ConceptBoardDetailPage /> },
          { path: 'dragon-tiger', element: <DragonTigerPage /> },
          { path: 'unlocks', element: <UnlockCalendarPage /> },
          { path: 'theme-heat', element: <ThemeHeatPage /> },
          { path: 'macro-news', element: <MacroNewsPage /> },
          { path: 'announcements', element: <AnnouncementsPage /> },
          { path: 'stock/:code', element: <StockDetailPage /> },
          { path: 'forecast/:code', element: <ForecastPage /> },
          { path: 'ai/:code', element: <AIAnalysisPage /> },
          { path: 'stocks', element: <StockListPage /> },
          { path: 'watchlist', element: <WatchlistPage /> },
          { path: 'strategy', element: <StrategyPage /> },
          { path: 'holdings', element: <HoldingsPage /> },
          { path: 'risk', element: <RiskDashboard /> },
          
          { path: 'risk/rules', element: <RiskRules /> },
          { path: 'live', element: <LiveTradingPage /> },
          { path: 'live/:id', element: <LiveRunDetailPage /> },
          { path: 'sentiment', element: <SentimentDashboard /> },
          { path: "limit-stats", element: <LimitStatsPage /> },
          { path: "fear-greed", element: <FearGreedPage /> },
          { path: "capital-flow", element: <CapitalFlowPage /> },
          { path: "market-style", element: <MarketStylePage /> },
          { path: "industries", element: <IndustryComparePage /> },

          { path: 'data', element: <DataManagementPage /> },
          { path: 'task-logs', element: <TaskLogPage /> },
          { path: 'settings', element: <SettingsPage /> },
          { path: "profile", element: <PersonalSettingsPage /> },
          { path: "cost", element: <UserCostPage /> },
          { path: 'admin', element: <AdminPage /> },
          { path: 'pk', element: <PkListPage /> },
          { path: 'pk/:id', element: <PkDetailPage /> },
          { path: 'pk/create', element: <PkAdminPage /> },
          { path: 'strategy/backtest/:id', element: <BacktestDetailPage /> },
          { path: 'pk/:id/entry/:entryId', element: <PkEntryDetailPage /> },
          { path: '*', element: <Navigate to="/" replace /> },
        ],
      },
    ],
  },
]);

export default router;
