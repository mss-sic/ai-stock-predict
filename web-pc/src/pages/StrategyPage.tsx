import { useState, useEffect, useCallback } from 'react';
import { Button, Input, InputNumber, Modal, Table, Select, Popconfirm, Tooltip, DatePicker, Message, Tag } from '@arco-design/web-react';
import { Target, Plus, Trash2, GripVertical, Play, Brain, BarChart4, TrendingUp, Shield, Settings, Sparkles, Beaker } from 'lucide-react';
import {
  fetchStrategies, createStrategy, updateStrategy, deleteStrategy, reorderStrategies,
  fetchStrategyConditions, saveStrategyConditions, aiGenerateStrategy, optimizePrompt,
  fetchIndicators, runBacktest, fetchBacktestHistory, testIndicator,
  startBacktest, getBacktestStatus, cancelBacktest, fetchBacktestTasks,
} from '../services/api';

type CondType = 'buy' | 'add' | 'sell' | 'reduce';
const COND_LABELS: Record<CondType, string> = { buy: '买入条件', add: '加仓条件', sell: '卖出条件', reduce: '减仓条件' };

// Cross-type indicator presets: semantic dropdown replaces manual operator+value input.
// Value encoding for ma_cross/ema_cross: int_part + frac_part/1000 (e.g. 5.020 = MA5×MA20)
const CROSS_PRESETS: Record<string, Array<{label: string, operator: string, value: number}>> = {
  ma_cross: [
    { label: 'MA5 上穿 MA10 (短线金叉)', operator: 'cross_up', value: 5.010 },
    { label: 'MA5 上穿 MA20 (金叉)', operator: 'cross_up', value: 5.020 },
    { label: 'MA5 上穿 MA30', operator: 'cross_up', value: 5.030 },
    { label: 'MA5 下穿 MA20 (死叉)', operator: 'cross_down', value: 5.020 },
    { label: 'MA10 上穿 MA20 (金叉)', operator: 'cross_up', value: 10.020 },
    { label: 'MA10 上穿 MA30', operator: 'cross_up', value: 10.030 },
    { label: 'MA10 下穿 MA20 (死叉)', operator: 'cross_down', value: 10.020 },
    { label: 'MA20 上穿 MA60 (中期金叉)', operator: 'cross_up', value: 20.060 },
    { label: 'MA20 下穿 MA60 (中期死叉)', operator: 'cross_down', value: 20.060 },
  ],
  ema_cross: [
    { label: 'EMA12 上穿 EMA26 (金叉)', operator: 'cross_up', value: 12.026 },
    { label: 'EMA12 下穿 EMA26 (死叉)', operator: 'cross_down', value: 12.026 },
    { label: 'EMA5 上穿 EMA20 (金叉)', operator: 'cross_up', value: 5.020 },
    { label: 'EMA5 下穿 EMA20 (死叉)', operator: 'cross_down', value: 5.020 },
  ],
  macd: [
    { label: 'MACD 金叉 (DIF↑DEA)', operator: 'cross_up', value: 0 },
    { label: 'MACD 死叉 (DIF↓DEA)', operator: 'cross_down', value: 0 },
  ],
};

const COND_COLORS: Record<CondType, string> = { buy: 'var(--stock-up)', add: 'var(--color-warning-text)', sell: 'var(--stock-down)', reduce: 'var(--color-info-text)' };

export default function StrategyPage() {
  const [strategies, setStrategies] = useState<any[]>([]);
  const [activeId, setActiveId] = useState<number | null>(null);
  const [activeStrategy, setActiveStrategy] = useState<any>(null);
  const [conditions, setConditions] = useState<any[]>([]);
  const [indicators, setIndicators] = useState<any[]>([]);
  const [tab, setTab] = useState<'conditions' | 'backtest'>('conditions');
  const [condTab, setCondTab] = useState<CondType>('buy');
  const [showAdd, setShowAdd] = useState(false);
  const [newName, setNewName] = useState('');
  const [showAIModal, setShowAIModal] = useState(false);
  const [aiStyle, setAiStyle] = useState('moderate');
  const [aiName, setAiName] = useState('');
  const [aiDesc, setAiDesc] = useState('');
  const [aiGenerating, setAiGenerating] = useState(false);
  const [aiOptimizing, setAiOptimizing] = useState(false);

  // Backtest state
  const [btStart, setBtStart] = useState('2025-01-01');
  const [btStocks, setBtStocks] = useState("");
  const [btEnd, setBtEnd] = useState('2026-06-05');
  const [btRunning, setBtRunning] = useState(false);
  const [btResult, setBtResult] = useState<any>(null);
  const [btHistory, setBtHistory] = useState<any[]>([]);
  const [btPositions, setBtPositions] = useState<any>(null);
  const [btLogs, setBtLogs] = useState<any[]>([]);
  const [btPhase, setBtPhase] = useState('');
  const [btProgress, setBtProgress] = useState('');
  const [btTaskId, setBtTaskId] = useState<number | null>(null);
  const [btOfflineMode, setBtOfflineMode] = useState(false);
  const [btTasks, setBtTasks] = useState<any[]>([]);
  const [btPollTimer, setBtPollTimer] = useState<any>(null);

  // Indicator test state
  const [testModalVisible, setTestModalVisible] = useState(false);
  const [testCond, setTestCond] = useState<any>(null);
  const [testStock, setTestStock] = useState('');
  const [testDate, setTestDate] = useState('');
  const [testResult, setTestResult] = useState<any>(null);
  const [testLoading, setTestLoading] = useState(false);

  const toast = (type: string, msg: string) => {
    window.dispatchEvent(new CustomEvent('app:toast', { detail: { type, message: msg } }));
  };

  const loadStrategies = useCallback(async () => {
    try {
      const { data: r } = await fetchStrategies();
      const list = r.data || [];
      setStrategies(list);
      if (list.length > 0 && !activeId) {
        setActiveId(list[0].id);
      }
    } catch {}
  }, [activeId]);

  const loadIndicators = useCallback(async () => {
    try {
      const { data: r } = await fetchIndicators();
      setIndicators(r.data || []);
    } catch {}
  }, []);

  useEffect(() => { loadStrategies(); loadIndicators(); }, []);

  // Load active strategy details
  useEffect(() => {
    if (!activeId) return;
    const s = strategies.find(s => s.id === activeId);
    setActiveStrategy(s || null);
    if (s) {
      fetchStrategyConditions(s.id).then(({ data: r }: any) => setConditions(r.data || [])).catch(() => {});
      fetchBacktestHistory(s.id).then(({ data: r }: any) => setBtHistory(r.data || [])).catch(() => {});
      fetchBacktestTasks(s.id).then(({ data: r }: any) => setBtTasks(r.data || [])).catch(() => {});
    }
  }, [activeId, strategies]);

  const handleAdd = async () => {
    if (!newName.trim()) return;
    try {
      const { data: r } = await createStrategy(newName.trim());
      setActiveId(r.data.id);
      setShowAdd(false);
      setNewName('');
      loadStrategies();
      toast('success', '策略已创建');
    } catch {}
  };

  const handleDelete = async (id: number) => {
    try {
      await deleteStrategy(id);
      if (activeId === id) {
        const remaining = strategies.filter(s => s.id !== id);
        setActiveId(remaining.length > 0 ? remaining[0].id : null);
      }
      loadStrategies();
      toast('success', '已删除');
    } catch {}
  };

  const handleUpdateStrategy = async (field: string, value: any) => {
    if (!activeId) return;
    try {
      await updateStrategy(activeId, { [field]: value });
      loadStrategies();
    } catch {}
  };

  const filteredConds = (t: CondType) => conditions.filter((c: any) => c.condType === t);

  const addCondition = (ct: CondType) => {
    setConditions([...conditions, {
      id: -(Date.now()),
      strategyId: activeId,
      condType: ct,
      indicator: 'algo_score',
      operator: 'gte',
      value: 0,
      logicGroup: 1,
      sortOrder: filteredConds(ct).length,
    }]);
  };

  // Smart defaults per indicator: operator + value when user selects a new indicator
  const getIndicatorDefaults = (key: string): { operator: string; value: number } => {
    const meta = indicators.find((i: any) => i.key === key);
    if (!meta || meta.type === 'cross') return { operator: 'cross_up', value: 0 };
    switch (key) {
      // Scores: ≥6
      case 'algo_score': case 'ai_score': case 'ai_fundamental': case 'ai_technical':
      case 'ai_valuation': case 'ai_growth': case 'ai_industry': case 'ai_capital':
        return { operator: 'gte', value: 6 };
      // RSI / KDJ / MFI / PSY: >70 overbought or <30 oversold
      case 'rsi': case 'rsi_6': case 'rsi_12': case 'rsi_24':
        return { operator: 'gte', value: 70 };
      case 'kdj_k': case 'kdj_d':
        return { operator: 'gte', value: 80 };
      case 'kdj_j':
        return { operator: 'gte', value: 90 };
      case 'mfi':
        return { operator: 'gte', value: 80 };
      case 'psy_12': case 'psy_ma':
        return { operator: 'gte', value: 60 };
      // Oversold: <30
      case 'williams_r':
        return { operator: 'lte', value: -80 };
      // CCI
      case 'cci':
        return { operator: 'gte', value: 100 };
      // Trend strength
      case 'adx':
        return { operator: 'gte', value: 25 };
      case 'dmi_plus': case 'dmi_minus':
        return { operator: 'gte', value: 20 };
      // Bollinger
      case 'boll_position':
        return { operator: 'gte', value: 80 };
      case 'boll_width':
        return { operator: 'gte', value: 10 };
      case 'boll_squeeze':
        return { operator: 'lte', value: 1 };
      case 'boll_upper': case 'boll_middle': case 'boll_lower':
        return { operator: 'gte', value: 0 };
      // MACD values
      case 'macd_dif': case 'macd_dea':
        return { operator: 'gte', value: 0 };
      // Momentum & daily change
      case 'daily_change':
        return { operator: 'gte', value: 2 };
      case 'momentum_5':
        return { operator: 'gte', value: 3 };
      case 'momentum_20':
        return { operator: 'gte', value: 5 };
      // MA values
      case 'ma_5': case 'ma_10': case 'ma_20': case 'ma_30': case 'ma_60':
        return { operator: 'gte', value: 0 };
      case 'ma_deviation':
        return { operator: 'gte', value: 5 };
      // Volume
      case 'volume_ratio':
        return { operator: 'gte', value: 2 };
      case 'volume_ma_ratio':
        return { operator: 'gte', value: 1.5 };
      case 'turnover_rate':
        return { operator: 'gte', value: 5 };
      case 'volume_trend':
        return { operator: 'gte', value: 1 };
      // ATR
      case 'atr':
        return { operator: 'lte', value: 0.5 };
      case 'atr_pct':
        return { operator: 'lte', value: 3 };
      // Drawdown / New high / Up days
      case 'drawdown_20':
        return { operator: 'gte', value: -10 };
      case 'new_high_20':
        return { operator: 'eq', value: 1 };
      case 'up_days_ratio':
        return { operator: 'gte', value: 60 };
      // Price position
      case 'price_position_20': case 'price_position_60':
        return { operator: 'gte', value: 70 };
      case 'gap_pct':
        return { operator: 'gte', value: 3 };
      case 'high_low_range':
        return { operator: 'lte', value: 5 };
      // Convergence / Trend
      case 'ma_convergence':
        return { operator: 'lte', value: 3 };
      case 'trend_strength':
        return { operator: 'gte', value: 2 };
      // Count-based
      case 'streak_count': case 'consecutive_days':
        return { operator: 'gte', value: 3 };
      case 'signal_value':
        return { operator: 'gte', value: 0.5 };
      // Valuation
      case 'pe':
        return { operator: 'lte', value: 20 };
      case 'pb':
        return { operator: 'lte', value: 2 };
      case 'ps':
        return { operator: 'lte', value: 2 };
      case 'pe_percentile': case 'pb_percentile':
        return { operator: 'lte', value: 30 };
      // Fundamentals
      case 'roe':
        return { operator: 'gte', value: 15 };
      case 'revenue_growth':
        return { operator: 'gte', value: 10 };
      case 'profit_growth':
        return { operator: 'gte', value: 15 };
      case 'gross_margin':
        return { operator: 'gte', value: 30 };
      case 'net_margin':
        return { operator: 'gte', value: 10 };
      case 'debt_ratio':
        return { operator: 'lte', value: 60 };
      case 'eps':
        return { operator: 'gte', value: 0.5 };
      // Market cap / shareholders
      case 'total_market_cap':
        return { operator: 'gte', value: 10000000000 }; // 100亿
      case 'shareholder_change':
        return { operator: 'lte', value: -5 };
      case 'inst_hold_ratio':
        return { operator: 'gte', value: 30 };
      // Prediction
      case 'prediction_upside':
        return { operator: 'gte', value: 10 };
      case 'prediction_consensus':
        return { operator: 'gte', value: 0.6 };
      // Index relative / VWAP
      case 'index_relative':
        return { operator: 'gte', value: 2 };
      case 'vwap_deviation':
        return { operator: 'gte', value: 2 };
      default:
        return { operator: 'gte', value: 0 };
    }
  };

  const handleIndicatorChange = (idx: number, newKey: string) => {
    const defaults = getIndicatorDefaults(newKey);
    setConditions(prev => prev.map((c, i) => i === idx ? { ...c, indicator: newKey, operator: defaults.operator, value: defaults.value } : c));
  };

  const updateCondition = (idx: number, field: string, value: any) => {
    setConditions(prev => prev.map((c, i) => i === idx ? { ...c, [field]: value } : c));
  };

  const removeCondition = (idx: number) => {
    setConditions(prev => prev.filter((_, i) => i !== idx));
  };

  const saveConditions = async () => {
    if (!activeId) return;
    // Clean up temp IDs
    const clean = conditions.map(c => ({ ...c, id: c.id < 0 ? 0 : c.id, strategyId: activeId }));
    try {
      await saveStrategyConditions(activeId, clean);
      toast('success', '条件已保存');
      // Reload
      const { data: r } = await fetchStrategyConditions(activeId);
      setConditions(r.data || []);
    } catch {}
  };

  const handleAIGenerate = async () => {
    if (!activeId) { toast('warning', '请先选择一个策略'); return; }
    setAiGenerating(true);
    try {
      const res = await aiGenerateStrategy({ name: activeStrategy?.name || '当前策略', description: aiDesc, style: aiStyle });
      const result = res.data?.data;
      // Check for API error response
      if (res.data?.code !== undefined && res.data?.code !== 0) {
        toast('error', res.data?.message || 'AI生成失败');
        setAiGenerating(false);
        return;
      }
      if (result && result.conditions && result.conditions.length > 0) {
        const params: any = {};
        if (result.stopProfit !== undefined) params.stopProfit = result.stopProfit;
        if (result.stopLoss !== undefined) params.stopLoss = result.stopLoss;
        if (result.maxHoldings) params.maxHoldings = result.maxHoldings;
        if (result.description) params.description = result.description;
        await updateStrategy(activeId, params);
        const cleanConds = result.conditions.map((c: any, i: number) => ({
          id: 0, strategyId: activeId, condType: c.condType, indicator: c.indicator,
          operator: c.operator, value: c.value,
          logicGroup: c.logicGroup || 1, sortOrder: i,
        }));
        await saveStrategyConditions(activeId, cleanConds);
        // Reload conditions directly instead of full reload
        const { data: condData } = await fetchStrategyConditions(activeId);
        setConditions(condData?.data || []);
        loadStrategies();
        setShowAIModal(false);
        toast('success', `AI已填充 ${cleanConds.length} 条条件到当前策略`);
      } else {
        toast('warning', 'AI未生成有效条件，请细化描述后重试');
      }
    } catch (err: any) {
      const msg = err?.response?.data?.message || err?.message || 'AI生成失败，请检查模型配置';
      toast('error', msg);
    }
    setAiGenerating(false);
  };

  const handleOptimizePrompt = async () => {
    if (!aiDesc.trim()) { toast('warning', '请先输入描述要求'); return; }
    setAiOptimizing(true);
    try {
      const { data: r } = await optimizePrompt(aiDesc, aiStyle);
      const optimized = r.data?.optimized || '';
      if (optimized) { setAiDesc(optimized); toast('success', 'AI已优化策略描述'); }
    } catch {}
    setAiOptimizing(false);
  };

  const handleRunBacktest = async () => {
    if (!activeId || !btStart || !btEnd) return;
    setBtRunning(true);
    setBtResult(null);
    setBtPositions(null);
    setBtLogs([]);
    setBtPhase('正在初始化...');
    setBtProgress('');

    const token = localStorage.getItem('aip_access_token') || '';
    const stockCodes = btStocks ? btStocks.split(',').map((s: string) => s.trim()).filter(Boolean) : [];
    const url = `http://127.0.0.1:8080/api/v1/strategies/${activeId}/backtest`;

    try {
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ startDate: btStart, endDate: btEnd, stockCodes }),
      });
      const reader = res.body?.getReader();
      if (!reader) throw new Error('No reader');
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';
        for (const line of lines) {
          if (!line.startsWith('data: ')) continue;
          const data = line.slice(6);
          try {
            const msg = JSON.parse(data);
            const { type, payload } = msg;
            switch (type) {
              case 'phase':
                setBtPhase(payload.message);
                break;
              case 'position':
                setBtPositions(payload);
                setBtProgress(`${payload.day}/${payload.totalDays} 交易日`);
                break;
              case 'trade':
                setBtLogs(prev => [...prev.slice(-199), payload]);
                break;
              case 'metric':
                setBtResult(payload);
                break;
              case 'error':
                toast('error', payload.message || '回测失败');
                break;
              case 'done':
                setBtPhase('回测完成');
                // Reload history
                fetchBacktestHistory(activeId).then(({ data: rh }: any) => setBtHistory(rh.data || [])).catch(() => {});
                break;
            }
          } catch {}
        }
      }
    } catch (err: any) {
      toast('error', '回测连接失败');
    }
    setBtRunning(false);
  };

  const handleStartBacktest = async () => {
    if (!activeId || !btStart || !btEnd) return;
    setBtRunning(true);
    setBtResult(null);
    setBtPositions(null);
    setBtLogs([]);
    setBtPhase('正在启动回测任务...');
    setBtProgress('');
    setBtOfflineMode(true);

    try {
      const stockCodes = btStocks ? btStocks.split(',').map((s: string) => s.trim()).filter(Boolean) : [];
      const { data: r } = await startBacktest(activeId, btStart, btEnd, stockCodes.length > 0 ? stockCodes : undefined);
      const taskId = r.data?.taskId;
      if (!taskId) throw new Error('No taskId');
      setBtTaskId(taskId);
      toast('success', '回测任务已启动，可关闭页面稍后查看');
      pollTaskStatus(taskId);
    } catch (err: any) {
      toast('error', '启动回测失败');
      setBtRunning(false);
      setBtOfflineMode(false);
    }
  };

  const pollTaskStatus = async (taskId: number) => {
    if (!activeId) return;
    try {
      const { data: r } = await getBacktestStatus(activeId, taskId);
      const t = r.data;
      if (!t) return;

      setBtPhase(t.phase || '');
      setBtProgress(`${t.currentDay}/${t.totalDays} 交易日`);
      if (t.currentPositions) {
        setBtPositions({ ...t.currentPositions, day: t.currentDay, totalDays: t.totalDays });
      }

      if (t.status === 'completed') {
        setBtRunning(false);
        setBtOfflineMode(false);
        setBtTaskId(null);
        setBtPhase('回测完成');
        if (t.resultId) {
          setBtResult({ totalReturn: 0, sharpeRatio: 0, maxDrawdown: 0, winRate: 0, tradeCount: 0 });
        }
        fetchBacktestHistory(activeId).then(({ data: rh }: any) => setBtHistory(rh.data || [])).catch(() => {});
        fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
        return;
      }

      if (t.status === 'failed') {
        setBtRunning(false);
        setBtOfflineMode(false);
        setBtTaskId(null);
        toast('error', t.errorMsg || '回测失败');
        fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
        return;
      }

      if (t.status === 'cancelled') {
        setBtRunning(false);
        setBtOfflineMode(false);
        setBtTaskId(null);
        setBtPhase('已取消');
        fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
        return;
      }

      // Still running, poll again
      const timer = setTimeout(() => pollTaskStatus(taskId), 2000);
      setBtPollTimer(timer);
    } catch {
      // Retry on error
      const timer = setTimeout(() => pollTaskStatus(taskId), 3000);
      setBtPollTimer(timer);
    }
  };

  const handleCancelBacktest = async () => {
    if (!activeId || !btTaskId) return;
    try {
      await cancelBacktest(activeId, btTaskId);
      if (btPollTimer) clearTimeout(btPollTimer);
      setBtPollTimer(null);
      setBtRunning(false);
      setBtOfflineMode(false);
      setBtTaskId(null);
      setBtPhase('已取消');
      toast('info', '回测已取消');
      fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
    } catch {
      toast('error', '取消失败');
    }
  };

  const handleReconnectTask = async (taskId: number) => {
    if (!activeId) return;
    setBtRunning(true);
    setBtResult(null);
    setBtPositions(null);
    setBtLogs([]);
    setBtOfflineMode(true);
    setBtTaskId(taskId);
    setBtPhase('正在重新连接...');

    // Connect SSE stream
    const token = localStorage.getItem('aip_access_token') || '';
    const url = `http://127.0.0.1:8080/api/v1/strategies/${activeId}/backtest/stream/${taskId}`;
    try {
      const res = await fetch(url, { headers: { 'Authorization': `Bearer ${token}` } });
      const reader = res.body?.getReader();
      if (!reader) { pollTaskStatus(taskId); return; }
      const decoder = new TextDecoder();
      let buffer = '';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';
        for (const line of lines) {
          if (!line.startsWith('data: ')) continue;
          try {
            const msg = JSON.parse(line.slice(6));
            const { type, payload } = msg;
            switch (type) {
              case 'phase': setBtPhase(payload.message); break;
              case 'position':
                if (payload.day) {
                  setBtPositions((prev: any) => ({ ...prev, ...payload }));
                  setBtProgress(`${payload.day || 0}/${payload.totalDays || 0} 交易日`);
                }
                break;
              case 'metric': setBtResult(payload); break;
              case 'error': toast('error', payload.message || '回测失败'); break;
              case 'done':
                setBtRunning(false); setBtOfflineMode(false); setBtTaskId(null);
                setBtPhase('回测完成');
                fetchBacktestHistory(activeId).then(({ data: rh }: any) => setBtHistory(rh.data || [])).catch(() => {});
                fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
                break;
            }
          } catch {}
        }
      }
    } catch {
      pollTaskStatus(taskId);
    }
  };

  const openTestModal = (cond: any) => {
    setTestCond(cond);
    setTestStock('');
    setTestDate(new Date().toISOString().slice(0, 10));
    setTestResult(null);
    setTestModalVisible(true);
  };

  const runTest = async () => {
    if (!testStock || !testDate || !testCond) return;
    setTestLoading(true);
    setTestResult(null);
    try {
      const { data: r } = await testIndicator({
        stockCode: testStock,
        date: testDate,
        indicator: testCond.indicator,
        operator: testCond.operator,
        value: testCond.value,
      });
      setTestResult(r.data || r);
    } catch (e: any) {
      toast('error', '测试失败: ' + (e?.response?.data?.message || e?.message || '未知错误'));
    } finally {
      setTestLoading(false);
    }
  };

  // Format a raw indicator value for display with proper unit
  const formatValue = (key: string, val: number | undefined | null): string => {
    if (val === undefined || val === null) return '—';
    const meta = indicators.find((i: any) => i.key === key);
    if (!meta) return String(val);
    if (key === 'total_market_cap') return (val / 100000000).toFixed(1) + ' 亿';
    if (meta.type === 'cross') return val > 0 ? '金叉 ↑' : val < 0 ? '死叉 ↓' : '无信号';
    if (meta.unit === '%') return val.toFixed(2) + '%';
    if (meta.unit === '元') return val.toFixed(2) + ' 元';
    if (meta.unit === '分') return val.toFixed(1) + ' 分';
    if (meta.unit === '次数') return String(Math.round(val)) + ' 次';
    return String(val);
  };

  const getIndicatorLabel = (key: string) => {
    const ind = indicators.find((i: any) => i.key === key);
    return ind ? ind.label : key;
  };

  const getOperators = (key: string) => {
    const ind = indicators.find((i: any) => i.key === key);
    return ind?.operators || ['gte', 'lte', 'gt', 'lt', 'eq'];
  };
  const getIndicatorInfo = (key: string) => indicators.find((i: any) => i.key === key);

  const inpStyle: React.CSSProperties = {
    width: '100%', padding: '6px 10px', borderRadius: 4, border: '1px solid var(--color-border-1)',
    background: 'var(--color-bg-1)', fontSize: 13, outline: 'none', boxSizing: 'border-box',
  };

  if (!strategies.length && !showAdd) {
    return (
      <div style={{ textAlign: 'center', padding: 80 }}>
        <Target size={48} color="var(--color-text-3)" />
        <p style={{ color: 'var(--color-text-3)', marginTop: 16 }}>还没有交易策略</p>
        <Button type="primary" icon={<Plus size={14} />} onClick={() => setShowAdd(true)} style={{ marginTop: 12 }}>
          创建第一个策略
        </Button>
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', gap: 16, height: 'calc(100vh - 140px)' }}>
      {/* Left: Strategy List */}
      <div style={{ width: 220, flexShrink: 0, background: 'var(--color-bg-1)', borderRadius: 8, padding: '12px 0', border: '1px solid var(--color-border-1)', display: 'flex', flexDirection: 'column' }}>
        <div style={{ padding: '0 12px 8px', borderBottom: '1px solid var(--color-border-1)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>我的策略</span>
          <Button size="mini" icon={<Plus size={12} />} type="text" onClick={() => { setShowAdd(true); setNewName(''); }} />
        </div>
        <div style={{ flex: 1, overflow: 'auto', padding: '4px 0' }}>
          {strategies.map((s: any) => (
            <div
              key={s.id}
              onClick={() => setActiveId(s.id)}
              style={{
                padding: '8px 12px', cursor: 'pointer', fontSize: 13,
                display: 'flex', alignItems: 'center', gap: 6,
                background: activeId === s.id ? 'var(--color-info-bg)' : 'transparent',
                borderLeft: activeId === s.id ? '3px solid var(--color-info-text)' : '3px solid transparent',
                color: activeId === s.id ? 'var(--color-info-text)' : 'var(--color-text-2)',
              }}
            >
              <GripVertical size={12} color="var(--color-text-3)" style={{ flexShrink: 0 }} />
              <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {s.isDefault && '⭐ '}{s.name}
              </span>
              <Popconfirm title="确定删除此策略？" onOk={() => handleDelete(s.id)}>
                <Button size="mini" type="text" icon={<Trash2 size={11} />} style={{ flexShrink: 0, opacity: 0.5 }} />
              </Popconfirm>
            </div>
          ))}
        </div>
      </div>

      {/* Right: Strategy Detail */}
      <div style={{ flex: 1, overflow: 'auto', background: 'var(--color-bg-1)', borderRadius: 8, border: '1px solid var(--color-border-1)', padding: 20 }}>
        {activeStrategy ? (
          <>
            {/* Header */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
              <div>
                <Input
                  value={activeStrategy.name}
                  onChange={v => { setActiveStrategy({ ...activeStrategy, name: v }); }}
                  onBlur={() => handleUpdateStrategy('name', activeStrategy.name)}
                  style={{ fontSize: 18, fontWeight: 700, border: 'none', padding: 0, background: 'transparent', width: 300 }}
                />
                <Input
                  value={activeStrategy.description || ''}
                  onChange={v => setActiveStrategy({ ...activeStrategy, description: v })}
                  onBlur={() => handleUpdateStrategy('description', activeStrategy.description)}
                  placeholder="添加策略描述..."
                  style={{ fontSize: 12, color: 'var(--color-text-3)', border: 'none', padding: 0, marginTop: 2, background: 'transparent', width: 400 }}
                />
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <Button size="small" icon={<Brain size={13} />} onClick={() => { setShowAIModal(true); setAiName(''); setAiDesc(''); setAiStyle('moderate'); }}>
                  AI 生成策略
                </Button>
              </div>
            </div>

            {/* Settings Bar */}
            {/* Settings Bar Row 1: Risk Mgmt */}
            <div style={{ display: 'flex', gap: 16, marginBottom: 10, padding: '10px 14px', background: 'var(--color-fill-2)', borderRadius: 6, flexWrap: 'wrap', alignItems: 'center' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-text-3)' }}>
                <Shield size={12} />止盈:
              </div>
              <InputNumber value={activeStrategy.stopProfit || 0} onChange={v => handleUpdateStrategy('stopProfit', v || 0)} min={0} max={100} suffix="%" style={{ width: 90 }} size="small" />
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-text-3)' }}>
                <Shield size={12} />止损:
              </div>
              <InputNumber value={activeStrategy.stopLoss || 0} onChange={v => handleUpdateStrategy('stopLoss', v || 0)} max={0} suffix="%" style={{ width: 90 }} size="small" />
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-text-3)' }}>
                最大持股:
              </div>
              <InputNumber value={activeStrategy.maxHoldings || 20} onChange={v => handleUpdateStrategy('maxHoldings', v || 20)} min={1} max={100} style={{ width: 80 }} size="small" />
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-text-3)' }}>
                初始资金:
              </div>
              <InputNumber value={activeStrategy.initialCapital || 100000} onChange={v => handleUpdateStrategy('initialCapital', v || 100000)} min={10000} step={10000} style={{ width: 120 }} size="small" />
            </div>

            {/* Settings Bar Row 2: Position Sizing + Investment */}
            <div style={{ display: 'flex', gap: 16, marginBottom: 16, padding: '10px 14px', background: 'var(--color-fill-2)', borderRadius: 6, flexWrap: 'wrap', alignItems: 'center' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-text-3)' }}>
                买入仓位:
              </div>
              <InputNumber value={activeStrategy.buyPositionPct || 15} onChange={v => handleUpdateStrategy('buyPositionPct', v || 15)} min={1} max={100} suffix="%" style={{ width: 85 }} size="small" />
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-text-3)' }}>
                加仓:
              </div>
              <InputNumber value={activeStrategy.addPositionPct || 10} onChange={v => handleUpdateStrategy('addPositionPct', v || 10)} min={1} max={100} suffix="%" style={{ width: 85 }} size="small" />
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-text-3)' }}>
                减仓:
              </div>
              <InputNumber value={activeStrategy.reducePositionPct || 50} onChange={v => handleUpdateStrategy('reducePositionPct', v || 50)} min={1} max={100} suffix="%" style={{ width: 85 }} size="small" />

              <div style={{ width: 1, height: 20, background: 'var(--color-border-1)' }} />

              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-text-3)' }}>
                投资方式:
              </div>
              <Select
                value={activeStrategy.investmentType || 'lump'}
                onChange={v => handleUpdateStrategy('investmentType', v)}
                style={{ width: 90 }}
                size="small"
                options={[
                  { label: '一次性', value: 'lump' },
                  { label: '定投', value: 'regular' },
                ]}
              />
              {activeStrategy.investmentType === 'regular' && (
                <>
                  <InputNumber value={activeStrategy.regularAmount || 0} onChange={v => handleUpdateStrategy('regularAmount', v || 0)} min={0} step={5000} style={{ width: 100 }} size="small" placeholder="定投金额" />
                  <Select
                    value={activeStrategy.regularInterval || 'monthly'}
                    onChange={v => handleUpdateStrategy('regularInterval', v)}
                    style={{ width: 80 }}
                    size="small"
                    options={[
                      { label: '每月', value: 'monthly' },
                      { label: '每周', value: 'weekly' },
                      { label: '每日', value: 'daily' },
                    ]}
                  />
                </>
              )}
            </div>

            {/* Tab: Conditions / Backtest */}
            <div className="seg" style={{ marginBottom: 14 }}>
              <button className={tab === 'conditions' ? 'active' : ''} onClick={() => setTab('conditions')}>
                <Settings size={13} style={{ marginRight: 4 }} />策略条件
              </button>
              <button className={tab === 'backtest' ? 'active' : ''} onClick={() => setTab('backtest')}>
                <BarChart4 size={13} style={{ marginRight: 4 }} />策略回测
              </button>
            </div>

            {tab === 'conditions' ? (
              <>
                {/* Elegant condition type tabs */}
                <div style={{ display: 'flex', gap: 6, marginBottom: 18 }}>
                  {(Object.keys(COND_LABELS) as CondType[]).map(ct => {
                    const count = filteredConds(ct).length;
                    const isActive = condTab === ct;
                    return (
                      <div
                        key={ct}
                        onClick={() => setCondTab(ct)}
                        style={{
                          flex: 1,
                          padding: '10px 14px',
                          cursor: 'pointer',
                          borderRadius: 10,
                          background: 'var(--color-fill-2)',
                          border: isActive ? '1.5px solid var(--color-border-2)' : '1.5px solid transparent',
                          transition: 'all 0.2s ease',
                          position: 'relative',
                          overflow: 'hidden',
                        }}
                      >
                        {isActive && <><div style={{
                          position: 'absolute', top: 0, left: 0, right: 0, height: 3,
                          background: COND_COLORS[ct], borderRadius: '0 0 3px 3px',
                        }} /><div style={{
                          position: 'absolute', inset: 0,
                          background: COND_COLORS[ct],
                          opacity: 0.06,
                          borderRadius: 10,
                        }} /></>}
                        <div style={{ fontSize: 12, fontWeight: isActive ? 700 : 500, color: isActive ? COND_COLORS[ct] : 'var(--color-text-3)' }}>
                          {COND_LABELS[ct]}
                        </div>
                        <div style={{ fontSize: 20, fontWeight: 800, color: isActive ? COND_COLORS[ct] : 'var(--color-text-3)', marginTop: 2 }}>
                          {count}
                        </div>
                      </div>
                    );
                  })}
                </div>

                {/* Conditions cards */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 16 }}>
                  {filteredConds(condTab).length === 0 ? (
                    <div style={{
                      padding: '40px 20px', textAlign: 'center',
                      background: 'linear-gradient(135deg, var(--color-fill-2) 0%, var(--color-fill-1) 100%)',
                      borderRadius: 12, border: '1.5px dashed var(--color-border-1)',
                    }}>
                      <div style={{ fontSize: 36, marginBottom: 8 }}>📋</div>
                      <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-2)', marginBottom: 4 }}>
                        暂无{COND_LABELS[condTab]}
                      </div>
                      <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 12 }}>
                        添加因子条件来定义何时触发{COND_LABELS[condTab]}
                      </div>
                      <Button size="small" type="outline" icon={<Plus size={12} />} onClick={() => addCondition(condTab)}>
                        添加条件
                      </Button>
                    </div>
                  ) : (
                    filteredConds(condTab).map((c: any, idx: number) => {
                      const globalIdx = conditions.indexOf(c);
                      const info = getIndicatorInfo(c.indicator);
                      const isCross = info?.type === 'cross';
                      const safeTag = info?.backtestSafe ? '🟢' : (info?.dataNote?.startsWith('🚫') ? '🚫' : '🟡');
                      return (
                        <div key={c.id || idx} style={{
                          display: 'flex', flexDirection: 'column', gap: 6,
                          padding: '10px 14px',
                          background: 'var(--color-bg-1)',
                          borderRadius: 10,
                          border: '1px solid var(--color-border-1)',
                          boxShadow: '0 1px 3px rgba(0,0,0,0.04)',
                          transition: 'box-shadow 0.2s',
                        }}
                        onMouseEnter={e => (e.currentTarget.style.boxShadow = '0 2px 8px rgba(0,0,0,0.08)')}
                        onMouseLeave={e => (e.currentTarget.style.boxShadow = '0 1px 3px rgba(0,0,0,0.04)')}
                        >
                          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                          {/* Logic connector */}
                          <div style={{
                            minWidth: 36, height: 28, display: 'flex', alignItems: 'center', justifyContent: 'center',
                            borderRadius: 14, fontSize: 11, fontWeight: 700,
                            background: idx === 0 ? 'var(--color-info-bg)' : 'var(--color-fill-2)',
                            color: idx === 0 ? 'var(--color-info-text)' : 'var(--color-text-3)',
                            letterSpacing: 0.5,
                          }}>
                            {idx === 0 ? 'IF' : 'AND'}
                          </div>

                          {/* Indicator selector with safety badge */}
                          <Tooltip content={<div style={{maxWidth:260}}>{info?.desc}<br/><span style={{color:'var(--color-text-3)',fontSize:11}}>{info?.dataNote}</span></div>} position="bottom">
                            <Select
                              value={c.indicator}
                              onChange={v => handleIndicatorChange(globalIdx, v)}
                              style={{ width: 180 }}
                              size="small"
                              placeholder="选择指标"
                              options={indicators.map((ind: any) => ({
                                label: `${ind.backtestSafe ? '🟢' : (ind.dataNote?.startsWith('🚫') ? '🚫' : '🟡')} ${ind.label}`,
                                value: ind.key,
                              }))}
                            />
                          </Tooltip>

                          {/* Cross-type: semantic preset dropdown */}
                          {isCross ? (
                            (() => {
                              const presets = (CROSS_PRESETS[c.indicator] || []).filter((p: any) => p.operator);
                              const matched = presets.find((p: any) => p.operator === c.operator && p.value === c.value);
                              return (
                                <Select value={matched ? matched.label : '__custom__'} onChange={(v: string) => {
                                  if (v === '__custom__') { updateCondition(globalIdx, 'operator', 'cross_up'); updateCondition(globalIdx, 'value', -1); return; }
                                  const p = presets.find((x: any) => x.label === v);
                                  if (p) { updateCondition(globalIdx, 'operator', p.operator); updateCondition(globalIdx, 'value', p.value); }
                                }} style={{ width: 240 }} size="small"
                                options={[...presets.map((p: any) => ({ label: p.label, value: p.label })), { label: '自定义...', value: '__custom__' }]} />
                              );
                            })()
                          ) : (
                            <>
                              <Select value={c.operator} onChange={v => updateCondition(globalIdx, 'operator', v)} style={{ width: 110 }} size="small"
                                options={getOperators(c.indicator).map((op: string) => {
                                  const opLabels: Record<string, string> = { gte: '≥ 大于等于', lte: '≤ 小于等于', gt: '> 大于', lt: '< 小于', eq: '= 等于', cross_up: '↑ 上穿', cross_down: '↓ 下穿' };
                                  return { label: opLabels[op] || op, value: op };
                                })} />
                              {c.indicator === 'total_market_cap' ? (
                              <InputNumber
                                value={c.value / 100000000}
                                onChange={v => updateCondition(globalIdx, 'value', (v ?? 0) * 100000000)}
                                style={{ width: 110, fontFamily: 'monospace' }}
                                size="small"
                                placeholder="市值(亿)"
                                suffix="亿"
                                step={10}
                              />
                            ) : c.indicator === 'new_high_20' ? (
                              <Select value={c.value} onChange={v => updateCondition(globalIdx, 'value', v ?? 0)} style={{ width: 90 }} size="small"
                                options={[{ label: '是', value: 1 }, { label: '否', value: 0 }]} />
                            ) : (
                              <InputNumber
                                value={c.value}
                                onChange={v => updateCondition(globalIdx, 'value', v ?? 0)}
                                style={{ width: 90, fontFamily: 'monospace' }}
                                size="small"
                                placeholder="阈值"
                                suffix={(() => { const u = info?.unit; if (!u) return undefined; if (u === '分' || u === '次数') return undefined; return u; })()}
                              />
                            )}
                          </>
                          )}

                          {/* If cross custom mode, show manual inputs */}
                          {isCross && (() => {
                            const presets = (CROSS_PRESETS[c.indicator] || []).filter((p: any) => p.operator);
                            return !presets.some((p: any) => p.operator === c.operator && p.value === c.value);
                          })() && (
                            <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
                              <Select value={c.operator} onChange={v => updateCondition(globalIdx, 'operator', v)} style={{ width: 110 }} size="small"
                                options={[{ label: '↑ 上穿 (金叉)', value: 'cross_up' }, { label: '↓ 下穿 (死叉)', value: 'cross_down' }]} />
                              <Input value={typeof c.value === 'number' ? `${Math.floor(c.value)}/${Math.round((c.value - Math.floor(c.value)) * 1000)}` : String(c.value)}
                                onChange={v => updateCondition(globalIdx, 'value', v)} style={{ width: 80, fontFamily: 'monospace' }} size="small" placeholder="如 5/20" />
                            </div>
                          )}

                          {/* Data tag */}
                          <Tooltip content={info?.dataNote || ''}>
                            <span style={{ fontSize: 11, color: 'var(--color-text-3)', cursor: 'help' }}>{safeTag}</span>
                          </Tooltip>

                          {/* Test indicator */}
                          <Tooltip content="用历史数据测试该指标是否准确触发">
                            <Button size="mini" type="text" style={{ color: 'var(--color-text-3)', padding: '0 4px' }} icon={<Beaker size={13} />}
                              onClick={() => openTestModal(c)}
                            />
                          </Tooltip>

                          {/* Spacer */}
                          <div style={{ flex: 1 }} />

                          {/* Delete */}
                          <Popconfirm title="移除该条件？" onOk={() => removeCondition(globalIdx)}>
                            <Button size="mini" type="text" style={{ color: 'var(--color-text-3)', padding: '0 4px' }} icon={<Trash2 size={13} />} />
                          </Popconfirm>
                          </div>
                          {/* Indicator suggestion — shown below condition row */}
                          {info?.suggestion && c.indicator && (
                            <div style={{ marginTop: 6, fontSize: 11, color: 'var(--color-info-text)', lineHeight: 1.5, paddingLeft: 4 }}>
                              💡 {info.suggestion}
                            </div>
                          )}
                          {/* Indicator detail panel */}
                          {info && c.indicator && (
                            <div style={{ marginTop: 6, padding: '8px 12px', background: 'var(--color-fill-2)', borderRadius: 6, borderLeft: '3px solid var(--color-border-1)' }}>
                              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                                <span style={{ fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap', marginTop: 1 }}>{safeTag}</span>
                                <div style={{ flex: 1 }}>
                                  <div style={{ fontSize: 12, color: 'var(--color-text-2)', lineHeight: 1.5 }}>
                                    {info.desc}
                                    {info.unit && <span style={{ marginLeft: 6, padding: '0 6px', background: 'var(--color-fill-1)', borderRadius: 4, fontSize: 10, color: 'var(--color-text-3)' }}>{info.unit}</span>}
                                    {info.type === 'cross' && <span style={{ marginLeft: 6, padding: '0 6px', background: '#fff7e6', borderRadius: 4, fontSize: 10, color: '#d48806' }}>交叉</span>}
                                  </div>
                                  {info.dataNote && !info.dataNote.startsWith('✅') && <div style={{ marginTop: 4, fontSize: 11, color: 'var(--color-warning-text)', lineHeight: 1.5 }}>📌 {info.dataNote}</div>}
                                </div>
                              </div>
                            </div>
                          )}
                        </div>
                      );
                    })
                  )}
                </div>

                {/* Action bar */}
                {filteredConds(condTab).length > 0 && (
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <Button size="small" icon={<Plus size={12} />} type="dashed" onClick={() => addCondition(condTab)}>
                      添加条件
                    </Button>
                    <div style={{ flex: 1 }} />
                    <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
                      共 {filteredConds(condTab).length} 条 · AND 逻辑
                    </span>
                    <Button size="small" type="primary" onClick={saveConditions} style={{ borderRadius: 8 }}>
                      保存条件
                    </Button>
                  </div>
                )}
              </>
            ) : (
              <>
                {/* Backtest */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 14 }}>
                  <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
                    <div>
                      <span style={{ fontSize: 12, color: 'var(--color-text-3)', marginRight: 4 }}>起始日期</span>
                      <Input value={btStart} onChange={setBtStart} style={{ width: 120 }} size="small" placeholder="2025-01-01" />
                    </div>
                    <div>
                      <span style={{ fontSize: 12, color: 'var(--color-text-3)', marginRight: 4 }}>结束日期</span>
                      <Input value={btEnd} onChange={setBtEnd} style={{ width: 120 }} size="small" placeholder="2026-06-05" />
                    </div>
                    <Button type="primary" icon={<Play size={13} />} loading={btRunning && !btOfflineMode} onClick={handleRunBacktest}>
                      实时运行
                    </Button>
                    <Button type="outline" icon={<Play size={13} />} loading={btRunning && btOfflineMode} onClick={handleStartBacktest}>
                      后台运行
                    </Button>
                    {btRunning && btOfflineMode && btTaskId && (
                      
                      <Button type="text" status="danger" size="small" onClick={handleCancelBacktest}>
                        取消
                      </Button>
                    )}
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ fontSize: 12, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>股票池:</span>
                    <Input
                      value={btStocks}
                      onChange={setBtStocks}
                      placeholder="留空=全部有K线数据的股票；输入股票代码用逗号分隔，如: 000001,600519"
                      style={{ flex: 1, maxWidth: 600 }}
                      size="small"
                    />
                  </div>
                </div>

                {/* Live progress */}
                {btPhase && (
                  <div style={{ marginBottom: 12, padding: '8px 12px', background: 'var(--color-info-bg)', borderRadius: 6, fontSize: 12, color: 'var(--color-info-text)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <span>{btPhase}</span>
                    {btProgress && <span style={{ color: 'var(--color-text-3)' }}>{btProgress}</span>}
                  </div>
                )}

                {/* Dual Panel: Positions + Trade Log */}
                {(btPositions || btLogs.length > 0 || btResult) && (
                  <div style={{ display: 'grid', gridTemplateColumns: btPositions ? '1fr 1fr' : '1fr', gap: 12, marginBottom: 16 }}>
                    {/* Left: Positions Panel */}
                    {btPositions && (
                      <div style={{ background: 'var(--color-bg-1)', border: '1px solid var(--color-border-1)', borderRadius: 8, padding: 12 }}>
                        <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8, display: 'flex', justifyContent: 'space-between' }}>
                          <span>📊 持仓快照 ({btPositions.date})</span>
                          <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
                            现金: ¥{btPositions.cash?.toLocaleString()} | 总权益: ¥{btPositions.totalEquity?.toLocaleString()}
                          </span>
                        </div>
                        <div style={{ fontSize: 13, marginBottom: 8, display: 'flex', gap: 16 }}>
                          <span style={{ color: btPositions.totalReturn >= 0 ? 'var(--stock-up)' : 'var(--stock-down)', fontWeight: 600 }}>
                            收益: {btPositions.totalReturn >= 0 ? '+' : ''}{btPositions.totalReturn}%
                          </span>
                          <span style={{ color: 'var(--color-text-3)' }}>持仓: {btPositions.positionCount} 只</span>
                        </div>
                        {btPositions.positions?.length > 0 ? (
                          <div style={{ maxHeight: 240, overflow: 'auto' }}>
                            <table style={{ width: '100%', fontSize: 12, borderCollapse: 'collapse' }}>
                              <thead>
                                <tr style={{ borderBottom: '1px solid var(--color-border-1)', color: 'var(--color-text-3)' }}>
                                  <th style={{ textAlign: 'left', padding: '3px 4px' }}>代码</th>
                                  <th style={{ textAlign: 'left', padding: '3px 4px' }}>名称</th>
                                  <th style={{ textAlign: 'right', padding: '3px 4px' }}>持仓</th>
                                  <th style={{ textAlign: 'right', padding: '3px 4px' }}>现价</th>
                                  <th style={{ textAlign: 'right', padding: '3px 4px' }}>市值</th>
                                  <th style={{ textAlign: 'right', padding: '3px 4px' }}>盈亏%</th>
                                </tr>
                              </thead>
                              <tbody>
                                {btPositions.positions.map((p: any, i: number) => (
                                  <tr key={i} style={{ borderBottom: '1px solid var(--color-border-1)' }}>
                                    <td style={{ padding: '3px 4px', fontFamily: 'monospace' }}>{p.code}</td>
                                    <td style={{ padding: '3px 4px' }}>{p.name}</td>
                                    <td style={{ textAlign: 'right', padding: '3px 4px' }}>{p.qty}股</td>
                                    <td style={{ textAlign: 'right', padding: '3px 4px', fontFamily: 'monospace' }}>{p.price?.toFixed(2)}</td>
                                    <td style={{ textAlign: 'right', padding: '3px 4px', fontFamily: 'monospace' }}>¥{p.marketVal?.toLocaleString()}</td>
                                    <td style={{ textAlign: 'right', padding: '3px 4px', fontFamily: 'monospace', color: p.pnlPct >= 0 ? 'var(--stock-up)' : 'var(--stock-down)', fontWeight: 600 }}>
                                      {p.pnlPct >= 0 ? '+' : ''}{p.pnlPct}%
                                    </td>
                                  </tr>
                                ))}
                              </tbody>
                            </table>
                          </div>
                        ) : (
                          <div style={{ color: 'var(--color-text-3)', fontSize: 12, padding: 20, textAlign: 'center' }}>暂无持仓</div>
                        )}
                      </div>
                    )}

                    {/* Right: Trade Log */}
                    <div style={{ background: 'var(--color-fill-1)', border: '1px solid var(--color-border-1)', borderRadius: 8, padding: 12, color: 'var(--color-text-2)', fontFamily: 'monospace', fontSize: 11, maxHeight: 360, overflow: 'auto' }}>
                      <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8, color: 'var(--color-text-1)' }}>📋 交易日志</div>
                      {btLogs.length === 0 && <div style={{ color: 'var(--color-text-3)' }}>等待交易信号...</div>}
                      {btLogs.map((t: any, i: number) => {
                        const colors: Record<string, string> = { buy: 'var(--stock-up)', add: 'var(--color-warning-text)', sell: 'var(--stock-down)', reduce: 'var(--color-info-text)' };
                        const labels: Record<string, string> = { buy: '买入', add: '加仓', sell: '卖出', reduce: '减仓' };
                        return (
                          <div key={i} style={{ marginBottom: 3, lineHeight: 1.5 }}>
                            <span style={{ color: 'var(--color-text-3)' }}>{t.date}</span>{' '}
                            <span style={{ color: colors[t.action] || 'var(--color-text-1)', fontWeight: 600 }}>[{labels[t.action] || t.action}]</span>{' '}
                            <span>{t.code} {t.name}</span>{' '}
                            <span>¥{t.price?.toFixed(2)} × {t.quantity}股</span>{' '}
                            {t.pnlPct !== undefined && t.pnlPct !== 0 && (
                              <span style={{ color: t.pnlPct > 0 ? 'var(--stock-up)' : 'var(--stock-down)' }}>
                                {t.pnlPct > 0 ? '+' : ''}{t.pnlPct?.toFixed(2)}%
                              </span>
                            )}
                            <div style={{ color: 'var(--color-text-3)', fontSize: 10 }}>  ↳ {t.reason}</div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}

                {/* Final metrics (shown after completion) */}
                {btResult && (
                  <div className="stat-grid mb16" style={{ gridTemplateColumns: 'repeat(4, 1fr)' }}>
                    <div className="stat-card">
                      <div className="stat-label">累计收益</div>
                      <div className={`stat-value ${btResult.totalReturn >= 0 ? 'up' : 'down'}`}>
                        {btResult.totalReturn >= 0 ? '+' : ''}{btResult.totalReturn}%
                      </div>
                    </div>
                    <div className="stat-card">
                      <div className="stat-label">夏普比率</div>
                      <div className="stat-value">{btResult.sharpeRatio}</div>
                    </div>
                    <div className="stat-card">
                      <div className="stat-label">最大回撤</div>
                      <div className="stat-value down">-{btResult.maxDrawdown}%</div>
                    </div>
                    <div className="stat-card">
                      <div className="stat-label">胜率 / 交易次数</div>
                      <div className="stat-value">{btResult.winRate}% <span style={{ fontSize: 14, color: 'var(--color-text-3)' }}>/ {btResult.tradeCount}</span></div>
                    </div>
                    {btResult.coveragePct !== undefined && (
                      <div className="stat-card">
                        <div className="stat-label">因子覆盖率</div>
                        <div className="stat-value" style={{ color: btResult.coveragePct >= 80 ? 'var(--stock-down)' : btResult.coveragePct >= 40 ? 'var(--color-warning-text)' : 'var(--stock-up)' }}>
                          {btResult.coveragePct}%
                        </div>
                        <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 2 }}>
                          🟢K线安全 {btResult.klineSafe} / 🟡需验证 {btResult.klineUnsafe}
                        </div>
                      </div>
                    )}
                  </div>
                )}

                {btHistory.length > 0 && (
                  <div className="card">
                    <div className="card-header"><span className="card-title">历史回测记录</span></div>
                    <Table
                      columns={[
                        { title: '时间', dataIndex: 'createdAt', render: (v: string) => v?.slice(0, 16) },
                        { title: '收益', dataIndex: 'totalReturn', render: (v: number) => <span style={{ color: v >= 0 ? 'var(--stock-up)' : 'var(--stock-down)' }}>{v >= 0 ? '+' : ''}{v}%</span> },
                        { title: '夏普', dataIndex: 'sharpeRatio' },
                        { title: '回撤', dataIndex: 'maxDrawdown', render: (v: number) => `-${v}%` },
                        { title: '胜率', dataIndex: 'winRate', render: (v: number) => `${v}%` },
                        { title: '交易', dataIndex: 'tradeCount' },
                      ]}
                      data={btHistory}
                      rowKey="id"
                      pagination={false}
                      size="small"
                    />
                  </div>
                )}

                {/* Task list */}
                {btTasks.length > 0 && (
                  <div className="card" style={{ marginTop: 12 }}>
                    <div className="card-header"><span className="card-title">回测任务</span></div>
                    <Table
                      columns={[
                        { title: '创建时间', dataIndex: 'createdAt', render: (v: string) => v?.slice(0, 16) },
                        { title: '状态', dataIndex: 'status', render: (v: string) => {
                          const statusMap: Record<string, { color: string; label: string }> = {
                            pending: { color: 'var(--color-text-3)', label: '排队中' },
                            running: { color: 'var(--color-info-text)', label: '运行中' },
                            completed: { color: 'var(--stock-down)', label: '已完成' },
                            failed: { color: 'var(--stock-up)', label: '失败' },
                            cancelled: { color: 'var(--color-warning-text)', label: '已取消' },
                          };
                          const s = statusMap[v] || { color: 'var(--color-text-3)', label: v };
                          return <span style={{ color: s.color }}>{s.label}</span>;
                        }},
                        { title: '进度', dataIndex: 'progressPct', render: (v: number) => `${v?.toFixed(0) || 0}%` },
                        { title: '阶段', dataIndex: 'phase', render: (v: string) => v || '-' },
                        { title: '操作', dataIndex: 'id', render: (id: number, record: any) => {
                          if (record.status === 'running' || record.status === 'pending') {
                            return <Button size="mini" type="text" onClick={() => handleReconnectTask(id)}>查看</Button>;
                          }
                          return null;
                        }},
                      ]}
                      data={btTasks}
                      rowKey="id"
                      pagination={false}
                      size="small"
                    />
                  </div>
                )}
              </>
            )}
          </>
        ) : (
          <div style={{ textAlign: 'center', padding: 60, color: 'var(--color-text-3)' }}>请从左侧选择一个策略</div>
        )}
      </div>

      {/* Add Strategy Modal */}
      <Modal visible={showAdd} title="新建策略" onOk={handleAdd} onCancel={() => setShowAdd(false)} okText="创建">
        <Input placeholder="策略名称" value={newName} onChange={setNewName} style={{ width: '100%' }} />
      </Modal>

      {/* AI Generate Modal */}
      <Modal visible={showAIModal} title="AI 生成策略" onOk={handleAIGenerate} onCancel={() => setShowAIModal(false)} okText="开始生成" confirmLoading={aiGenerating}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>策略名称 (可选)</div>
            <Input placeholder="留空则AI自动命名" value={aiName} onChange={setAiName} />
          </div>
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>描述/要求 (可选)</div>
            <div style={{ position: 'relative' }}>
              <Input.TextArea placeholder="例如：偏好低估值蓝筹，设置严格的止盈止损..." value={aiDesc} onChange={setAiDesc} rows={3} style={{ paddingRight: 32 }} />
              <button
                onClick={handleOptimizePrompt}
                disabled={aiOptimizing || !aiDesc.trim()}
                title="AI 优化策略描述"
                style={{
                  position: 'absolute', bottom: 8, right: 8,
                  background: aiOptimizing ? 'var(--color-border-1)' : 'var(--color-info-bg)',
                  border: 'none', borderRadius: 4, cursor: aiOptimizing || !aiDesc.trim() ? 'not-allowed' : 'pointer',
                  padding: '3px 6px', display: 'flex', alignItems: 'center', opacity: aiDesc.trim() ? 1 : 0.4,
                }}
              >
                <Sparkles size={13} color={aiOptimizing ? 'var(--color-text-3)' : 'var(--color-info-text)'} style={{ animation: aiOptimizing ? 'spin 1s linear infinite' : 'none' }} />
              </button>
            </div>
          </div>
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>风险偏好</div>
            <Select
              value={aiStyle}
              onChange={setAiStyle}
              style={{ width: '100%' }}
              options={[
                { label: '稳健 (推荐)', value: 'moderate' },
                { label: '激进', value: 'aggressive' },
                { label: '保守', value: 'conservative' },
              ]}
            />
          </div>
        </div>
      </Modal>

      {/* ═══ Indicator Test Modal ═══ */}
      <Modal
        title={<span style={{ display: 'flex', alignItems: 'center', gap: 8 }}><Beaker size={16} color="var(--color-info-text)" />指标测试</span>}
        visible={testModalVisible}
        onCancel={() => { setTestModalVisible(false); setTestResult(null); }}
        footer={null}
        style={{ width: 520 }}
        unmountOnExit
      >
        {testCond && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            {/* Condition summary */}
            <div style={{ padding: '12px 16px', background: 'var(--color-fill-2)', borderRadius: 8, border: '1px solid var(--color-border-1)' }}>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>测试条件</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                <Tag color={COND_COLORS[testCond.condType as CondType]}>{COND_LABELS[testCond.condType as CondType]}</Tag>
                <span style={{ fontWeight: 600, fontSize: 14 }}>{getIndicatorLabel(testCond.indicator)}</span>
                <span style={{ color: 'var(--color-text-2)', fontSize: 14 }}>
                  {getOperators(testCond.indicator).find((o: string) => o === testCond.operator) ? 
                    ({ gte: '≥', lte: '≤', gt: '>', lt: '<', eq: '=', cross_up: '↑上穿', cross_down: '↓下穿' } as any)[testCond.operator] : testCond.operator}
                </span>
                <span style={{ fontWeight: 700, fontSize: 15, color: 'var(--color-info-text)' }}>{testCond.indicator === 'total_market_cap' ? (testCond.value / 100000000).toFixed(0) + '亿' : testCond.value}{getIndicatorInfo(testCond.indicator)?.unit === '%' ? '%' : ''}</span>
              </div>
            </div>

            {/* Inputs */}
            <div style={{ display: 'flex', gap: 12 }}>
              <Input
                placeholder="股票代码 (如 000001)"
                value={testStock}
                onChange={setTestStock}
                style={{ flex: 1 }}
                size="small"
              />
              <DatePicker
                value={testDate}
                onChange={(v: string) => setTestDate(v)}
                style={{ width: 160 }}
                size="small"
                placeholder="选择日期"
              />
              <Button
                size="small"
                type="primary"
                icon={<Beaker size={12} />}
                loading={testLoading}
                onClick={runTest}
                disabled={!testStock || !testDate}
              >
                测试
              </Button>
            </div>

            {/* Result */}
            {testResult && (
              <div style={{
                padding: '16px 20px',
                background: testResult.hasData ? (testResult.conditionMet ? 'var(--color-success-bg)' : 'var(--color-warning-bg)') : 'var(--color-danger-bg)',
                borderRadius: 10,
                border: `1px solid ${testResult.hasData ? (testResult.conditionMet ? 'var(--color-success-border)' : 'var(--color-warning-border)') : 'var(--color-danger-border)'}`,
              }}>
                {!testResult.hasData ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span style={{ fontSize: 20 }}>⚠️</span>
                      <span style={{ fontWeight: 600, fontSize: 14, color: 'var(--stock-up)' }}>无数据</span>
                    </div>
                    <span style={{ fontSize: 13, color: 'var(--color-text-2)' }}>{testResult.error || '该股票在指定日期无对应数据'}</span>
                    {testResult.stockName && <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>股票：{testResult.stockName} ({testResult.stockCode})</span>}
                  </div>
                ) : (
                  <>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 10 }}>
                      <div>
                        <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 2 }}>股票 / 日期</div>
                        <div style={{ fontWeight: 600, fontSize: 14 }}>
                          {testResult.stockName || testResult.stockCode}
                          <span style={{ marginLeft: 8, fontWeight: 400, color: 'var(--color-text-2)', fontSize: 13 }}>{testResult.date}</span>
                        </div>
                      </div>
                      {testResult.conditionMet ? (
                        <Tag color="green" style={{ fontSize: 13, padding: '4px 12px' }}>✅ 条件满足</Tag>
                      ) : (
                        <Tag color="orange" style={{ fontSize: 13, padding: '4px 12px' }}>❌ 条件不满足</Tag>
                      )}
                    </div>
                    <div style={{ display: 'flex', gap: 24, marginBottom: 10 }}>
                      <div>
                        <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>指标计算值</div>
                        <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--color-info-text)', fontFamily: 'monospace' }}>{formatValue(testCond.indicator, testResult.computedValue)}</div>
                      </div>
                      <div>
                        <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>阈值</div>
                        <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-2)', fontFamily: 'monospace' }}>
                          {({ gte: '≥', lte: '≤', gt: '>', lt: '<', eq: '=', cross_up: '↑', cross_down: '↓' } as any)[testResult.operator]} {testCond.indicator === 'total_market_cap' ? (testResult.threshold / 100000000).toFixed(0) + '亿' : testResult.threshold}
                        </div>
                      </div>
                    </div>
                    <div style={{ borderTop: '1px solid var(--color-border-1)', paddingTop: 10, display: 'flex', gap: 24 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                        <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>数据表</span>
                        <code style={{ fontSize: 11, background: 'var(--color-fill-2)', padding: '1px 6px', borderRadius: 4 }}>{testResult.dataSource}</code>
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                        <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>数据质量</span>
                        <span style={{ fontSize: 11 }}>{testResult.dataNote}</span>
                      </div>
                    </div>
                  </>
                )}
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
}
