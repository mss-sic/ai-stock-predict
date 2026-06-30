import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Input, InputNumber, Modal, Table, Select, Popconfirm, Tooltip, DatePicker, Message, Tag } from '@arco-design/web-react';
import { Target, Plus, Trash2, GripVertical, Play, Brain, BarChart4, Shield, Settings, Sparkles, Beaker, Gauge, Factory, Layers, SlidersHorizontal, Search, CheckCircle, AlertTriangle, ScrollText, TrendingUp, TrendingDown, Grid3X3, DollarSign, ChevronDown, ChevronRight, Activity } from 'lucide-react';
import {
  fetchStrategies, createStrategy, updateStrategy, deleteStrategy, reorderStrategies,
  fetchStrategyConditions, saveStrategyConditions, aiGenerateStrategy, optimizePrompt,
  fetchIndicators, runBacktest, fetchBacktestHistory, testIndicator,
  startBacktest, getBacktestStatus, cancelBacktest, fetchBacktestTasks, fetchBacktestTaskLogs, fetchBacktestResult,
  fetchOrchestration, saveOrchestration, fetchConditionTemplates, createConditionTemplate, fetchAIDecisions,
} from '../services/api';
import TemplateSelector, { STRATEGY_TEMPLATES } from './TemplateSelector';
import IndicatorPicker from './IndicatorPicker';

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
  const navigate = useNavigate();
  const [strategies, setStrategies] = useState<any[]>([]);
  const [activeId, setActiveId] = useState<number | null>(null);
  const [activeStrategy, setActiveStrategy] = useState<any>(null);
  const [conditions, setConditions] = useState<any[]>([]);
  const [indicators, setIndicators] = useState<any[]>([]);
  const [tab, setTab] = useState<'conditions' | 'backtest' | 'positionMgmt' | 'riskControl'>('conditions');
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
  const [logCursor, setLogCursor] = useState(0);
  const [logFilter, setLogFilter] = useState('all'); // all | trade | signal | system
  const logEndRef = useRef<HTMLDivElement>(null);
  const [btPhase, setBtPhase] = useState('');
  const [btProgress, setBtProgress] = useState('');
  const [btTaskId, setBtTaskId] = useState<number | null>(null);
    const [btTasks, setBtTasks] = useState<any[]>([]);
  const btPollTimerRef = useRef<NodeJS.Timeout | null>(null);

  // Indicator test state
  const [testModalVisible, setTestModalVisible] = useState(false);
  const [orchTab, setOrchTab] = useState(false);
  const [riskPreset, setRiskPreset] = useState<string>('balanced');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [orchConfig, setOrchConfig] = useState<any>({
    orchestrationMode: 'hybrid',
    enableMarketContext: true,
    marketCompositeMin: -3.0,
    marketPositionBias: 1.0,
    enableAIAgent: false,
    aiAgentMode: 'advisory',
    aiAgentReviewScope: 'all',
    aiAgentMaxDailyTrades: 5,
    industryFilter: '',
    enableSectorRotation: false,
    policyMode: 'rule',
    aggressiveThreshold: 1.5,
    defensiveThreshold: 0.0,
    policyAggressive: { buyPct: 20, addPct: 15, reducePct: 60, stopProfit: 25, stopLoss: -8, allowAdd: true, buyLogic: 'or', addLogic: 'or' },
    policyDefensive: { buyPct: 10, addPct: 0, reducePct: 50, stopProfit: 15, stopLoss: -5, allowAdd: false, buyLogic: 'and', addLogic: 'and' },
    policyCash: { buyPct: 0, addPct: 0, reducePct: 80, stopProfit: 10, stopLoss: -3, allowAdd: false, buyLogic: 'and', addLogic: 'and' },
  });
  const [orchSaving, setOrchSaving] = useState(false);
  const [activeRegime, setActiveRegime] = useState('policyAggressive');
  const [condAdvOpen, setCondAdvOpen] = useState<Record<number, boolean>>({});

  const [testCond, setTestCond] = useState<any>(null);
  const [testStock, setTestStock] = useState('');
  const [testDate, setTestDate] = useState('');
  const [testResult, setTestResult] = useState<any>(null);
  const [testLoading, setTestLoading] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<string | null>(null);
  const [templatePopulating, setTemplatePopulating] = useState(false);



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
    } catch (err) { console.error('[StrategyPage] load failed:', err); }
  }, [activeId]);

  const loadIndicators = useCallback(async () => {
    try {
      const { data: r } = await fetchIndicators();
      setIndicators(r.data || []);
    } catch (err) { console.error('[StrategyPage] indicators load failed:', err); }
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
      fetchOrchestration(s.id).then(({ data: r }: any) => { if (r.data) setOrchConfig(r.data); }).catch(() => {});
      fetchConditionTemplates().then(() => {}).catch(() => {});
    }
  }, [activeId, strategies]);

  const handleAdd = async () => {
    if (!newName.trim()) return;
    if (!selectedTemplate) { toast('warning', '请选择一个策略模板'); return; }
    setTemplatePopulating(true);
    try {
      const { data: r } = await createStrategy(newName.trim());
      const sid = r.data.id;
      setActiveId(sid);
      const tmpl = STRATEGY_TEMPLATES[selectedTemplate];
      if (tmpl) {
        const allConds = [...(tmpl.buyConds || []), ...(tmpl.sellConds || [])];
        const cleanConds = allConds.map((c: any, i: number) => ({
          id: 0, strategyId: sid, condType: c.condType, indicator: c.indicator,
          operator: c.operator, value: c.value,
          logicGroup: c.logicGroup || 1, sortOrder: i,
          weight: c.weight, lookbackDays: c.lookbackDays, industryRelative: c.industryRelative,
        }));
        if (cleanConds.length > 0) {
          await saveStrategyConditions(sid, cleanConds);
        }
        if (tmpl.regimes) {
          await saveOrchestration(sid, {
            orchestrationMode: 'hybrid',
            enableMarketContext: true,
            ...tmpl.regimes,
          });
        }
      }
      setShowAdd(false);
      setNewName('');
      setSelectedTemplate(null);
      loadStrategies();
      toast('success', `策略「${newName.trim()}」已创建 (${tmpl?.label || '自定义'})`);
    } catch (err) { console.error('[StrategyPage] op failed:', err); toast('error', '创建失败'); }
    finally { setTemplatePopulating(false); }
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
    } catch (err) { console.error('[StrategyPage] op failed:', err); }
  };

  const handleUpdateStrategy = async (field: string, value: any) => {
    if (!activeId) return;
    try {
      await updateStrategy(activeId, { [field]: value });
      loadStrategies();
    } catch (err) { console.error('[StrategyPage] op failed:', err); }
  };

  const filteredConds = (t: CondType) => conditions.filter((c: any) => c.condType === t);

  const addConditionToGroup = (ct: CondType, groupId: number) => {
    const sameGroup = conditions.filter((c: any) => c.condType === ct && (c.logicGroup || 1) === groupId);
    const maxSort = sameGroup.reduce((max: number, c: any) => Math.max(max, c.sortOrder || 0), -1);
    setConditions([...conditions, {
      id: -(Date.now()),
      strategyId: activeId,
      condType: ct,
      indicator: 'algo_score',
      operator: 'gte',
      value: 0,
      logicGroup: groupId,
      sortOrder: maxSort + 1,
    }]);
  };

  const addConditionGroup = (ct: CondType) => {
    const sameType = conditions.filter((c: any) => c.condType === ct);
    const maxGroup = sameType.length > 0 ? sameType.reduce((max: number, c: any) => Math.max(max, c.logicGroup || 1), 0) : 0;
    const newGroup = maxGroup + 1;
    setConditions([...conditions, {
      id: -(Date.now()),
      strategyId: activeId,
      condType: ct,
      indicator: 'algo_score',
      operator: 'gte',
      value: 0,
      logicGroup: newGroup,
      sortOrder: 0,
    }]);
    return newGroup;
  };

  const removeConditionGroup = (ct: CondType, groupId: number) => {
    setConditions(prev => prev.filter((c: any) => !(c.condType === ct && (c.logicGroup || 1) === groupId)));
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
    } catch (err) { console.error('[StrategyPage] op failed:', err); }
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
    } catch (err) { console.error('[StrategyPage] op failed:', err); }
    setAiOptimizing(false);
  };



  const handleStartBacktest = async () => {
    if (!activeId || !btStart || !btEnd) return;
    setBtRunning(true);
    setBtResult(null);
    setBtPositions(null);
    setBtLogs([]);
    setBtPhase('正在启动回测任务...');
    setBtProgress('');
    try {
      const stockCodes = btStocks ? btStocks.split(',').map((s: string) => s.trim()).filter(Boolean) : [];
      const { data: r } = await startBacktest(activeId, btStart, btEnd, stockCodes.length > 0 ? stockCodes : undefined);
      const taskId = r.data?.taskId;
      if (!taskId) throw new Error('No taskId');
      setBtTaskId(taskId);
      sessionStorage.setItem('bt_active_task', JSON.stringify({ sid: activeId, tid: taskId, start: Date.now() }));
      toast('success', '回测任务已启动，可关闭页面稍后查看');
      pollTaskStatus(taskId);
    } catch (err: any) {
      toast('error', '启动回测失败');
      setBtRunning(false);
    }
  };

  const pollTaskStatus = async (taskId: number) => {
    if (!activeId) return;
    try {
      const { data: r } = await getBacktestStatus(activeId, taskId);
      const t = r.data;
      if (!t) { scheduleNext(); return; }
      setBtPhase(t.phase || '');
      setBtProgress(`${t.currentDay || 0}/${t.totalDays || 0} 交易日`);
      if (t.currentPositions) {
        setBtPositions({ ...t.currentPositions, day: t.currentDay, totalDays: t.totalDays });
      }
      // Cursor-based log fetching: only fetch new logs
      fetchBacktestTaskLogs(activeId, taskId, logCursor > 0 ? logCursor : undefined).then(({ data: lr }: any) => {
        const newLogs = lr.data?.logs || lr.data || [];
        const cursor = lr.data?.cursor || 0;
        if (newLogs.length > 0) {
          setBtLogs(prev => {
            const merged = [...prev, ...newLogs];
            // Deduplicate by id
            const seen = new Set<number>();
            const deduped = merged.filter(l => { const k = l.id || 0; if (seen.has(k)) return false; seen.add(k); return true; });
            return deduped.slice(-500);
          });
          setLogCursor(cursor);
        }
      }).catch(() => {});

      if (t.status === 'completed') {
        finishTask(taskId, '回测完成');
        return;
      }
      if (t.status === 'failed') {
        finishTask(taskId, 'failed', t.errorMsg);
        return;
      }
      if (t.status === 'cancelled') {
        finishTask(taskId, 'cancelled');
        return;
      }
      scheduleNext();
    } catch {
      scheduleNext(2000);
    }

    function scheduleNext(delay = 1000) {
      btPollTimerRef.current = setTimeout(() => pollTaskStatus(taskId), delay);
    }
    function finishTask(taskId: number, phase: string, errorMsg?: string) {
      setBtRunning(false);
      setBtTaskId(null);
      setBtPhase(phase);
      sessionStorage.removeItem('bt_active_task');
      if (btPollTimerRef.current) clearTimeout(btPollTimerRef.current);
      if (errorMsg) toast('error', errorMsg || '回测失败');
      fetchBacktestHistory(activeId!).then(({ data: rh }: any) => {
        const results = rh.data || [];
        setBtHistory(results);
        // Set btResult from latest completed history entry
        if (results.length > 0 && phase === '回测完成') setBtResult(results[0]);
      }).catch(() => {});
      fetchBacktestTasks(activeId!).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
      // Fetch full logs for completed task
      if (phase === '回测完成') {
        fetchBacktestTaskLogs(activeId!, taskId).then(({ data: lr }: any) => {
          const all = lr.data?.logs || lr.data || [];
          if (all.length > 0) { setBtLogs(all); setLogCursor(lr.data?.cursor || 0); }
        }).catch(() => {});
      }
    }
  };

  // Resume active task on page mount (survives refresh)
  useEffect(() => {
    const saved = sessionStorage.getItem('bt_active_task');
    if (!saved || !activeId) return;
    try {
      const { sid, tid } = JSON.parse(saved);
      if (sid === activeId) {
        setBtRunning(true);
        setBtTaskId(tid);
        setBtPhase('重新连接中...');
        pollTaskStatus(tid);
      }
    } catch {}
  }, [activeId]);

  // Auto-scroll log panel to bottom when new logs arrive
  useEffect(() => {
    if (logEndRef.current && btLogs.length > 0 && btRunning) {
      logEndRef.current.scrollTop = logEndRef.current.scrollHeight;
    }
  }, [btLogs.length, btRunning]);

  // Cleanup timer on unmount
  useEffect(() => {
    return () => {
      if (btPollTimerRef.current) clearTimeout(btPollTimerRef.current);
    };
  }, []);

  const handleCancelBacktest = async () => {
    if (!activeId || !btTaskId) return;
    try {
      await cancelBacktest(activeId, btTaskId);
      if (btPollTimerRef.current) clearTimeout(btPollTimerRef.current);
      setBtTaskId(null);
      setBtPhase('已取消');
      sessionStorage.removeItem("bt_active_task");
      toast('info', '回测已取消');
      fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
    } catch {
      toast('error', '取消失败');
    }
  };

  // Reconnect to a running task: simply start polling (same as fresh start)
  const handleReconnectTask = async (taskId: number) => {
    if (!activeId) return;
    setBtRunning(true);
    setBtResult(null);
    setBtPositions(null);
    setBtLogs([]);
    setBtTaskId(taskId);
    setBtPhase('正在重新连接...');
    sessionStorage.setItem('bt_active_task', JSON.stringify({ sid: activeId, tid: taskId, start: Date.now() }));
    pollTaskStatus(taskId);
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
              <button className={tab === 'positionMgmt' ? 'active' : ''} onClick={() => setTab('positionMgmt')}>
                <TrendingUp size={13} style={{ marginRight: 4 }} />持仓管理
              </button>
              <button className={tab === 'riskControl' ? 'active' : ''} onClick={() => setTab('riskControl')}>
                <Shield size={13} style={{ marginRight: 4 }} />智能风控
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
                      <div style={{ fontSize: 36, marginBottom: 8 }}><Layers size={36} style={{ color: 'var(--color-text-3)' }} /></div>
                      <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-2)', marginBottom: 4 }}>
                        暂无{COND_LABELS[condTab]}
                      </div>
                      <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 12 }}>
                        新建条件组来定义触发规则（组内 AND · 组间 OR）
                      </div>
                      <Button size="small" type="outline" icon={<Plus size={12} />} onClick={() => addConditionGroup(condTab)}>
                        新建条件组
                      </Button>
                    </div>
                  ) : (
                    (() => {
                      // Group conditions by logicGroup for card rendering
                      const grouped = filteredConds(condTab).reduce((acc: Record<number, any[]>, c: any) => {
                        const g = c.logicGroup || 1;
                        if (!acc[g]) acc[g] = [];
                        acc[g].push(c);
                        return acc;
                      }, {} as Record<number, any[]>);
                      const groupIds = Object.keys(grouped).map(Number).sort((a, b) => a - b);

                      return groupIds.map((gid, gIdx) => {
                        const groupConds = grouped[gid];
                        const isLastGroup = gIdx === groupIds.length - 1;
                        return (
                          <React.Fragment key={gid}>
                            {/* Group card */}
                            <div style={{
                              background: 'var(--color-bg-2)',
                              borderRadius: 10,
                              border: '1px solid var(--color-border-2)',
                              overflow: 'hidden',
                            }}>
                              {/* Group header */}
                              <div style={{
                                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                                padding: '8px 14px',
                                background: 'var(--color-fill-1)',
                                borderBottom: '1px solid var(--color-border-1)',
                              }}>
                                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                  <span style={{
                                    display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                                    minWidth: 28, height: 24, borderRadius: 12,
                                    background: 'var(--color-info-bg)',
                                    color: 'var(--color-info-text)',
                                    fontSize: 11, fontWeight: 700, padding: '0 8px',
                                  }}>
                                    G{gid}
                                  </span>
                                  <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
                                    {groupConds.length} 条条件 (AND)
                                  </span>
                                </div>
                                <Popconfirm title="删除整组条件？" onOk={() => removeConditionGroup(condTab, gid)}>
                                  <Button size="mini" type="text" style={{ color: 'var(--color-text-3)' }} icon={<Trash2 size={12} />} />
                                </Popconfirm>
                              </div>
                              {/* Group body: condition list */}
                              <div style={{ padding: '8px 14px', display: 'flex', flexDirection: 'column', gap: 8 }}>
                                {groupConds.map((c: any, cIdx: number) => {
                                  const globalIdx = conditions.indexOf(c);
                                  const info = getIndicatorInfo(c.indicator);
                                  const isCross = info?.type === 'cross';
                                  const safeTag = info?.backtestSafe ? '🟢' : (info?.dataNote?.startsWith('🚫') ? '🚫' : '🟡');
                                  return (
                                    <div key={c.id || cIdx} style={{
                                      display: 'flex', flexDirection: 'column', gap: 6,
                                      padding: '8px 12px',
                                      background: 'var(--color-bg-1)',
                                      borderRadius: 8,
                                      border: '1px solid var(--color-border-1)',
                                    }}>
                                      {/* Condition row */}
                                      <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                                        {/* Indicator selector */}
                                        <Tooltip content={<div style={{maxWidth:260}}>{info?.desc}<br/><span style={{color:'var(--color-text-3)',fontSize:11}}>{info?.dataNote}</span></div>} position="bottom">
                                          <IndicatorPicker
                                            value={c.indicator}
                                            onChange={v => handleIndicatorChange(globalIdx, v)}
                                            indicators={indicators}
                                            size="small"
                                            style={{ width: 180 }}
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

                                        <div style={{ flex: 1 }} />

                                        {/* Delete condition */}
                                        <Popconfirm title="移除该条件？" onOk={() => removeCondition(globalIdx)}>
                                          <Button size="mini" type="text" style={{ color: 'var(--color-text-3)', padding: '0 4px' }} icon={<Trash2 size={13} />} />
                                        </Popconfirm>
                                      </div>
                                      {/* Indicator suggestion */}
                                      {info?.suggestion && c.indicator && (
                                        <div style={{ fontSize: 11, color: 'var(--color-info-text)', lineHeight: 1.5, paddingLeft: 4 }}>
                                          💡 {info.suggestion}
                                        </div>
                                      )}
                                      {/* Indicator detail panel */}
                                      {info && c.indicator && (
                                        <div style={{ padding: '6px 12px', background: 'var(--color-fill-2)', borderRadius: 6, borderLeft: '3px solid var(--color-border-1)' }}>
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
                                      {/* Cross custom mode inputs */}
                                      {isCross && (() => {
                                        const presets = (CROSS_PRESETS[c.indicator] || []).filter((p: any) => p.operator);
                                        return !presets.some((p: any) => p.operator === c.operator && p.value === c.value);
                                      })() && (
                                        <div style={{ display: 'flex', gap: 8 }}>
                                          <Select value={c.operator} onChange={v => updateCondition(globalIdx, 'operator', v)} style={{ width: 110 }} size="small"
                                            options={[{ label: '↑ 上穿 (金叉)', value: 'cross_up' }, { label: '↓ 下穿 (死叉)', value: 'cross_down' }]} />
                                          <Input value={typeof c.value === 'number' ? `${Math.floor(c.value)}/${Math.round((c.value - Math.floor(c.value)) * 1000)}` : String(c.value)}
                                            onChange={v => updateCondition(globalIdx, 'value', v)} style={{ width: 80, fontFamily: 'monospace' }} size="small" placeholder="如 5/20" />
                                        </div>
                                      )}
                                      {/* Advanced Options V2 */}
                                      {(() => {
                                        const isOpen = condAdvOpen[globalIdx] || false;
                                        const isScoringMode = (orchConfig.orchestrationMode || 'hybrid') === 'scoring' || (orchConfig.orchestrationMode || 'hybrid') === 'hybrid';
                                        const isDecisionTreeMode = orchConfig.orchestrationMode === 'decision_tree';
                                        const showWeight = isScoringMode && (c.condType === 'buy' || c.condType === 'add');
                                        const showFuzzy = isScoringMode;
                                        const showLookback = true;
                                        const showConsecutive = true;
                                        const showTrend = c.condType === 'buy' || c.condType === 'add';
                                        const showIndustryRel = ['pe_ttm', 'pb', 'ps_ttm', 'roe', 'roa', 'debt_ratio'].includes(c.indicator);
                                        const showTimeframe = isScoringMode;
                                        const showTreeOp = isDecisionTreeMode && (c.condType === 'sell' || c.condType === 'reduce');
                                        const hasAdv = showWeight || showFuzzy || showLookback || showConsecutive || showTrend || showIndustryRel || showTimeframe || showTreeOp;
                                        if (!hasAdv) return null;
                                        return (
                                          <div style={{ marginTop: 4 }}>
                                            <Button size="mini" type="text" style={{ fontSize: 10, color: 'var(--color-text-3)', padding: '0 2px' }}
                                              onClick={() => setCondAdvOpen(prev => ({ ...prev, [globalIdx]: !isOpen }))}>
                                              {isOpen ? '收起高级选项' : '高级选项 ▸'} {isOpen ? '▴' : '▾'}
                                            </Button>
                                            {isOpen && (
                                              <div style={{ marginTop: 6, padding: '8px 12px', background: 'var(--color-fill-2)', borderRadius: 6, display: 'flex', flexDirection: 'column', gap: 6 }}>
                                                {showWeight && (
                                                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                                    <Tooltip content="该条件在总分中的权重（0~5），越高对最终买卖决策影响越大">
                                                    <span style={{ fontSize: 11, color: 'var(--color-text-3)', minWidth: 60, cursor: 'help', borderBottom: '1px dotted var(--color-text-3)' }}>权重</span>
                                                  </Tooltip>
                                                    <InputNumber value={c.weight || 1.0} onChange={v => updateCondition(globalIdx, 'weight', v ?? 1.0)}
                                                      style={{ width: 80 }} size="mini" min={0} max={5} step={0.5} />
                                                  </div>
                                                )}
                                                {showFuzzy && (
                                                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                                    <Tooltip content="模糊度 σ：0=精确阈值判断，越大越宽松（如 PE<15 在 σ=2 时 PE=16.5 也能部分得分）">
                                                    <span style={{ fontSize: 11, color: 'var(--color-text-3)', minWidth: 60, cursor: 'help', borderBottom: '1px dotted var(--color-text-3)' }}>模糊评分</span>
                                                  </Tooltip>
                                                    <InputNumber value={c.fuzzySigma || 0} onChange={v => updateCondition(globalIdx, 'fuzzySigma', v ?? 0)}
                                                      style={{ width: 80 }} size="mini" min={0} max={5} step={0.5} />
                                                    <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>σ, 0=精确</span>
                                                  </div>
                                                )}
                                                {showLookback && (
                                                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                                    <Tooltip content="统计过去 N 个交易日中该条件成立的占比，用于时序评分（如 N=5 中 4 天成立 → 80% 得分）">
                                                    <span style={{ fontSize: 11, color: 'var(--color-text-3)', minWidth: 60, cursor: 'help', borderBottom: '1px dotted var(--color-text-3)' }}>回溯天数</span>
                                                  </Tooltip>
                                                    <InputNumber value={c.lookbackDays || 1} onChange={v => updateCondition(globalIdx, 'lookbackDays', v ?? 1)}
                                                      style={{ width: 80 }} size="mini" min={1} max={60} />
                                                  </div>
                                                )}
                                                {showConsecutive && (
                                                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                                    <Tooltip content="要求条件连续 N 天成立才给满分，否则得分减半。用于过滤昙花一现的假信号">
                                                    <span style={{ fontSize: 11, color: 'var(--color-text-3)', minWidth: 60, cursor: 'help', borderBottom: '1px dotted var(--color-text-3)' }}>连续天数</span>
                                                  </Tooltip>
                                                    <InputNumber value={c.consecutiveDays || 1} onChange={v => updateCondition(globalIdx, 'consecutiveDays', v ?? 1)}
                                                      style={{ width: 80 }} size="mini" min={1} max={10} />
                                                  </div>
                                                )}
                                                {showTrend && (
                                                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                                    <Tooltip content="额外加分：改善中→近期动量向上加分，恶化中→近期动量向下减分。用于过滤逆势信号">
                                                    <span style={{ fontSize: 11, color: 'var(--color-text-3)', minWidth: 60, cursor: 'help', borderBottom: '1px dotted var(--color-text-3)' }}>趋势方向</span>
                                                  </Tooltip>
                                                    <Select value={c.trendDirection || 'none'} onChange={v => updateCondition(globalIdx, 'trendDirection', v)}
                                                      style={{ width: 120 }} size="mini"
                                                      options={[
                                                        { label: '不关注', value: 'none' },
                                                        { label: '改善中', value: 'improving' },
                                                        { label: '恶化中', value: 'deteriorating' },
                                                      ]} />
                                                  </div>
                                                )}
                                                {showIndustryRel && (
                                                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                                    <Tooltip content="开启后，阈值自动相对行业中位数调整（如 PE < 行业中位数 50% 而非固定值 15）">
                                                    <span style={{ fontSize: 11, color: 'var(--color-text-3)', minWidth: 60, cursor: 'help', borderBottom: '1px dotted var(--color-text-3)' }}>行业相对</span>
                                                  </Tooltip>
                                                    <input type="checkbox" checked={c.industryRelative || false}
                                                      onChange={e => updateCondition(globalIdx, 'industryRelative', e.target.checked)}
                                                      style={{ accentColor: '#165DFF' }} />
                                                    <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>阈值相对行业中位数</span>
                                                  </div>
                                                )}
                                                {showTimeframe && (
                                                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                                    <Tooltip content="周线/月线回测暂不支持，仅实盘可用">
                                                      <span style={{ fontSize: 11, color: 'var(--color-text-3)', minWidth: 60, cursor: 'help', borderBottom: '1px dotted var(--color-text-3)' }}>K线周期</span>
                                                    </Tooltip>
                                                    <Select value={c.timeframe || 'daily'} onChange={v => updateCondition(globalIdx, 'timeframe', v)}
                                                      style={{ width: 100 }} size="mini"
                                                      options={[
                                                        { label: '日线', value: 'daily' },
                                                        { label: '周线 ⚠', value: 'weekly', disabled: true },
                                                        { label: '月线 ⚠', value: 'monthly', disabled: true },
                                                      ]} />
                                                  </div>
                                                )}
                                                {showTreeOp && (
                                                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                                    <Tooltip content="该条件与上级条件的逻辑关系：AND=同时满足，OR=任一满足，NOT=反转条件">
                                                    <span style={{ fontSize: 11, color: 'var(--color-text-3)', minWidth: 60, cursor: 'help', borderBottom: '1px dotted var(--color-text-3)' }}>条件逻辑</span>
                                                  </Tooltip>
                                                    <Select value={c.treeOperator || 'and'} onChange={v => updateCondition(globalIdx, 'treeOperator', v)}
                                                      style={{ width: 100 }} size="mini"
                                                      options={[
                                                        { label: 'AND', value: 'and' },
                                                        { label: 'OR', value: 'or' },
                                                        { label: 'NOT', value: 'not' },
                                                      ]} />
                                                  </div>
                                                )}
                                              </div>
                                            )}
                                          </div>
                                        );
                                      })()}
                                    </div>
                                  );
                                })}
                                {/* Add condition to this group */}
                                <Button size="mini" type="dashed" icon={<Plus size={11} />}
                                  onClick={() => addConditionToGroup(condTab, gid)}
                                  style={{ alignSelf: 'flex-start', fontSize: 11 }}>
                                  添加条件
                                </Button>
                              </div>
                            </div>
                            {/* OR separator between groups */}
                            {!isLastGroup && (
                              <div style={{
                                display: 'flex', alignItems: 'center', gap: 8, padding: '0 4px',
                              }}>
                                <div style={{ flex: 1, height: 0, borderTop: '1px dashed var(--color-border-2)' }} />
                                <span style={{
                                  fontSize: 10, fontWeight: 700, color: 'var(--color-warning-text)',
                                  padding: '2px 10px', borderRadius: 10,
                                  background: 'var(--color-fill-2)', letterSpacing: 1,
                                }}>或</span>
                                <div style={{ flex: 1, height: 0, borderTop: '1px dashed var(--color-border-2)' }} />
                              </div>
                            )}
                          </React.Fragment>
                        );
                      });
                    })()
                  )}
                </div>

                {/* Action bar */}
                {filteredConds(condTab).length > 0 && (
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <Button size="small" icon={<Plus size={12} />} type="dashed" onClick={() => addConditionGroup(condTab)}>
                      新建条件组
                    </Button>
                    <div style={{ flex: 1 }} />
                    <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
                      {(() => {
                        const grp = filteredConds(condTab).reduce((acc: Record<number, any[]>, c: any) => {
                          const g = c.logicGroup || 1;
                          if (!acc[g]) acc[g] = [];
                          acc[g].push(c);
                          return acc;
                        }, {} as Record<number, any[]>);
                        return `共 ${Object.keys(grp).length} 组 · ${filteredConds(condTab).length} 条条件`;
                      })()}
                    </span>
                    <Button size="small" type="primary" onClick={saveConditions} style={{ borderRadius: 8 }}>
                      保存条件
                    </Button>
                  </div>
                )}
              </>
            ) : tab === "positionMgmt" ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
                {/* ── 移动止盈 ── */}
                <div style={{ padding: '16px 18px', background: 'var(--color-bg-1)', borderRadius: 10, border: '1px solid var(--color-border-1)' }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <TrendingUp size={16} style={{ color: '#00B42A' }} />
                      <span style={{ fontSize: 14, fontWeight: 600 }}>移动止盈</span>
                    </div>
                    <Tag color={activeStrategy.enableTrailingStop ? 'green' : 'gray'} size="small">
                      {activeStrategy.enableTrailingStop ? '已启用' : '未启用'}
                    </Tag>
                  </div>
                  <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 10 }}>
                    {activeStrategy.enableTrailingStop
                      ? `从峰值回落 ${activeStrategy.trailingStopDrawdown || 8}% 自动卖出，让利润奔跑`
                      : '涨了不止盈，等回撤再卖。开启后替代固定止盈'}
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
                    <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>启用</span>
                    <input type="checkbox" checked={activeStrategy.enableTrailingStop || false}
                      onChange={e => handleUpdateStrategy('enableTrailingStop', e.target.checked)}
                      style={{ accentColor: '#00B42A' }} />
                  </div>
                  {activeStrategy.enableTrailingStop && (
                    <>
                      <div style={{ padding: '8px 12px', background: 'var(--color-fill-1)', borderRadius: 6, fontSize: 11, color: 'var(--color-text-3)', lineHeight: 1.6, marginBottom: 10 }}>
                        💡 例：买入价¥10 → 涨到¥15(激活{activeStrategy.trailingStopActivation || 15}%) → 继续涨到¥20 → 回撤{activeStrategy.trailingStopDrawdown || 8}%到¥18.4 → 自动卖出锁定84%利润
                      </div>
                      <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', alignItems: 'center' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>激活阈值</span>
                          <InputNumber value={activeStrategy.trailingStopActivation || 15}
                            onChange={v => handleUpdateStrategy('trailingStopActivation', v || 15)}
                            min={5} max={50} step={1} suffix="%" style={{ width: 80 }} size="small" />
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>回撤比例</span>
                          <InputNumber value={activeStrategy.trailingStopDrawdown || 8}
                            onChange={v => handleUpdateStrategy('trailingStopDrawdown', v || 8)}
                            min={3} max={25} step={1} suffix="%" style={{ width: 80 }} size="small" />
                        </div>
                      </div>
                      <div style={{ marginTop: 10, padding: '8px 10px', background: 'var(--color-fill-2)', borderRadius: 6, fontSize: 10, color: 'var(--color-text-4)' }}>
                        🟢 牛市: 回撤10% · 🟠 结构: 回撤8% · 🟡 轮动: 回撤5% · 🟤 磨底: 回撤4%
                      </div>
                    </>
                  )}
                </div>

                {/* ── 抄底反弹 ── */}
                <div style={{ padding: '16px 18px', background: 'var(--color-bg-1)', borderRadius: 10, border: '1px solid var(--color-border-1)' }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <TrendingDown size={16} style={{ color: '#F7BA1E' }} />
                      <span style={{ fontSize: 14, fontWeight: 600 }}>抄底反弹</span>
                    </div>
                    <Tag color={activeStrategy.enableDipBuy ? 'green' : 'gray'} size="small">
                      {activeStrategy.enableDipBuy ? '已启用' : '未启用'}
                    </Tag>
                  </div>
                  <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 10 }}>
                    {activeStrategy.enableDipBuy
                      ? `持仓跌超 ${activeStrategy.dipBuyThreshold || -15}% 自动补仓, 反弹 ${activeStrategy.dipTargetReturn || 5}% 达标卖出`
                      : '持仓大跌时自动补仓做波段反弹，达标自动卖出'}
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
                    <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>启用</span>
                    <input type="checkbox" checked={activeStrategy.enableDipBuy || false}
                      onChange={e => handleUpdateStrategy('enableDipBuy', e.target.checked)}
                      style={{ accentColor: '#F7BA1E' }} />
                  </div>
                  {activeStrategy.enableDipBuy && (
                    <>
                      <div style={{ padding: '8px 12px', background: 'var(--color-fill-1)', borderRadius: 6, fontSize: 11, color: 'var(--color-text-3)', lineHeight: 1.6, marginBottom: 10 }}>
                        💡 例：建仓价¥10 → 跌到¥{((activeStrategy.dipBuyThreshold || -15) < 0 ? (10 + 10*(activeStrategy.dipBuyThreshold || -15)/100).toFixed(1) : '8.5')}(跌{Math.abs(activeStrategy.dipBuyThreshold || -15)}%) 自动抄底 → 反弹{activeStrategy.dipTargetReturn || 5}%到目标价 → 卖出抄底批次，保留底仓
                      </div>
                      <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', alignItems: 'center', marginBottom: 8 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>触发跌幅</span>
                          <InputNumber value={activeStrategy.dipBuyThreshold || -15}
                            onChange={v => handleUpdateStrategy('dipBuyThreshold', v || -15)}
                            min={-30} max={-5} step={1} suffix="%" style={{ width: 80 }} size="small" />
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>抄底仓位</span>
                          <InputNumber value={activeStrategy.dipBuyAmountPct || 10}
                            onChange={v => handleUpdateStrategy('dipBuyAmountPct', v || 10)}
                            min={3} max={25} step={1} suffix="%" style={{ width: 80 }} size="small" />
                        </div>
                      </div>
                      <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', alignItems: 'center', marginBottom: 8 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>目标收益</span>
                          <InputNumber value={activeStrategy.dipTargetReturn || 5}
                            onChange={v => handleUpdateStrategy('dipTargetReturn', v || 5)}
                            min={2} max={15} step={1} suffix="%" style={{ width: 80 }} size="small" />
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>最大持有</span>
                          <InputNumber value={activeStrategy.dipMaxHoldDays || 3}
                            onChange={v => handleUpdateStrategy('dipMaxHoldDays', v || 3)}
                            min={1} max={10} step={1} suffix="天" style={{ width: 80 }} size="small" />
                        </div>
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>冷却期</span>
                        <InputNumber value={activeStrategy.dipCooldownDays || 10}
                          onChange={v => handleUpdateStrategy('dipCooldownDays', v || 10)}
                          min={5} max={30} step={1} suffix="天" style={{ width: 80 }} size="small" />
                      </div>
                    </>
                  )}
                </div>

                {/* ── 网格做T ── */}
                <div style={{ padding: '16px 18px', background: 'var(--color-bg-1)', borderRadius: 10, border: '1px solid var(--color-border-1)' }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <Grid3X3 size={16} style={{ color: '#722ED1' }} />
                      <span style={{ fontSize: 14, fontWeight: 600 }}>网格做T</span>
                    </div>
                    <Tag color={activeStrategy.enableGrid ? 'green' : 'gray'} size="small">
                      {activeStrategy.enableGrid ? '已启用' : '未启用'}
                    </Tag>
                  </div>
                  <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 10 }}>
                    {activeStrategy.enableGrid
                      ? `BOLL收口<${activeStrategy.gridTriggerSqueeze || 8}激活, ${activeStrategy.gridLevels || 3}层网格 每格${activeStrategy.gridLotPct || 5}%`
                      : '震荡市自动高抛低吸，保留底仓。BOLL收口自动激活网格'}
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
                    <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>启用</span>
                    <input type="checkbox" checked={activeStrategy.enableGrid || false}
                      onChange={e => handleUpdateStrategy('enableGrid', e.target.checked)}
                      style={{ accentColor: '#722ED1' }} />
                  </div>
                  {activeStrategy.enableGrid && (
                    <>
                      <div style={{ padding: '8px 12px', background: 'var(--color-fill-1)', borderRadius: 6, fontSize: 11, color: 'var(--color-text-3)', lineHeight: 1.6, marginBottom: 10 }}>
                        💡 BOLL收口(squeeze&lt;{activeStrategy.gridTriggerSqueeze || 8})激活 → {activeStrategy.gridLevels || 3}层网格布林带内自动买卖 → 每格赚3%+就卖 → BOLL宽口(squeeze&gt;30)退役清仓
                      </div>
                      <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', alignItems: 'center' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>震荡阈值</span>
                          <InputNumber value={activeStrategy.gridTriggerSqueeze || 8}
                            onChange={v => handleUpdateStrategy('gridTriggerSqueeze', v || 8)}
                            min={3} max={15} step={1} style={{ width: 80 }} size="small" />
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>网格层数</span>
                          <InputNumber value={activeStrategy.gridLevels || 3}
                            onChange={v => handleUpdateStrategy('gridLevels', v || 3)}
                            min={2} max={6} step={1} style={{ width: 80 }} size="small" />
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)', minWidth: 56 }}>每格仓位</span>
                          <InputNumber value={activeStrategy.gridLotPct || 5}
                            onChange={v => handleUpdateStrategy('gridLotPct', v || 5)}
                            min={2} max={15} step={1} suffix="%" style={{ width: 80 }} size="small" />
                        </div>
                      </div>
                    </>
                  )}
                </div>
              </div>
            ) : tab === "riskControl" ? (

              <>
                {/* ══ 智能风控 ══ */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: 14, marginBottom: 16 }}>
                  
                  {/* ── 风控强度预设 ── */}
                  <div style={{ padding: '14px 18px', background: 'var(--color-bg-1)', borderRadius: 10, border: '1px solid var(--color-border-1)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
                      <Shield size={16} style={{ color: '#165DFF' }} />
                      <span style={{ fontSize: 14, fontWeight: 600 }}>风控强度</span>
                    </div>

                    {/* Preset selector */}
                    {(() => {
                      const presets = [
                        { key: 'conservative', label: '🐢 保守', desc: '熊市/新手', defT: 0.0, aggT: 2.0, meltT: -2.0, bias: 0.8, color: '#00B42A' },
                        { key: 'balanced', label: '⚖️ 均衡', desc: '日常推荐', defT: -1.0, aggT: 1.5, meltT: -3.0, bias: 1.0, color: '#165DFF' },
                        { key: 'aggressive', label: '🐇 激进', desc: '牛市/结构行情', defT: -2.0, aggT: 1.0, meltT: -4.0, bias: 1.2, color: '#F7BA1E' },
                        { key: 'custom', label: '🔧 自定义', desc: '高级用户', defT: null, aggT: null, meltT: null, bias: null, color: 'var(--color-text-3)' },
                      ];
                      const applyPreset = (p: any) => {
                        setRiskPreset(p.key);
                        if (p.key === 'custom') return;
                        setOrchConfig((prev: any) => ({
                          ...prev,
                          defensiveThreshold: p.defT,
                          aggressiveThreshold: p.aggT,
                          marketCompositeMin: p.meltT,
                          marketPositionBias: p.bias,
                        }));
                      };
                      return (
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr 1fr', gap: 8, marginBottom: 12 }}>
                          {presets.map(p => {
                            const isActive = riskPreset === p.key;
                            return (
                              <div key={p.key}
                                onClick={() => applyPreset(p)}
                                style={{
                                  padding: '10px 8px', borderRadius: 8, cursor: 'pointer', textAlign: 'center',
                                  border: isActive ? `2px solid ${p.color}` : '2px solid var(--color-border-1)',
                                  background: isActive ? `${p.color}10` : 'var(--color-bg-2)',
                                }}>
                                <div style={{ fontSize: 13, fontWeight: 600 }}>{p.label}</div>
                                <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 2 }}>{p.desc}</div>
                              </div>
                            );
                          })}
                        </div>
                      );
                    })()}

                    {/* Rules visualization — scenario-based */}
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                      <div style={{ padding: '10px 14px', borderRadius: 8, background: '#00B42A10', border: '1px solid #00B42A25', fontSize: 11, lineHeight: 1.6 }}>
                        <div style={{ fontWeight: 700, fontSize: 12, color: '#00B42A', marginBottom: 2 }}>🟢 市场好时</div>
                        <div style={{ color: 'var(--color-text-2)' }}>
                          综合分 ≥ <b style={{ fontFamily: "'SF Mono',monospace" }}>{orchConfig.aggressiveThreshold ?? 1.5}</b> →
                          <b style={{ color: '#00B42A' }}>全仓进攻</b>
                          · 单票最多买 <b>20%</b> · <b>可以加仓</b>
                        </div>
                        <div style={{ color: 'var(--color-text-4)', marginTop: 2 }}>适合：牛市普涨、温和上涨行情</div>
                      </div>
                      <div style={{ padding: '10px 14px', borderRadius: 8, background: '#F7BA1E10', border: '1px solid #F7BA1E25', fontSize: 11, lineHeight: 1.6 }}>
                        <div style={{ fontWeight: 700, fontSize: 12, color: '#F7BA1E', marginBottom: 2 }}>🟡 市场一般时</div>
                        <div style={{ color: 'var(--color-text-2)' }}>
                          综合分 ≥ <b style={{ fontFamily: "'SF Mono',monospace" }}>{orchConfig.defensiveThreshold ?? 0}</b> →
                          <b style={{ color: '#F7BA1E' }}>半仓防御</b>
                          · 单票最多买 <b>10%</b> · <b>不加仓</b>
                        </div>
                        <div style={{ color: 'var(--color-text-4)', marginTop: 2 }}>适合：结构分化、震荡轮动行情</div>
                      </div>
                      <div style={{ padding: '10px 14px', borderRadius: 8, background: '#F53F3F10', border: '1px solid #F53F3F25', fontSize: 11, lineHeight: 1.6 }}>
                        <div style={{ fontWeight: 700, fontSize: 12, color: '#F53F3F', marginBottom: 2 }}>🔴 市场不好时</div>
                        <div style={{ color: 'var(--color-text-2)' }}>
                          综合分 ≥ <b style={{ fontFamily: "'SF Mono',monospace" }}>{orchConfig.marketCompositeMin ?? -2}</b> →
                          <b style={{ color: '#F53F3F' }}>空仓观望</b>
                          · <b>不买新股</b> · 只平仓止损
                        </div>
                        <div style={{ color: 'var(--color-text-4)', marginTop: 2 }}>适合：熊市下跌、底部磨底行情</div>
                      </div>
                      <div style={{ padding: '10px 14px', borderRadius: 8, background: '#1A1A1A10', border: '1px solid #1A1A1A25', fontSize: 11, lineHeight: 1.6 }}>
                        <div style={{ fontWeight: 700, fontSize: 12, color: '#1A1A1A', marginBottom: 2 }}>⚫ 极端行情</div>
                        <div style={{ color: 'var(--color-text-2)' }}>
                          综合分 &lt; <b style={{ fontFamily: "'SF Mono',monospace" }}>{orchConfig.marketCompositeMin ?? -2}</b> →
                          <b style={{ color: '#1A1A1A' }}>强制清仓</b>
                          · <b>全部卖出</b> · 保护本金
                        </div>
                        <div style={{ color: 'var(--color-text-4)', marginTop: 2 }}>适合：恐慌暴跌行情</div>
                      </div>
                    </div>
                  </div>

                  {/* ── 仓位上限 ── */}
                  <div style={{ padding: '14px 18px', background: 'var(--color-bg-1)', borderRadius: 10, border: '1px solid var(--color-border-1)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                      <DollarSign size={16} style={{ color: '#00B42A' }} />
                      <span style={{ fontSize: 14, fontWeight: 600 }}>仓位 & 风控上限</span>
                    </div>
                    <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', alignItems: 'center' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>单票最大仓位</span>
                        <InputNumber value={(activeStrategy.positionConcentrationLimit || 0.25) * 100} onChange={v => handleUpdateStrategy('positionConcentrationLimit', (v || 25) / 100)} min={5} max={50} step={5} suffix="%" style={{ width: 85 }} size="small" />
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>单日最大亏损</span>
                        <InputNumber value={activeStrategy.maxDailyLoss || -5} onChange={v => handleUpdateStrategy('maxDailyLoss', v || -5)} max={0} step={1} suffix="%" style={{ width: 85 }} size="small" />
                      </div>
                    </div>
                  </div>

                  {/* ── 高级参数 (折叠) ── */}
                  {/* Market review link */}
                  <a href="/market-review" style={{
                    display: 'flex', alignItems: 'center', gap: 8, padding: '10px 14px',
                    background: 'linear-gradient(135deg, #FF7D0010, #F7BA1E10)',
                    borderRadius: 8, border: '1px solid #FF7D0020',
                    textDecoration: 'none', color: 'var(--color-text-2)', fontSize: 12,
                    marginBottom: 0,
                  }}>
                    <Activity size={14} style={{ color: '#FF7D00' }} />
                    <span>查看市场风格 →</span>
                    <span style={{ marginLeft: 'auto', fontSize: 10, color: 'var(--color-text-4)' }}>市场风格识别 · 结构性分析 · T-1 复盘</span>
                  </a>

                  <div style={{ padding: '14px 18px', background: 'var(--color-bg-1)', borderRadius: 10, border: '1px solid var(--color-border-1)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }} onClick={() => setShowAdvanced(!showAdvanced)}>
                      {showAdvanced ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                      <Settings size={14} style={{ color: 'var(--color-text-3)' }} />
                      <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-3)' }}>高级参数</span>
                      <span style={{ fontSize: 10, color: 'var(--color-text-4)' }}>阈值微调 · 编排引擎 · 各模式参数</span>
                    </div>
                    
                    {showAdvanced && (
                      <div style={{ marginTop: 12, display: 'flex', flexDirection: 'column', gap: 12 }}>
                        {/* Threshold fine-tuning */}
                        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'center' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                            <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>进攻阈值</span>
                            <InputNumber value={orchConfig.aggressiveThreshold ?? 1.5} onChange={v => { setRiskPreset('custom'); setOrchConfig((prev: any) => ({ ...prev, aggressiveThreshold: v ?? 1.5 })); }} min={0} max={5} step={0.5} style={{ width: 65 }} size="small" />
                          </div>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                            <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>防御阈值</span>
                            <InputNumber value={orchConfig.defensiveThreshold ?? 0} onChange={v => { setRiskPreset('custom'); setOrchConfig((prev: any) => ({ ...prev, defensiveThreshold: v ?? 0 })); }} min={-5} max={5} step={0.5} style={{ width: 65 }} size="small" />
                          </div>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                            <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>熔断阈值</span>
                            <InputNumber value={orchConfig.marketCompositeMin ?? -2} onChange={v => { setRiskPreset('custom'); setOrchConfig((prev: any) => ({ ...prev, marketCompositeMin: v ?? -2 })); }} min={-5} max={5} step={0.5} style={{ width: 65 }} size="small" />
                          </div>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                            <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>仓位乘数</span>
                            <InputNumber value={orchConfig.marketPositionBias ?? 1} onChange={v => { setRiskPreset('custom'); setOrchConfig((prev: any) => ({ ...prev, marketPositionBias: v ?? 1 })); }} min={0.5} max={1.5} step={0.1} style={{ width: 65 }} size="small" />
                          </div>
                        </div>

                        {/* Orchestration mode */}
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <span style={{ fontSize: 11, color: 'var(--color-text-3)', minWidth: 56 }}>编排引擎</span>
                          <Select value={orchConfig.orchestrationMode || 'hybrid'} onChange={v => setOrchConfig((prev: any) => ({ ...prev, orchestrationMode: v }))} style={{ width: 160 }} size="small"
                            options={[
                              { label: '混合模式 (推荐)', value: 'hybrid' },
                              { label: '纯评分', value: 'scoring' },
                              { label: '纯决策树', value: 'decision_tree' },
                            ]} />
                        </div>
                      </div>
                    )}
                  </div>



                  {/* AI Agent */}
                  <div style={{ padding: '14px 18px', background: 'var(--color-bg-1)', borderRadius: 10, border: '1px solid var(--color-border-1)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                      <Brain size={16} style={{ color: '#FF7D00' }} />
                      <span style={{ fontSize: 14, fontWeight: 600 }}>AI 代理监督</span>
                      <div style={{ flex: 1 }} />
                      <Tag color={orchConfig.enableAIAgent ? 'orangered' : 'gray'} size="small">
                        {orchConfig.enableAIAgent ? '已启用' : '未启用'}
                      </Tag>
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                      <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>启用</span>
                      <input type="checkbox" checked={orchConfig.enableAIAgent || false}
                        onChange={e => setOrchConfig((prev: any) => ({ ...prev, enableAIAgent: e.target.checked }))}
                        style={{ accentColor: '#165DFF' }} />
                    </div>
                    {orchConfig.enableAIAgent && (
                      <div style={{ marginTop: 10, display: 'flex', gap: 16, flexWrap: 'wrap', alignItems: 'center' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>模式</span>
                          <Select value={orchConfig.aiAgentMode || 'advisory'} onChange={v => setOrchConfig((prev: any) => ({ ...prev, aiAgentMode: v }))}
                            style={{ width: 120 }} size="small"
                            options={[
                              { label: '仅审查 (advisory)', value: 'advisory' },
                              { label: '自动执行 (auto)', value: 'auto' },
                            ]} />
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>审查范围</span>
                          <Select value={orchConfig.aiAgentReviewScope || 'all'} onChange={v => setOrchConfig((prev: any) => ({ ...prev, aiAgentReviewScope: v }))}
                            style={{ width: 110 }} size="small"
                            options={[
                              { label: '全部', value: 'all' },
                              { label: '仅买入', value: 'buy_only' },
                              { label: '仅卖出', value: 'sell_only' },
                            ]} />
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>日最大交易</span>
                          <InputNumber value={orchConfig.aiAgentMaxDailyTrades || 5} onChange={v => setOrchConfig((prev: any) => ({ ...prev, aiAgentMaxDailyTrades: v ?? 5 }))}
                            min={1} max={20} style={{ width: 70 }} size="small" />
                        </div>
                      </div>
                    )}
                  </div>


                  {/* Industry */}
                  <div style={{ padding: '14px 18px', background: 'var(--color-bg-1)', borderRadius: 10, border: '1px solid var(--color-border-1)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                      <Factory size={16} style={{ color: '#00B42A' }} />
                      <span style={{ fontSize: 14, fontWeight: 600 }}>行业上下文</span>
                    </div>
                    <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'center' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>行业过滤</span>
                        <Input value={orchConfig.industryFilter || ''} onChange={v => setOrchConfig((prev: any) => ({ ...prev, industryFilter: v }))}
                          style={{ width: 240 }} size="small" placeholder="逗号分隔，空=全部行业" />
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>板块轮动</span>
                        <input type="checkbox" checked={orchConfig.enableSectorRotation || false}
                          onChange={e => setOrchConfig((prev: any) => ({ ...prev, enableSectorRotation: e.target.checked }))}
                          style={{ accentColor: '#165DFF' }} />
                      </div>
                    </div>
                  </div>

                  {/* Save Button */}
                  <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                    <Button size="small" type="primary" loading={orchSaving}
                      onClick={async () => {
                        if (!activeId) return;
                        setOrchSaving(true);
                        try {
                          await saveOrchestration(activeId, orchConfig);
                          window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'success', message: '编排配置已保存' } }));
                        } catch (e) {
                          window.dispatchEvent(new CustomEvent('app:toast', { detail: { type: 'error', message: '保存失败' } }));
                        } finally {
                          setOrchSaving(false);
                        }
                      }}>
                      保存编排配置
                    </Button>
                  </div>
                </div>
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
                    <Button type="primary" icon={<Play size={13} />} loading={btRunning} onClick={handleStartBacktest}>
                      {btRunning ? '运行中...' : '运行回测'}
                    </Button>
                    {btRunning && btTaskId && (
                      
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

                {/* Live progress with mini progress bar */}
                {btPhase && (() => {
                  // Extract current stage from phase
                  const getStage = (p: string) => {
                    if (p.includes('评分中')) return { name: '评分', color: '#722ED1', icon: '📊' };
                    if (p.includes('执行信号')) return { name: '执行', color: '#0ea5e9', icon: '⚡' };
                    if (p.includes('生成信号')) return { name: '决策', color: '#14b8a6', icon: '🧠' };
                    if (p.includes('初始化')) return { name: '初始化', color: '#909399', icon: '⏳' };
                    if (p.includes('回测完成')) return { name: '完成', color: 'var(--stock-up)', icon: '✅' };
                    if (p.includes('重新连接')) return { name: '重连', color: '#e6a23c', icon: '🔄' };
                    if (p.includes('已取消')) return { name: '取消', color: '#f56c6c', icon: '⏹' };
                    return { name: p, color: 'var(--color-info-text)', icon: '▶' };
                  };
                  const stage = getStage(btPhase);
                  // Check for scoring progress in phase
                  const scoringMatch = btPhase.match(/评分中: (\d+)\/(\d+) 候选(\d+)/);
                  return (
                  <div style={{ marginBottom: 12 }}>
                    <div style={{
                      padding: '8px 12px', background: 'var(--color-info-bg)', borderRadius: '6px 6px 0 0',
                      fontSize: 12, color: 'var(--color-info-text)', display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                    }}>
                      <span style={{ fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
                        {btRunning && <span style={{
                          display: 'inline-block', width: 8, height: 8, borderRadius: '50%',
                          background: stage.color,
                          animation: 'pulse 1.2s ease-in-out infinite',
                        }} />}
                        <span style={{
                          display: 'inline-block', padding: '1px 8px', borderRadius: 10,
                          background: stage.color + '20', color: stage.color,
                          fontSize: 11, fontWeight: 500,
                        }}>{stage.icon} {stage.name}</span>
                        <span style={{ color: 'var(--color-text-2)' }}>{btPhase}</span>
                      </span>
                      {btProgress && <span style={{ color: 'var(--color-text-3)', fontSize: 11 }}>{btProgress}</span>}
                    </div>
                    {/* Mini progress bar */}
                    {(() => {
                      let pct = 0;
                      // Scoring phase: use exact scored/total from phase
                      if (scoringMatch) {
                        const scored = parseInt(scoringMatch[1]) || 0;
                        const total = parseInt(scoringMatch[2]) || 1;
                        pct = Math.min(100, Math.round((scored / total) * 100));
                      } else if (btProgress) {
                        const parts = btProgress.split('/');
                        const cur = parseInt(parts[0]) || 0;
                        const total = parseInt(parts[1]) || 1;
                        pct = Math.min(100, Math.round((cur / total) * 100));
                      }
                      if (pct <= 0) return null;
                      return (
                        <div style={{
                          height: 3, background: 'var(--color-fill-2)', borderRadius: '0 0 6px 6px',
                          overflow: 'hidden',
                        }}>
                          <div style={{
                            height: '100%', width: `${pct}%`,
                            background: `linear-gradient(90deg, ${stage.color}, var(--color-primary))`,
                            transition: 'width 0.3s ease',
                            borderRadius: '0 0 0 6px',
                          }} />
                        </div>
                      );
                    })()}
                  </div>
                  );
                })()}

                {/* Live summary stats during backtest run */}
                {btRunning && btLogs.length > 0 && (() => {
                  const parseDetail = (l: any) => { try { return typeof l.detail === 'string' ? JSON.parse(l.detail) : (l.detail || {}); } catch { return {}; } };
                  const trades = btLogs.filter((l: any) => { const d = parseDetail(l); const a = d.action || l.action; return a === 'buy' || a === 'sell'; });
                  const buyCount = btLogs.filter((l: any) => { const d = parseDetail(l); const a = d.action || l.action; return a === 'buy' || a === 'add'; }).length;
                  const sellCount = btLogs.filter((l: any) => { const d = parseDetail(l); const a = d.action || l.action; return a === 'sell' || a === 'reduce'; }).length;
                  const completedSells = btLogs.filter((l: any) => { const d = parseDetail(l); const a = d.action || l.action; const p = d.pnlPct ?? l.pnlPct; return (a === 'sell' || a === 'reduce') && p !== undefined && p !== null && p !== 0; });
                  const wins = completedSells.filter((l: any) => { const d = parseDetail(l); return (d.pnlPct ?? l.pnlPct ?? 0) > 0; }).length;
                  return (
                    <div style={{ marginBottom: 12, padding: '6px 12px', background: 'var(--color-bg-2)', borderRadius: 6, fontSize: 11, color: 'var(--color-text-2)', display: 'flex', gap: 20, flexWrap: 'wrap', border: '1px solid var(--color-border-1)' }}>
                      <span>交易: <b style={{ color: 'var(--color-text-1)' }}>{trades.length}</b> 笔</span>
                      <span>买入: <b style={{ color: 'var(--color-info-text)' }}>{buyCount}</b></span>
                      <span>卖出: <b style={{ color: 'var(--color-warning-text)' }}>{sellCount}</b></span>
                      {completedSells.length > 0 && (
                        <span>胜率: <b style={{ color: wins / completedSells.length >= 0.5 ? 'var(--stock-up)' : 'var(--stock-down)' }}>{(wins / completedSells.length * 100).toFixed(0)}%</b> ({wins}/{completedSells.length})</span>
                      )}
                    </div>
                  );
                })()}

                {/* Dual Panel: Positions + Trade Log */}
                {(btPositions || btLogs.length > 0 || btResult) && (
                  <div style={{ display: 'grid', gridTemplateColumns: btPositions ? '1fr 1fr' : '1fr', gap: 12, marginBottom: 16 }}>
                    {/* Left: Positions Panel */}
                    {btPositions && (
                      <div style={{ background: 'var(--color-bg-1)', border: '1px solid var(--color-border-1)', borderRadius: 8, padding: 12 }}>
                        <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8, display: 'flex', justifyContent: 'space-between' }}>
                          <span><BarChart4 size={14} style={{ marginRight: 4, verticalAlign: 'middle' }} />持仓快照 ({btPositions.date})</span>
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

                    {/* Right: Console — Daily Timeline */}
                    <div style={{ background: 'var(--color-fill-1)', border: '1px solid var(--color-border-1)', borderRadius: 8, padding: 12, display: 'flex', flexDirection: 'column', maxHeight: 400 }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                        <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>
                          <ScrollText size={14} style={{ marginRight: 4, verticalAlign: 'middle' }} />控制台
                          <span style={{ fontSize: 11, color: 'var(--color-text-3)', marginLeft: 8, fontWeight: 400 }}>
                            {btLogs.length} 条
                          </span>
                        </div>
                        <div style={{ display: 'flex', gap: 4 }}>
                          {(['all', 'trade', 'signal', 'system'] as const).map(f => {
                            const labels: Record<string, string> = { all: '全部', trade: '交易', signal: '信号', system: '系统' };
                            return (
                              <button key={f}
                                onClick={() => setLogFilter(f)}
                                style={{
                                  padding: '2px 8px', borderRadius: 4, border: 'none', cursor: 'pointer', fontSize: 11,
                                  background: logFilter === f ? 'var(--color-info-bg)' : 'transparent',
                                  color: logFilter === f ? 'var(--color-info-text)' : 'var(--color-text-3)',
                                  fontWeight: logFilter === f ? 600 : 400,
                                }}
                              >{labels[f]}</button>
                            );
                          })}
                        </div>
                      </div>
                      <div ref={logEndRef} style={{ flex: 1, overflow: 'auto', color: 'var(--color-text-2)', fontSize: 11 }}>
                        {btLogs.length === 0 && <div style={{ color: 'var(--color-text-3)', padding: 20, textAlign: 'center' }}>等待日志...</div>}
                        {/* Group logs by date for timeline view */}
                        {(() => {
                          const filtered = logFilter === 'all' ? btLogs : btLogs.filter((l: any) => {
                            if (logFilter === 'trade') return l.logType === 'trade';
                            if (logFilter === 'signal') return l.logType === 'signal';
                            if (logFilter === 'system') return l.logType === 'system';
                            return true;
                          });
                          // Group by date
                          const groups: Record<string, any[]> = {};
                          filtered.forEach((l: any) => {
                            const d = l.date || '系统';
                            if (!groups[d]) groups[d] = [];
                            groups[d].push(l);
                          });
                          return Object.entries(groups).map(([date, logs]) => {
                            // Day summary stats
                            const dayTrades = logs.filter((l: any) => l.logType === 'trade');
                            const daySignals = logs.filter((l: any) => l.logType === 'signal');
                            const buySignals = daySignals.filter((l: any) => l.message?.includes('[buy]') || l.message?.includes('[add]'));
                            const sellSignals = daySignals.filter((l: any) => l.message?.includes('[sell]') || l.message?.includes('[reduce]') || l.message?.includes('[stop]'));
                            const isSystemDate = date === '系统';
                            return (
                              <div key={date} style={{ marginBottom: isSystemDate ? 0 : 6 }}>
                                {/* Day header */}
                                {!isSystemDate && (
                                  <div style={{
                                    padding: '4px 8px', borderRadius: 4, cursor: 'pointer',
                                    background: 'var(--color-bg-2)', border: '1px solid var(--color-border-1)',
                                    marginBottom: 2, display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                                    fontSize: 11,
                                  }}>
                                    <span style={{ fontWeight: 600, color: 'var(--color-text-1)' }}>📅 {date}</span>
                                    <span style={{ color: 'var(--color-text-3)' }}>
                                      {dayTrades.length > 0 && <span style={{ marginRight: 8 }}>交易 {dayTrades.length}笔</span>}
                                      {buySignals.length > 0 && <span style={{ color: 'var(--stock-up)', marginRight: 8 }}>买入信号 {buySignals.length}</span>}
                                      {sellSignals.length > 0 && <span style={{ color: 'var(--stock-down)' }}>卖出信号 {sellSignals.length}</span>}
                                      <span style={{ marginLeft: 8 }}>{logs.length}条日志</span>
                                    </span>
                                  </div>
                                )}
                                {/* Day logs */}
                                <div style={{ paddingLeft: isSystemDate ? 0 : 4 }}>
                                  {logs.map((l: any, i: number) => {
                                    if (l.logType === 'system') {
                                      const msg = l.message || '';
                                      // Semantic color coding based on log type
                                      const getSysColor = (m: string) => {
                                        if (m.includes('▸ 风控')) return '#e6a23c';
                                        if (m.includes('▸ 市场扫描')) return 'var(--color-info-text)';
                                        if (m.includes('▸ 评分漏斗')) return '#722ED1';
                                        if (m.includes('▸ 决策完成')) return 'var(--stock-up)';
                                        if (m.includes('▸ 持仓检查')) return '#0ea5e9';
                                        if (m.includes('▸ 日终结算')) return 'var(--color-primary)';
                                        if (m.includes('▸ 信号输出')) return '#14b8a6';
                                        if (m.includes('▸ 跳过买入') || m.includes('▸ 今日无信号')) return '#909399';
                                        if (m.includes('⏱')) return 'var(--color-text-3)';
                                        if (m.includes('⏭')) return 'var(--color-warning-text)';
                                        if (m.includes('结束') || m.includes('完成')) return 'var(--color-primary)';
                                        return 'var(--color-text-3)';
                                      };
                                      const sysColor = getSysColor(msg);
                                      const isDayBoundary = msg.includes('━━');
                                      if (isSystemDate) {
                                        return (
                                          <div key={l.id || i} style={{
                                            padding: '3px 0', color: sysColor,
                                            fontWeight: msg.includes('▸') ? 600 : 400,
                                            borderBottom: msg.includes('━━') ? '1px solid var(--color-border-1)' : 'none',
                                          }}>{l.message}</div>
                                        );
                                      }
                                      // Skip day boundary lines in daily groups (already shown in header)
                                      if (isDayBoundary) return null;
                                      return (
                                        <div key={l.id || i} style={{
                                          padding: '1px 0', color: sysColor,
                                          fontFamily: 'monospace', fontWeight: msg.includes('▸') ? 500 : 400,
                                        }}>{l.message}</div>
                                      );
                                    }
                                    if (l.logType === 'signal') {
                                      const isSkip = l.message?.includes('跳过') || l.level === 'warn';
                                      return (
                                        <div key={l.id || i} style={{
                                          padding: '1px 0', color: isSkip ? 'var(--color-warning-text)' : 'var(--color-info-text)',
                                          fontFamily: 'monospace',
                                        }}>{l.message}</div>
                                      );
                                    }
                                    // Trade log
                                    let detail: any = {};
                                    try { detail = typeof l.detail === 'string' ? JSON.parse(l.detail) : (l.detail || {}); } catch {}
                                    const action = detail.action || l.action || '';
                                    const price = detail.price || l.price || 0;
                                    const quantity = detail.quantity || l.quantity || 0;
                                    const pnlPct = detail.pnlPct ?? l.pnlPct;
                                    const reason = detail.reason || l.reason || l.message || '';
                                    const actColors: Record<string, string> = { buy: 'var(--stock-up)', add: 'var(--color-warning-text)', sell: 'var(--stock-down)', reduce: 'var(--color-info-text)', stop: 'var(--color-warning-text)' };
                                    const actLabels: Record<string, string> = { buy: 'B', add: '+', sell: 'S', reduce: '-', stop: 'X' };
                                    return (
                                      <div key={l.id || i} style={{
                                        padding: '1px 0', fontFamily: 'monospace',
                                        borderLeft: `2px solid ${actColors[action] || 'transparent'}`,
                                        paddingLeft: 6,
                                      }}>
                                        <span style={{ color: actColors[action], fontWeight: 600 }}>[{actLabels[action]}]</span>{' '}
                                        <span>{l.stockCode}</span>{' '}
                                        {price > 0 && <span>@{typeof price === 'number' ? price.toFixed(2) : price} ×{quantity}</span>}{' '}
                                        {pnlPct !== undefined && pnlPct !== null && pnlPct !== 0 && (
                                          <span style={{ color: pnlPct > 0 ? 'var(--stock-up)' : 'var(--stock-down)', fontWeight: 600 }}>
                                            {pnlPct > 0 ? '+' : ''}{typeof pnlPct === 'number' ? pnlPct.toFixed(2) : pnlPct}%
                                          </span>
                                        )}
                                        {reason && <span style={{ color: 'var(--color-text-3)', marginLeft: 4 }}>{reason}</span>}
                                      </div>
                                    );
                                  })}
                                </div>
                              </div>
                            );
                          });
                        })()}
                      </div>
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
                          <CheckCircle size={11} style={{ color: '#00B42A', verticalAlign: 'middle' }} /> K线安全 {btResult.klineSafe} / <AlertTriangle size={11} style={{ color: '#F7BA1E', verticalAlign: 'middle' }} /> 需验证 {btResult.klineUnsafe}
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
                      onRow={(record: any) => ({
                        onClick: () => record.id && navigate(`/strategy/backtest/${record.id}`),
                        style: { cursor: 'pointer' },
                      })}
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
      <Modal
        visible={showAdd}
        title="新建策略"
        onOk={handleAdd}
        onCancel={() => { setShowAdd(false); setSelectedTemplate(null); }}
        okText="创建"
        confirmLoading={templatePopulating}
        okButtonProps={{ disabled: !selectedTemplate || !newName.trim() }}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <TemplateSelector selected={selectedTemplate} onSelect={setSelectedTemplate} />
          <Input placeholder="策略名称" value={newName} onChange={setNewName} style={{ width: '100%' }} />
        </div>
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
