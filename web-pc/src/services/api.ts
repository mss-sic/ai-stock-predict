import axios from 'axios';

const api = axios.create({
  baseURL: 'http://127.0.0.1:8080/api/v1',
  timeout: 15000,
});

api.interceptors.response.use(
  (res) => res.data,
  (err) => {
    console.error('API error:', err.message);
    return Promise.reject(err);
  }
);

// Stock
export const fetchStocks = (params: Record<string, any>) => api.get('/stocks', { params });
export const fetchQuote = (code: string) => api.get(`/stocks/${code}/quote`);
export const fetchStockDetail = (code: string) => api.get(`/stocks/${code}`);
export const fetchKLine = (code: string, from?: string, to?: string) =>
  api.get(`/stocks/${code}/kline`, { params: { from, to } });
export const fetchIndicator = (code: string) => api.get(`/stocks/${code}/indicator`);

// Board
export const fetchTodayBoard = () => api.get('/board/today');
export const fetchHistoryBoard = (date: string) => api.get('/board/history', { params: { date } });
export const fetchEnrichedHeatmap = (from?: string, to?: string) => api.get("/board/heatmap-enriched", { params: { from, to } });
export const fetchHeatmap = (from?: string, to?: string) => api.get('/board/heatmap', { params: { from, to } });
export const fetchStockHeatmap = (code: string) => api.get(`/board/heatmap/${code}`);
export const fetchSignal = (code: string) => api.get(`/stocks/${code}/signal`);

// Forecast & AI
export const fetchForecast = (code: string, horizon: number = 10) =>
  api.get(`/forecast/${code}`, { params: { horizon } });
export const postAIAnalyze = (code: string, question: string) =>
  api.post('/ai/analyze', { code, question });
export const postAIStream = (code: string, question: string) =>
  fetch('http://127.0.0.1:8080/api/v1/ai/analyze/stream', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code, question }),
  });

// Settings
export const fetchAISettings = () => api.get('/settings/ai');
export const saveAISettings = (data: any) => api.put('/settings/ai', data);
export const testAIConnection = (data: any) => api.post('/settings/ai/test', data);

// Prediction
export const runPrediction = (code: string, horizon: number = 10) =>
  api.post(`/prediction/${code}?horizon=${horizon}`);
export const runPredictionBatch = (codes: string[], horizon: number = 10) =>
  api.post('/prediction/batch', { codes, horizon });
export const fetchPredictionResult = (code: string) => api.get(`/prediction/${code}`);

// Import
export const uploadExcel = (file: File) => {
  const form = new FormData();
  form.append('file', file);
  return api.post('/import/excel', form, { headers: { 'Content-Type': 'multipart/form-data' } });
};
export const fetchImportHistory = () => api.get("/import/history");

// Collector
export const triggerCollection = (phases?: string[]) => api.post('/collector/trigger', { phases });
export const fetchCollectorProgress = () => api.get("/collector/status");
export const fetchCollectorHistory = () => api.get("/collector/history");

// Watchlist
export const fetchWatchlist = (userId: number = 1) => api.get('/watchlist', { params: { userId } });
export const addWatchlist = (stockCode: string, userId: number = 1) => api.post('/watchlist', { stockCode, userId });
export const removeWatchlist = (stockCode: string, userId: number = 1) => api.delete(`/watchlist/${stockCode}`, { params: { userId } });

// Financial / Shareholder / News
export const fetchFinancials = (code: string) => api.get(`/stocks/${code}/financials`);
export const fetchShareholders = (code: string) => api.get(`/stocks/${code}/shareholders`);
export const fetchStockNews = (code: string, limit: number = 20) => api.get(`/stocks/${code}/news`, { params: { limit } });

export const fetchReports = (code: string, limit = 20) => api.get(`/stocks/${code}/reports?limit=${limit}`);
export const fetchIndustryReports = (industry: string, limit = 20) => api.get(`/reports/industry?industry=${encodeURIComponent(industry)}&limit=${limit}`);
export const fetchIndices = () => api.get('/indices');
