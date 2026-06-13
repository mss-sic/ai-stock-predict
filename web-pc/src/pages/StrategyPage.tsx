import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { Button, Input, InputNumber, Modal, Table, Select, Popconfirm, Tooltip, Message, Tag, Tabs } from '@arco-design/web-react';
import { Target, Plus, Trash2, GripVertical, Play, Brain, BarChart4, TrendingUp, Shield, Settings, Sparkles, Beaker, Clock, History, Activity, AlertCircle, ClipboardList, FileSearch, PieChart, Wallet, TrendingDown, LineChart, List, Code, Radio, Terminal, XCircle, Zap } from 'lucide-react';
import {
  fetchStrategies, createStrategy, updateStrategy, deleteStrategy, reorderStrategies,
  fetchStrategyConditions, saveStrategyConditions, aiGenerateStrategy, optimizePrompt,
  fetchIndicators, runBacktest, fetchBacktestHistory, testIndicator,
  startBacktest, getBacktestStatus, cancelBacktest, fetchBacktestTasks,
  deleteBacktestResult, deleteBacktestTask, fetchStockPool,
  fetchBacktestTaskLogs, fetchTaskSnapshots,
  fetchStockAnalysis,
  fetchKLine,
} from '../services/api';
import KLineChart from '../components/KLineChart';

type CondType = 'buy' | 'add' | 'sell' | 'reduce';
const COND_LABELS: Record<CondType, string> = { buy: '买入条件', add: '加仓条件', sell: '卖出条件', reduce: '减仓条件' };
const COND_COLORS: Record<CondType, string> = { buy: 'var(--stock-up)', add: 'var(--color-warning-text)', sell: 'var(--stock-down)', reduce: 'var(--color-primary)' };

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
  const [btEnd, setBtEnd] = useState('2026-06-05');
  const [btRunning, setBtRunning] = useState(false);
  const [btResult, setBtResult] = useState<any>(null);
  const [btHistory, setBtHistory] = useState<any[]>([]);
  const [btPositions, setBtPositions] = useState<any>(null);
  const [btLogs, setBtLogs] = useState<any[]>([]);
  const [btExecLogs, setBtExecLogs] = useState<any[]>([]);
  const btLogCursorRef = useRef(0);
  const [btPhase, setBtPhase] = useState('');
  const [btProgress, setBtProgress] = useState('');
  const [btTaskId, setBtTaskId] = useState<number | null>(null);
  const [btOfflineMode, setBtOfflineMode] = useState(false);
  const [btTasks, setBtTasks] = useState<any[]>([]);
  const [btPollTimer, setBtPollTimer] = useState<any>(null);
  const [btStockPool, setBtStockPool] = useState('all');
  // Stock analysis states
  const [stockAnalysis, setStockAnalysis] = useState<any[]>([]);
  const [stockDetailVisible, setStockDetailVisible] = useState(false);
  const [selectedStock, setSelectedStock] = useState<any>(null);
  const [stockKline, setStockKline] = useState<any[]>([]);
  const [stockMarkers, setStockMarkers] = useState<any[]>([]);
  const [stockPools, setStockPools] = useState<any[]>([]);
  const [btDetailVisible, setBtDetailVisible] = useState(false);
  const [btDetailResult, setBtDetailResult] = useState<any>(null);
  const [btDetailLogs, setBtDetailLogs] = useState<any[]>([]);
  const [btDetailTab, setBtDetailTab] = useState('trades');

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
      if (list.length > 0 && !activeId) setActiveId(list[0].id);
    } catch {}
  }, [activeId]);

  const loadIndicators = useCallback(async () => {
    try { const { data: r } = await fetchIndicators(); setIndicators(r.data || []); } catch {}
  }, []);

  useEffect(() => { loadStrategies(); loadIndicators(); }, []);

  useEffect(() => {
    if (!activeId) return;
    const s = strategies.find(s => s.id === activeId);
    setActiveStrategy(s || null);
    if (s) {
      fetchStrategyConditions(s.id).then(({ data: r }: any) => setConditions(r.data || [])).catch(() => {});
      fetchBacktestHistory(s.id).then(({ data: r }: any) => setBtHistory(r.data || [])).catch(() => {});
      fetchBacktestTasks(s.id).then(({ data: r }: any) => setBtTasks(r.data || [])).catch(() => {});
      fetchStockPool().then(({ data: r }: any) => setStockPools(r.data || [])).catch(() => {});
    }
  }, [activeId, strategies]);

  const handleAdd = async () => {
    if (!newName.trim()) return;
    try {
      const { data: r } = await createStrategy(newName.trim());
      setActiveId(r.data.id); setShowAdd(false); setNewName(''); loadStrategies();
      toast('success', '策略已创建');
    } catch {}
  };

  const handleDelete = async (id: number) => {
    try {
      await deleteStrategy(id);
      if (activeId === id) setActiveId(strategies.filter(s => s.id !== id)[0]?.id || null);
      loadStrategies(); toast('success', '已删除');
    } catch {}
  };

  const handleUpdateStrategy = async (field: string, value: any) => {
    if (!activeId) return;
    // Optimistic update: update local state immediately
    setActiveStrategy((prev: any) => prev ? { ...prev, [field]: value } : prev);
    try { await updateStrategy(activeId, { [field]: value }); } catch {}
  };

  const filteredConds = (t: CondType) => conditions.filter((c: any) => c.condType === t);

  const addCondition = (ct: CondType) => {
    setConditions([...conditions, { id: -(Date.now()), strategyId: activeId, condType: ct, indicator: 'algo_score', operator: 'gte', value: 0, logicGroup: 1, sortOrder: filteredConds(ct).length }]);
  };

  const updateCondition = (idx: number, field: string, value: any) => {
    setConditions(prev => prev.map((c, i) => i === idx ? { ...c, [field]: value } : c));
  };

  const removeCondition = (idx: number) => { setConditions(prev => prev.filter((_, i) => i !== idx)); };

  const saveConditions = async () => {
    if (!activeId) return;
    const clean = conditions.map(c => ({ ...c, id: c.id < 0 ? 0 : c.id, strategyId: activeId }));
    try {
      await saveStrategyConditions(activeId, clean);
      toast('success', '条件已保存');
      const { data: r } = await fetchStrategyConditions(activeId);
      setConditions(r.data || []);
    } catch {}
  };

  const handleAIGenerate = async () => {
    if (!activeId) { toast('warning', '请先选择一个策略'); return; }
    setAiGenerating(true);
    try {
      const { data: r } = await aiGenerateStrategy({ name: activeStrategy?.name || '当前策略', description: aiDesc, style: aiStyle });
      const result = r.data;
      if (result?.conditions) {
        const params: any = {};
        if (result.stopProfit !== undefined) params.stopProfit = result.stopProfit;
        if (result.stopLoss !== undefined) params.stopLoss = result.stopLoss;
        if (result.maxHoldings) params.maxHoldings = result.maxHoldings;
        if (result.description) params.description = result.description;
        await updateStrategy(activeId, params);
        const cleanConds = (result.conditions || []).map((c: any, i: number) => ({
          id: 0, strategyId: activeId, condType: c.condType, indicator: c.indicator,
          operator: c.operator, value: c.value, logicGroup: c.logicGroup || 1, sortOrder: i,
        }));
        await saveStrategyConditions(activeId, cleanConds);
        loadStrategies(); setShowAIModal(false);
        toast('success', `AI已填充 ${cleanConds.length} 条条件`);
      } else { toast('warning', 'AI未生成有效条件'); }
    } catch {}
    setAiGenerating(false);
  };

  const handleOptimizePrompt = async () => {
    if (!aiDesc.trim()) { toast('warning', '请先输入描述'); return; }
    setAiOptimizing(true);
    try {
      const { data: r } = await optimizePrompt(aiDesc, aiStyle);
      if (r.data?.optimized) { setAiDesc(r.data.optimized); toast('success', 'AI已优化描述'); }
    } catch {}
    setAiOptimizing(false);
  };

  // ── Backtest handlers ──
  const handleRunBacktest = async () => {
    if (!activeId || !btStart || !btEnd) return;
    setBtRunning(true); setBtResult(null); setBtPositions(null); setBtExecLogs([]); btLogCursorRef.current = 0; setBtActiveTab('positions');
    setBtPhase('正在初始化...'); setBtProgress('');
    const token = localStorage.getItem('aip_access_token') || '';
    try {
      const res = await fetch(`/api/v1/strategies/${activeId}/backtest`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ startDate: btStart, endDate: btEnd, stockCodes: [], stockPool: btStockPool }),
      });
      const reader = res.body?.getReader();
      if (!reader) throw new Error('No reader');
      const decoder = new TextDecoder(); let buffer = '';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n'); buffer = lines.pop() || '';
        for (const line of lines) {
          if (!line.startsWith('data: ')) continue;
          try {
            const msg = JSON.parse(line.slice(6));
            const { type, payload } = msg;
            switch (type) {
              case 'phase': setBtPhase(payload.message); break;
              case 'position':
                setBtPositions(payload);
                setBtProgress(`${payload.day}/${payload.totalDays}`);
                // Extract embedded trade events
                if (payload.recentTrades && Array.isArray(payload.recentTrades)) {
                  setBtLogs(prev => {
                    const existing = new Set(prev.map((t: any) => `${t.date}-${t.action}-${t.code}-${t.price}-${t.quantity}`));
                    const newTrades = payload.recentTrades.filter((t: any) => !existing.has(`${t.date}-${t.action}-${t.code}-${t.price}-${t.quantity}`));
                    return [...prev, ...newTrades];
                  });
                }
                break;
              case 'trade': setBtLogs(prev => [...prev, payload]); break;
              case 'metric': setBtResult(payload); break;
              case 'trades': setBtLogs(Array.isArray(payload) ? payload : []); break;
              case 'equity': setBtResult(prev => ({ ...prev, equityCurve: { dates: payload.map((p: any) => p.date), values: payload.map((p: any) => p.equity) } })); break;
              case 'error': toast('error', payload.message); break;
              case 'done': setBtRunning(false); fetchBacktestHistory(activeId).then(({ data: rh }: any) => setBtHistory(rh.data || [])).catch(() => {});
                // Fetch final snapshot for last-day display
                fetchTaskSnapshots(activeId, 0, 0).then(({ data: sr }: any) => {
                  if (sr.data?.length > 0) {
                    const last = sr.data[sr.data.length - 1];
                    setBtPositions({
                      date: last.date, day: last.dayIndex, totalDays: last.dayIndex,
                      cash: last.cash, totalEquity: last.totalEquity, totalReturn: last.cumulativeReturn,
                      positions: last.positionsData || [], positionCount: last.positionCount,
                      recentTrades: [],
                    });
                  }
                }).catch(() => {});
                break;
            }
          } catch {}
        }
      }
    } catch { toast('error', '回测连接失败'); }
    setBtRunning(false);
  };

  const handleStartBacktest = async () => {
    if (!activeId || !btStart || !btEnd) return;
    setBtRunning(true); setBtResult(null); setBtPositions(null);
    setBtPhase('正在启动回测任务...'); setBtProgress(''); setBtOfflineMode(true);
    try {
      const { data: r } = await startBacktest(activeId, btStart, btEnd, [], btStockPool);
      const taskId = r.data?.taskId;
      if (!taskId) throw new Error('No taskId');
      setBtTaskId(taskId);
      toast('success', '回测任务已启动，可关闭页面稍后查看');
      pollTaskStatus(taskId);
    } catch { toast('error', '启动回测失败'); setBtRunning(false); setBtOfflineMode(false); }
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
        const pos = typeof t.currentPositions === 'string' ? JSON.parse(t.currentPositions) : t.currentPositions;
        setBtPositions({ ...pos, day: t.currentDay, totalDays: t.totalDays });
        // Trade events are embedded in exec logs from the API
        // Incremental execution log fetching
        fetchBacktestTaskLogs(activeId, taskId).then(({ data: lr }: any) => {
          if (lr.data?.logs?.length > 0) {
            const newLogs = lr.data.logs.filter((l: any) => l.id > btLogCursorRef.current);
            if (newLogs.length > 0) {
              setBtExecLogs(prev => [...prev, ...newLogs]);
              btLogCursorRef.current = Math.max(btLogCursorRef.current, ...newLogs.map((l: any) => l.id));
            }
          }
        }).catch(() => {});
      }
      if (t.status === 'completed') {
        setBtRunning(false); setBtOfflineMode(false); setBtPhase('回测完成');
        if (t.resultId) {
          const { data: rr } = await fetchBacktestHistory(activeId);
          setBtHistory(rr.data || []);
        }
        fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
        // Fetch full logs and final snapshot
        if (taskId) {
          fetchBacktestTaskLogs(activeId, taskId).then(({ data: lr }: any) => {
            if (lr.data?.logs) setBtExecLogs(lr.data.logs);
          }).catch(() => {});
          fetchTaskSnapshots(activeId, taskId, 0).then(({ data: sr }: any) => {
            if (sr.data?.length > 0) {
              const last = sr.data[sr.data.length - 1];
              setBtPositions({
                date: last.date, day: last.dayIndex, totalDays: t.totalDays,
                cash: last.cash, totalEquity: last.totalEquity, totalReturn: last.cumulativeReturn,
                positions: last.positionsData || [], positionCount: last.positionCount,
                recentTrades: [],
              });
            }
          }).catch(() => {});
        }
        return;
      }
      if (t.status === 'failed') {
        setBtRunning(false); setBtOfflineMode(false); setBtTaskId(null);
        toast('error', t.errorMsg || '回测失败');
        fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
        return;
      }
      if (t.status === 'cancelled') {
        setBtRunning(false); setBtOfflineMode(false); setBtTaskId(null); setBtPhase('已取消');
        fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
        return;
      }
      const timer = setTimeout(() => pollTaskStatus(taskId), 2000);
      setBtPollTimer(timer);
    } catch {
      const timer = setTimeout(() => pollTaskStatus(taskId), 3000);
      setBtPollTimer(timer);
    }
  };

  const handleCancelBacktest = async () => {
    if (!activeId || !btTaskId) return;
    try {
      await cancelBacktest(activeId, btTaskId);
      if (btPollTimer) clearTimeout(btPollTimer);
      setBtRunning(false); setBtOfflineMode(false); setBtTaskId(null); setBtPhase('已取消');
      toast('info', '回测已取消');
      fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
    } catch { toast('error', '取消失败'); }
  };

  const handleCancelTask = async (taskId: number) => {
    if (!activeId) return;
    try {
      await cancelBacktest(activeId, taskId);
      Message.success('已取消');
      fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
    } catch { Message.error('取消失败'); }
  };

  const handleReconnectTask = async (taskId: number) => {
    if (!activeId) return;
    setBtRunning(true); setBtOfflineMode(true); setBtTaskId(taskId); setBtPhase('正在重新连接...');
    const token = localStorage.getItem('aip_access_token') || '';
    try {
      const res = await fetch(`/api/v1/strategies/${activeId}/backtest/stream/${taskId}`, { headers: { 'Authorization': `Bearer ${token}` } });
      const reader = res.body?.getReader();
      if (!reader) { pollTaskStatus(taskId); return; }
      const decoder = new TextDecoder(); let buffer = '';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n'); buffer = lines.pop() || '';
        for (const line of lines) {
          if (!line.startsWith('data: ')) continue;
          try {
            const msg = JSON.parse(line.slice(6));
            const { type, payload } = msg;
            switch (type) {
              case 'phase': setBtPhase(payload.message); break;
              case 'position':
                setBtPositions(payload);
                setBtProgress(`${payload.day}/${payload.totalDays}`);
                // Extract embedded trade events
                if (payload.recentTrades && Array.isArray(payload.recentTrades)) {
                  setBtLogs(prev => {
                    const existing = new Set(prev.map((t: any) => `${t.date}-${t.action}-${t.code}-${t.price}-${t.quantity}`));
                    const newTrades = payload.recentTrades.filter((t: any) => !existing.has(`${t.date}-${t.action}-${t.code}-${t.price}-${t.quantity}`));
                    return [...prev, ...newTrades];
                  });
                }
                break;
              case 'trade': setBtLogs(prev => [...prev, payload]); break;
              case 'metric': setBtResult(payload); break;
              case 'trades': setBtLogs(Array.isArray(payload) ? payload : []); break;
              case 'equity': setBtResult(prev => ({ ...prev, equityCurve: { dates: payload.map((p: any) => p.date), values: payload.map((p: any) => p.equity) } })); break;
              case 'done': setBtRunning(false); setBtOfflineMode(false); setBtTaskId(null); setBtPhase('回测完成');
                fetchBacktestHistory(activeId).then(({ data: rh }: any) => setBtHistory(rh.data || [])).catch(() => {});
                fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
                // Fetch final snapshot for last-day position display
                if (taskId) {
                  fetchTaskSnapshots(activeId, taskId, 0).then(({ data: sr }: any) => {
                    if (sr.data?.length > 0) {
                      const last = sr.data[sr.data.length - 1];
                      setBtPositions({
                        date: last.date, day: last.dayIndex, totalDays: last.dayIndex,
                        cash: last.cash, totalEquity: last.totalEquity, totalReturn: last.cumulativeReturn,
                        positions: last.positionsData || [], positionCount: last.positionCount,
                        recentTrades: [],
                      });
                    }
                  }).catch(() => {});
                }
                break;
            }
          } catch {}
        }
      }
    } catch { pollTaskStatus(taskId); }
  };

  const fetchBtStockAnalysis = async () => {
    if (!activeId || !btDetailResult?.taskId) return;
    try {
      const { data: r }: any = await fetchStockAnalysis(activeId, btDetailResult.taskId);
      setStockAnalysis(r.data?.stocks || []);
    } catch {}
  };

  const handleViewStockDetail = async (stock: any) => {
    setSelectedStock(stock);
    setStockKline([]);
    setStockMarkers([]);
    setStockDetailVisible(true);
    // Fetch K-line for this stock
    try {
      const { data: r }: any = await fetchKLine(stock.stockCode);
      const kl = r.data || r || [];
      const cleaned = Array.isArray(kl) ? kl : [];
      setStockKline(cleaned);
      // Build markers from trades
      const markers: any[] = [];
      stock.trades.forEach((t: any) => {
        const execDate = (t.execDate || '').slice(0, 10);
        const idx = cleaned.findIndex((k: any) => {
          const d = (k.tradeDate || k.date || '').slice(0, 10);
          return d === execDate;
        });
        if (idx >= 0 && (t.actionType === 'buy' || t.actionType === 'add')) {
          markers.push({ i: idx, type: 'buy', label: `¥${t.execPrice?.toFixed(1)}` });
        } else if (idx >= 0 && (t.actionType === 'sell' || t.actionType === 'reduce' || t.actionType === 'stop')) {
          const pnlLabel = t.pnlPct ? `${t.pnlPct > 0 ? '+' : ''}${t.pnlPct?.toFixed(1)}%` : '';
          markers.push({ i: idx, type: 'sell', label: `¥${t.execPrice?.toFixed(1)} ${pnlLabel}` });
        }
      });
      setStockMarkers(markers);
    } catch {}
  };

  const handleViewBacktestDetail = (result: any) => {
    setBtDetailResult(result);
    setBtDetailVisible(true);
    setBtDetailLogs([]);
    setBtDetailTab('trades');
    // Fetch execution logs for this result
    if (result.taskId && activeId) {
      fetchBacktestTaskLogs(activeId, result.taskId).then(({ data: r }: any) => {
        setBtDetailLogs(r.data?.logs || []);
      }).catch(() => {});
    }
  };

  // Load stock analysis when tab changes
  useEffect(() => {
    if (btDetailTab === 'analysis' && btDetailResult?.taskId) {
      fetchBtStockAnalysis();
    }
  }, [btDetailTab]);

  const handleDeleteTask = async (taskId: number) => {
    if (!activeId) return;
    try {
      await deleteBacktestTask(activeId, taskId);
      Message.success('已删除');
      fetchBacktestTasks(activeId).then(({ data: rt }: any) => setBtTasks(rt.data || [])).catch(() => {});
    } catch { Message.error('删除失败'); }
  };

  const handleDeleteBacktestResult = async (id: number) => {
    try {
      await deleteBacktestResult(id);
      toast('success', '已删除');
      if (activeId) fetchBacktestHistory(activeId).then(({ data: r }: any) => setBtHistory(r.data || [])).catch(() => {});
    } catch {}
  };

  // ── Indicator test ──
  const openTestModal = (cond: any) => {
    setTestCond(cond); setTestStock('000001');
    setTestDate(new Date().toISOString().slice(0, 10));
    setTestResult(null); setTestModalVisible(true);
  };

  const runTest = async () => {
    if (!testStock || !testDate || !testCond) return;
    setTestLoading(true); setTestResult(null);
    try {
      const { data: r } = await testIndicator({ stockCode: testStock, date: testDate, indicator: testCond.indicator, operator: testCond.operator, value: testCond.value });
      setTestResult(r.data || r);
    } catch (e: any) {
      toast('error', '测试失败: ' + (e?.response?.data?.message || e?.message || '未知错误'));
    } finally { setTestLoading(false); }
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

  if (!strategies.length && !showAdd) {
    return (
      <div style={{ textAlign: 'center', padding: 80 }}>
        <Target size={48} color="var(--color-text-3)" />
        <p style={{ color: 'var(--color-text-3)', marginTop: 16 }}>还没有交易策略</p>
        <Button type="primary" icon={<Plus size={14} />} onClick={() => setShowAdd(true)} style={{ marginTop: 12 }}>创建第一个策略</Button>
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
            <div key={s.id} onClick={() => setActiveId(s.id)} style={{
              padding: '8px 12px', cursor: 'pointer', fontSize: 13,
              display: 'flex', alignItems: 'center', gap: 6,
              background: activeId === s.id ? 'var(--color-info-bg)' : 'transparent',
              borderLeft: activeId === s.id ? '3px solid var(--color-primary)' : '3px solid transparent',
              color: activeId === s.id ? '#165dff' : 'var(--color-text-2)',
            }}>
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
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
              <div>
                <Input value={activeStrategy.name} onChange={v => { setActiveStrategy({ ...activeStrategy, name: v }); }}
                  onBlur={() => handleUpdateStrategy('name', activeStrategy.name)}
                  style={{ fontSize: 18, fontWeight: 700, width: 280, border: 'none', padding: 0 }} />
              </div>
              <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                <Button size="small" type="outline" icon={<Brain size={13} />} onClick={() => setShowAIModal(true)}>AI生成</Button>
                <Button size="small" type="text" icon={<Settings size={13} />} onClick={() => {}}>设置</Button>
              </div>
            </div>

            {/* Strategy params row */}
            <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', marginBottom: 16, padding: '10px 14px', background: 'var(--color-fill-2)', borderRadius: 8 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12 }}>
                <span style={{ color: 'var(--color-text-3)' }}>初始资金</span>
                <InputNumber value={activeStrategy.initialCapital || 100000} onChange={v => handleUpdateStrategy('initialCapital', v)} size="small" style={{ width: 100 }} suffix="元" />
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12 }}>
                <span style={{ color: 'var(--color-text-3)' }}>止盈</span>
                <InputNumber value={activeStrategy.stopProfit} onChange={v => handleUpdateStrategy('stopProfit', v)} size="small" style={{ width: 70 }} suffix="%" />
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12 }}>
                <span style={{ color: 'var(--color-text-3)' }}>止损</span>
                <InputNumber value={activeStrategy.stopLoss} onChange={v => handleUpdateStrategy('stopLoss', v)} size="small" style={{ width: 70 }} suffix="%" />
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12 }}>
                <span style={{ color: 'var(--color-text-3)' }}>最大持仓</span>
                <InputNumber value={activeStrategy.maxHoldings} onChange={v => handleUpdateStrategy('maxHoldings', v)} size="small" style={{ width: 60 }} suffix="只" />
              </div>
            </div>

            {/* Tab switcher */}
            <div style={{ display: 'flex', gap: 2, marginBottom: 16, background: 'var(--color-fill-2)', borderRadius: 8, padding: 3, width: 'fit-content' }}>
              <button onClick={() => setTab('conditions')} style={{
                padding: '6px 16px', borderRadius: 6, border: 'none', cursor: 'pointer', fontSize: 13, fontWeight: 600,
                background: tab === 'conditions' ? 'var(--color-bg-1)' : 'transparent',
                color: tab === 'conditions' ? '#165dff' : 'var(--color-text-3)',
                boxShadow: tab === 'conditions' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
              }}><Target size={13} style={{ marginRight: 4 }} />策略条件</button>
              <button onClick={() => setTab('backtest')} style={{
                padding: '6px 16px', borderRadius: 6, border: 'none', cursor: 'pointer', fontSize: 13, fontWeight: 600,
                background: tab === 'backtest' ? 'var(--color-bg-1)' : 'transparent',
                color: tab === 'backtest' ? '#165dff' : 'var(--color-text-3)',
                boxShadow: tab === 'backtest' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
              }}><BarChart4 size={13} style={{ marginRight: 4 }} />策略回测</button>
            </div>

            {tab === 'conditions' ? (
              <>
                <div style={{ display: 'flex', gap: 6, marginBottom: 18 }}>
                  {(Object.keys(COND_LABELS) as CondType[]).map(ct => {
                    const count = filteredConds(ct).length;
                    const isActive = condTab === ct;
                    return (
                      <div key={ct} onClick={() => setCondTab(ct)} style={{
                        flex: 1, padding: '10px 14px', cursor: 'pointer', borderRadius: 10,
                        background: isActive ? `linear-gradient(135deg, ${COND_COLORS[ct]}18, ${COND_COLORS[ct]}08)` : 'var(--color-fill-2)',
                        border: isActive ? `1.5px solid ${COND_COLORS[ct]}40` : '1.5px solid transparent',
                        transition: 'all 0.2s ease', position: 'relative', overflow: 'hidden',
                      }}>
                        {isActive && <div style={{ position: 'absolute', top: 0, left: 0, right: 0, height: 3, background: COND_COLORS[ct], borderRadius: '0 0 3px 3px' }} />}
                        <div style={{ fontSize: 12, fontWeight: isActive ? 700 : 500, color: isActive ? COND_COLORS[ct] : 'var(--color-text-3)' }}>{COND_LABELS[ct]}</div>
                        <div style={{ fontSize: 20, fontWeight: 800, color: isActive ? COND_COLORS[ct] : 'var(--color-text-3)', marginTop: 2 }}>{count}</div>
                      </div>
                    );
                  })}
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 16 }}>
                  {filteredConds(condTab).length === 0 ? (
                    <div style={{ padding: '40px 20px', textAlign: 'center', background: 'linear-gradient(135deg, var(--color-fill-1) 0%, var(--color-fill-2) 100%)', borderRadius: 12, border: '1.5px dashed var(--color-border-1)' }}>
                      <div style={{ fontSize: 36, marginBottom: 8, opacity: 0.3 }}><ClipboardList size={36} /></div>
                      <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-2)', marginBottom: 4 }}>暂无{COND_LABELS[condTab]}</div>
                      <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 12 }}>添加因子条件来定义何时触发{COND_LABELS[condTab]}</div>
                      <Button size="small" type="outline" icon={<Plus size={12} />} onClick={() => addCondition(condTab)}>添加条件</Button>
                    </div>
                  ) : (
                    filteredConds(condTab).map((c: any, idx: number) => {
                      const globalIdx = conditions.indexOf(c);
                      const info = getIndicatorInfo(c.indicator);
                      const isCross = getOperators(c.indicator).includes('cross_up');
                      const safeTag = info?.backtestSafe ? '🟢' : (info?.dataNote?.startsWith('🚫') ? '🚫' : '🟡');
                      return (
                        <div key={c.id || idx} style={{ padding: '12px 14px', background: 'var(--color-bg-1)', borderRadius: 10, border: '1px solid var(--color-border-1)', boxShadow: '0 1px 3px rgba(0,0,0,0.04)' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                            <div style={{ minWidth: 36, height: 28, display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: 14, fontSize: 11, fontWeight: 700, background: idx === 0 ? '#e8f3ff' : 'var(--color-fill-2)', color: idx === 0 ? '#165dff' : 'var(--color-text-3)' }}>{idx === 0 ? 'IF' : 'AND'}</div>
                            <Select value={c.indicator} onChange={v => updateCondition(globalIdx, 'indicator', v)} style={{ width: 180 }} size="small" placeholder="选择指标"
                              options={indicators.map((ind: any) => ({ label: `${ind.backtestSafe ? '🟢' : (ind.dataNote?.startsWith('🚫') ? '🚫' : '🟡')} ${ind.label}`, value: ind.key }))} />
                            <Select value={c.operator} onChange={v => updateCondition(globalIdx, 'operator', v)} style={{ width: 72 }} size="small"
                              options={getOperators(c.indicator).map((op: string) => {
                                const opLabels: Record<string, string> = { gte: '≥', lte: '≤', gt: '>', lt: '<', eq: '=', cross_up: '↑', cross_down: '↓' };
                                return { label: opLabels[op] || op, value: op };
                              })} />
                            {isCross ? (
                              <Input value={typeof c.value === 'number' ? `${Math.floor(c.value)}/${Math.round((c.value - Math.floor(c.value)) * 1000)}` : String(c.value)}
                                onChange={v => updateCondition(globalIdx, 'value', v)} style={{ width: 80, fontFamily: 'monospace' }} size="small" placeholder="如 5/20" />
                            ) : (
                              <InputNumber value={c.value} onChange={v => updateCondition(globalIdx, 'value', v ?? 0)} style={{ width: 90, fontFamily: 'monospace' }} size="small" placeholder="阈值" />
                            )}
                            <Tooltip content="用历史数据测试该指标">
                              <Button size="mini" type="text" style={{ color: 'var(--color-text-3)', padding: '0 4px' }} icon={<Beaker size={13} />} onClick={() => openTestModal(c)} />
                            </Tooltip>
                            <div style={{ flex: 1 }} />
                            <Popconfirm title="移除该条件？" onOk={() => removeCondition(globalIdx)}>
                              <Button size="mini" type="text" style={{ color: 'var(--color-text-3)', padding: '0 4px' }} icon={<Trash2 size={13} />} />
                            </Popconfirm>
                          </div>
                          {info && c.indicator && (
                            <div style={{ marginTop: 10, padding: '8px 12px', background: 'var(--color-fill-2)', borderRadius: 6, borderLeft: '3px solid var(--color-primary)' }}>
                              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                                <span style={{ fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap', marginTop: 1 }}>{safeTag}</span>
                                <div style={{ flex: 1 }}>
                                  <div style={{ fontSize: 12, color: 'var(--color-text-2)', lineHeight: 1.5 }}>{info.desc}</div>
                                  {info.suggestion && <div style={{ marginTop: 4, fontSize: 11, color: 'var(--color-primary)', lineHeight: 1.5 }}>💡 {info.suggestion}</div>}
                                </div>
                              </div>
                            </div>
                          )}
                        </div>
                      );
                    })
                  )}
                </div>

                {filteredConds(condTab).length > 0 && (
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <Button size="small" icon={<Plus size={12} />} type="dashed" onClick={() => addCondition(condTab)}>添加条件</Button>
                    <div style={{ flex: 1 }} />
                    <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>共 {filteredConds(condTab).length} 条 · AND 逻辑</span>
                    <Button size="small" type="primary" onClick={saveConditions} style={{ borderRadius: 8 }}>保存条件</Button>
                  </div>
                )}
              </>
            ) : (
              <>
                {/* ── Backtest Tab ── */}
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
                    <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                      <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>股票池</span>
                      <Select
                        value={btStockPool}
                        onChange={setBtStockPool}
                        style={{ width: 200 }}
                        size="small"
                        options={stockPools.map((p: any) => ({
                          label: `${p.label} (${p.count}只)`,
                          value: p.key,
                        }))}
                        placeholder="选择股票池"
                      />
                    </div>
                    <Button type="primary" icon={<Play size={13} />} loading={btRunning} onClick={handleStartBacktest}>
                      {btRunning && btOfflineMode ? '运行中...' : '开始回测'}
                    </Button>
                    {btRunning && btOfflineMode && btTaskId && (
                      <Button type="text" status="danger" size="small" onClick={handleCancelBacktest}>取消</Button>
                    )}
                  </div>
                </div>

                {/* Live progress */}
                {btPhase && (
                  <div style={{ marginBottom: 12, padding: '8px 12px', background: 'var(--color-info-bg)', borderRadius: 6, fontSize: 12, color: 'var(--color-primary)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <span>{btPhase}</span>
                    {btProgress && <span style={{ color: 'var(--color-text-3)' }}>{btProgress}</span>}
                  </div>
                )}

                {/* ═══ Live Console + Positions ═══ */}
                {(btPositions || btExecLogs.length > 0 || btLogs.length > 0) && (
                  <div style={{ marginBottom: 16, display: 'flex', gap: 12 }}>
                    {/* Left: Position snapshot (compact) */}
                    <div style={{
                      flex: 1, background: 'var(--color-bg-1)', border: '1px solid var(--color-border-1)',
                      borderRadius: 10, padding: 14, maxHeight: 420, overflow: 'auto',
                    }}>
                      <div style={{ fontSize: 13, fontWeight: 700, color: 'var(--color-text-1)', marginBottom: 10 }}><Wallet size={14} style={{ marginRight: 6 }} />持仓</div>
                      {btPositions ? (
                        <>
                          <div style={{
                            display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '4px 12px',
                            fontSize: 12, marginBottom: 8, padding: '6px 8px',
                            background: 'var(--color-fill-1)', borderRadius: 6,
                          }}>
                            <div><span style={{ color: 'var(--color-text-3)' }}>日期</span> {(btPositions.date || '').slice(0, 10) || `第${btPositions.day}天`}</div>
                            <div><span style={{ color: 'var(--color-text-3)' }}>持仓</span> {btPositions.positionCount || 0}只{(btPositions.soldCount || 0) > 0 ? <span style={{ color: 'var(--color-text-3)', marginLeft: 4 }}>(今日卖出 {btPositions.soldCount})</span> : ''}</div>
                            <div><span style={{ color: 'var(--color-text-3)' }}>现金</span> ¥{(btPositions.cash || 0).toLocaleString()}</div>
                            <div><span style={{ color: 'var(--color-text-3)' }}>总权益</span> <b style={{ color: (btPositions.totalReturn || 0) >= 0 ? '#f53f3f' : '#00b42a' }}>¥{(btPositions.totalEquity || 0).toLocaleString()}</b></div>
                            <div style={{ gridColumn: '1 / -1' }}>
                              <span style={{ color: 'var(--color-text-3)' }}>累计收益</span> <b style={{ color: (btPositions.totalReturn || 0) >= 0 ? '#f53f3f' : '#00b42a' }}>{(btPositions.totalReturn || 0) >= 0 ? '+' : ''}{btPositions.totalReturn}%</b>
                            </div>
                          </div>
                          {btPositions.positions?.length > 0 ? btPositions.positions.map((p: any, i: number) => (
                            <div key={i} style={{
                              padding: '8px 0', borderBottom: '1px solid var(--color-table-row-border)', fontSize: 12,
                              opacity: p.sold ? 0.65 : 1,
                              background: p.sold ? 'var(--color-fill-1)' : 'transparent',
                              borderRadius: p.sold ? 4 : 0,
                              paddingLeft: p.sold ? 8 : 0,
                              paddingRight: p.sold ? 8 : 0,
                            }}>
                              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <span style={{ fontWeight: 600, fontSize: 12, textDecoration: p.sold ? 'line-through' : 'none' }}>
                                  {p.name || p.code}
                                  {p.sold && <span style={{ fontSize: 10, color: '#00B42A', marginLeft: 4, textDecoration: 'none', display: 'inline-block' }}>已卖出</span>}
                                </span>
                                <span style={{
                                  fontWeight: 600, fontSize: 11,
                                  color: (p.pnlPct || 0) >= 0 ? '#f53f3f' : '#00b42a',
                                }}>{(p.pnlPct || 0) >= 0 ? '+' : ''}{p.pnlPct?.toFixed(1)}%</span>
                              </div>
                              <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 2, color: 'var(--color-text-3)', fontSize: 11 }}>
                                <span>{p.sold ? <><span style={{ color: '#00B42A' }}>卖出</span> {p.soldQty}股</> : <>{p.qty}股</>} × ¥{p.price?.toFixed(2)}</span>
                                <span>{p.sold ? <span style={{ color: (p.pnl || 0) >= 0 ? '#f53f3f' : '#00b42a' }}>盈亏 ¥{Math.abs(p.pnl || 0).toLocaleString()}</span> : <>¥{(p.marketVal || 0).toLocaleString()}</>}</span>
                              </div>
                              {!p.sold && <div style={{ color: 'var(--color-text-3)', fontSize: 10 }}>成本 ¥{p.costPrice?.toFixed(2)}</div>}
                            </div>
                          )) : (
                            <div style={{ color: 'var(--color-text-3)', fontSize: 12, padding: 16, textAlign: 'center', background: 'var(--color-fill-1)', borderRadius: 6 }}>
                              <Wallet size={14} style={{ marginRight: 4, opacity: 0.5 }} />空仓<br/><span style={{ fontSize: 10 }}>现金 ¥{(btPositions.cash || 0).toLocaleString()}</span>
                            </div>
                          )}
                        </>
                      ) : (
                        <div style={{ color: 'var(--color-text-3)', fontSize: 13, padding: 24, textAlign: 'center' }}>等待数据...</div>
                      )}
                    </div>

                    {/* Right: Console-style execution log */}
                    <div style={{
                      flex: 1, minWidth: 0, background: '#121215', border: '1px solid #27272a',
                      borderRadius: 10, overflow: 'hidden', display: 'flex', flexDirection: 'column',
                    }}>
                      <div style={{
                        padding: '8px 14px', background: '#1c1c20', borderBottom: '1px solid #27272a',
                        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                      }}>
                        <span style={{ fontSize: 12, fontWeight: 600, color: '#7ec8e3' }}>💻 策略执行控制台</span>
                        <span style={{ fontSize: 10, color: '#4a6a8a' }}>
                          {btExecLogs.length} 条日志{btRunning ? ' · 实时更新中' : ''}
                        </span>
                      </div>
                      <div style={{
                        flex: 1, overflow: 'auto', padding: '8px 12px',
                        fontFamily: "'SF Mono', 'Monaco', 'Menlo', monospace",
                        fontSize: 11, lineHeight: '1.7', color: '#c9d1d9',
                        maxHeight: 380, minHeight: 200,
  }} ref={(el) => { if (el && btExecLogs.length > 0) { el.scrollTop = el.scrollHeight; } }}>
                        {btExecLogs.length > 0 ? btExecLogs.map((l: any, i: number) => {
                          const typeStyles: Record<string, { icon: string; color: string }> = {
                            system: { icon: '⚙', color: '#58a6ff' },
                            trade: { icon: '💹', color: '#7ee787' },
                            condition_eval: { icon: '🔍', color: '#d2a8ff' },
                            signal: { icon: '📶', color: '#f0883e' },
                            error: { icon: '❌', color: '#f85149' },
                          };
                          const s = typeStyles[l.logType] || { icon: '·', color: '#8b949e' };
                          const levelDim = l.level === 'debug' ? { opacity: 0.6 } : {};
                          return (
                            <div key={i} style={{
                              padding: '1px 0', display: 'flex', gap: 6,
                              borderBottom: l.logType === 'system' ? '1px solid var(--color-border-1)' : 'none',
                              paddingBottom: l.logType === 'system' ? 4 : 1,
                              marginBottom: l.logType === 'system' ? 4 : 0,
                              ...levelDim,
                            }}>
                              <span style={{ color: '#484f58', whiteSpace: 'nowrap', minWidth: 42 }}>{l.date?.slice(5) || ''}</span>
                              <span style={{ color: s.color }}>{s.icon}</span>
                              <span style={{ color: '#8b949e', fontSize: 10 }}>[{l.logType}]</span>
                              {l.stockCode && <span style={{ color: '#f0883e', fontWeight: 600 }}>{l.stockCode}</span>}
                              <span style={{ color: s.color, wordBreak: 'break-all' }}>{l.message}</span>
                            </div>
                          );
                        }) : (
                          <div style={{ color: '#484f58', padding: 32, textAlign: 'center' }}>
                            {btRunning ? '等待扫描开始...' : '暂无执行日志'}
                          </div>
                        )}
                      </div>
                    </div>
                  </div>
                )}

                {/* Final metrics — always show after backtest */}
                {btResult && (
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 16 }}>
                    <div style={{ padding: '14px 16px', background: 'var(--color-bg-1)', borderRadius: 8, border: '1px solid var(--color-border-1)' }}>
                      <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>累计收益</div>
                      <div style={{ fontSize: 24, fontWeight: 700, color: btResult.totalReturn >= 0 ? '#f53f3f' : '#00b42a' }}>
                        {btResult.totalReturn >= 0 ? '+' : ''}{btResult.totalReturn}%
                      </div>
                    </div>
                    <div style={{ padding: '14px 16px', background: 'var(--color-bg-1)', borderRadius: 8, border: '1px solid var(--color-border-1)' }}>
                      <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>夏普比率</div>
                      <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--color-text-1)' }}>{btResult.sharpeRatio}</div>
                    </div>
                    <div style={{ padding: '14px 16px', background: 'var(--color-bg-1)', borderRadius: 8, border: '1px solid var(--color-border-1)' }}>
                      <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>最大回撤</div>
                      <div style={{ fontSize: 24, fontWeight: 700, color: '#f53f3f' }}>-{btResult.maxDrawdown}%</div>
                    </div>
                    <div style={{ padding: '14px 16px', background: 'var(--color-bg-1)', borderRadius: 8, border: '1px solid var(--color-border-1)' }}>
                      <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>胜率 / 交易</div>
                      <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--color-text-1)' }}>{btResult.winRate}%
                        <span style={{ fontSize: 14, color: 'var(--color-text-3)', marginLeft: 8 }}>/ {btResult.tradeCount}次</span>
                      </div>
                    </div>
                  </div>
                )}

                {/* History — always visible */}
                <div style={{
                  background: 'var(--color-bg-1)', borderRadius: 12, border: '1px solid var(--color-border-1)',
                  boxShadow: '0 1px 3px rgba(0,0,0,0.04)', marginBottom: 16, overflow: 'hidden',
                }}>
                  <div style={{
                    padding: '14px 20px', borderBottom: btHistory.length > 0 ? '1px solid var(--color-border-1)' : 'none',
                    display: 'flex', alignItems: 'center', gap: 10,
                  }}>
                    <div style={{ width: 36, height: 36, borderRadius: 10, background: 'var(--color-info-bg)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                      <History size={18} color="var(--color-primary)" />
                    </div>
                    <div>
                      <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text-1)' }}>历史回测记录</div>
                      {btHistory.length > 0 && <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>共 {btHistory.length} 条记录</div>}
                    </div>
                  </div>
                  {btHistory.length > 0 ? (
                    <Table
                      columns={[
                        { title: '时间', dataIndex: 'createdAt', width: 120, render: (v: string) => <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>{v?.slice(0, 16)}</span> },
                        { title: '股票池', dataIndex: 'stockCode', width: 120, render: (v: string) => <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>{v || '多只'}</span> },
                        { title: '区间', dataIndex: 'startDate', width: 160, render: (_: any, r: any) => <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>{r.startDate?.slice(0,10)} → {r.endDate?.slice(0,10)}</span> },
                        { title: '收益', dataIndex: 'totalReturn', width: 85, render: (v: number) => (
                          <span style={{
                            fontWeight: 700, fontSize: 13, fontFamily: 'monospace',
                            color: v >= 0 ? '#f53f3f' : '#00b42a'
                          }}>{v >= 0 ? '+' : ''}{v}%</span>
                        )},
                        { title: '夏普', dataIndex: 'sharpeRatio', width: 70, render: (v: number) => <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{v}</span> },
                        { title: '回撤', dataIndex: 'maxDrawdown', width: 70, render: (v: number) => <span style={{ color: '#f53f3f', fontSize: 12 }}>-{v}%</span> },
                        { title: '胜率', dataIndex: 'winRate', width: 70, render: (v: number) => <span style={{ fontSize: 12 }}>{v}%</span> },
                        { title: '交易', dataIndex: 'tradeCount', width: 60 },
                        { title: '操作', dataIndex: 'id', width: 100, render: (id: number, record: any) => (
                          <div style={{ display: 'flex', gap: 6 }}>
                            <Button size="mini" type="outline" onClick={() => handleViewBacktestDetail(record)}
                              style={{ borderColor: 'var(--color-primary)', color: 'var(--color-primary)', fontSize: 11 }}>详情</Button>
                            <Popconfirm title="确定删除？" onOk={() => handleDeleteBacktestResult(id)}>
                              <Button size="mini" type="text" status="danger" style={{ fontSize: 11 }}>删除</Button>
                            </Popconfirm>
                          </div>
                        )},
                      ]}
                      data={btHistory}
                      rowKey="id"
                      pagination={false}
                      size="small"
                      scroll={{ x: 800 }}
                      stripe
                    />
                  ) : (
                    <div style={{ padding: '40px 20px', textAlign: 'center' }}>
                      <div style={{ width: 56, height: 56, borderRadius: 16, background: 'var(--color-fill-2)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', marginBottom: 12 }}>
                        <BarChart4 size={26} color="var(--color-text-3)" />
                      </div>
                      <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-2)', marginBottom: 4 }}>暂无回测记录</div>
                      <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>设置参数后点击「开始回测」，完成后记录将显示在此处</div>
                    </div>
                  )}
                </div>

                {/* Task list — always visible if any tasks */}
                {btTasks.length > 0 && (
                  <div style={{
                    background: 'var(--color-bg-1)', borderRadius: 12, border: '1px solid var(--color-border-1)',
                    boxShadow: '0 1px 3px rgba(0,0,0,0.04)', overflow: 'hidden',
                  }}>
                    <div style={{
                      padding: '14px 20px', borderBottom: '1px solid var(--color-border-1)',
                      display: 'flex', alignItems: 'center', gap: 10,
                    }}>
                      <div style={{ width: 36, height: 36, borderRadius: 10, background: 'var(--color-warning-bg)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                        <Activity size={18} color="#F77234" />
                      </div>
                      <div>
                        <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text-1)' }}>回测任务</div>
                        <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>
                          {btTasks.filter((t: any) => t.status === 'running').length} 运行中 · {btTasks.filter((t: any) => t.status === 'pending').length} 排队
                        </div>
                      </div>
                    </div>
                    <Table
                      columns={[
                        { title: '创建时间', dataIndex: 'createdAt', width: 140, render: (v: string) => <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>{v?.slice(0, 16)}</span> },
                        { title: '状态', dataIndex: 'status', width: 90, render: (v: string) => {
                          const statusMap: Record<string, { bg: string; color: string; label: string; icon: any }> = {
                            pending: { bg: 'var(--color-fill-2)', color: 'var(--color-text-3)', label: '排队中', icon: <Clock size={11} /> },
                            running: { bg: '#e8f3ff', color: 'var(--color-primary)', label: '运行中', icon: <Activity size={11} /> },
                            completed: { bg: 'var(--color-success-bg)', color: 'var(--stock-down)', label: '已完成', icon: <Shield size={11} /> },
                            failed: { bg: 'var(--color-danger-bg)', color: 'var(--stock-up)', label: '失败', icon: <AlertCircle size={11} /> },
                            cancelled: { bg: 'var(--color-warning-bg)', color: 'var(--color-warning-text)', label: '已取消', icon: <AlertCircle size={11} /> },
                          };
                          const s = statusMap[v] || { bg: 'var(--color-fill-2)', color: 'var(--color-text-3)', label: v, icon: null };
                          return (
                            <span style={{
                              display: 'inline-flex', alignItems: 'center', gap: 4,
                              padding: '2px 10px', borderRadius: 12, fontSize: 11, fontWeight: 600,
                              background: s.bg, color: s.color,
                            }}>{s.icon}{s.label}</span>
                          );
                        }},
                        { title: '进度', dataIndex: 'progressPct', width: 100, render: (v: number, record: any) => (
                          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                            <div style={{ flex: 1, height: 4, background: 'var(--color-border-1)', borderRadius: 2, overflow: 'hidden' }}>
                              <div style={{
                                height: '100%', borderRadius: 2,
                                width: `${Math.min(100, v || 0)}%`,
                                background: record.status === 'failed' ? '#f53f3f' : record.status === 'completed' ? '#00b42a' : '#165dff',
                                transition: 'width 0.3s',
                              }} />
                            </div>
                            <span style={{ fontSize: 11, color: 'var(--color-text-3)', minWidth: 32 }}>{(v || 0).toFixed(0)}%</span>
                          </div>
                        )},
                        { title: '阶段', dataIndex: 'phase', width: 160, render: (v: string) => <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>{v || '-'}</span> },
                        { title: '操作', dataIndex: 'id', width: 80, render: (id: number, record: any) => {
                          if (record.status === 'running' || record.status === 'pending') {
                            return (
                              <div style={{ display: 'flex', gap: 4 }}>
                                <Button size="mini" type="outline" onClick={() => handleReconnectTask(id)}
                                  style={{ borderColor: 'var(--color-primary)', color: 'var(--color-primary)', fontSize: 11 }}>查看</Button>
                                <Popconfirm title="确定取消？" onOk={() => handleCancelTask(id)}>
                                  <Button size="mini" type="text" status="warning" style={{ fontSize: 11 }}>取消</Button>
                                </Popconfirm>
                              </div>
                            );
                          }
                          return (
                            <Popconfirm title="确定删除？" onOk={() => handleDeleteTask(id)}>
                              <Button size="mini" type="text" status="danger" style={{ fontSize: 11 }}>删除</Button>
                            </Popconfirm>
                          );
                        }},
                      ]}
                      data={btTasks}
                      rowKey="id"
                      pagination={false}
                      size="small"
                    scroll={{ x: 800 }}
                      stripe
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
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>风格</div>
            <Select value={aiStyle} onChange={setAiStyle} style={{ width: '100%' }}
              options={[{ label: '稳健型 (Recommended)', value: 'moderate' }, { label: '激进型', value: 'aggressive' }, { label: '保守型', value: 'conservative' }]} />
          </div>
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>描述/要求</div>
            <div style={{ position: 'relative' }}>
              <Input.TextArea placeholder="例如：偏好低估值蓝筹，设置严格的止盈止损..." value={aiDesc} onChange={setAiDesc} rows={3} style={{ paddingRight: 32 }} />
              <button onClick={handleOptimizePrompt} disabled={aiOptimizing || !aiDesc.trim()} title="AI 优化描述"
                style={{ position: 'absolute', bottom: 8, right: 8, background: aiOptimizing ? 'var(--color-border-1)' : '#e8f3ff', border: 'none', borderRadius: 4, cursor: aiOptimizing || !aiDesc.trim() ? 'not-allowed' : 'pointer', padding: '2px 8px', fontSize: 11, color: 'var(--color-primary)' }}>
                {aiOptimizing ? '优化中...' : '✨ 优化'}
              </button>
            </div>
          </div>
        </div>
      </Modal>

      {/* Indicator Test Modal */}
      <Modal visible={testModalVisible} title="测试指标" onCancel={() => setTestModalVisible(false)} footer={null} width={520}>
        {testCond && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{ padding: '10px 14px', background: 'var(--color-fill-2)', borderRadius: 8, fontSize: 13, color: 'var(--color-text-2)' }}>
              测试指标: <b style={{ color: 'var(--color-primary)' }}>{getIndicatorLabel(testCond.indicator)}</b>
              {' '}({({ gte: '≥', lte: '≤', gt: '>', lt: '<', eq: '=', cross_up: '↑', cross_down: '↓' } as any)[testCond.operator]}) {' '}
              <b style={{ color: 'var(--color-primary)' }}>{testCond.value}</b>
            </div>
            <div style={{ display: 'flex', gap: 12 }}>
              <Input placeholder="股票代码" value={testStock} onChange={setTestStock} style={{ flex: 1 }} size="small" />
              <Input placeholder="日期 (YYYY-MM-DD)" value={testDate} onChange={setTestDate} style={{ width: 160 }} size="small" />
              <Button size="small" type="primary" icon={<Beaker size={12} />} loading={testLoading} onClick={runTest} disabled={!testStock || !testDate}>测试</Button>
            </div>
            {testResult && (
              <div style={{ padding: '16px 20px', background: testResult.hasData ? (testResult.conditionMet ? 'var(--color-success-bg)' : 'var(--color-warning-bg)') : 'var(--color-danger-bg)', borderRadius: 10, border: `1px solid ${testResult.hasData ? (testResult.conditionMet ? 'var(--color-success-border)' : 'var(--color-warning-border)') : 'var(--color-danger-border)'}` }}>
                {!testResult.hasData ? (
                  <div><span style={{ fontSize: 20 }}>⚠️</span> <span style={{ fontWeight: 600, color: '#f53f3f' }}>无数据</span>
                    <div style={{ fontSize: 13, color: 'var(--color-text-2)', marginTop: 4 }}>{testResult.error || '该股票在指定日期无对应数据'}</div>
                  </div>
                ) : (
                  <>
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 10 }}>
                      <div>
                        <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>股票 / 日期</div>
                        <div style={{ fontWeight: 600 }}>{testResult.stockName || testResult.stockCode} <span style={{ color: 'var(--color-text-2)', marginLeft: 8 }}>{testResult.date}</span></div>
                      </div>
                      <Tag color={testResult.conditionMet ? 'green' : 'orange'}>{testResult.conditionMet ? '✅ 条件满足' : '❌ 条件不满足'}</Tag>
                    </div>
                    <div style={{ display: 'flex', gap: 24 }}>
                      <div><div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>指标值</div><div style={{ fontSize: 22, fontWeight: 700, color: 'var(--color-primary)', fontFamily: 'monospace' }}>{testResult.computedValue}</div></div>
                      <div><div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>阈值</div><div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-2)', fontFamily: 'monospace' }}>{({ gte: '≥', lte: '≤', gt: '>', lt: '<', eq: '=', cross_up: '↑', cross_down: '↓' } as any)[testResult.operator]} {testResult.threshold}</div></div>
                    </div>
                  </>
                )}
              </div>
            )}
          </div>
        )}
      </Modal>

      {/* ── Backtest Detail Modal ── */}
      {/* ── Backtest Detail Full-Page Overlay ── */}
      {btDetailVisible && btDetailResult && (
        <div style={{
          position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, zIndex: 1000,
          background: 'var(--color-fill-2)', overflow: 'auto',
        }}>
          {/* Header */}
          <div style={{
            background: 'var(--color-bg-1)', borderBottom: '1px solid var(--color-border-1)',
            padding: '12px 24px', display: 'flex', alignItems: 'center', gap: 16,
            position: 'sticky', top: 0, zIndex: 10, boxShadow: '0 1px 4px rgba(0,0,0,0.04)',
          }}>
            <Button type="text" icon={<span style={{ fontSize: 18 }}>←</span>} onClick={() => setBtDetailVisible(false)}>
              返回
            </Button>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 16, fontWeight: 700, color: 'var(--color-text-1)' }}>
                回测详情
              </div>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 2 }}>
                {btDetailResult.startDate?.slice(0,10)} → {btDetailResult.endDate?.slice(0,10)} · 股票: {btDetailResult.stockCode || '多只'}
              </div>
            </div>
          </div>

          {/* Body */}
          <div style={{ maxWidth: 1200, margin: '0 auto', padding: '20px 24px' }}>
            {/* Summary Cards */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 14, marginBottom: 24 }}>
              <div style={{
                background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 16px', textAlign: 'center',
                boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)',
              }}>
                <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>累计收益</div>
                <div style={{
                  fontSize: 28, fontWeight: 800,
                  color: btDetailResult.totalReturn >= 0 ? '#F53F3F' : '#00B42A',
                  fontFamily: 'monospace',
                }}>
                  {btDetailResult.totalReturn >= 0 ? '+' : ''}{btDetailResult.totalReturn}%
                </div>
              </div>
              <div style={{
                background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 16px', textAlign: 'center',
                boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)',
              }}>
                <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>夏普比率 · 最大回撤</div>
                <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--color-text-1)', fontFamily: 'monospace' }}>
                  {btDetailResult.sharpeRatio}
                  <span style={{ fontSize: 15, color: 'var(--stock-up)', marginLeft: 8 }}>-{btDetailResult.maxDrawdown}%</span>
                </div>
              </div>
              <div style={{
                background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 16px', textAlign: 'center',
                boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)',
              }}>
                <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>胜率 · 交易次数</div>
                <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--color-text-1)', fontFamily: 'monospace' }}>
                  {btDetailResult.winRate}%
                  <span style={{ fontSize: 15, color: 'var(--color-text-3)', marginLeft: 8 }}>/ {btDetailResult.tradeCount}笔</span>
                </div>
              </div>
              <div style={{
                background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 16px', textAlign: 'center',
                boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)',
              }}>
                <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>回测区间</div>
                <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-2)' }}>
                  {btDetailResult.startDate?.slice(0,10)}
                </div>
                <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>→</div>
                <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-2)' }}>
                  {btDetailResult.endDate?.slice(0,10)}
                </div>
              </div>
            </div>

            {/* Equity Curve - larger */}
            <div style={{
              background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 24px', marginBottom: 20,
              boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)',
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 14 }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text-1)' }}><TrendingUp size={14} style={{ marginRight: 6 }} />收益曲线</div>
                <EquityModeToggle />
              </div>
              {(() => {
                const eqData = btDetailResult.equityCurve?.data || btDetailResult.equityCurve;
                const eqRaw = Array.isArray(eqData) ? eqData : (eqData?.points || []);
                const eqDates = eqRaw.map((p: any) => p.date);
                const eqValues = eqRaw.map((p: any) => p.equity);
                const baseline = eqValues[0] || 100000;
                return eqDates.length > 1 ? (
                  <div style={{ background: 'var(--color-bg-1)', borderRadius: 10, padding: '20px 16px 12px' }}>
                    <ProfitCurveChart data={{ dates: eqDates, values: eqValues, baseline }} />
                  </div>
                ) : (
                  <div style={{ padding: 60, textAlign: 'center', color: 'var(--color-text-3)' }}>暂无收益曲线数据</div>
                );
              })()}
            </div>

            {/* Trades & Logs */}
            <div style={{
              background: 'var(--color-bg-1)', borderRadius: 12, overflow: 'hidden',
              boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)',
            }}>
              <Tabs
                activeTab={btDetailTab}
                onChange={setBtDetailTab}
                style={{ padding: '16px 20px 0' }}
                type="line"
              >
                <Tabs.TabPane key="trades" title={
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <ClipboardList size={14} style={{ marginRight: 4 }} />操作记录
                    {(() => {
                      const arr = btDetailResult.trades?.data || btDetailResult.trades || [];
                      const c = Array.isArray(arr) ? arr.length : 0;
                      return c > 0 ? <span style={{ background: 'var(--color-info-bg)', color: 'var(--color-primary)', fontSize: 11, fontWeight: 600, padding: '1px 8px', borderRadius: 10 }}>{c}</span> : null;
                    })()}
                  </span>
                }>
                  <div style={{ padding: '0 0 16px' }}>
                    {(() => {
                      const tradesArr = btDetailResult.trades?.data || btDetailResult.trades || [];
                      if (!Array.isArray(tradesArr) || tradesArr.length === 0) {
                        return <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)', background: 'var(--color-fill-1)', borderRadius: 8, border: '1px dashed var(--color-border-1)', fontSize: 13 }}>暂无交易记录</div>;
                      }
                      return (
                        <Table
                          columns={[
                            { title: '信号日', dataIndex: 'signalDate', width: 100, render: (v: string, r: any) => v ? <span style={{ fontSize: 11, fontFamily: 'monospace', color: 'var(--color-text-3)' }}>{v}</span> : <span style={{ color: 'var(--color-text-3)' }}>—</span> },
                            { title: '成交日', dataIndex: 'date', width: 100, render: (v: string) => <span style={{ fontSize: 11, fontFamily: 'monospace', color: 'var(--color-text-2)' }}>{v}</span> },
                            { title: '操作', dataIndex: 'action', width: 72, render: (v: string) => {
                              const labels: Record<string, string> = { buy: '买入', add: '加仓', sell: '卖出', reduce: '减仓', stop: '止盈/止损' };
                              const colors: Record<string, string> = { buy: 'var(--stock-up)', add: 'var(--color-warning-text)', sell: 'var(--stock-down)', reduce: 'var(--color-primary)', stop: 'var(--stock-up)' };
                              const bgs: Record<string, string> = { buy: 'var(--color-danger-bg)', add: 'var(--color-warning-bg)', sell: 'var(--color-success-bg)', reduce: 'var(--color-info-bg)', stop: 'var(--color-danger-bg)' };
                              return <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, background: bgs[v] || 'var(--color-fill-2)', color: colors[v] || 'var(--color-text-3)', fontWeight: 700, fontSize: 11 }}>{labels[v] || v}</span>;
                            }},
                            { title: '股票', dataIndex: 'name', width: 100, render: (v: string, r: any) => (
                              <div><div style={{ fontWeight: 600, fontSize: 12 }}>{v || r.code}</div><div style={{ fontSize: 10, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{r.code}</div></div>
                            )},
                            { title: '价格', dataIndex: 'price', width: 76, render: (v: number) => <span style={{ fontFamily: 'monospace', fontSize: 12 }}>¥{v?.toFixed(2)}</span> },
                            { title: '数量', dataIndex: 'quantity', width: 64, render: (v: number) => <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{v}股</span> },
                            { title: '金额', dataIndex: 'quantity', width: 80, render: (v: number, r: any) => <span style={{ fontFamily: 'monospace', fontSize: 11, color: 'var(--color-text-2)' }}>¥{((r.price * v) || 0).toLocaleString('zh-CN', { maximumFractionDigits: 0 })}</span> },
                            { title: '盈亏', dataIndex: 'pnlPct', width: 72, render: (v: number) => v ? <span style={{ color: v > 0 ? '#F53F3F' : '#00B42A', fontWeight: 600, fontSize: 12, fontFamily: 'monospace' }}>{v > 0 ? '+' : ''}{v?.toFixed(1)}%</span> : <span style={{ color: 'var(--color-text-3)' }}>—</span> },
                            { title: '原因', dataIndex: 'reason', width: 120, render: (v: string) => <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{v}</span> },
                          ]}
                          data={tradesArr}
                          rowKey={(_, i) => i}
                          pagination={{ pageSize: 20, sizeCanChange: true, showTotal: true }}
                          size="small"
                          stripe
                        />
                      );
                    })()}
                  </div>
                </Tabs.TabPane>

                <Tabs.TabPane key="logs" title={
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <FileSearch size={14} style={{ marginRight: 4 }} />执行日志
                    {btDetailLogs.length > 0 && <span style={{ background: 'var(--color-info-bg)', color: 'var(--color-primary)', fontSize: 11, fontWeight: 600, padding: '1px 8px', borderRadius: 10 }}>{btDetailLogs.length}</span>}
                  </span>
                }>
                  <div style={{ padding: '0 0 16px' }}>
                    {btDetailLogs.length > 0 ? (
                      <div style={{
                        background: '#121215', borderRadius: 8, padding: '12px 16px',
                        fontFamily: "'SF Mono', Monaco, Menlo, monospace",
                        fontSize: 11, lineHeight: '1.8', color: '#c9d1d9',
                        maxHeight: 500, overflow: 'auto',
                      }}>
                        {btDetailLogs.map((l: any, i: number) => {
                          const ts: Record<string, { icon: string; color: string }> = {
                            system: { icon: '⚙', color: '#58a6ff' },
                            trade: { icon: '💹', color: '#7ee787' },
                            condition_eval: { icon: '🔍', color: '#d2a8ff' },
                            signal: { icon: '📶', color: '#f0883e' },
                            error: { icon: '❌', color: '#f85149' },
                          };
                          const s = ts[l.logType] || { icon: '·', color: '#8b949e' };
                          return (
                            <div key={i} style={{
                              padding: '1px 0', display: 'flex', gap: 6,
                              borderBottom: l.logType === 'system' ? '1px solid var(--color-border-1)' : 'none',
                              paddingBottom: l.logType === 'system' ? 4 : 1,
                              marginBottom: l.logType === 'system' ? 4 : 0,
                              opacity: l.level === 'debug' ? 0.6 : 1,
                            }}>
                              <span style={{ color: '#484f58', whiteSpace: 'nowrap', minWidth: 42 }}>{(l.date || '').slice(5)}</span>
                              <span style={{ color: s.color }}>{s.icon}</span>
                              <span style={{ color: '#8b949e', fontSize: 10 }}>[{l.logType}]</span>
                              {l.stockCode && <span style={{ color: '#f0883e', fontWeight: 600 }}>{l.stockCode}</span>}
                              <span style={{ color: s.color, wordBreak: 'break-all' }}>{l.message}</span>
                            </div>
                          );
                        })}
                      </div>
                    ) : (
                      <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13, background: 'var(--color-fill-1)', borderRadius: 8, border: '1px dashed var(--color-border-1)' }}>
                        {btDetailResult.taskId ? '该回测无执行日志（可能是旧版本运行）' : '暂无执行日志'}
                      </div>
                    )}
                  </div>
                </Tabs.TabPane>

                <Tabs.TabPane key="analysis" title={
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <PieChart size={14} style={{ marginRight: 4 }} />收益分析
                    {stockAnalysis.length > 0 && <span style={{ background: 'var(--color-info-bg)', color: 'var(--color-primary)', fontSize: 11, fontWeight: 600, padding: '1px 8px', borderRadius: 10 }}>{stockAnalysis.length}</span>}
                  </span>
                }>
                  <div style={{ padding: '0 0 16px' }}>
                    {stockAnalysis.length > 0 ? (
                      <Table
                        columns={[
                          { title: '#', width: 40, render: (_: any, __: any, i: number) => <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{i + 1}</span> },
                          { title: '股票', dataIndex: 'stockName', width: 110, render: (v: string, r: any) => (
                            <div>
                              <div style={{ fontWeight: 600, fontSize: 12 }}>{v || r.stockCode}</div>
                              <div style={{ fontSize: 10, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{r.stockCode}</div>
                            </div>
                          )},
                          { title: '总盈亏', dataIndex: 'totalPnl', width: 100, sorter: (a: any, b: any) => a.totalPnl - b.totalPnl, render: (v: number) => (
                            <span style={{ 
                              color: v >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 700, fontSize: 13, fontFamily: 'monospace' 
                            }}>
                              {v >= 0 ? '+' : ''}{v?.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                            </span>
                          )},
                          { title: '收益率', dataIndex: 'totalPnlPct', width: 85, sorter: (a: any, b: any) => a.totalPnlPct - b.totalPnlPct, render: (v: number) => (
                            <span style={{ 
                              color: v >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 600, fontSize: 12, fontFamily: 'monospace' 
                            }}>
                              {v >= 0 ? '+' : ''}{v?.toFixed(2)}%
                            </span>
                          )},
                          { title: '买入', dataIndex: 'buyCount', width: 55, sorter: (a: any, b: any) => a.buyCount - b.buyCount, render: (v: number) => <span style={{ fontSize: 12, color: 'var(--color-text-2)', fontFamily: 'monospace' }}>{v}次</span> },
                          { title: '卖出', dataIndex: 'sellCount', width: 55, sorter: (a: any, b: any) => a.sellCount - b.sellCount, render: (v: number) => <span style={{ fontSize: 12, color: 'var(--color-text-2)', fontFamily: 'monospace' }}>{v}次</span> },
                          { title: '操作', width: 70, render: (_: any, r: any) => (
                            <Button size="mini" type="text" onClick={() => handleViewStockDetail(r)}>
                              详情 →
                            </Button>
                          )},
                        ]}
                        data={stockAnalysis}
                        rowKey="stockCode"
                        pagination={{ pageSize: 15, sizeCanChange: true, showTotal: true }}
                        size="small"
                        stripe
                      />
                    ) : (
                      <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>
                        暂无收益分析数据
                      </div>
                    )}
                  </div>
                </Tabs.TabPane>
              </Tabs>
            </div>
          </div>
        </div>
      )}
      {/* ── Stock Detail Modal ── */}
      {stockDetailVisible && selectedStock && (
        <div style={{
          position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, zIndex: 1100,
          background: 'var(--color-fill-2)', overflow: 'auto',
        }}>
          {/* Header */}
          <div style={{
            background: 'var(--color-bg-1)', borderBottom: '1px solid var(--color-border-1)',
            padding: '12px 24px', display: 'flex', alignItems: 'center', gap: 16,
            position: 'sticky', top: 0, zIndex: 10, boxShadow: '0 1px 4px rgba(0,0,0,0.04)',
          }}>
            <Button type="text" icon={<span style={{ fontSize: 18 }}>←</span>} onClick={() => setStockDetailVisible(false)}>
              返回
            </Button>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 16, fontWeight: 700, color: 'var(--color-text-1)' }}>
                {selectedStock.stockName} <span style={{ fontSize: 12, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{selectedStock.stockCode}</span>
              </div>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 2 }}>
                累计盈亏: <span style={{ color: selectedStock.totalPnl >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 600 }}>
                  {selectedStock.totalPnl >= 0 ? '+' : ''}{selectedStock.totalPnl?.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                </span>
                {' · '}
                买入 {selectedStock.buyCount} 次 · 卖出 {selectedStock.sellCount} 次
              </div>
            </div>
          </div>

          {/* Body */}
          <div style={{ maxWidth: 1200, margin: '0 auto', padding: '20px 24px' }}>
            {/* Performance Summary */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 14, marginBottom: 24 }}>
              <div style={{
                background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 16px', textAlign: 'center',
                boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)',
              }}>
                <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>总盈亏</div>
                <div style={{
                  fontSize: 24, fontWeight: 800,
                  color: selectedStock.totalPnl >= 0 ? '#F53F3F' : '#00B42A',
                  fontFamily: 'monospace',
                }}>
                  {selectedStock.totalPnl >= 0 ? '+' : ''}{selectedStock.totalPnl?.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                </div>
              </div>
              <div style={{
                background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 16px', textAlign: 'center',
                boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)',
              }}>
                <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>收益率</div>
                <div style={{
                  fontSize: 24, fontWeight: 800,
                  color: selectedStock.totalPnlPct >= 0 ? '#F53F3F' : '#00B42A',
                  fontFamily: 'monospace',
                }}>
                  {selectedStock.totalPnlPct >= 0 ? '+' : ''}{selectedStock.totalPnlPct?.toFixed(2)}%
                </div>
              </div>
              <div style={{
                background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 16px', textAlign: 'center',
                boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)',
              }}>
                <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>买入次数</div>
                <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--stock-up)', fontFamily: 'monospace' }}>
                  {selectedStock.buyCount}
                </div>
              </div>
              <div style={{
                background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 16px', textAlign: 'center',
                boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)',
              }}>
                <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>卖出次数</div>
                <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--stock-down)', fontFamily: 'monospace' }}>
                  {selectedStock.sellCount}
                </div>
              </div>
            </div>

            {/* K-Line Chart with Markers */}
            <div style={{
              background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 24px', marginBottom: 20,
              boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)',
            }}>
              <div style={{ fontSize: 14, fontWeight: 700, color: 'var(--color-text-1)', marginBottom: 14 }}>
                📈 K线图 · 交易标记
              </div>
              {stockKline.length > 0 ? (
                <KLineChart data={stockKline} markers={stockMarkers} height={420} />
              ) : (
                <div style={{ padding: 60, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>
                  加载K线数据中...
                </div>
              )}
            </div>

            {/* Trade Records Table */}
            <div style={{
              background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 24px',
              boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)',
            }}>
              <div style={{ fontSize: 14, fontWeight: 700, color: 'var(--color-text-1)', marginBottom: 14 }}>
                <List size={14} style={{ marginRight: 6 }} />交易记录
              </div>
              <Table
                columns={[
                  { title: '信号日', dataIndex: 'signalDate', width: 100, render: (v: string) => <span style={{ fontSize: 11, fontFamily: 'monospace', color: 'var(--color-text-3)' }}>{v}</span> },
                  { title: '成交日', dataIndex: 'execDate', width: 100, render: (v: string) => <span style={{ fontSize: 11, fontFamily: 'monospace', color: 'var(--color-text-2)' }}>{v}</span> },
                  { title: '操作', dataIndex: 'actionType', width: 68, render: (v: string, record: any) => {
                    const labels: Record<string, string> = { buy: '买入', add: '加仓', sell: '卖出', reduce: '减仓', stop: '止盈/止损' };
                    const colors: Record<string, string> = { buy: 'var(--stock-up)', add: 'var(--color-warning-text)', sell: 'var(--stock-down)', reduce: 'var(--color-primary)', stop: 'var(--stock-up)' };
                    const bgs: Record<string, string> = { buy: 'var(--color-danger-bg)', add: 'var(--color-warning-bg)', sell: 'var(--color-success-bg)', reduce: 'var(--color-info-bg)', stop: 'var(--color-danger-bg)' };
                        if (v === 'stop') {
                          const isProfit = (record as any).reason === '止盈' || (record as any).pnlPct > 0;
                          return <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, background: isProfit ? 'rgba(245,63,63,0.08)' : 'rgba(0,180,42,0.08)', color: isProfit ? '#F53F3F' : '#00B42A', fontWeight: 700, fontSize: 11 }}>{isProfit ? '止盈' : '止损'}</span>;
                        }
                    return <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, background: bgs[v] || 'var(--color-fill-2)', color: colors[v] || 'var(--color-text-3)', fontWeight: 700, fontSize: 11 }}>{labels[v] || v}</span>;
                  }},
                  { title: '价格', dataIndex: 'execPrice', width: 76, render: (v: number) => <span style={{ fontFamily: 'monospace', fontSize: 12 }}>¥{v?.toFixed(2)}</span> },
                  { title: '数量', dataIndex: 'execQty', width: 64, render: (v: number) => <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{v}股</span> },
                  { title: '金额', dataIndex: 'execAmount', width: 90, render: (v: number) => <span style={{ fontFamily: 'monospace', fontSize: 11, color: 'var(--color-text-2)' }}>¥{v?.toLocaleString('zh-CN', { maximumFractionDigits: 0 })}</span> },
                  { title: '盈亏', dataIndex: 'pnl', width: 80, render: (v: number, r: any) => {
                    if (v === 0 && r.actionType === 'buy') return <span style={{ color: 'var(--color-text-3)', fontSize: 11 }}>—</span>;
                    return <span style={{ color: v > 0 ? '#F53F3F' : '#00B42A', fontWeight: 600, fontSize: 12, fontFamily: 'monospace' }}>{v > 0 ? '+' : ''}{v?.toFixed(2)}</span>;
                  }},
                  { title: '盈亏%', dataIndex: 'pnlPct', width: 72, render: (v: number) => v ? <span style={{ color: v > 0 ? '#F53F3F' : '#00B42A', fontWeight: 600, fontSize: 12, fontFamily: 'monospace' }}>{v > 0 ? '+' : ''}{v?.toFixed(1)}%</span> : <span style={{ color: 'var(--color-text-3)', fontSize: 11 }}>—</span> },
                  { title: '原因', dataIndex: 'reason', width: 120, render: (v: string) => <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{v || '—'}</span> },
                ]}
                data={selectedStock.trades || []}
                rowKey={(_, i: number) => i}
                pagination={{ pageSize: 20, sizeCanChange: true, showTotal: true }}
                size="small"
                stripe
              />
            </div>
          </div>
        </div>
      )}

    </div>
  );
}



// Equity curve mode toggle (shared state via a simple global var)
let equityModeGlobal: 'asset' | 'return' = 'asset';
let equityModeListeners: (() => void)[] = [];
function useEquityMode() {
  const [mode, setMode] = useState<'asset' | 'return'>(equityModeGlobal);
  useEffect(() => {
    const listener = () => setMode(equityModeGlobal);
    equityModeListeners.push(listener);
    return () => { equityModeListeners = equityModeListeners.filter(l => l !== listener); };
  }, []);
  const toggle = (m: 'asset' | 'return') => {
    equityModeGlobal = m;
    equityModeListeners.forEach(l => l());
  };
  return { mode, toggle };
}
function EquityModeToggle() {
  const { mode, toggle } = useEquityMode();
  return (
    <div style={{ display: 'flex', background: 'var(--color-fill-2)', borderRadius: 6, padding: 2 }}>
      <button onClick={() => toggle('asset')} style={{
        padding: '4px 12px', border: 'none', borderRadius: 4, cursor: 'pointer',
        fontSize: 12, fontWeight: mode === 'asset' ? 600 : 400,
        background: mode === 'asset' ? 'var(--color-bg-1)' : 'transparent',
        color: mode === 'asset' ? '#165dff' : 'var(--color-text-3)',
        boxShadow: mode === 'asset' ? '0 1px 2px rgba(0,0,0,0.08)' : 'none',
      }}>总资产</button>
      <button onClick={() => toggle('return')} style={{
        padding: '4px 12px', border: 'none', borderRadius: 4, cursor: 'pointer',
        fontSize: 12, fontWeight: mode === 'return' ? 600 : 400,
        background: mode === 'return' ? 'var(--color-bg-1)' : 'transparent',
        color: mode === 'return' ? '#165dff' : 'var(--color-text-3)',
        boxShadow: mode === 'return' ? '0 1px 2px rgba(0,0,0,0.08)' : 'none',
      }}>收益率</button>
    </div>
  );
}

// Optimized profit curve with dual-mode (asset yuan / return pct)
function ProfitCurveChart({ data }: { data: any }) {
  const svgRef = useRef<SVGSVGElement>(null);
  const [hoverIdx, setHoverIdx] = useState<number | null>(null);
  const { mode } = useEquityMode();
  const W = 900, H = 280, padL = 70, padR = 30, padT = 16, padB = 40;
  const dates: string[] = data.dates || [];
  const rawValues: number[] = data.values || [];
  const baseline: number = data.baseline || rawValues[0] || 100000;

  // Convert raw equity values to display values based on mode
  const values: number[] = mode === 'return'
    ? rawValues.map(v => baseline > 0 ? ((v - baseline) / baseline) * 100 : 0)
    : rawValues;

  if (values.length < 2) return <div style={{ color: 'var(--color-text-3)', textAlign: 'center', padding: 30 }}>数据不足</div>;

  const minVal = mode === 'return' ? Math.min(0, ...values) : Math.min(...values) * 0.995;
  const maxVal = mode === 'return' ? Math.max(0, ...values) : Math.max(...values) * 1.005;
  const range = maxVal - minVal || 1;
  const plotW = W - padL - padR;
  const plotH = H - padT - padB;
  const stepX = plotW / (values.length - 1);

  const px = (i: number) => padL + i * stepX;
  const py = (v: number) => padT + plotH - ((v - minVal) / range) * plotH;

  let pathD = '';
  values.forEach((v: number, i: number) => {
    pathD += i === 0 ? 'M' + px(i).toFixed(1) + ',' + py(v).toFixed(1) : ' L' + px(i).toFixed(1) + ',' + py(v).toFixed(1);
  });

  // Area fill to zero-line
  const zeroVal = mode === 'return' ? 0 : baseline;
  const zeroY = py(zeroVal);
  let areaD = pathD + ' L' + px(values.length - 1).toFixed(1) + ',' + zeroY.toFixed(1) + ' L' + px(0).toFixed(1) + ',' + zeroY.toFixed(1) + ' Z';

  // Y grid - format based on mode
  const yTicks = 5;
  const gridLines = Array.from({ length: yTicks }, (_, i) => {
    const v = minVal + (range * i) / (yTicks - 1);
    const label = mode === 'return'
      ? v.toFixed(1) + '%'
      : v >= 10000 ? (v / 10000).toFixed(1) + '万' : v.toFixed(0);
    return { y: py(v), label, isZero: mode === 'return' && Math.abs(v) < 0.01 };
  });

  // X labels (~6-8)
  const xLabels: { i: number; label: string }[] = [];
  const xStep = Math.max(1, Math.floor(values.length / 7));
  for (let i = 0; i < values.length; i += xStep) {
    xLabels.push({ i, label: dates[i]?.slice(5) || '' });
  }
  const lastX = xLabels[xLabels.length - 1];
  if (!lastX || lastX.i !== values.length - 1) {
    xLabels.push({ i: values.length - 1, label: dates[values.length - 1]?.slice(5) || '' });
  }

  const isUp = values[values.length - 1] >= values[0];
  const lineColor = isUp ? 'var(--stock-up)' : 'var(--stock-down)';
  const areaColor = isUp ? 'rgba(245,63,63,0.15)' : 'rgba(0,180,42,0.15)';

  const handleMouseMove = (e: React.MouseEvent<SVGSVGElement>) => {
    if (!svgRef.current) return;
    const rect = svgRef.current.getBoundingClientRect();
    const scaleX = W / rect.width;
    const mx = (e.clientX - rect.left) * scaleX;
    if (mx < padL || mx > padL + plotW) { setHoverIdx(null); return; }
    const idx = Math.round((mx - padL) / stepX);
    if (idx >= 0 && idx < values.length) setHoverIdx(idx);
    else setHoverIdx(null);
  };

  const startVal = values[0];
  const endVal = values[values.length - 1];
  const chgPct = startVal !== 0 ? ((endVal - startVal) / Math.abs(startVal)) * 100 : 0;

  const formatVal = (v: number) => mode === 'return'
    ? v.toFixed(2) + '%'
    : '¥' + (v >= 10000 ? (v / 10000).toFixed(2) + '万' : v.toFixed(2));

  const formatHoverVal = (v: number) => mode === 'return'
    ? v.toFixed(2) + '%'
    : '¥' + v.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

  return (
    <div style={{ position: 'relative' }}>
      <svg ref={svgRef} viewBox={'0 0 ' + W + ' ' + H} width="100%" height={H}
        onMouseMove={handleMouseMove} onMouseLeave={() => setHoverIdx(null)}
        style={{ cursor: 'crosshair' }}>
        {/* Grid */}
        {gridLines.map((g, i) => (
          <g key={i}>
            <line x1={padL} y1={g.y} x2={W - padR} y2={g.y} stroke={g.isZero ? 'var(--color-border-2)' : 'var(--color-border-1)'} strokeWidth={g.isZero ? '1' : '0.5'} strokeDasharray={g.isZero ? '' : '4 3'} />
            <text x={padL - 8} y={g.y + 4} fontSize="10" fill="var(--color-text-3)" textAnchor="end">{g.label}</text>
          </g>
        ))}
        {/* Zero / Baseline line */}
        <line x1={padL} y1={zeroY} x2={W - padR} y2={zeroY} stroke="#444" strokeWidth="1" />
        {/* Area fill */}
        <path d={areaD} fill={areaColor} />
        {/* Line */}
        <path d={pathD} fill="none" stroke={lineColor} strokeWidth="2.5" strokeLinejoin="round" />
        {/* Dots at start and end */}
        <circle cx={px(0)} cy={py(values[0])} r="4" fill={lineColor} />
        <circle cx={px(values.length - 1)} cy={py(values[values.length - 1])} r="4" fill={lineColor} stroke="#fff" strokeWidth="1.5" />
        {/* Hover indicator */}
        {hoverIdx !== null && (
          <g>
            <line x1={px(hoverIdx)} y1={padT} x2={px(hoverIdx)} y2={H - padB} stroke="#fff" strokeWidth="1" strokeDasharray="3 3" opacity="0.5" />
            <circle cx={px(hoverIdx)} cy={py(values[hoverIdx])} r="5" fill={lineColor} stroke="#fff" strokeWidth="2" />
          </g>
        )}
        {/* X labels */}
        {xLabels.map((xl, i) => (
          <text key={i} x={px(xl.i)} y={H - 10} fontSize="10" fill="var(--color-text-3)" textAnchor="middle">{xl.label}</text>
        ))}
        {/* Y axis label */}
        <text x={12} y={padT + plotH/2} fontSize="10" fill="var(--color-text-3)" textAnchor="middle" transform={'rotate(-90,12,' + (padT + plotH/2) + ')'}>
          {mode === 'return' ? '收益率' : '总资产'}
        </text>
      </svg>
      {/* Hover tooltip */}
      {hoverIdx !== null && (
        <div style={{
          position: 'absolute',
          left: (() => {
            const rect = svgRef.current?.getBoundingClientRect();
            const scale = W / (rect?.width || 1);
            const svgX = px(hoverIdx);
            return Math.min(svgX / scale + 12, (rect?.width || 400) - 140);
          })(),
          top: (() => {
            const rect = svgRef.current?.getBoundingClientRect();
            const scale = H / (rect?.height || 1);
            const svgY = py(values[hoverIdx]);
            return Math.max(4, svgY / scale - 50);
          })(),
          background: 'rgba(29,33,41,0.92)', color: '#fff', padding: '6px 10px',
          borderRadius: 6, fontSize: 11, fontFamily: 'monospace', pointerEvents: 'none',
          whiteSpace: 'nowrap', zIndex: 20,
        }}>
          <div style={{ color: 'var(--color-text-3)', fontSize: 10 }}>{dates[hoverIdx]}</div>
          <div style={{ fontWeight: 600, color: lineColor }}>{formatHoverVal(values[hoverIdx])}</div>
          {mode === 'asset' && <div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>原始: {'¥' + rawValues[hoverIdx].toLocaleString()}</div>}
        </div>
      )}
      {/* Summary stats */}
      <div style={{ display: 'flex', gap: 20, marginTop: 8, paddingLeft: padL, fontSize: 11, color: 'var(--color-text-3)', flexWrap: 'wrap' }}>
        <span>起点: <b style={{ color: 'var(--color-text-3)' }}>{formatVal(startVal)}</b></span>
        <span>终点: <b style={{ color: isUp ? '#F53F3F' : '#00B42A' }}>{formatVal(endVal)}</b></span>
        <span>变化: <b style={{ color: isUp ? '#F53F3F' : '#00B42A' }}>{chgPct >= 0 ? '+' : ''}{chgPct.toFixed(2)}%</b></span>
        <span>最高: <b style={{ color: 'var(--stock-up)' }}>{formatVal(maxVal)}</b></span>
        <span>最低: <b style={{ color: 'var(--stock-down)' }}>{formatVal(minVal)}</b></span>
      </div>
    </div>
  );
}
