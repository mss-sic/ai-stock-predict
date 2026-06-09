import axios from 'axios';

const api = axios.create({ baseURL: '/api/v1', timeout: 30000 });

// ── Token management ──
const TOKEN_KEY = 'aip_access_token';
const REFRESH_KEY = 'aip_refresh_token';

export function getAccessToken() { return localStorage.getItem(TOKEN_KEY); }
export function setTokens(access: string, refresh: string) {
  localStorage.setItem(TOKEN_KEY, access);
  localStorage.setItem(REFRESH_KEY, refresh);
}
export function clearTokens() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

// ── Request interceptor: attach token ──
api.interceptors.request.use((config) => {
  const token = getAccessToken();
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

// ── Response interceptor: handle 401 → refresh → retry ──
let isRefreshing = false;
let refreshSubscribers: ((token: string) => void)[] = [];

function onRefreshed(token: string) {
  refreshSubscribers.forEach(cb => cb(token));
  refreshSubscribers = [];
}

api.interceptors.response.use(
  (res) => {
    // Normalize response: if backend returns {code, message, data}
    const body = res.data;
    if (body && typeof body === 'object' && 'code' in body && body.code !== 0) {
      // Business error — show message and reject
      const msg = body.message || ('服务异常 (code=' + body.code + ')');
      console.warn('[api] business error:', body.code, msg, res.config?.url);
      window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'error', message: msg } }));
      const err = new Error(msg);
      (err as any).code = body.code;
      (err as any).response = res;
      return Promise.reject(err);
    }
    return res;
  },
  async (error) => {
    const original = error.config;
    // Normalize error message from backend response
    const errMsg = error.response?.data?.message || error.response?.data?.error;
    if (errMsg) {
      console.warn('[api] http error:', error.response?.status, errMsg, error.config?.url);
      error.message = errMsg;
      (error as any).code = error.response?.data?.code;
      // Only show toast for real errors (skip 401 auth errors + 404 not-found)
      if (error.response?.status !== 401 && error.response?.status !== 404) {
        window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'error', message: errMsg } }));
      }
    }
    if (error.response?.status === 401 && !original._retry) {
      const refreshToken = localStorage.getItem(REFRESH_KEY);
      if (!refreshToken) {
        clearTokens();
        window.dispatchEvent(new Event('auth:logout'));
        return Promise.reject(error);
      }

      if (isRefreshing) {
        return new Promise((resolve) => {
          refreshSubscribers.push((token: string) => {
            original.headers.Authorization = `Bearer ${token}`;
            resolve(api(original));
          });
        });
      }

      original._retry = true;
      isRefreshing = true;
      try {
        const { data } = await axios.post('/api/v1/auth/refresh', { refreshToken });
        setTokens(data.data.accessToken, data.data.refreshToken);
        onRefreshed(data.data.accessToken);
        original.headers.Authorization = `Bearer ${data.data.accessToken}`;
        return api(original);
      } catch {
        clearTokens();
        window.dispatchEvent(new Event('auth:logout'));
        return Promise.reject(error);
      } finally {
        isRefreshing = false;
      }
    }
    return Promise.reject(error);
  }
);

export default api;

// ── Auth APIs ──
export const login = (username: string, password: string) =>
  api.post('/auth/login', { username, password });

export const refreshToken = (refreshToken: string) =>
  api.post('/auth/refresh', { refreshToken });

export const logout = (refreshToken?: string) =>
  api.post('/auth/logout', { refreshToken });

export const getMe = () => api.get('/auth/me');

export const updateProfile = (nickname: string) =>
  api.put("/auth/profile", { nickname });

export const changePassword = (oldPassword: string, newPassword: string) =>
  api.post('/auth/change-password', { oldPassword, newPassword });

export const getMySessions = () => api.get('/auth/sessions');

export const revokeSession = (id: number) => api.delete(`/auth/sessions/${id}`);

export const heartbeat = () => api.post('/auth/heartbeat');

export const fetchLoginLogs = (params?: any) => api.get('/admin/login-logs', { params });

export const kickUser = (userId: number) => api.post('/admin/users/kick', { userId });

// Admin

export const listUsers = () => api.get('/admin/users');

export const createUser = (username: string, password: string, role: string) =>
  api.post('/admin/users', { username, password, role });

export const resetUserPassword = (userId: number, newPassword: string) =>
  api.post('/admin/users/reset-password', { userId, newPassword });

export const toggleUser = (userId: number, isActive: boolean) =>
  api.post('/admin/users/toggle', { userId, isActive });

// ── Stock APIs ──
export const fetchStocks = (params?: any) => api.get('/stocks', { params });
export const fetchStockDetail = (code: string) => api.get(`/stocks/${code}`);
export const fetchKLine = (code: string) => api.get(`/stocks/${code}/kline`);
export const fetchIndicator = (code: string) => api.get(`/stocks/${code}/indicator`);
export const fetchSignal = (code: string) => api.get(`/stocks/${code}/signal`);
export const fetchFinancials = (code: string) => api.get(`/stocks/${code}/financials`);
export const fetchShareholders = (code: string) => api.get(`/stocks/${code}/shareholders`);
export const fetchStockNews = (code: string) => api.get(`/stocks/${code}/news`);
export const fetchStockReports = (code: string) => api.get(`/stocks/${code}/reports`);
export const fetchIndustryReports = () => api.get('/reports/industry');
export const getReportPdfUrl = (infoCode: string) => `/api/v1/reports/pdf?code=${infoCode}`;

// ── Board APIs ──
export const fetchTodayBoard = () => api.get('/board/today');
export const fetchBoardDates = () => api.get('/board/dates');
export const fetchBoardHistory = (date: string) => api.get('/board/history', { params: { date } });
export const fetchHeatmap = () => api.get('/board/heatmap');
export const fetchHeatmapEnriched = () => api.get('/board/heatmap-enriched');
export const fetchStockHeatmap = (code: string) => api.get(`/board/heatmap/${code}`);

// ── Import APIs ──
export const uploadExcel = (file: File) => {
  const fd = new FormData();
  fd.append('file', file);
  return api.post('/import/excel', fd, { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 60000 });
};

export const uploadPrediction = (file: File) => {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      try {
        const json = JSON.parse(reader.result as string);
        api.post('/internal/predictions/sync', json, { params: { filename: file.name }, timeout: 120000 })
          .then(resolve).catch(reject);
      } catch (e) { reject(new Error('JSON 解析失败: ' + (e as Error).message)); }
    };
    reader.onerror = () => reject(new Error('文件读取失败'));
    reader.readAsText(file);
  });
};
export const fetchImportHistory = () => api.get('/import/history');

// ── Watchlist APIs ──
export const fetchWatchlist = (groupId?: number) => api.get('/watchlist', { params: groupId !== undefined ? { groupId } : {} });
export const addToWatchlist = (stockCode: string, groupId?: number, addedPrice?: number) =>
  api.post('/watchlist', { stockCode, groupId: groupId || 0, addedPrice: addedPrice || 0 });
export const removeFromWatchlist = (code: string) => api.delete(`/watchlist/${code}`);
export const clearWatchlist = (groupId?: number) => api.delete('/watchlist', { params: groupId !== undefined ? { groupId } : {} });
export const moveWatchlistStock = (code: string, groupId: number) => api.put(`/watchlist/${code}/move`, { groupId });

// Watchlist groups
export const fetchWatchlistGroups = () => api.get('/watchlist/groups');
export const createWatchlistGroup = (name: string) => api.post('/watchlist/groups', { name });
export const renameWatchlistGroup = (id: number, name: string) => api.put(`/watchlist/groups/${id}`, { name });
export const deleteWatchlistGroup = (id: number) => api.delete(`/watchlist/groups/${id}`);
export const reorderWatchlistGroups = (ids: number[]) => api.put('/watchlist/groups/reorder', { ids });

// ── Stock search ──
export const searchStock = (keyword: string) => api.get("/stocks", { params: { keyword, pageSize: 10 } });


// ── Strategy ──
export const fetchStrategies = () => api.get("/strategies");
export const createStrategy = (name: string) => api.post("/strategies", { name });
export const updateStrategy = (id: number, data: any) => api.put(`/strategies/${id}`, data);
export const deleteStrategy = (id: number) => api.delete(`/strategies/${id}`);
export const reorderStrategies = (ids: number[]) => api.put("/strategies/reorder", { ids });
export const fetchStrategyConditions = (id: number) => api.get(`/strategies/${id}/conditions`);
export const saveStrategyConditions = (id: number, conditions: any[]) => api.put(`/strategies/${id}/conditions`, { conditions });
export const aiGenerateStrategy = (data: any) => api.post("/strategies/ai-generate", data);
export const optimizePrompt = (prompt: string, style: string) => api.post("/strategies/optimize-prompt", { prompt, style });
export const fetchIndicators = () => api.get("/strategies/indicators");
export const testIndicator = (data: any) => api.post("/strategies/test-indicator", data);
export const runBacktest = (id: number, startDate: string, endDate: string, stockCodes?: string[]) => api.post(`/strategies/${id}/backtest`, { startDate, endDate, stockCodes });
export const fetchBacktestHistory = (strategyId?: number) => api.get("/strategies/backtest-history", { params: strategyId ? { strategyId } : {} });

export const startBacktest = (id: number, startDate: string, endDate: string, stockCodes?: string[], stockPool?: string) => api.post(`/strategies/${id}/backtest/start`, { startDate, endDate, stockCodes, stockPool });
export const getBacktestStatus = (id: number, taskId: number) => api.get(`/strategies/${id}/backtest/status/${taskId}`);
export const cancelBacktest = (id: number, taskId: number) => api.post(`/strategies/${id}/backtest/cancel/${taskId}`);
export const fetchBacktestTasks = (id: number) => api.get(`/strategies/${id}/backtest/tasks`);
export const deleteBacktestTask = (strategyId: number, taskId: number) => api.delete(`/strategies/${strategyId}/backtest/tasks/${taskId}`);
export const fetchBacktestTaskLogs = (strategyId: number, taskId: number) => api.get(`/strategies/${strategyId}/backtest/tasks/${taskId}/logs`);
export const fetchTaskSnapshots = (strategyId: number, taskId: number, limit?: number) => api.get(`/strategies/${strategyId}/backtest/tasks/${taskId}/snapshots`, { params: limit ? { limit } : {} });
export const deleteBacktestResult = (id: number) => api.delete(`/strategies/backtest-history/${id}`);
export const fetchStockPool = () => api.get('/strategies/stock-pool');


// ── Holdings ──
export const fetchHoldings = () => api.get("/holdings");
export const createHolding = (stockCode: string, costPrice: number, quantity: number) =>
  api.post("/holdings", { stockCode, costPrice, quantity });
export const updateHolding = (id: number, costPrice: number, quantity: number) =>
  api.put(`/holdings/${id}`, { costPrice, quantity });
export const deleteHolding = (id: number) => api.delete(`/holdings/${id}`);

// ── Risk APIs ──
export const fetchRiskAlerts = () => api.get('/risks');
export const ignoreRiskAlert = (id: number) => api.put(`/risks/${id}/ignore`);
export const triggerRiskScan = () => api.post('/admin/risks/scan');

// ── Scheduled Tasks ──
export const fetchScheduledTasks = () => api.get('/admin/scheduled-tasks');
export const createScheduledTask = (data: any) => api.post('/admin/scheduled-tasks', data);
export const updateScheduledTask = (id: number, data: any) => api.put(`/admin/scheduled-tasks/${id}`, data);
export const deleteScheduledTask = (id: number) => api.delete(`/admin/scheduled-tasks/${id}`);
export const runTaskNow = (id: number) => api.post(`/admin/scheduled-tasks/${id}/run`);
export const toggleTask = (id: number) => api.post(`/admin/scheduled-tasks/${id}/toggle`);
export const resetTaskStatus = (id: number) => api.post(`/admin/scheduled-tasks/${id}/reset`);
export const initDefaultTasks = () => api.post('/admin/scheduled-tasks/init-defaults');
export const fetchTaskLogs = (taskId?: number, limit?: number) => api.get('/admin/task-logs', { params: { taskId, limit } });

// ── Collector APIs ──
export const triggerCollection = (phases?: string[]) => api.post('/collector/trigger', { phases });
export const fetchCollectorProgress = () => api.get('/collector/status');
export const fetchCollectorHistory = () => api.get('/collector/history');
export const clearCollectorHistory = (type?: string) => api.delete("/collector/history/clear", { params: { type } });
export const fetchDataStats = () => api.get('/admin/data-stats');
export const fetchDataDetail = (type: string) => api.get(`/admin/data-stats/${type}/detail`);
export const collectSingleStock = (code: string, phases?: string[]) => api.post(`/collector/stock/${code}`, { phases });

// ── AI APIs ──
export const aiAnalyze = (code: string, question: string) => api.post('/ai/analyze', { code, question });
export const aiStreamUrl = () => '/api/v1/ai/analyze/stream';
export const fetchAIHistory = (code: string) => api.get(`/ai/history/${code}`);
export const clearAIHistory = (code: string) => api.delete(`/ai/history/${code}`);
export const fetchAIScore = (code: string) => api.get(`/ai/score/${code}`);
export const runAIScore = (code: string) => api.post(`/ai/score/${code}`);

// ── Settings APIs ──
export const fetchAIConfig = () => api.get('/settings/ai');
export const saveAIConfig = (config: any) => api.put('/settings/ai', config);
export const testAIConnection = (config: any) => api.post('/settings/ai/test', config);
export const listAIModels = (baseUrl: string, apiKey: string) => api.post('/settings/ai/models', { baseUrl, apiKey });

// ── Prediction APIs ──
export const runPrediction = (code: string) => api.post(`/prediction/${code}`);
export const runBatchPrediction = () => api.post('/prediction/batch');
export const fetchPredictionResult = (code: string) => api.get(`/prediction/${code}`);
export const fetchPredictionHitRate = (code: string) => api.get(`/prediction/${code}/hitrate`);

// ── Index / Market APIs ──
export const fetchIndices = () => api.get('/indices');

// ── Backward-compatible aliases ──
export const fetchEnrichedHeatmap = () => fetchHeatmapEnriched();
export const fetchForecast = (code: string, horizon?: number) => api.get(`/forecast/${code}`, { params: horizon ? { horizon } : {} });


export const fetchReports = (code: string) => fetchStockReports(code);

// ── Auth-aware fetch wrapper (for SSE and raw fetch calls) ──
// Wraps authFetch response JSON with automatic code/message checking
export async function checkAPIError(json: any) {
  if (json && typeof json === 'object' && json.code !== undefined && json.code !== 0) {
    const msg = json.message || '请求失败';
    window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'error', message: msg } }));
    throw new Error(msg);
  }
  return json;
}

export function authFetch(url: string, options: RequestInit = {}): Promise<Response> {
  const token = getAccessToken();
  const headers = new Headers(options.headers || {});
  if (token) headers.set('Authorization', `Bearer ${token}`);
  if (!headers.has('Content-Type') && !(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json');
  }
  return fetch(url, { ...options, headers });
}

// authFetchJSON — like authFetch but returns parsed JSON with automatic error handling
export async function authFetchJSON(url: string, options: RequestInit = {}): Promise<any> {
  const res = await authFetch(url, options);
  const json = await res.json();
  if (json && typeof json === 'object' && json.code !== undefined && json.code !== 0) {
    const msg = json.message || '请求失败';
    window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'error', message: msg } }));
    throw new Error(msg);
  }
  return json;
}
