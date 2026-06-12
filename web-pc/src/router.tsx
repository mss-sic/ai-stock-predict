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
import RiskPage from './pages/RiskPage';
import StockListPage from './pages/StockListPage';
import DataManagementPage from './pages/DataManagementPage';
import SettingsPage from './pages/SettingsPage';
import PersonalSettingsPage from "./pages/PersonalSettingsPage";
import LoginPage from './pages/LoginPage';
import AdminPage from './pages/AdminPage';
import PkListPage from './pages/PkListPage';
import PkDetailPage from './pages/PkDetailPage';
import PkAdminPage from './pages/PkAdminPage';

function ErrorBoundary() {
  const error = useRouteError() as any;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100vh', background: '#f0f4ff', gap: 12 }}>
      <span style={{ fontSize: 48 }}>😵</span>
      <h2 style={{ color: '#1d2129', margin: 0 }}>页面出错了</h2>
      <p style={{ color: '#86909c', fontSize: 13, maxWidth: 400, textAlign: 'center' }}>
        {error?.status === 404 ? '页面不存在' : error?.message || '未知错误'}
      </p>
      <a href="/" style={{ color: '#165dff', fontSize: 13 }}>返回首页</a>
    </div>
  );
}

function ProtectedRoute() {
  const { user, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh', background: '#f0f4ff' }}>
        <span style={{ color: '#86909c', fontSize: 14 }}>加载中...</span>
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
          { path: 'stock/:code', element: <StockDetailPage /> },
          { path: 'forecast/:code', element: <ForecastPage /> },
          { path: 'ai/:code', element: <AIAnalysisPage /> },
          { path: 'stocks', element: <StockListPage /> },
          { path: 'watchlist', element: <WatchlistPage /> },
          { path: 'strategy', element: <StrategyPage /> },
          { path: 'holdings', element: <HoldingsPage /> },
          { path: 'risk', element: <RiskPage /> },
          { path: 'data', element: <DataManagementPage /> },
          { path: 'settings', element: <SettingsPage /> },
          { path: "profile", element: <PersonalSettingsPage /> },
          { path: 'admin', element: <AdminPage /> },
          { path: 'pk', element: <PkListPage /> },
          { path: 'pk/:id', element: <PkDetailPage /> },
          { path: 'pk/create', element: <PkAdminPage /> },
          { path: '*', element: <Navigate to="/" replace /> },
        ],
      },
    ],
  },
]);

export default router;
