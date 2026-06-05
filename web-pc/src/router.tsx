import { createBrowserRouter } from 'react-router-dom';
import AppLayout from './App';
import BoardPage from './pages/BoardPage';
import HistoryBoardPage from './pages/HistoryBoardPage';
import HeatmapPage from './pages/HeatmapPage';
import StockDetailPage from './pages/StockDetailPage';
import ForecastPage from './pages/ForecastPage';
import AIAnalysisPage from './pages/AIAnalysisPage';
import WatchlistPage from './pages/WatchlistPage';
import StrategyPage from './pages/StrategyPage';
import HoldingsPage from './pages/HoldingsPage';
import RiskPage from './pages/RiskPage';
import DataManagementPage from './pages/DataManagementPage';

const router = createBrowserRouter([
  {
    path: '/',
    element: <AppLayout />,
    children: [
      { index: true, element: <BoardPage /> },
      { path: 'board/history', element: <HistoryBoardPage /> },
      { path: 'board/heatmap', element: <HeatmapPage /> },
      { path: 'stock/:code', element: <StockDetailPage /> },
      { path: 'forecast/:code', element: <ForecastPage /> },
      { path: 'ai/:code', element: <AIAnalysisPage /> },
      { path: 'watchlist', element: <WatchlistPage /> },
      { path: 'strategy', element: <StrategyPage /> },
      { path: 'holdings', element: <HoldingsPage /> },
      { path: 'risk', element: <RiskPage /> },
      { path: 'data', element: <DataManagementPage /> },
    ],
  },
]);

export default router;
