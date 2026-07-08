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
export const fetchCostLogs = (params?: any) => api.get("/admin/cost-logs", { params });
export const fetchCostSummary = (params?: any) => api.get("/admin/cost-summary", { params });
export const fetchModelPrices = () => api.get("/admin/model-prices");
export const updateModelPrice = (modelName: string, data: any) => api.put(`/admin/model-prices/${modelName}`, data);

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
// Market snapshot (public)
export const fetchMarketSnapshot = () => api.get("/stocks/market-snapshot");

// Stock ranking
export const fetchStockRanking = (params?: any) => api.get("/stocks/ranking", { params });

// Unusual activity
export const fetchAppearanceStats = (params?: any) => api.get("/stocks/appearance-stats", { params });

// Market Style
export const fetchMarketStyleCurve = (params?: any) => api.get("/market/style-curve", { params });
export const fetchMarketDailyReview = (params?: any) => api.get("/market/daily-review", { params });
export const fetchMarketLatestStyle = () => api.get("/market/latest-style");

export const fetchUnusualStocks = (params?: any) => api.get("/stocks/unusual", { params });

// Board type counts
export const fetchBoardTypeCounts = () => api.get("/stocks/board-type-counts");

export const fetchStocks = (params?: any) => api.get('/stocks', { params });
export const fetchStockDetail = (code: string) => api.get(`/stocks/${code}`);
export const fetchKLine = (code: string, from?: string, to?: string) => api.get(`/stocks/${code}/kline`, { params: { from, to } });
export const fetchIndexKLine = (code: string, from?: string, to?: string) => api.get(`/sentiment/index-kline/${code}`, { params: { from, to } });
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

export const uploadKline = (file: File) => {
  const fd = new FormData();
  fd.append('file', file);
  return api.post('/import/kline', fd, { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 120000 });
};

export const uploadProfile = (file: File) => {
  const fd = new FormData();
  fd.append('file', file);
  return api.post('/import/profile', fd, { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 120000 });
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
export const searchStock = (keyword: string) => api.get("/stocks", { params: { keyword, pageSize: 20 } });


// ── Strategy ──
export const fetchStrategies = () => api.get("/strategies");
export const createStrategy = (name: string) => api.post("/strategies", { name });
export const updateStrategy = (id: number, data: any) => api.put(`/strategies/${id}`, data);
export const deleteStrategy = (id: number) => api.delete(`/strategies/${id}`);
export const reorderStrategies = (ids: number[]) => api.put("/strategies/reorder", { ids });
export const fetchStrategyConditions = (id: number) => api.get(`/strategies/${id}/conditions`);
export const saveStrategyConditions = (id: number, conditions: any[]) => api.put(`/strategies/${id}/conditions`, { conditions });
export const aiGenerateStrategy = (data: any) => api.post("/strategies/ai-generate", data, { timeout: 120000 });
export const optimizePrompt = (prompt: string, style: string) => api.post("/strategies/optimize-prompt", { prompt, style });
export const fetchIndicators = () => api.get("/strategies/indicators");

// ── Strategy Orchestration v2 ──
export const fetchOrchestration = (id: number) => api.get(`/strategies/${id}/orchestration`);
export const saveOrchestration = (id: number, data: any) => api.put(`/strategies/${id}/orchestration`, data);
export const fetchConditionTemplates = () => api.get("/strategies/templates");
export const createConditionTemplate = (data: any) => api.post("/strategies/templates", data);
export const fetchAIDecisions = (id: number) => api.get(`/strategies/${id}/ai-decisions`);
export const triggerAIReview = (id: number) => api.post(`/strategies/${id}/ai-review`);

export const fetchIndicatorGuide = () => api.get("/strategies/indicator-guide");
export const testIndicator = (data: any) => api.post("/strategies/test-indicator", data);
export const runBacktest = (id: number, startDate: string, endDate: string, stockCodes?: string[]) => api.post(`/strategies/${id}/backtest`, { startDate, endDate, stockCodes });
export const fetchBacktestHistory = (strategyId?: number) => api.get("/strategies/backtest-history", { params: strategyId ? { strategyId } : {} });
export const fetchBacktestResult = (id: number) => api.get(`/strategies/backtest-history/${id}`);

export const startBacktest = (id: number, startDate: string, endDate: string, stockCodes?: string[], stockPool?: string, engine?: string) => api.post(`/strategies/${id}/backtest/start`, { startDate, endDate, stockCodes, stockPool, engine });
export const getBacktestStatus = (id: number, taskId: number) => api.get(`/strategies/${id}/backtest/status/${taskId}`);
export const cancelBacktest = (id: number, taskId: number) => api.post(`/strategies/${id}/backtest/cancel/${taskId}`);
export const fetchBacktestTasks = (id: number) => api.get(`/strategies/${id}/backtest/tasks`);
export const deleteBacktestTask = (strategyId: number, taskId: number) => api.delete(`/strategies/${strategyId}/backtest/tasks/${taskId}`);
export const fetchBacktestTaskLogs = (strategyId: number, taskId: number, afterSeq?: number) => api.get(`/strategies/${strategyId}/backtest/tasks/${taskId}/logs`, { params: afterSeq !== undefined ? { afterSeq } : {} });
export const fetchTaskSnapshots = (strategyId: number, taskId: number, limit?: number) => api.get(`/strategies/${strategyId}/backtest/tasks/${taskId}/snapshots`, { params: limit ? { limit } : {} });
export const fetchStockAnalysis = (strategyId: number, taskId: number) => api.get(`/strategies/${strategyId}/backtest/tasks/${taskId}/stock-analysis`);
export const deleteBacktestResult = (id: number) => api.delete(`/strategies/backtest-history/${id}`);
export const fetchStockPool = () => api.get('/strategies/stock-pool');

// ── Live Trading (实盘交易) ──
// Multi-account
export const fetchLiveAccounts = () => api.get('/live/accounts');
export const createLiveAccount = (data: { name: string; broker?: string; accountType?: string; accountNumber?: string; initialCapital?: number }) =>
  api.post('/live/accounts', data);
export const updateLiveAccount = (id: number, data: Record<string, any>) => api.put(`/live/accounts/${id}`, data);
export const deleteLiveAccount = (id: number) => api.delete(`/live/accounts/${id}`);
export const fetchLiveAccount = () => api.get('/live/account');
// Broker integration
export const syncFromBroker = (accountId: number) => api.post(`/live/accounts/${accountId}/sync`);
export const getBrokerStatus = (accountId: number) => api.get(`/live/accounts/${accountId}/broker-status`);
export const getBrokerOrders = (accountId: number) => api.get(`/live/accounts/${accountId}/broker-orders`);
export const placeBrokerOrder = (accountId: number, data: { stockCode: string; orderType: string; price: number; quantity: number; useMarketPrice: boolean }) => api.post(`/live/accounts/${accountId}/broker-order`, data);
export const cancelBrokerOrder = (accountId: number, data: { orderId: string; stockCode: string }) => api.post(`/live/accounts/${accountId}/broker-cancel`, data);


// Strategy runs
export const createLiveRun = (data: { strategyId: number; accountId?: number; name?: string; initialCapital?: number; pctOfAccount?: number; stockPool?: string; startDate?: string; notifyEnabled?: boolean; notifyConfigs?: { channel: string; name: string; webhookUrl: string }[] }) =>
  api.post('/live/runs', data);
export const fetchLiveRuns = (strategyId?: number) => {
  const params = strategyId ? '?strategy_id=' + strategyId : '';
  return api.get('/live/runs' + params);
};
export const fetchLiveRun = (id: number) => api.get(`/live/runs/${id}`);
export const updateLiveRunStatus = (id: number, status: string) => api.put(`/live/runs/${id}/status`, { status });
export const updateLiveRunConfig = (id: number, config: { autoDailyCron?: string; autoTradeExecCron?: string; aiReviewEnabled?: boolean; notifyEnabled?: boolean; notifyChannels?: string; executionMode?: string }) => api.put(`/live/runs/${id}/config`, config);

// Daily execution
// Daily execution (async — returns task ID immediately)
export const runLiveDaily = (tradeDate: string, mode?: string, runId?: number) => api.post('/live/daily-run', { tradeDate, mode: mode || 'after_close', runId }, { timeout: 10000 });
export const fetchDailyRunTask = (id: number) => api.get(`/live/daily-run/tasks/${id}`);
export const fetchLatestDailyRunTask = (tradeDate?: string, runId?: number) => api.get(`/live/daily-run/tasks/latest`, { params: { ...(tradeDate ? { tradeDate } : {}), ...(runId ? { runId } : {}) } });

// Signal execution
export const executeLiveSignal = (signalId: number, body?: { action?: string; actualPrice?: number; actualQty?: number }) => api.post(`/live/signals/${signalId}/execute`, body || {});

export const syncSignalOrder = (signalId: number) => api.post(`/live/signals/${signalId}/sync-order`);

// Batch sync all pending orders, optional runId filter
export const syncOrders = (runId?: number) => api.post('/live/order-sync', {}, { params: runId ? { runId } : {} });
export const reconcileFromBroker = (runId: number, accountId: number) => api.post('/live/reconcile', {}, { params: { runId, accountId } });
export const fetchReconciliation = (runId: number) => api.get(`/live/runs/${runId}/reconciliation`);

// Pre-market
export const runTradeExec = (tradeDate: string, skipAi?: boolean, runId?: number, force?: boolean) => api.post(`/live/runs/${runId}/trade-exec`, { tradeDate, skipAi, force }, { timeout: 120000 });
export const fetchTradeExecTask = (id: number) => api.get(`/live/trade-exec/tasks/${id}`);
export const fetchLatestTradeExecTask = (tradeDate?: string, runId?: number) => api.get('/live/trade-exec/tasks/latest', { params: { ...(tradeDate ? { tradeDate } : {}), ...(runId ? { runId } : {}) } });
export const fetchTradeExecDecisions = (tradeDate?: string) => api.get('/live/trade-exec/decisions', { params: tradeDate ? { tradeDate } : {} });

// Execution logs
export const fetchRunLogs = (id: number, date?: string) => api.get(`/live/runs/${id}/logs`, { params: date ? { date } : {} });

// Notification configs
export const fetchNotificationConfigs = () => api.get('/live/notification-configs');
export const createNotificationConfig = (data: { channel: string; name: string; config: Record<string, any> }) =>
  api.post('/live/notification-configs', data);
export const updateNotificationConfig = (id: number, data: Record<string, any>) =>
  api.put(`/live/notification-configs/${id}`, data);
export const deleteNotificationConfig = (id: number) => api.delete(`/live/notification-configs/${id}`);

/** 发送测试消息到指定通知渠道 */
export const testNotification = (ncid: number) => api.post(`/live/notify-configs/${ncid}/test`);

// Positions & trades
export const fetchLivePositions = (runId: number) => api.get(`/live/runs/${runId}/positions`);
export const fetchLiveTrades = (runId: number) => api.get(`/live/runs/${runId}/trades`);
export const fetchLiveSnapshots = (runId: number) => api.get(`/live/runs/${runId}/snapshots`);
export const sendLiveRunNotification = (runId: number) => api.post(`/live/runs/${runId}/notify`);
export const updateLiveSignal = (id: number, data: { plannedPrice?: number; plannedQty?: number; reason?: string }) => api.put(`/live/signals/${id}`, data);
export const clearLiveSignals = (runId: number, date: string) => api.delete(`/live/runs/${runId}/signals?date=${encodeURIComponent(date)}`);
export const deleteLiveSignal = (id: number) => api.delete(`/live/signals/${id}`);

// ── Holdings ──
export const fetchHoldingsSummary = (accountType?: string) => api.get("/holdings/summary", { params: accountType ? { accountType } : {} });
export const fetchHoldings = (accountId?: number, accountType?: string) => api.get("/holdings", { params: { ...(accountId ? { accountId } : {}), ...(accountType ? { accountType } : {}) } });
export const fetchAccountsOverview = (accountType?: string) => api.get("/holdings/accounts-overview", { params: accountType ? { accountType } : {} });
export const createHolding = (stockCode: string, costPrice: number, quantity: number, buyDate?: string, accountId?: number) =>
  api.post("/holdings", { stockCode, costPrice, quantity, buyDate, accountId });
export const updateHolding = (id: number, costPrice: number, quantity: number, buyDate?: string) =>
  api.put(`/holdings/${id}`, { costPrice, quantity, buyDate });
export const deleteHolding = (id: number) => api.delete(`/holdings/${id}`);
export const fetchHoldingAccounts = () => api.get("/holdings/account");
export const updateAccount = (action: string, amount: number, accountId?: number) =>
  api.put("/holdings/account", { action, amount, accountId });
export const fetchTradeRecords = () => api.get("/holdings/trades");

// ── Risk APIs ──
export const fetchRiskAlerts = () => api.get('/risks');
export const ignoreRiskAlert = (id: number) => api.put(`/risks/${id}/ignore`);
export const triggerRiskScan = () => api.post('/admin/risks/scan');


// ── Scheduler v2 (统一调度中心) ──
export const fetchSchedulerDefinitions = (kind?: string) => api.get('/admin/scheduler/v2/definitions', { params: kind ? { kind } : {} });
export const fetchSchedulerTasks = (definitionId?: string, ownerKind?: string, ownerId?: number) => {
  const params: any = {};
  if (definitionId !== undefined) params.definitionId = definitionId;
  if (ownerKind !== undefined) params.ownerKind = ownerKind;
  if (ownerId !== undefined) params.ownerId = ownerId;
  return api.get('/admin/scheduler/v2/tasks', { params });
};
export const triggerSchedulerTask = (id: number) => api.post(`/admin/scheduler/v2/tasks/${id}/trigger`);
export const fetchSchedulerHealth = () => api.get('/admin/scheduler/v2/health');
export const fetchSchedulerHistory = (definitionId?: string, limit?: number) => api.get('/admin/scheduler/v2/history', { params: { definitionId, limit } });
export const fetchSchedulerQueues = () => api.get('/admin/scheduler/v2/queues');
export const fetchSchedulerAlerts = () => api.get('/admin/scheduler/v2/alerts');
export const clearSchedulerAlerts = () => api.delete('/admin/scheduler/v2/alerts');
export const fetchSchedulerReadiness = () => api.get('/admin/scheduler/v2/readiness');

// ── Scheduled Tasks ──
export const fetchScheduledTasks = () => api.get('/admin/scheduled-tasks');
export const createScheduledTask = (data: any) => api.post('/admin/scheduled-tasks', data);
export const updateScheduledTask = (id: number, data: any) => api.put(`/admin/scheduled-tasks/${id}`, data);
export const deleteScheduledTask = (id: number) => api.delete(`/admin/scheduled-tasks/${id}`);
export const runTaskNow = (id: number, args?: string[]) => api.post(`/admin/scheduled-tasks/${id}/run`, { args: args || [] });
export const toggleTask = (id: number) => api.post(`/admin/scheduled-tasks/${id}/toggle`);
export const resetTaskStatus = (id: number) => api.post(`/admin/scheduled-tasks/${id}/reset`);
export const initDefaultTasks = () => api.post('/admin/scheduled-tasks/init-defaults');
export const bulkComputeMarketStyle = () => api.post('/market/bulk-compute');
export const fetchTaskLogs = (taskId?: number, limit?: number) => api.get('/admin/task-logs', { params: { taskId, limit } });

// ── Collector APIs ──
export const triggerCollection = (phases?: string[]) => api.post('/collector/trigger', { phases });
export const fetchCollectorProgress = () => api.get('/collector/status');
export const fetchCollectorHistory = () => api.get('/collector/history');
export const clearCollectorHistory = (type?: string) => api.delete("/collector/history/clear", { params: { type } });
export const fetchDataStats = () => api.get('/admin/data-stats');
export const fetchDataDetail = (type: string) => api.get(`/admin/data-stats/${type}/detail`);
export const collectSingleStock = (code: string, phases?: string[]) => api.post(`/collector/stock/${code}`, { phases });
export const fetchRealtimeQuotes = () => api.post('/collector/realtime');
export const fetchRealtimeQuoteSingle = (code: string) => api.post(`/collector/realtime/${code}`);

// ── AI APIs ──
export const aiAnalyze = (code: string, question: string) => api.post('/ai/analyze', { code, question });
export const aiStreamUrl = () => '/api/v1/ai/analyze/stream';
export const fetchAIHistory = (code: string) => api.get(`/ai/history/${code}`);
export const clearAIHistory = (code: string) => api.delete(`/ai/history/${code}`);
export const fetchAIScore = (code: string) => api.get(`/ai/score/${code}`);
export const runAIScore = (code: string) => api.post(`/ai/score/${code}`);
export const fetchProfile = (code: string) => api.get(`/ai/profile/${code}`);
export const runProfile = (code: string) => api.post(`/ai/profile/${code}`);
export const runProfileBatch = () => api.post('/ai/profile-batch');
export const repairStock = (code: string) => api.post(`/stocks/${code}/repair`);

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


// ── Concept Board APIs ──
export const fetchConceptBoards = (type?: string) => api.get('/concept-boards', { params: type ? { type } : {} });
export const fetchConceptBoardStocks = (code: string) => api.get(`/concept-boards/${code}/stocks`);
export const fetchConceptAnalysis = (code: string, refresh?: boolean) => api.get(`/concept-boards/${code}/analysis`, { params: { refresh: refresh ? '1' : '0' }, timeout: 180000 });
export const fetchConceptBoardKline = (code: string, days?: number) => api.get(`/concept-boards/${code}/kline`, { params: { days: days || 60 } });
export const fetchConceptHeatmap = () => api.get('/concept-boards/heatmap');
export const fetchIndustryHeatmap = () => api.get('/industry/heatmap');
export const fetchStockConceptTags = (stockCode: string) => api.get(`/stocks/${stockCode}/concept-tags`);

// ── Index / Market APIs ──
export const fetchIndices = () => api.get('/indices');

// ── Industry Comparison APIs ──
export const fetchIndustries = (date?: string, industryType?: string) => api.get("/industries", { params: { ...(date ? { date } : {}), ...(industryType ? { type: industryType } : {}) } });
export const fetchIndustryStocks = (name: string, date?: string, sort?: string, industryType?: string) => api.get(`/industries/${encodeURIComponent(name)}/stocks`, { params: { ...(date ? { date } : {}), ...(sort ? { sort } : {}), ...(industryType ? { type: industryType } : {}) } });

// ── Backward-compatible aliases ──
export const fetchEnrichedHeatmap = () => fetchHeatmapEnriched();
export const fetchForecast = (code: string, horizon?: number) => api.get(`/forecast/${code}`, { params: horizon ? { horizon } : {} });




export const fetchDailyDragonTigerEnriched = (date: string) => api.get('/dragon-tiger/enriched', { params: { date } });
export const fetchDailyDragonTiger = (date?: string) => api.get('/dragon-tiger', { params: { date } });
export const fetchDragonTigerSeats = (code: string, date: string) => api.get(`/dragon-tiger/${code}/seats`, { params: { date } });

export const fetchDragonTiger = (code: string) => api.get(`/stocks/${code}/dragon-tiger`);
export const fetchBlockTrades = (code: string) => api.get(`/stocks/${code}/block-trades`);
export const fetchAnnouncements = (code: string) => api.get(`/stocks/${code}/announcements`);
export const fetchUnlocks = (code: string) => api.get(`/stocks/${code}/unlocks`);
export const fetchAllFutureUnlocks = (days: number = 90) => api.get('/unlocks', { params: { days } });
export const fetchFundFlow = (code: string) => api.get(`/stocks/${code}/fund-flow`);
export const fetchFundFlowMinute = (code: string) => api.get(`/stocks/${code}/fund-flow-minute`);
export const fetchBuySellFlow = (code: string) => api.get(`/stocks/${code}/buy-sell-flow`);
export const fetchEpsForecast = (code: string) => api.get(`/stocks/${code}/eps-forecast`);
export const fetchMacroNews = (category: string = '', limit: number = 50) => api.get('/macro-news', { params: { category, limit } });
export const fetchMacroCategories = () => api.get('/macro-news/categories');
export const fetchThsHotConcepts = (days: number = 7) => api.get('/ths-hot-concepts', { params: { days } });

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

// ── Market Sentiment ──
export const fetchLatestSentiment = () => api.get('/sentiment/latest');
export const fetchSentimentHistory = (days: number = 90) => api.get('/sentiment/history', { params: { days } });
export const fetchSentimentDetail = (date: string) => api.get('/sentiment/detail', { params: { date } });
export const fetchSentimentRange = (start: string, end: string) => api.get('/sentiment/range', { params: { start, end } });
export const fetchNorthbound = (days: number = 30) => api.get('/northbound', { params: { days } });

export const fetchLimitStats = (days?: number) => api.get("/sentiment/limit-stats", { params: { days: days || 60 } });

export const fetchReturnDistribution = () => api.get('/sentiment/distribution');

// ── Risk Dashboard APIs (new) ──
export const fetchRiskDashboard = () => api.get('/risk/dashboard');
export const fetchRiskAlertList = (params?: {
  page?: number; pageSize?: number; level?: string; dimension?: string; status?: string;
}) => api.get('/risk/alerts', { params });
export const fetchRiskAlertDetail = (id: number) => api.get(`/risk/alerts/${id}`);
export const acknowledgeRiskAlert = (id: number) => api.put(`/risk/alerts/${id}/acknowledge`);
export const fetchRiskRules = () => api.get('/risk/rules');
export const updateRiskRule = (key: string, data: { enabled?: boolean; thresholds?: any; weight?: number }) =>
  api.put(`/risk/rules/${key}`, data);
export const fetchRiskSnapshots = (days?: number) => api.get('/risk/snapshots', { params: { days } });
export const fetchRiskAggregated = () => api.get("/risk/aggregated");
export const fetchCircuitBreakerStatus = () => api.get('/risk/circuit-breaker');
