import axios from 'axios';

const api = axios.create({
  baseURL: 'http://localhost:8080/api/v1',
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
export const fetchStockDetail = (code: string) => api.get(`/stocks/${code}`);
export const fetchKLine = (code: string, from?: string, to?: string) =>
  api.get(`/stocks/${code}/kline`, { params: { from, to } });
export const fetchIndicator = (code: string) => api.get(`/stocks/${code}/indicator`);

// Board
export const fetchTodayBoard = () => api.get('/board/today');
export const fetchHistoryBoard = (date: string) => api.get('/board/history', { params: { date } });
export const fetchHeatmap = (from?: string, to?: string) => api.get('/board/heatmap', { params: { from, to } });
export const fetchStockHeatmap = (code: string) => api.get(`/board/heatmap/${code}`);

// Forecast & AI
export const fetchForecast = (code: string, horizon: number = 10) =>
  api.get(`/forecast/${code}`, { params: { horizon } });
export const postAIAnalyze = (code: string, question: string) =>
  api.post('/ai/analyze', { code, question });

// Import
export const uploadExcel = (file: File) => {
  const form = new FormData();
  form.append('file', file);
  return api.post('/import/excel', form, { headers: { 'Content-Type': 'multipart/form-data' } });
};

// Collector
export const triggerCollection = () => api.post('/collector/trigger');
export const fetchCollectorStatus = () => api.get('/collector/status');
