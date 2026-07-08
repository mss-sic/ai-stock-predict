import { useState, useEffect, useCallback, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Table, Tag, Button, Spin, Message, Tabs, Select, Divider, Alert, Progress, Modal, Drawer, Dropdown, Menu, TimePicker, Input, Switch, Popconfirm , } from '@arco-design/web-react';
import ReactECharts from 'echarts-for-react';
import { ArrowLeft, TrendingUp, Wallet, Zap, Settings, Activity, Calendar, RefreshCw, Loader, XCircle, Bell, FileText, Building2, Cpu } from 'lucide-react';
import { fetchLiveRun, fetchLiveSnapshots, runLiveDaily, fetchDailyRunTask, fetchLatestDailyRunTask, runTradeExec, executeLiveSignal, syncSignalOrder, fetchTradeExecTask, fetchLatestTradeExecTask, updateLiveRunConfig, fetchNotificationConfigs, createNotificationConfig, deleteNotificationConfig, testNotification, sendLiveRunNotification, updateLiveSignal, deleteLiveSignal, clearLiveSignals, fetchRunLogs } from '../services/api';

interface Run { id: number; strategyId: number; name: string; status: string; startDate: string; initialCapital: number; currentEquity: number; totalReturn: number; maxDrawdown: number; winRate: number; tradeCount: number; lastRunDate: string; autoDailyCron?: string; autoTradeExecCron?: string; notifyEnabled?: boolean; notifyChannels?: string; executionMode?: string; aiReviewEnabled?: boolean; }
interface Strategy { id: number; name: string; description: string; stopProfit: number; stopLoss: number; maxHoldings: number; buyPositionPct: number; addPositionPct: number; positionSizing: string; positionConcentrationLimit: number; maxDailyLoss: number; initialCapital: number; enableAIAgent?: boolean; }
interface Allocation { id: number; allocatedCapital: number; currentCash: number; pctOfAccount: number; status: string; }
interface Position { id: number; stockCode: string; stockName: string; quantity: number; avgCost: number; currentPrice: number; unrealizedPnl: number; unrealizedPnlPct: number; realizedPnl: number; holdDays: number; }
interface Trade { id: number; tradeDate: string; stockCode: string; stockName: string; actionType: string; price: number; quantity: number; amount: number; pnl: number; pnlPct: number; reason: string; }
interface Snapshot { id: number; snapshotDate: string; cash: number; positionValue: number; totalEquity: number; dailyReturnPct: number; cumulativeReturn: number; maxDrawdownPct: number; }
interface Signal { id: number; signalDate: string; execDate: string; stockCode: string; stockName: string; actionType: string; plannedPrice: number; plannedQty: number; plannedAmount: number; status: string; reason: string; brokerOrderId?: string; suggestedPremium: number; orderPrice: number; orderPriceLimit: number; suggestedQty: number; originalQty: number; openPrice: number; openDeviation: number; decisionRule: string; }
interface Condition { id: number; condType: string; indicator: string; operator: string; value: string; period: string; }
interface Decision { id: number; signalId: number; tradeDate: string; stockCode: string; stockName: string; status: string; finalAction: string; finalPrice: number; finalAmount: number; confidence: number; source: string; reason: string; suggestedPremium: number; orderPrice: number; orderPriceLimit: number; suggestedQty: number; openPrice: number; openDeviation: number; decisionRule: string; taReasoning?: string; taDebateJson?: string; }


const getMarketTag = (code: string): string => {
  if (!code) return '';
  if (code.startsWith('688') || code.startsWith('689')) return '科创';
  if (code.startsWith('300') || code.startsWith('301')) return '创业';
  if (code.startsWith('4') || code.startsWith('8')) return '北交';
  return '';
};

const pnlColor = (v: number) => v >= 0 ? '#00B42A' : '#F53F3F';
const pnlSign = (v: number) => v >= 0 ? '+' : '';

// Phase label map for display
const phaseLabels: Record<string, string> = {
  init: '初始化', analysts: '分析师报告', debate: '牛熊辩论',
  trader: '交易员决策', risk: '风控审核', matrix: '决策矩阵', done: '完成'
};
const phaseColors: Record<string, string> = {
  init: '#94a3b8', analysts: '#60a5fa', debate: '#fbbf24',
  trader: '#f97316', risk: '#ef4444', matrix: '#8b5cf6', done: '#4ade80'
};

// Single log line renderer
const LogLine = ({ line }: { line: string }) => {
  let c = '#a0a0b0';
  if (line.startsWith('✅') || line.includes('确认')) c = '#4ade80';
  else if (line.startsWith('❌') || line.includes('驳回')) c = '#f87171';
  else if (line.startsWith('🔍') || line.startsWith('📋') || line.startsWith('──')) c = '#60a5fa';
  else if (line.startsWith('ℹ')) c = '#94a3b8';
  else if (line.startsWith('⏭')) c = '#fbbf24';
  else if (line.startsWith('═══')) c = '#c084fc';
  else if (line.startsWith('📊')) c = '#60a5fa';
  else if (line.includes('🐂') || line.includes('🐻')) c = '#fbbf24';
  else if (line.startsWith('💰')) c = '#f97316';
  else if (line.startsWith('🛡')) c = '#ef4444';
  else if (line.startsWith('🔢')) c = '#8b5cf6';
  return <div style={{ color: c, whiteSpace: 'pre-wrap', wordBreak: 'break-all', fontSize: 10, lineHeight: 1.6 }}>{`> ${line}`}</div>;
};


const StatCard = ({ label, value, color }: { label: string; value: number; color: string }) => (
  <div style={{
    flex: 1, minWidth: 80, padding: '10px 14px',
    background: 'var(--color-fill-1)', borderRadius: 8,
    border: '1px solid var(--color-border-1)',
  }}>
    <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginBottom: 2 }}>{label}</div>
    <div style={{ fontSize: 20, fontWeight: 700, color }}>{value}</div>
  </div>
);

export default function LiveRunDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [run, setRun] = useState<Run | null>(null);
  const [linkedAccount, setLinkedAccount] = useState<any>(null);
  const [strategy, setStrategy] = useState<Strategy | null>(null);
  const [allocation, setAllocation] = useState<Allocation | null>(null);
  const [positions, setPositions] = useState<Position[]>([]);
  const [trades, setTrades] = useState<Trade[]>([]);
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [signals, setSignals] = useState<Signal[]>([]);
  const [conditions, setConditions] = useState<Condition[]>([]);
  const [decisions, setDecisions] = useState<Decision[]>([]);
  const [loading, setLoading] = useState(true);
  const [executing, setExecuting] = useState<number | null>(null);
  const [syncing, setSyncing] = useState<number | null>(null);
  const [executeModal, setExecuteModal] = useState<{ open: boolean; signal: Signal | null }>({ open: false, signal: null });
  const [execForm, setExecForm] = useState({ actualPrice: '', actualQty: '' });
  const [editSignalModal, setEditSignalModal] = useState<{ open: boolean; signal: Signal | null }>({ open: false, signal: null });
  const [editSignalForm, setEditSignalForm] = useState({ plannedPrice: '', plannedQty: '', actionType: '', reason: '' });
  const [savingSignal, setSavingSignal] = useState(false);
  const [clearModalOpen, setClearModalOpen] = useState(false);
  const [clearPendingCount, setClearPendingCount] = useState(0);
  const [runningDaily, setRunningDaily] = useState(false);
  const [runningTradeExec, setRunningTradeExec] = useState(false);
  const [runDailyMode, setRunDailyMode] = useState<string | null>(null); // pending confirm
  const [debateViewer, setDebateViewer] = useState<{ open: boolean; title: string; content: any[] }>({ open: false, title: '', content: [] });
  const [dailyLogs, setDailyLogs] = useState<string[]>([]);
  const [tradeLogs, setTradeLogs] = useState<string[]>([]);
  const [logsCollapsed, setLogsCollapsed] = useState(false);
  const [strategyExecTime, setStrategyExecTime] = useState('');
  const [tradeExecTime, setTradeExecTime] = useState('');
    const [signalDate, setSignalDate] = useState<string>('');
    const [activeTab, setActiveTab] = useState('positions');
  // Config tab state
  const [configSaving, setConfigSaving] = useState(false);
  const [configAutoDaily, setConfigAutoDaily] = useState('');
  const [configAutoTradeExec, setConfigAutoTradeExec] = useState('');
  const [configNotifyEnabled, setConfigNotifyEnabled] = useState(false);
  const [configExecutionMode, setConfigExecutionMode] = useState('manual');
  const [configAiReviewEnabled, setConfigAiReviewEnabled] = useState(false);
  const [removedNotifyIds, setRemovedNotifyIds] = useState<number[]>([]);
  const [configChannels, setConfigChannels] = useState<{ id?: number; channel: string; name: string; webhookUrl: string; keyword?: string; secret?: string }[]>([]);
  const [configNewChannel, setConfigNewChannel] = useState({ channel: 'dingtalk_bot', webhookUrl: '', keyword: '', secret: '' });
  const [configLoaded, setConfigLoaded] = useState(false);
  const [taskRunning, setTaskRunning] = useState(false);
  const [taskId, setTaskId] = useState<number | null>(null);
  const [taskProgress, setTaskProgress] = useState(0);
  const [taskStage, setTaskStage] = useState('');
  const [taskCode, setTaskCode] = useState('');
  const [taskCompleted, setTaskCompleted] = useState(0);
  const [taskTotal, setTaskTotal] = useState(0);
  const [taskResult, setTaskResult] = useState<any>(null);

  const handleClearSignals = () => {
    if (!signalDate) return;
    const pendingOnDate = signals.filter(s => s.execDate === signalDate && s.status === 'pending');
    if (pendingOnDate.length === 0) return;
    setClearPendingCount(pendingOnDate.length);
    setClearModalOpen(true);
  };

  const doClearSignals = async () => {
    setClearModalOpen(false);
    try {
      const res = await clearLiveSignals(Number(id), signalDate);
      alert('已清空 ' + (res.data?.deleted || 0) + ' 条信号');
      load();
    } catch (e: any) {
      alert('清空失败: ' + (e?.response?.data?.message || e?.message || '未知'));
    }
  };

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const { data: r } = await fetchLiveRun(Number(id));
      const d = r.data || {};
      setRun(d.run || null);
      setStrategy(d.strategy || null);
      setAllocation(d.allocation || null);
      setPositions(d.positions || []);
      setTrades(d.trades || []);
      setSignals(d.signals || []);
      setConditions(d.conditions || []);
      setDecisions(d.decisions || []);
      setLinkedAccount(d.account || null);
      if (!dailyLogs.length) setDailyLogs(d.persistedLogs || []);
      const dates = [...new Set((d.signals || []).map((s: Signal) => s.execDate))].sort().reverse();
      if (dates.length && !signalDate) { setSignalDate(dates[0] as string); }
      // Load execution logs for the latest date
      loadLogs(dates[0] as string);
      try { const { data: s } = await fetchLiveSnapshots(Number(id)); setSnapshots(s.data || []); } catch {}
    } catch (e) { console.error(e); }
    setLoading(false);
  }, [id]);

  useEffect(() => { load(); }, [load]);

  // Resume polling if a task was already running when page loaded
  useEffect(() => {
    let active = true;
    let pollTimer: ReturnType<typeof setInterval> | null = null;
    let failCount = 0;

    const pollTask = async () => {
      try {
        const { data: r } = await fetchLatestTradeExecTask(undefined as any, id ? Number(id) : undefined);
        if (!active) return;
        const pd = r?.data || {};
        failCount = 0;

        if (pd.status === 'running' || pd.status === 'pending') {
          setTaskRunning(true);
          setRunningTradeExec(true);
          setTaskId(pd.id);
          setTaskProgress(pd.progress || 0);
          setTaskStage(pd.currentStage || '');
          setTaskCompleted(pd.completedSignals || 0);
          setTaskTotal(pd.totalSignals || 0);
          if (pd.logs && Array.isArray(pd.logs) && pd.logs.length > 0) setDailyLogs(pd.logs);
          if (pd.stageDetails && Array.isArray(pd.stageDetails)) {
            setTaskResult((prev: any) => ({ ...(prev || {}), stageDetails: pd.stageDetails }));
          }
          // Start polling if not already
          if (!pollTimer) {
            pollTimer = setInterval(pollTask, 2000);
          }
          return;
        }

        if (pd.status === 'completed') {
          setTaskRunning(false);
          setRunningTradeExec(false);
          setTaskResult(pd.result || {});
          load();
          return;
        }

        if (pd.status === 'failed') {
          setTaskRunning(false);
          setRunningTradeExec(false);
          Message.warning('上次任务执行失败: ' + (pd.error || '未知'));
          return;
        }

        // No active task found — stop polling
        if (pollTimer) {
          clearInterval(pollTimer);
          pollTimer = null;
        }
      } catch (e) {
        failCount++;
        console.error('[TradeExec] poll error:', e);
        if (failCount >= 5 && pollTimer) {
          clearInterval(pollTimer);
          pollTimer = null;
        }
      }
    };

    // Initial poll
    pollTask();

    return () => {
      active = false;
      if (pollTimer) clearInterval(pollTimer);
    };
  }, [id]);

  // Resume polling if a daily-run task was already running when page loaded
  useEffect(() => {
    let active = true;
    let pollTimer: ReturnType<typeof setInterval> | null = null;
    let failCount = 0;

    const pollDailyTask = async () => {
      try {
        const { data: r } = await fetchLatestDailyRunTask(undefined as any, id ? Number(id) : undefined);
        if (!active) return;
        const pd = r?.data || {};
        failCount = 0;

        if (pd.status === 'running' || pd.status === 'pending') {
          setRunningDaily(true);
          if (pd.logs && Array.isArray(pd.logs) && pd.logs.length > 0) setDailyLogs(pd.logs);
          if (!pollTimer) {
            pollTimer = setInterval(pollDailyTask, 2000);
          }
          return;
        }

        if (pd.status === 'completed') {
          setRunningDaily(false);
          if (pd.logs && Array.isArray(pd.logs) && pd.logs.length > 0) setDailyLogs(pd.logs);
          Message.success(`策略执行完成: ${pd.signalCount || 0} 个信号`);
          load();
          return;
        }

        if (pd.status === 'failed') {
          setRunningDaily(false);
          return;
        }

        if (pollTimer) {
          clearInterval(pollTimer);
          pollTimer = null;
        }
      } catch (e) {
        failCount++;
        if (failCount >= 5 && pollTimer) {
          clearInterval(pollTimer);
          pollTimer = null;
        }
      }
    };

    pollDailyTask();

    return () => {
      active = false;
      if (pollTimer) clearInterval(pollTimer);
    };
  }, [id]);

  const loadConfig = useCallback(async () => {
    if (!run || configLoaded) return;
    setConfigAutoDaily(run.autoDailyCron || '18:00');
    setConfigAutoTradeExec(run.autoTradeExecCron || '09:00');
    setConfigNotifyEnabled(run.notifyEnabled || false);
    setConfigExecutionMode(run.executionMode || 'manual');
    setConfigAiReviewEnabled(run.aiReviewEnabled || false);
    // Load notification configs
    try {
      const { data: ncs } = await fetchNotificationConfigs();
      const allConfigs = ncs?.data || [];
      const runChannelIds: number[] = (() => { try { return JSON.parse(run.notifyChannels || '[]'); } catch { return []; } })();
      const linked = allConfigs.filter((nc: any) => runChannelIds.includes(nc.id)).map((nc: any) => ({
        id: nc.id, channel: nc.channel, name: nc.name, webhookUrl: nc.config?.webhook_url || '', keyword: nc.config?.keyword || '', secret: nc.config?.secret || ''
      }));
      setRemovedNotifyIds([]); setConfigChannels(linked);
    } catch(e) { console.error("load notification configs failed", e); Message.warning("加载通知配置失败"); setConfigChannels([]); }
    setConfigLoaded(true);
  }, [run, configLoaded]);

  const handleSaveConfig = async () => {
    setConfigSaving(true);
    try {
      // Handle notification config changes
      const origChannelIds: number[] = (() => { try { return JSON.parse(run?.notifyChannels || '[]'); } catch { return []; } })();
      const keepIds = configChannels.filter(c => c.id).map(c => c.id as number);
      const toDelete = removedNotifyIds.filter(id => origChannelIds.includes(id));
      // Create new configs
      const newIds: number[] = [];
      for (const nc of configChannels) {
        if (!nc.id) {
          try {
            const cfgObj: any = { webhook_url: nc.webhookUrl }; if (nc.keyword) cfgObj.keyword = nc.keyword; if (nc.secret) cfgObj.secret = nc.secret; const { data: created } = await createNotificationConfig({ channel: nc.channel, name: nc.name, config: cfgObj });
            if (created?.data?.id) newIds.push(created.data.id);
          } catch (e) { console.error("create notif config failed", e); Message.error("通知渠道创建失败: " + ((e as any)?.message || "")); }
        }
      }
      for (const id of toDelete) {
        try { await deleteNotificationConfig(id); } catch (e) { console.error('delete notif config failed', e); }
      }
      const finalChannelIds = [...keepIds, ...newIds];
      const notifyChannels = JSON.stringify(finalChannelIds);

      // Update run config
      const rid = run!.id;
      await updateLiveRunConfig(rid, {
        autoDailyCron: configAutoDaily,
        autoTradeExecCron: configAutoTradeExec,
        notifyEnabled: configNotifyEnabled && finalChannelIds.length > 0,
        notifyChannels: configNotifyEnabled ? notifyChannels : '[]',
        executionMode: configExecutionMode,
        aiReviewEnabled: configAiReviewEnabled,
      });
      Message.success('配置已保存');
      setConfigLoaded(false); // force reload on next tab switch
      load();
    } catch (e: any) { Message.error('保存失败: ' + (e?.message || '未知')); }
    setConfigSaving(false);
  };

  // Load config when tab switches to 'config'
  useEffect(() => {
    if (activeTab === 'config' && run) {
      loadConfig();
    }
  }, [activeTab, run, loadConfig]);

  const [sendingNotify, setSendingNotify] = useState(false);
  const handleSendNotify = async () => {
    if (!id) return;
    setSendingNotify(true);
    try {
      const { data: r } = await sendLiveRunNotification(Number(id));
      Message.success(r?.data?.message || r?.message || '通知已发送');
    } catch (e: any) { Message.error('发送失败: ' + (e?.message || '未知')); }
    setSendingNotify(false);
  };
  const handleRunDaily = async (mode: string) => {
    setRunningDaily(true); setDailyLogs([]);
    if (!id) { Message.error('缺少运行ID'); setRunningDaily(false); return; }
    const label = mode === 'after_close' ? '策略执行' : mode === 'trade_exec' ? '信号刷新' : '盘中刷新';
    try {
      const { data: r } = await runLiveDaily('', mode, Number(id));
      const d = r.data || {};
      const tid = d.taskId;
      if (!tid) {
        Message.error('创建任务失败');
        setRunningDaily(false);
        return;
      }
      Message.info(`${label}任务已启动，正在异步执行...`);
      
      const poll = setInterval(async () => {
        try {
          const { data: pollR } = await fetchDailyRunTask(tid);
          const pd = pollR.data || {};
          if (pd.logs && pd.logs.length > 0) setDailyLogs(pd.logs);
          if (pd.status === 'completed') {
            clearInterval(poll);
            setRunningDaily(false);
            Message.success(`${label}: ${pd.signalCount || 0} 个信号`);
            load();
            loadLogs(signalDate);
          } else if (pd.status === 'failed') {
            clearInterval(poll);
            setRunningDaily(false);
            Message.error(`${label}失败: ${pd.error || '未知错误'}`);
            load();
          }
        } catch {
          clearInterval(poll);
          setRunningDaily(false);
        }
      }, 2000);
      // Safety timeout: 10 minutes
      setTimeout(() => { clearInterval(poll); if (runningDaily) setRunningDaily(false); }, 600000);
    } catch (e: any) { Message.error('执行失败: ' + (e?.message || '未知')); setRunningDaily(false); }
  };

  const refreshSignals = async () => {
    if (!id) return;
    try {
      const { data: res } = await fetchLiveRun(Number(id));
      const d = res.data || {};
      if (d.signals) setSignals(d.signals || []);
      if (d.run) setRun(d.run);
      if (d.trades) setTrades(d.trades || []);
      if (d.positions) setPositions(d.positions || []);
      if (d.decisions) setDecisions(d.decisions || []);
    } catch { /* silent background refresh */ }
  };

  const loadLogs = async (date: string) => {
    if (!id) return;
    try {
      const { data: res } = await fetchRunLogs(Number(id), date);
      const d = res.data || {};
      const logs = d.logs || {};
      setDailyLogs(logs.strategy || []);
      setTradeLogs(logs.trade_exec || []);
      setStrategyExecTime(d.strategyTime || '');
      setTradeExecTime(d.tradeExecTime || '');
    } catch { /* silent */ }
  };

  // Reload logs when signalDate changes
  useEffect(() => {
    if (signalDate && id) loadLogs(signalDate);
  }, [signalDate, id]);

  const handleTradeExec = async () => {
    setRunningTradeExec(true); setTaskRunning(true);
    setTaskProgress(0); setTaskStage('init'); setTaskResult(null);
    try {
      const skipAI = !(run?.aiReviewEnabled);
      const { data: r } = await runTradeExec(signalDate, skipAI, id ? Number(id) : undefined, true);
      const d = r.data || {};
      // Sync response from TradeExecService.ExecuteForRun
      setTaskResult(d);
      setRunningTradeExec(false);
      setTaskRunning(false);
      if (d.totalSignals === 0) {
        Message.info('没有待执行的信号');
      } else if (d.executed > 0) {
        Message.success(`交易执行完成: ${d.executed} 笔成交, ${d.confirmed} 已确认, ${d.rejected} 已驳回`);
      } else if (d.failed > 0) {
        Message.warning(`交易执行部分失败: ${d.confirmed} 已确认, ${d.rejected} 已驳回, ${d.failed} 失败`);
      } else {
        Message.info(`交易执行完成: ${d.confirmed} 已确认, ${d.rejected} 已驳回`);
      }
      if (d.logs && d.logs.length > 0) setTradeLogs(d.logs);
      // Refresh signal statuses and reload log timestamps
      refreshSignals();
      loadLogs(signalDate);
    } catch (e: any) {
      Message.error('交易执行失败: ' + (e?.response?.data?.message || e?.message || '网络错误'));
      setRunningTradeExec(false);
      setTaskRunning(false);
    }
  };


  const openDebate = (dec: Decision) => {
    try {
      const content = typeof dec.taDebateJson === 'string' ? JSON.parse(dec.taDebateJson) : dec.taDebateJson;
      setDebateViewer({
        open: true,
        title: `${dec.stockName || dec.stockCode} 多轮辩论`,
        content: Array.isArray(content) ? content : [],
      });
    } catch {
      Message.warning('无法解析辩论数据');
    }
  };

  const openExecuteModal = (signal: Signal) => {
    setExecuteModal({ open: true, signal });
    setExecForm({
      actualPrice: signal.plannedPrice ? String(signal.plannedPrice) : '',
      actualQty: signal.plannedQty ? String(signal.plannedQty) : '',
    });
  };

  const handleConfirmExecute = async () => {
    const sig = executeModal.signal;
    if (!sig) return;
    setExecuteModal({ open: false, signal: null });
    setExecuting(sig.id);
    try {
      const price = parseFloat(execForm.actualPrice) || 0;
      const qty = parseInt(execForm.actualQty) || 0;
      await executeLiveSignal(sig.id, { action: 'execute', actualPrice: price, actualQty: qty });
      Message.success('交易已执行');
      load();
    } catch (e: any) { Message.error('执行失败: ' + (e?.message || '未知')); }
    setExecuting(null);
  };

  const handleOpenEditSignal = (sig: Signal) => {
    setEditSignalForm({
      plannedPrice: String(sig.plannedPrice || ''),
      plannedQty: String(sig.plannedQty || ''),
      actionType: sig.actionType || '',
      reason: sig.reason || '',
    });
    setEditSignalModal({ open: true, signal: sig });
  };

  const handleSaveEditSignal = async () => {
    const sig = editSignalModal.signal;
    if (!sig) return;
    setSavingSignal(true);
    try {
      const updates: any = {};
      const newPrice = parseFloat(editSignalForm.plannedPrice);
      const newQty = parseInt(editSignalForm.plannedQty);
      if (!isNaN(newPrice)) updates.plannedPrice = newPrice;
      if (!isNaN(newQty)) updates.plannedQty = newQty;
      if (editSignalForm.reason !== sig.reason) updates.reason = editSignalForm.reason;
      if (Object.keys(updates).length === 0) { Message.info('无变更'); setSavingSignal(false); return; }
      await updateLiveSignal(sig.id, updates);
      Message.success('信号已更新');
      setEditSignalModal({ open: false, signal: null });
      load();
    } catch (e: any) { Message.error('更新失败: ' + (e?.message || '未知')); }
    setSavingSignal(false);
  };

  const handleSyncOrder = async (sig: Signal) => {
    setSyncing(sig.id);
    try {
      const resp = await syncSignalOrder(sig.id);
      const status = resp.data?.data?.status || resp.data?.status || 'ok';
      Message.success(`订单同步完成: ${status}`);
      loadSignals();
    } catch (e: any) {
      // Interceptor already shows toast, just log
      console.error('[syncOrder] failed:', e?.message || e);
    } finally {
      setSyncing(null);
    }
  };

  const handleDeleteSignal = async (sig: Signal) => {
    try {
      await deleteLiveSignal(sig.id);
      Message.success('信号已删除');
      load();
    } catch (e: any) { Message.error('删除失败: ' + (e?.message || '未知')); }
  };

  const handleAbandonSignal = async () => {
    const sig = executeModal.signal;
    if (!sig) return;
    setExecuteModal({ open: false, signal: null });
    setExecuting(sig.id);
    try {
      await executeLiveSignal(sig.id, { action: 'abandon' });
      Message.info('信号已放弃');
      load();
    } catch (e: any) { Message.error('操作失败: ' + (e?.message || '未知')); }
    setExecuting(null);
  };

  const posValue = useMemo(() => positions.reduce((s, p) => s + p.currentPrice * p.quantity, 0), [positions]);
  const totalEquity = (allocation?.currentCash || 0) + posValue;
  const dateOptions = useMemo(() => {
    const dates = [...new Set(signals.map(s => s.execDate))].sort().reverse();
    return dates.map(d => ({ label: d, value: d }));
  }, [signals]);

  const filteredSignals = useMemo(() => {
    const dates = [...new Set(signals.map(s => s.execDate))].sort().reverse();
    const activeDate = signalDate || dates[0] || '';
    if (!activeDate) return [];
    return signals.filter(s => s.execDate === activeDate);
  }, [signals, signalDate]);

  const pendingCount = filteredSignals.filter(s => s.status === 'pending').length;

  const signalSummary = useMemo(() => {
    const pending = filteredSignals.filter(s => s.status === 'pending');
    const buys = pending.filter(s => s.actionType === 'buy');
    const sells = pending.filter(s => s.actionType === 'sell' || s.actionType === 'stop');
    const adds = pending.filter(s => s.actionType === 'add');
    const reduces = pending.filter(s => s.actionType === 'reduce');
    return {
      buyCount: buys.length, buyAmount: buys.reduce((s, x) => s + x.plannedAmount, 0),
      sellCount: sells.length, sellAmount: sells.reduce((s, x) => s + x.plannedAmount, 0),
      addCount: adds.length, addAmount: adds.reduce((s, x) => s + x.plannedAmount, 0),
      reduceCount: reduces.length, reduceAmount: reduces.reduce((s, x) => s + x.plannedAmount, 0),
    };
  }, [filteredSignals]);

  if (loading) return <div style={{ display: 'flex', justifyContent: 'center', padding: 100 }}><Spin size={30} /></div>;
  if (!run) return <div style={{ padding: 100, textAlign: 'center', color: 'var(--color-text-2)' }}>未找到运行实例</div>;

  return (
    <div style={{ padding: 20, maxWidth: 1600, margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <Button type="text" icon={<ArrowLeft size={16} />} onClick={() => navigate('/live')}>返回</Button>
          <h2 style={{ margin: 0, fontSize: 20, fontWeight: 700 }}>{run.name}</h2>
          <Tag color={run.status === 'active' ? 'green' : 'orange'}>{run.status === 'active' ? '运行中' : run.status}</Tag>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <Button size="small" icon={<Settings size={12} />} onClick={() => navigate(`/strategy?strategy=${strategy?.id || run.strategyId}`)}>策略配置</Button>
        </div>
      </div>

      {/* Strategy + Account */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14, marginBottom: 16 }}>
        <Card title={<span><Settings size={15} style={{ marginRight: 6 }} />策略配置</span>} style={{ borderRadius: 10 }} bodyStyle={{ padding: '12px 16px' }}>
          {strategy && (
            <div style={{ fontSize: 12, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '6px 16px' }}>
              <div><span style={{ color: 'var(--color-text-3)' }}>策略名</span> {strategy.name}</div>
              <div><span style={{ color: 'var(--color-text-3)' }}>仓位模式</span> {strategy.positionSizing || 'fixed_pct'}</div>
              <div><span style={{ color: 'var(--color-text-3)' }}>止盈/止损</span> {strategy.stopProfit > 0 ? `+${strategy.stopProfit}%` : '—'} / {strategy.stopLoss < 0 ? `${strategy.stopLoss}%` : '—'}</div>
              <div><span style={{ color: 'var(--color-text-3)' }}>最大持仓</span> {strategy.maxHoldings}只</div>
              <div><span style={{ color: 'var(--color-text-3)' }}>单票/首仓/加仓</span> {(strategy.positionConcentrationLimit*100).toFixed(0)}% / {strategy.buyPositionPct}% / {strategy.addPositionPct}%</div>
              <div><span style={{ color: 'var(--color-text-3)' }}>条件数</span> {conditions.length}条</div>
            </div>
          )}
        </Card>
        <Card title={<span><Wallet size={15} style={{ marginRight: 6 }} />资金概览</span>} style={{ borderRadius: 10 }} bodyStyle={{ padding: '12px 16px' }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 8 }}>
            {[
              { l: '当前权益', v: `¥${totalEquity.toLocaleString()}`, c: '#165DFF' },
              { l: '可用现金', v: `¥${(allocation?.currentCash || 0).toLocaleString()}`, c: '#0FC6C2' },
              { l: '持仓市值', v: `¥${posValue.toLocaleString()}`, c: '#722ED1' },
              { l: '累计收益', v: `${pnlSign(run.totalReturn)}${(run.totalReturn||0).toFixed(2)}%`, c: pnlColor(run.totalReturn) },
              { l: '最大回撤', v: `${(run.maxDrawdown||0).toFixed(2)}%`, c: '#F53F3F' },
              { l: '胜率/交易', v: `${(run.winRate||0).toFixed(0)}% / ${run.tradeCount||0}笔`, c: '#F7BA1E' },
            ].map((item, i) => (
              <div key={i} style={{ background: 'var(--color-fill-1)', borderRadius: 8, padding: '10px 12px' }}>
                <div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>{item.l}</div>
                <div style={{ fontSize: 15, fontWeight: 700, color: item.c, fontFamily: "'SF Mono', monospace" }}>{item.v}</div>
              </div>
            ))}
          </div>
          {linkedAccount && (
            <div style={{ marginTop: 12, padding: '8px 12px', background: 'var(--color-fill-1)', borderRadius: 8, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <Building2 size={14} style={{ color: 'var(--color-text-3)' }} />
                <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>{linkedAccount.name}</span>
                <span style={{ fontSize: 11, color: 'var(--color-text-4)' }}>#{linkedAccount.id}</span>
              </div>
              <div style={{ display: 'flex', gap: 6 }}>
                {(linkedAccount.brokerMode === 'mx_moni') && <Tag color="blue" style={{ fontSize: 10 }}>妙想自动</Tag>}
                {(linkedAccount.brokerMode === 'lobster') && <Tag color="green" style={{ fontSize: 10 }}>龙虾自动</Tag>}
                {(linkedAccount.brokerMode === 'manual' || !linkedAccount.brokerMode) && <Tag style={{ fontSize: 10 }}>手动</Tag>}
              </div>
            </div>
          )}
        </Card>
      </div>

      {/* Equity Curve */}
      {snapshots.length > 1 && (() => {
        const initialEquity = snapshots[0]?.totalEquity || 0;
        const dates = snapshots.map(s => s.snapshotDate);
        const equities = snapshots.map(s => s.totalEquity);
        const cashData = snapshots.map(s => s.cash);
        const posData = snapshots.map(s => s.positionValue);
        const returns = snapshots.map(s => s.cumulativeReturn);
        const drawdowns = snapshots.map(s => s.maxDrawdownPct);

        const option = {
          tooltip: {
            trigger: 'axis',
            backgroundColor: 'rgba(20,20,30,0.92)',
            borderColor: '#333',
            textStyle: { color: '#fff', fontSize: 12, fontFamily: "'SF Mono', monospace" },
            formatter: (params: any) => {
              const d = params[0]?.axisValue || '';
              const eq = params.find((p: any) => p.seriesName === '总权益');
              const ret = params.find((p: any) => p.seriesName === '累计收益率');
              const dd = params.find((p: any) => p.seriesName === '最大回撤');
              let html = `<div style="font-weight:700;margin-bottom:6px">${d}</div>`;
              if (eq) html += `<div>📊 总权益 <b style="float:right;margin-left:20px">¥${eq.value.toLocaleString()}</b></div>`;
              if (ret) html += `<div style="color:${ret.value>=0?'#F53F3F':'#00B42A'}">📈 累计收益 <b style="float:right;margin-left:20px">${ret.value>=0?'+':''}${ret.value.toFixed(2)}%</b></div>`;
              if (dd) html += `<div style="color:#F7BA1E">⚠ 最大回撤 <b style="float:right;margin-left:20px">${dd.value.toFixed(2)}%</b></div>`;
              return html;
            },
          },
          legend: {
            data: ['总权益', '可用资金', '持仓市值'],
            bottom: 0,
            textStyle: { fontSize: 11, color: '#999' },
            itemWidth: 14, itemHeight: 8,
          },
          grid: { top: 20, right: 20, bottom: 35, left: 60 },
          xAxis: {
            type: 'category', data: dates,
            axisLine: { lineStyle: { color: '#e0e0e0' } },
            axisLabel: { fontSize: 10, color: '#999', formatter: (v: string) => v.slice(5) },
          },
          yAxis: [
            {
              type: 'value',
              axisLabel: { fontSize: 10, color: '#999', formatter: (v: number) => (v/10000).toFixed(0)+'万' },
              splitLine: { lineStyle: { color: '#f0f0f0', type: 'dashed' } },
            },
            {
              type: 'value',
              axisLabel: { fontSize: 10, color: '#999', formatter: (v: number) => v.toFixed(0)+'%' },
              splitLine: { show: false },
            },
          ],
          series: [
            {
              name: '持仓市值', type: 'bar', stack: 'total',
              data: posData, itemStyle: { color: '#722ED1', borderRadius: [0,0,0,0] },
              barMaxWidth: 36, barWidth: '60%', emphasis: { itemStyle: { color: '#9254DE' } },
            },
            {
              name: '可用资金', type: 'bar', stack: 'total',
              data: cashData, itemStyle: { color: '#0FC6C2', borderRadius: [4,4,0,0] },
              barMaxWidth: 36, barWidth: '60%', emphasis: { itemStyle: { color: '#36D1C8' } },
            },
            {
              name: '总权益', type: 'line',
              data: equities, yAxisIndex: 0,
              lineStyle: { color: '#165DFF', width: 2.5 },
              itemStyle: { color: '#165DFF' },
              symbol: 'none',
              markLine: {
                silent: true,
                symbol: 'none',
                data: [{ yAxis: initialEquity, label: { formatter: '初始', fontSize: 10, color: '#999' }, lineStyle: { color: '#999', type: 'dashed', width: 1 } }],
              },
            },
            {
              name: '累计收益率', type: 'line',
              data: returns, yAxisIndex: 1,
              lineStyle: { color: '#F7BA1E', width: 1.5, type: 'dashed' },
              itemStyle: { color: '#F7BA1E' },
              symbol: 'none',
            },
            {
              name: '最大回撤', type: 'line',
              data: drawdowns, yAxisIndex: 1,
              lineStyle: { color: '#F53F3F', width: 1, type: 'dotted' },
              itemStyle: { color: '#F53F3F' },
              symbol: 'none',
            },
          ],
        };

        return (
          <Card title={<span><TrendingUp size={15} style={{ marginRight: 6 }} />权益曲线</span>}
            style={{ borderRadius: 10, marginBottom: 16 }}
            bodyStyle={{ padding: '8px 12px' }}>
            <ReactECharts option={option} style={{ height: 260 }} notMerge />
            <div style={{ display: 'flex', gap: 24, justifyContent: 'center', padding: '8px 0 4px', fontSize: 12, color: 'var(--color-text-2)' }}>
              <span>📅 起始: <b>{dates[0]}</b></span>
              <span>💰 初始权益: <b>¥{initialEquity.toLocaleString()}</b></span>
              <span>📊 当前: <b style={{ color: equities[equities.length-1] >= initialEquity ? '#F53F3F' : '#00B42A' }}>¥{equities[equities.length-1].toLocaleString()}</b></span>
              <span>📈 累计收益: <b style={{ color: returns[returns.length-1] >= 0 ? '#F53F3F' : '#00B42A' }}>{returns[returns.length-1] >= 0 ? '+' : ''}{returns[returns.length-1].toFixed(2)}%</b></span>
            </div>
          </Card>
        );
      })()}

      {/* Tabs: 当前持仓 | 信号决策 | 最近交易 */}
      <Card style={{ borderRadius: 10 }} bodyStyle={{ padding: '12px 16px' }}>
        <Tabs activeTab={activeTab} onChange={setActiveTab}>
          {/* 当前持仓 */}
          <Tabs.TabPane key="positions" title={`当前持仓 (${positions.filter(p => p.quantity > 0).length})`}>
            <Table data={positions.filter(p => p.quantity > 0)} rowKey="id" size="small" pagination={false}
              columns={[
                { title: '代码', dataIndex: 'stockCode', width: 75, render: (v: string) => <span style={{ fontWeight: 600, cursor: 'pointer', color: '#165DFF' }} onClick={() => navigate(`/stock/${v}`)}>{v}</span> },
                { title: '名称', dataIndex: 'stockName', width: 85 },
                { title: '持仓', dataIndex: 'quantity', width: 55 },
                { title: '成本', dataIndex: 'avgCost', width: 70, render: (v: number) => `¥${v.toFixed(2)}` },
                { title: '现价', dataIndex: 'currentPrice', width: 70, render: (v: number) => `¥${v.toFixed(2)}` },
                { title: '市值', width: 85, render: (_: any, r: Position) => `¥${(r.currentPrice * r.quantity).toLocaleString()}` },
                { title: '浮动盈亏', width: 120, render: (_: any, r: Position) => (
                  <span style={{ color: pnlColor(r.unrealizedPnl), fontWeight: 600, fontSize: 12 }}>
                    {pnlSign(r.unrealizedPnl)}¥{Math.abs(r.unrealizedPnl).toFixed(0)} ({pnlSign(r.unrealizedPnlPct)}{r.unrealizedPnlPct.toFixed(2)}%)
                  </span>
                )},
                { title: '持天数', dataIndex: 'holdDays', width: 50 },
              ]}
            />
          </Tabs.TabPane>

          {/* 信号决策 */}
          <Tabs.TabPane key="signals" title={`信号决策 (${pendingCount}/${filteredSignals.length})`}>
            {/* Toolbar */}
            <div style={{
              display: 'flex', justifyContent: 'space-between', alignItems: 'center',
              marginBottom: 12, padding: '8px 12px',
              background: 'var(--color-fill-1)', borderRadius: 8
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: 'var(--color-text-2)' }}>
                <Calendar size={14} />
                <span>查看日期</span>
                <Select size="small" style={{ width: 130 }} value={signalDate || undefined}
                  onChange={(v: any) => setSignalDate(v || '')}
                  options={dateOptions} placeholder="最新日期"
                  allowClear
                />
                {signalDate && <Tag size="small" color="arcoblue">{signalDate}</Tag>}
              </div>
              <div style={{ display: 'flex', gap: 6 }}>
                <Dropdown
                  droplist={
                    <Menu onClickMenuItem={(key) => setRunDailyMode(key)}>
                      <Menu.Item key="after_close">📅 盘后执行 (生成T+1信号)</Menu.Item>
                      <Menu.Item key="trade_exec">🌅 信号刷新 (刷新当日信号)</Menu.Item>
                      <Menu.Item key="intraday">📊 盘中刷新 (刷新未执行信号)</Menu.Item>
                    </Menu>
                  }
                  trigger="click"
                >
                  <Button size="small" icon={<RefreshCw size={12} />} loading={runningDaily}>
                    策略执行
                  </Button>
                </Dropdown>
                <Button size="small" type="primary" icon={<Zap size={12} />} loading={runningTradeExec} onClick={handleTradeExec}>
                  交易执行
                </Button>
              </div>
            </div>

            {/* Dual Log Panels: Strategy | Trade — collapsible */}
            {(dailyLogs.length > 0 || tradeLogs.length > 0) && (
              <div style={{ marginBottom: 12 }}>
                <div
                  onClick={() => setLogsCollapsed(!logsCollapsed)}
                  style={{
                    cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6, padding: '6px 10px',
                    background: 'var(--color-fill-1)', borderRadius: '8px 8px 0 0', border: '1px solid var(--color-border-2)',
                    borderBottom: logsCollapsed ? '1px solid var(--color-border-2)' : 'none',
                    userSelect: 'none',
                  }}
                >
                  <span style={{ fontSize: 11, color: 'var(--color-text-3)', transition: 'transform 0.2s', transform: logsCollapsed ? 'rotate(-90deg)' : 'rotate(0)' }}>▼</span>
                  <Activity size={12} style={{ color: '#165DFF' }} />
                  <span style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-text-2)' }}>执行控制台</span>
                  <Tag size="small" color="blue" style={{ fontSize: 9 }}>{dailyLogs.length}行</Tag>
                  {strategyExecTime ? <span style={{ fontSize: 9, color: '#165DFF' }}>策略 {strategyExecTime}</span> : <span style={{ fontSize: 9, color: 'var(--color-text-4)' }}>策略 —</span>}
                  <span style={{ width: 1, height: 12, background: 'var(--color-border-2)', margin: '0 6px' }} />
                  <Zap size={12} style={{ color: '#722ED1' }} />
                  <Tag size="small" color="purple" style={{ fontSize: 9 }}>{tradeLogs.length}行</Tag>
                  {tradeExecTime ? <span style={{ fontSize: 9, color: '#722ED1' }}>交易 {tradeExecTime}</span> : <span style={{ fontSize: 9, color: 'var(--color-text-4)' }}>交易 —</span>}
                  <div style={{ flex: 1 }} />
                  <Button size="mini" type="text" style={{ color: '#64748b', fontSize: 9, padding: '0 4px' }}
                    onClick={e => { e.stopPropagation(); setDailyLogs([]); setTradeLogs([]); }}>清空全部</Button>
                </div>
                {!logsCollapsed && (
              <div style={{ display: 'flex', gap: 8 }}>
                {/* Strategy Execution Logs */}
                <div style={{
                  flex: 1, background: '#1a1a2e', borderRadius: 10, border: '1px solid rgba(22,93,255,0.2)',
                  overflow: 'hidden', minWidth: 0
                }}>
                  <div style={{
                    padding: '8px 14px', background: 'rgba(22,93,255,0.1)',
                    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                    borderBottom: '1px solid rgba(22,93,255,0.1)'
                  }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                      <Activity size={12} style={{ color: '#165DFF' }} />
                      <span style={{ fontSize: 11, fontWeight: 600, color: '#94a3b8' }}>策略执行</span>
                      <Tag size="small" color="blue" style={{ fontSize: 9 }}>{dailyLogs.length} 行</Tag>
                      {strategyExecTime && <span style={{ fontSize: 9, color: '#165DFF', fontFamily: "'SF Mono', monospace" }}>{strategyExecTime}</span>}
                    </div>
                    <Button size="mini" type="text" style={{ color: '#64748b', fontSize: 9, padding: '0 4px' }}
                      onClick={() => setDailyLogs([])}>清除</Button>
                  </div>
                  <div style={{ padding: '8px 14px', maxHeight: 200, overflowY: 'auto', fontFamily: "'SF Mono', monospace", fontSize: 10, minHeight: dailyLogs.length > 0 ? 60 : 0 }}>
                    {dailyLogs.length === 0 ? <div style={{ color: '#555', fontSize: 10, textAlign: 'center', padding: '12px 0' }}>暂无日志</div> : dailyLogs.map((line, i) => <LogLine key={i} line={line} />)}
                  </div>
                </div>
                {/* Trade Execution Logs */}
                <div style={{
                  flex: 1, background: '#1a1a2e', borderRadius: 10, border: '1px solid rgba(114,46,209,0.2)',
                  overflow: 'hidden', minWidth: 0
                }}>
                  <div style={{
                    padding: '8px 14px', background: 'rgba(114,46,209,0.1)',
                    display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                    borderBottom: '1px solid rgba(114,46,209,0.1)'
                  }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                      <Zap size={12} style={{ color: '#722ED1' }} />
                      <span style={{ fontSize: 11, fontWeight: 600, color: '#94a3b8' }}>交易执行</span>
                      <Tag size="small" color="purple" style={{ fontSize: 9 }}>{tradeLogs.length} 行</Tag>
                      {tradeExecTime && <span style={{ fontSize: 9, color: '#722ED1', fontFamily: "'SF Mono', monospace" }}>{tradeExecTime}</span>}
                    </div>
                    <Button size="mini" type="text" style={{ color: '#64748b', fontSize: 9, padding: '0 4px' }}
                      onClick={() => setTradeLogs([])}>清除</Button>
                  </div>
                  <div style={{ padding: '8px 14px', maxHeight: 200, overflowY: 'auto', fontFamily: "'SF Mono', monospace", fontSize: 10, minHeight: tradeLogs.length > 0 ? 60 : 0 }}>
                    {tradeLogs.length === 0 ? <div style={{ color: '#555', fontSize: 10, textAlign: 'center', padding: '12px 0' }}>暂无日志</div> : tradeLogs.map((line, i) => <LogLine key={i} line={line} />)}
                  </div>
                </div>
              </div>
                )}
              </div>
            )}

            {/* Task Progress — per-signal phase cards with live logs */}
            {taskRunning && (
              <div style={{
                background: 'linear-gradient(135deg, #1a1a2e, #16213e)', borderRadius: 10, padding: 14, marginBottom: 12,
                border: '1px solid rgba(22,93,255,0.2)'
              }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <Loader size={16} style={{ color: '#165DFF' }} />
                    <span style={{ fontSize: 13, fontWeight: 600, color: '#e0e0ff' }}>交易执行执行中</span>
                    <Tag size="small" color="blue">{taskCompleted}/{taskTotal} 信号</Tag>
                  </div>
                  <Progress percent={taskProgress} status="active" style={{ width: 120, marginBottom: 0 }} />
                </div>
                {/* Per-signal progress cards */}
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 8 }}>
                  {(taskResult?.stageDetails || []).map((sd: any, i: number) => {
                    const isDone = sd.stage === 'done';
                    const phaseColor = phaseColors[sd.stage] || '#165DFF';
                    return (
                      <div key={i} style={{
                        background: isDone ? 'rgba(0,180,42,0.06)' : 'rgba(22,93,255,0.04)',
                        borderRadius: 8, padding: 10,
                        border: '1px solid ' + (isDone ? 'rgba(0,180,42,0.25)' : phaseColor + '30')
                      }}>
                        {/* Header */}
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 6 }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                            <div style={{ width: 6, height: 6, borderRadius: 3, background: phaseColor,
                              animation: !isDone ? 'pulse 1.2s infinite' : 'none' }} />
                            <span style={{ fontSize: 12, fontWeight: 600, color: '#e0e0ff' }}>{sd.name}</span>
                            <span style={{ fontSize: 10, color: '#666' }}>{sd.code}</span>
                          </div>
                          <Tag size="small" color={isDone ? 'green' : 'blue'} style={{ fontSize: 9 }}>
                            {phaseLabels[sd.stage] || sd.stage}
                          </Tag>
                        </div>
                        {/* Phase-specific logs */}
                        <div style={{
                          background: 'rgba(0,0,0,0.2)', borderRadius: 4, padding: '4px 8px',
                          maxHeight: 80, overflowY: 'auto', fontFamily: "'SF Mono', monospace"
                        }}>
                          {sd.logs && sd.logs.length > 0
                            ? sd.logs.slice(-4).map((l: string, j: number) => <LogLine key={j} line={l} />)
                            : <div style={{ color: '#555', fontSize: 10 }}>等待中...</div>
                          }
                        </div>
                        {/* Action badge */}
                        <div style={{ marginTop: 4, fontSize: 9, color: '#666' }}>
                          {sd.action === 'buy' ? '🟢 买入' : sd.action === 'sell' ? '🔴 卖出' : sd.action === 'add' ? '🟢 加仓' : sd.action === 'reduce' ? '🔴 减仓' : sd.action === 'hold' ? '🔵 持仓分析' : '⚪ ' + sd.action}
                        </div>
                      </div>
                    );
                  })}
                </div>
                {/* Global logs if available */}
                {dailyLogs.length > 0 && (
                  <div style={{ marginTop: 10, background: 'rgba(0,0,0,0.15)', borderRadius: 6, padding: '6px 10px', maxHeight: 100, overflowY: 'auto' }}>
                    <div style={{ fontSize: 10, color: '#555', marginBottom: 4 }}>全局日志</div>
                    {dailyLogs.map((line, i) => <LogLine key={i} line={line} />)}
                  </div>
                )}
              </div>
            )}



            {/* 交易信号（含AI决策） */}
            <div style={{ marginBottom: 16 }}>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8, display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ width: 4, height: 14, borderRadius: 2, background: '#165DFF', display: 'inline-block' }} />
                交易信号 · AI 交易执行
                <Tag size="small" color="arcoblue">{filteredSignals.length} 条</Tag>
                {filteredSignals.some((s: Signal) => s.status === "pending") && <Button size="mini" type="text" status="danger" onClick={handleClearSignals} style={{ fontSize: 11, padding: "0 6px", height: 20 }}>清空待执行</Button>}
                <Tag size="small" color="purple">{decisions.filter((d: Decision) => filteredSignals.some((s: Signal) => s.id === d.signalId)).length} 已验证</Tag>
              </div>

              {/* Signal Summary Bar */}
              {signalSummary.buyCount + signalSummary.sellCount > 0 && (
                <div style={{ display: 'flex', gap: 16, padding: '8px 12px', marginBottom: 8, background: 'var(--color-fill-1)', borderRadius: 8, fontSize: 12, alignItems: 'center', flexWrap: 'wrap' }}>
                  <span style={{ fontWeight: 600, color: 'var(--color-text-2)' }}>待执行汇总</span>
                  {signalSummary.buyCount > 0 && (
                    <span style={{ color: '#F53F3F', fontFamily: "'SF Mono', 'Inter', monospace" }}>
                      📈 买入 {signalSummary.buyCount} 笔 ¥{signalSummary.buyAmount.toLocaleString()}
                    </span>
                  )}
                  {signalSummary.addCount > 0 && (
                    <span style={{ color: '#F53F3F', fontFamily: "'SF Mono', 'Inter', monospace" }}>
                      ➕ 加仓 {signalSummary.addCount} 笔 ¥{signalSummary.addAmount.toLocaleString()}
                    </span>
                  )}
                  {signalSummary.sellCount > 0 && (
                    <span style={{ color: '#00B42A', fontFamily: "'SF Mono', 'Inter', monospace" }}>
                      📉 卖出 {signalSummary.sellCount} 笔 ¥{signalSummary.sellAmount.toLocaleString()}
                    </span>
                  )}
                  {signalSummary.reduceCount > 0 && (
                    <span style={{ color: '#00B42A', fontFamily: "'SF Mono', 'Inter', monospace" }}>
                      🔻 减仓 {signalSummary.reduceCount} 笔 ¥{signalSummary.reduceAmount.toLocaleString()}
                    </span>
                  )}
                  <span style={{ fontFamily: "'SF Mono', 'Inter', monospace", fontWeight: 600, color: 'var(--color-text-1)', marginLeft: 'auto' }}>
                    净变动 ¥{(signalSummary.sellAmount + signalSummary.reduceAmount - signalSummary.buyAmount - signalSummary.addAmount).toLocaleString()}
                  </span>
                </div>
              )}

              {filteredSignals.length === 0 ? (
                <div style={{ padding: 20, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 12, background: 'var(--color-fill-1)', borderRadius: 8 }}>
                  该日期暂无交易信号
                </div>
              ) : (
                <Table data={filteredSignals} rowKey="id" size="small" pagination={false}
                  expandedRowRender={(record: Signal) => {
                    const dec = decisions.find((d: Decision) => d.signalId === record.id);
                    if (!dec) return (
                      <div style={{ padding: '8px 16px', color: 'var(--color-text-3)', fontSize: 11, background: 'var(--color-fill-1)', borderRadius: 6 }}>
                        该信号尚未进行 AI 盘前验证
                      </div>
                    );
                    return (
                      <div style={{ padding: '8px 16px', background: 'var(--color-fill-1)', borderRadius: 6, fontSize: 11, lineHeight: 1.8 }}>
                        <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', marginBottom: 6 }}>
                          <span><b style={{ color: 'var(--color-text-2)' }}>决策规则：</b>
                            <Tag size="small" color="arcoblue" style={{ marginLeft: 4 }}>{dec.decisionRule || '默认'}</Tag>
                          </span>
                          <span><b style={{ color: 'var(--color-text-2)' }}>开盘价：</b>¥{(dec.openPrice || 0).toFixed(2)}</span>
                          <span><b style={{ color: 'var(--color-text-2)' }}>偏离度：</b>
                            <span style={{ color: (dec.openDeviation || 0) >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 600 }}>
                              {(dec.openDeviation || 0) >= 0 ? '+' : ''}{(dec.openDeviation || 0).toFixed(2)}%
                            </span>
                          </span>
                          <span><b style={{ color: 'var(--color-text-2)' }}>建议挂单：</b>¥{(dec.orderPrice || 0).toFixed(2)}</span>
                          <span><b style={{ color: 'var(--color-text-2)' }}>建议数量：</b>{dec.suggestedQty || '—'}</span>
                          <span><b style={{ color: 'var(--color-text-2)' }}>溢价：</b>
                            <span style={{ color: (dec.suggestedPremium || 0) > 0 ? '#F53F3F' : '#00B42A', fontWeight: 600 }}>
                              {(dec.suggestedPremium || 0) > 0 ? '+' : ''}{(dec.suggestedPremium || 0).toFixed(2)}%
                            </span>
                          </span>
                        </div>
                        <div style={{ marginBottom: 4 }}>
                          <b style={{ color: 'var(--color-text-2)' }}>决策理由：</b>
                          <div style={{ color: 'var(--color-text-1)', marginTop: 2, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                            {dec.reason || '无'}
                          </div>
                        </div>
                        {dec.taReasoning && (
                          <details style={{ marginTop: 4 }}>
                            <summary style={{ cursor: 'pointer', color: '#722ED1', fontSize: 11 }}>AI 推理详情</summary>
                            <div style={{ marginTop: 4, padding: '6px 10px', background: 'rgba(114,46,209,0.06)', borderRadius: 4, color: 'var(--color-text-2)', fontSize: 10, whiteSpace: 'pre-wrap', maxHeight: 200, overflowY: 'auto' }}>
                              {dec.taReasoning}
                            </div>
                          </details>
                        )}
                      </div>
                    );
                  }}
                  columns={[
                    { title: '代码', dataIndex: 'stockCode', width: 70, render: (v: string) => <span style={{ fontWeight: 600, cursor: 'pointer', color: '#165DFF', fontSize: 11 }} onClick={() => navigate(`/stock/${v}`)}>{v}</span> },
                    { title: '名称', dataIndex: 'stockName', width: 65 },
                    { title: '市场', width: 42, render: (_: any, r: Signal) => {
                      const tag = getMarketTag(r.stockCode);
                      if (!tag) return <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>—</span>;
                      return <Tag size="small" color="purple" style={{ fontSize: 9, padding: '0 4px', lineHeight: '16px' }}>{tag}</Tag>;
                    }},
                    { title: '数量', dataIndex: 'plannedQty', width: 48, render: (v: number, r: Signal) => {
                      const qty = r.suggestedQty || v || 0;
                      return <span style={{ fontSize: 11, fontWeight: 500 }}>{qty > 0 ? qty.toLocaleString() : '—'}</span>;
                    }},
                    { title: '操作', dataIndex: 'actionType', width: 45, render: (v: string) => {
    const colors: Record<string, string> = { buy: 'green', add: 'green', sell: 'red', reduce: 'red', stop: 'red', hold: 'arcoblue' };
    const labels: Record<string, string> = { buy: '买入', add: '加仓', sell: '卖出', reduce: '减仓', stop: '止损', hold: '持有' };
    return <Tag size="small" color={colors[v] || 'gray'}>{labels[v] || v}</Tag>;
  } },
                    { title: '价格', dataIndex: 'plannedPrice', width: 55, render: (v: number) => <span style={{ fontSize: 11 }}>¥{v.toFixed(2)}</span> },
                    { title: '金额', dataIndex: 'plannedAmount', width: 65, render: (v: number) => `¥${v.toLocaleString()}` },
                    { title: 'AI决策', width: 70, render: (_: any, r: Signal) => {
                      const dec = decisions.find((d: Decision) => d.signalId === r.id);
                      if (!dec) return <Tag size="small" color="gray">未验证</Tag>;
                      const overridden = dec.finalAction !== r.actionType;
                      if (dec.status === 'confirmed') return <Tag size="small" color="green">确认</Tag>;
                      if (dec.status === 'rejected') return <Tag size="small" color="red">驳回</Tag>;
                      if (dec.status === 'modified') return <span style={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                        <Tag size="small" color="orange">变更</Tag>
                        {overridden && <span style={{ fontSize: 9, color: '#F53F3F' }}>{r.actionType}→{dec.finalAction}</span>}
                      </span>;
                      return <Tag size="small">{dec.status}</Tag>;
                    }},
                    { title: '置信度', width: 48, render: (_: any, r: Signal) => {
                      const dec = decisions.find((d: Decision) => d.signalId === r.id);
                      if (!dec) return <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>—</span>;
                      return <span style={{ fontWeight: 600, fontSize: 10, color: dec.confidence >= 60 ? '#00B42A' : dec.confidence >= 30 ? '#F7BA1E' : '#F53F3F' }}>{dec.confidence.toFixed(0)}%</span>;
                    }},
                    { title: '状态', dataIndex: 'status', width: 65, render: (v: string) => {
                      const statusMap: Record<string, [string, string]> = {
                        pending:        ['orange', '待执行'],
                        confirmed:      ['blue', '已确认'],
                        pending_order:  ['purple', '委托中'],
                        pending_manual: ['gray', '待手动'],
                        pending_auto:   ['gray', '待自动'],
                        executed:       ['green', '已成交'],
                        partial_filled: ['arcoblue', '部分成交'],
                        order_failed:   ['red', '下单失败'],
                        cancelled:      ['gray', '已撤单'],
                        rejected:       ['red', '已驳回'],
                        skipped:        ['gray', '已跳过'],
                      };
                      const [color, label] = statusMap[v] || ['gray', v];
                      return <Tag size="small" color={color} style={{ fontSize: 10 }}>{label}</Tag>;
                    }},
                    { title: '原因', dataIndex: 'reason', width: 200, ellipsis: true, render: (v: string) => (
    <span style={{ fontSize: 10, maxWidth: 190, display: 'inline-block' }} title={(v || '').replace(/\| /g, String.fromCharCode(10))}>{v || '—'}</span>
  ) },
                    { title: '操作', width: 120, render: (_: any, r: Signal) => (
                      <div style={{ display: 'flex', gap: 4 }}>
                        {(r.status === 'pending' || r.status === 'confirmed' || r.status === 'pending_order') && (
                          <Button size="mini" type="primary" loading={executing === r.id} onClick={() => openExecuteModal(r)}>交易</Button>
                        )}
                        {(r.status === 'pending_order' || r.status === 'partial_filled') && r.brokerOrderId && (
                          <Button size="mini" type="outline" loading={syncing === r.id} onClick={() => handleSyncOrder(r)}>同步</Button>
                        )}
                        {(r.status === 'pending' || r.status === 'pending_order') && (
                          <>
                            <Button size="mini" type="text" onClick={() => handleOpenEditSignal(r)}>编辑</Button>
                            <Popconfirm title="确定删除此信号？" onOk={() => handleDeleteSignal(r)}>
                              <Button size="mini" type="text" status="danger">删除</Button>
                            </Popconfirm>
                          </>
                        )}
                      </div>
                    )},
                  ]}
                />
              )}
            {/* 信号执行报告（无AI模式）*/}
            {/* 决策报告 — 交易执行摘要 + 结构化分析（适合钉钉/飞书推送） */}
            {decisions.filter((d: Decision) => filteredSignals.some((s: Signal) => s.id === d.signalId)).length > 0 && (() => {
              const verifiedDecisions = decisions.filter((d: Decision) => filteredSignals.some((s: Signal) => s.id === d.signalId));
              const confirmed = verifiedDecisions.filter(d => d.status === 'confirmed' || d.status === 'modified');
              const rejected = verifiedDecisions.filter(d => d.status === 'rejected');
              const buyList = confirmed.filter(d => d.finalAction === 'buy' || d.finalAction === 'add');
              const sellList = confirmed.filter(d => d.finalAction === 'sell' || d.finalAction === 'stop' || d.finalAction === 'reduce');
              const holdList = confirmed.filter(d => d.finalAction === 'hold');
              const totalBuy = buyList.reduce((s, d) => s + (d.finalAmount || 0), 0);
              const totalSell = sellList.reduce((s, d) => s + (d.finalAmount || 0), 0);

              return (
                <div style={{ marginTop: 20 }}>
                  {/* ====== 交易执行指令（最上方，最突出）====== */}
                  {confirmed.length > 0 && (
                    <Card style={{
                      borderRadius: 12, marginBottom: 16,
                      border: '2px solid #165DFF', boxShadow: '0 2px 16px rgba(22,93,255,0.12)'
                    }} bodyStyle={{ padding: 0 }}>
                      {/* 标题栏 */}
                      <div style={{
                        background: 'linear-gradient(135deg, #165DFF, #3c7eff)', borderRadius: '10px 10px 0 0',
                        padding: '12px 20px', display: 'flex', alignItems: 'center', justifyContent: 'space-between'
                      }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <TrendingUp size={18} color="#fff" />
                          <span style={{ fontSize: 15, fontWeight: 700, color: '#fff' }}>交易执行指令</span>
                          <span style={{ fontSize: 11, color: 'rgba(255,255,255,0.85)', background: 'rgba(255,255,255,0.18)', padding: '2px 8px', borderRadius: 4 }}>{signalDate || '今日'}</span>
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                          <span style={{ fontSize: 12, color: 'rgba(255,255,255,0.8)' }}>
                            {confirmed.length} 笔 · 买入{buyList.length} 卖出{sellList.length} 持有{holdList.length}
                          </span>
                          <Button size="mini" loading={sendingNotify} onClick={handleSendNotify}
                            style={{ background: 'rgba(255,255,255,0.2)', border: '1px solid rgba(255,255,255,0.3)', color: '#fff', fontSize: 11, borderRadius: 6 }}>
                            <Bell size={11} style={{ marginRight: 3 }} />发送通知
                          </Button>
                        </div>
                      </div>

                      {/* 指令列表 */}
                      <div style={{ padding: '12px 16px' }}>
                        {confirmed.map((dec, i) => {
                          const sig = filteredSignals.find((s: Signal) => s.id === dec.signalId);
                          const isBuy = dec.finalAction === 'buy' || dec.finalAction === 'add';
                          return (
                            <div key={dec.id} style={{
                              display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap',
                              padding: '10px 0', borderBottom: i < confirmed.length - 1 ? '1px solid var(--color-border-1)' : 'none'
                            }}>
                              {/* 序号 + 股票 */}
                              <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 140 }}>
                                <span style={{
                                  width: 22, height: 22, borderRadius: 6, background: isBuy ? '#00B42A' : '#F53F3F',
                                  color: '#fff', fontSize: 11, fontWeight: 700, display: 'flex', alignItems: 'center', justifyContent: 'center'
                                }}>{i + 1}</span>
                                <span style={{ fontSize: 14, fontWeight: 700, color: '#1a1a2e' }}>
                                  {dec.stockName || sig?.stockName}
                                </span>
                                <span style={{ fontSize: 11, color: '#888' }}>{dec.stockCode}</span>
                              </div>

                              {/* 操作类型 */}
                              <Tag color={isBuy ? 'green' : dec.finalAction === 'hold' ? 'arcoblue' : 'red'} size="small" style={{ fontWeight: 600 }}>
                                {isBuy ? '买入' : dec.finalAction === 'hold' ? '持有' : dec.finalAction === 'reduce' ? '减仓' : '卖出'}
                              </Tag>

                              {/* 交易参数 - 核心信息 */}
                              <div style={{ display: 'flex', gap: 16, alignItems: 'center', flexWrap: 'wrap' }}>
                                <span style={{ fontSize: 13 }}>
                                  价格: <b style={{ color: '#165DFF', fontSize: 15 }}>¥{dec.orderPrice > 0 ? dec.orderPrice.toFixed(2) : dec.finalPrice.toFixed(2)}</b>
                                </span>
                                {dec.suggestedPremium !== 0 && (
                                  <span style={{ fontSize: 11, color: dec.suggestedPremium > 0 ? '#F53F3F' : '#00B42A' }}>
                                    溢价 <b>{dec.suggestedPremium > 0 ? '+' : ''}{dec.suggestedPremium.toFixed(1)}%</b>
                                  </span>
                                )}
                                <span style={{ fontSize: 13 }}>
                                  数量: <b>{dec.suggestedQty > 0 ? dec.suggestedQty.toLocaleString() : dec.finalQty > 0 ? dec.finalQty : '—'} 股</b>
                                </span>
                                <span style={{ fontSize: 13 }}>
                                  金额: <b style={{ color: '#1a1a2e' }}>¥{dec.finalAmount > 0 ? dec.finalAmount.toLocaleString() : '—'}</b>
                                </span>
                              </div>

                              {/* 置信度 */}
                              <span style={{
                                fontSize: 11, fontWeight: 600, marginLeft: 'auto',
                                color: dec.confidence >= 60 ? '#00B42A' : dec.confidence >= 30 ? '#F7BA1E' : '#F53F3F'
                              }}>
                                AI {dec.confidence.toFixed(0)}%
                              </span>
                            </div>
                          );
                        })}

                        {/* 风险提示 */}
                        {(buyList.length > 0 || sellList.length > 0) && allocation && (
                          <div style={{
                            marginTop: 10, padding: '8px 12px', background: '#fffbe6', borderRadius: 6,
                            fontSize: 11, color: '#876800', display: 'flex', alignItems: 'center', gap: 6
                          }}>
                            <Zap size={12} color="#F7BA1E" />
                            可用资金 ¥{allocation.currentCash?.toLocaleString() || 0}
                            {totalBuy > 0 && <span> · 计划买入 ¥{totalBuy.toLocaleString()}</span>}
                            {totalSell > 0 && <span> · 计划卖出 ¥{totalSell.toLocaleString()}</span>}
                            {holdList.length > 0 && <span> · {holdList.length} 只建议持有观望</span>}
                          </div>
                        )}
                      </div>
                    </Card>
                  )}

                  {/* ====== 决策分析详情（全展开，无折叠）====== */}
                  <div style={{ fontSize: 14, fontWeight: 700, color: '#1a1a2e', marginBottom: 12, marginTop: 8 }}>
                    决策分析
                  </div>
                  {verifiedDecisions.map((dec, i) => {
                    const sig = filteredSignals.find((s: Signal) => s.id === dec.signalId);
                    const isConfirmed = dec.status === 'confirmed';
                    const isRejected = dec.status === 'rejected';
                    const isBuy = dec.finalAction === 'buy' || dec.finalAction === 'add';
                    return (
                      <Card key={dec.id} style={{
                        borderRadius: 10, marginBottom: 12,
                        border: '1px solid ' + (isConfirmed ? '#d4edda' : isRejected ? '#f8d7da' : '#fff3cd'),
                        background: isConfirmed ? '#fcfff5' : isRejected ? '#fffbfb' : '#fffefa'
                      }} bodyStyle={{ padding: '16px 20px' }}>
                        {/* 标题行 */}
                        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 12, flexWrap: 'wrap' }}>
                          <div style={{
                            width: 28, height: 28, borderRadius: 8,
                            background: isConfirmed ? '#00B42A' : isRejected ? '#F53F3F' : '#F7BA1E',
                            color: '#fff', fontSize: 12, fontWeight: 700,
                            display: 'flex', alignItems: 'center', justifyContent: 'center'
                          }}>{i + 1}</div>
                          <span style={{ fontSize: 15, fontWeight: 700, color: '#1a1a2e' }}>
                            {dec.stockName || sig?.stockName}
                          </span>
                          <span style={{ fontSize: 12, color: '#888' }}>{dec.stockCode}</span>
                          <Tag color={isConfirmed ? 'green' : isRejected ? 'red' : 'orange'} size="small">
                            {isConfirmed ? '确认执行' : isRejected ? '驳回' : '变更'}
                          </Tag>
                          {sig && dec.finalAction !== sig.actionType && (
                            <span style={{ fontSize: 10, color: '#F53F3F', background: '#fff0f0', padding: '1px 6px', borderRadius: 3 }}>
                              原始: {sig.actionType} → AI: {dec.finalAction}
                            </span>
                          )}
                          <Tag size="small" color={isBuy ? 'green' : 'red'}>{dec.finalAction}</Tag>
                          <span style={{
                            fontSize: 12, fontWeight: 600,
                            color: dec.confidence >= 60 ? '#00B42A' : dec.confidence >= 30 ? '#F7BA1E' : '#F53F3F'
                          }}>AI 置信度 {dec.confidence.toFixed(0)}%</span>
                          {dec.decisionRule && (
                            <Tag size="small" color="arcoblue" style={{ fontSize: 9 }}>{dec.decisionRule}</Tag>
                          )}
                          {dec.taDebateJson && (
                            <Button size="mini" type="text" style={{ fontSize: 10, color: '#722ED1', marginLeft: 4 }}
                              onClick={() => openDebate(dec)}>
                              查看完整对话
                            </Button>
                          )}
                        </div>

                        {/* 执行参数 */}
                        <div style={{
                          display: 'flex', gap: 20, flexWrap: 'wrap', marginBottom: 12,
                          padding: '10px 14px', background: '#fff', borderRadius: 8,
                          border: '1px solid var(--color-border-1)', fontSize: 12
                        }}>
                          {dec.finalPrice > 0 && (
                            <span>计划价 <b style={{ color: '#1a1a2e' }}>¥{dec.finalPrice.toFixed(2)}</b></span>
                          )}
                          {dec.orderPrice > 0 && dec.orderPrice !== dec.finalPrice && (
                            <span>挂单价 <b style={{ color: '#165DFF' }}>¥{dec.orderPrice.toFixed(2)}</b></span>
                          )}
                          {dec.suggestedPremium !== 0 && (
                            <span>溢价 <b style={{ color: dec.suggestedPremium > 0 ? '#F53F3F' : '#00B42A' }}>
                              {dec.suggestedPremium > 0 ? '+' : ''}{dec.suggestedPremium.toFixed(1)}%
                            </b></span>
                          )}
                          <span>数量 <b>{dec.suggestedQty > 0 ? dec.suggestedQty.toLocaleString() : dec.finalQty > 0 ? dec.finalQty : '—'} 股</b></span>
                          {dec.finalAmount > 0 && (
                            <span>金额 <b style={{ color: '#1a1a2e' }}>¥{dec.finalAmount.toLocaleString()}</b></span>
                          )}
                          {dec.openPrice > 0 && (
                            <span>开盘 <b>¥{dec.openPrice.toFixed(2)}</b>
                              <span style={{ color: dec.openDeviation >= 0 ? '#F53F3F' : '#00B42A', marginLeft: 4 }}>
                                ({dec.openDeviation >= 0 ? '+' : ''}{dec.openDeviation.toFixed(2)}%)
                              </span>
                            </span>
                          )}
                        </div>

                        {/* 决策理由（始终展开） */}
                        {dec.reason && (
                          <div style={{
                            fontSize: 12, color: '#444', lineHeight: 1.8,
                            padding: '10px 14px', background: 'rgba(0,0,0,0.02)', borderRadius: 6, marginBottom: 8
                          }}>
                            <div style={{ fontWeight: 700, color: '#666', fontSize: 11, marginBottom: 4, display: 'flex', alignItems: 'center', gap: 4 }}>
                              <span style={{ width: 3, height: 12, borderRadius: 2, background: '#165DFF', display: 'inline-block' }} />
                              决策理由
                            </div>
                            {dec.reason}
                          </div>
                        )}

                        {/* AI 推理过程（始终展开） */}
                        {dec.taReasoning && (
                          <div style={{
                            fontSize: 11, color: '#555', lineHeight: 1.8,
                            padding: '10px 14px', background: 'rgba(114,46,209,0.03)', borderRadius: 6,
                            borderLeft: '3px solid #d4c5f0'
                          }}>
                            <div style={{ fontWeight: 700, color: '#722ED1', fontSize: 11, marginBottom: 4, display: 'flex', alignItems: 'center', gap: 4 }}>
                              <span style={{ width: 3, height: 12, borderRadius: 2, background: '#722ED1', display: 'inline-block' }} />
                              AI 推理过程
                            </div>
                            {dec.taReasoning}
                          </div>
                        )}
                      </Card>
                    );
                  })}
            {/* 报告摘要 — 结构化展示，不上 Markdown */}
            {taskResult && (taskResult.total > 0 || taskResult.report) && (
              <Card style={{ borderRadius: 12, marginTop: 16, borderColor: 'var(--color-border-2)' }} bodyStyle={{ padding: '20px 24px' }}
                title={
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <FileText size={16} style={{ color: 'var(--color-text-2)' }} />
                    <span style={{ fontSize: 14, fontWeight: 600 }}>报告摘要</span>
                    <span style={{ fontSize: 11, color: 'var(--color-text-3)', marginLeft: 4 }}>{signalDate || tradeDate}</span>
                  </div>
                }
                extra={
                  <Button size="mini" loading={sendingNotify} onClick={handleSendNotify}
                    style={{ fontSize: 11, borderRadius: 6 }}>
                    <Bell size={11} style={{ marginRight: 3 }} />发送通知
                  </Button>
                }>
                {/* 统计概览 */}
                <div style={{ display: 'flex', gap: 16, marginBottom: 16, flexWrap: 'wrap' }}>
                  <StatCard label="信号总数" value={taskResult.total || signals.length} color="#165DFF" />
                  <StatCard label="AI 确认" value={taskResult.confirmed || 0} color="#00B42A" />
                  <StatCard label="AI 驳回" value={taskResult.rejected || 0} color="#F53F3F" />
                  <StatCard label="AI 修正" value={taskResult.modified || 0} color="#F7BA1E" />
                </div>

                {/* 信号+决策合并表 */}
                {filteredSignals.length > 0 && (
                  <>
                    <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)', marginBottom: 8 }}>信号与决策明细</div>
                    <Table data={filteredSignals} rowKey="id" size="small" pagination={false}
                      columns={[
                        { title: '股票', width: 110, render: (_: any, s: Signal) => <span><b style={{ fontSize: 11 }}>{s.stockName}</b><span style={{ fontSize: 10, color: '#888', marginLeft: 4 }}>{s.stockCode}</span></span> },
                        { title: '信号', width: 55, render: (_: any, s: Signal) => {
                          const labels: Record<string,string> = {buy:'买入',add:'加仓',sell:'卖出',reduce:'减仓',stop:'止损'};
                          const colors: Record<string,string> = {buy:'green',add:'green',sell:'red',reduce:'red',stop:'red'};
                          return <Tag size="small" color={colors[s.actionType]||'gray'}>{labels[s.actionType]||s.actionType}</Tag>;
                        }},
                        { title: '触发条件', width: 160, ellipsis: true, render: (_: any, s: Signal) => <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>{s.reason || '—'}</span> },
                        { title: 'AI 决策', width: 65, render: (_: any, s: Signal) => {
                          const dec = decisions.find((d: Decision) => d.signalId === s.id);
                          if (!dec) return <Tag size="small" color="gray">待验证</Tag>;
                          if (dec.status === 'confirmed') return <Tag size="small" color="green">确认</Tag>;
                          if (dec.status === 'rejected') return <Tag size="small" color="red">驳回</Tag>;
                          return <Tag size="small" color="orange">修正</Tag>;
                        }},
                        { title: '信心', width: 45, render: (_: any, s: Signal) => {
                          const dec = decisions.find((d: Decision) => d.signalId === s.id);
                          if (!dec) return <span style={{ fontSize: 10 }}>—</span>;
                          return <span style={{ fontWeight: 600, fontSize: 10, color: dec.confidence >= 60 ? '#00B42A' : dec.confidence >= 30 ? '#F7BA1E' : '#F53F3F' }}>{dec.confidence.toFixed(0)}%</span>;
                        }},
                        { title: '金额', width: 65, render: (_: any, s: Signal) => <span style={{ fontSize: 10 }}>¥{s.plannedAmount.toLocaleString()}</span> },
                      ]}
                    />
                  </>
                )}

                {/* 空状态 */}
                {filteredSignals.length === 0 && (
                  <div style={{ padding: 20, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 12, background: 'var(--color-fill-1)', borderRadius: 8 }}>
                    当日无交易信号
                  </div>
                )}
              </Card>
            )}

                  {/* ====== 驳回详情 ====== */}
                  {rejected.length > 0 && (
                    <Card style={{
                      borderRadius: 10, marginTop: 12,
                      border: '1px solid #f8d7da', background: '#fffbfb'
                    }} bodyStyle={{ padding: '14px 20px' }}>
                      <div style={{ fontSize: 13, fontWeight: 700, color: '#F53F3F', marginBottom: 8, display: 'flex', alignItems: 'center', gap: 6 }}>
                        <XCircle size={14} />
                        已驳回信号 ({rejected.length})
                      </div>
                      {rejected.map((dec, i) => (
                        <div key={dec.id} style={{
                          fontSize: 12, color: '#666', padding: '6px 0',
                          borderBottom: i < rejected.length - 1 ? '1px solid #fde8e8' : 'none'
                        }}>
                          <b>{dec.stockName}</b> ({dec.stockCode}) — {dec.reason || '无理由'}
                        </div>
                      ))}
                    </Card>
                  )}
                </div>
              );
            })()}

            </div>

          </Tabs.TabPane>

          {/* 最近交易 */}
          <Tabs.TabPane key="trades" title={`最近交易 (${trades.length})`}>            {trades.length === 0 ? (
              <div style={{ padding: 20, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 12, background: 'var(--color-fill-1)', borderRadius: 8 }}>
                暂无交易记录
              </div>
            ) : (
              <Table data={trades.slice(0, 30)} rowKey="id" size="small" pagination={false}
                columns={[
                  { title: '日期', dataIndex: 'tradeDate', width: 90 },
                  { title: '代码', dataIndex: 'stockCode', width: 75 },
                  { title: '名称', dataIndex: 'stockName', width: 85 },
                  { title: '操作', dataIndex: 'actionType', width: 50, render: (v: string) => <Tag size="small" color={v === 'buy' || v === 'add' ? 'green' : 'red'}>{v}</Tag> },
                  { title: '价格', dataIndex: 'price', width: 70, render: (v: number) => `¥${v.toFixed(2)}` },
                  { title: '数量', dataIndex: 'quantity', width: 50 },
                  { title: '金额', dataIndex: 'amount', width: 80, render: (v: number) => `¥${v.toLocaleString()}` },
                  { title: '盈亏', dataIndex: 'pnl', width: 80, render: (v: number) => v ? <span style={{ color: v >= 0 ? '#00B42A' : '#F53F3F', fontWeight: 600 }}>{pnlSign(v)}¥{Math.abs(v).toFixed(0)}</span> : '-' },
                ]}
              />
            )}
          </Tabs.TabPane>

          {/* 运行配置 */}
          <Tabs.TabPane key="config" title="运行配置">
            <div style={{ maxWidth: 600, display: 'flex', flexDirection: 'column', gap: 16, padding: '8px 0' }}>
              {/* 自动调度 */}
              <div>
                <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)', marginBottom: 10 }}>
                  <Activity size={14} style={{ marginRight: 6, verticalAlign: -2 }} />自动调度
                </div>
                <div style={{ fontSize: 11, color: 'var(--color-text-4)', marginBottom: 10 }}>设定每日执行时间，系统自动跳过非交易日</div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <span style={{ fontSize: 12, color: 'var(--color-text-2)', minWidth: 100 }}>盘后信号生成</span>
                    <TimePicker value={configAutoDaily || undefined} format="HH:mm" placeholder="选择时间"
                      style={{ width: 140 }} onChange={(s) => setConfigAutoDaily(s)} allowClear />
                    {configAutoDaily && <span style={{ fontSize: 11, color: 'var(--color-text-4)' }}>每日 {configAutoDaily}</span>}
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <span style={{ fontSize: 12, color: 'var(--color-text-2)', minWidth: 100 }}>交易执行</span>
                    <TimePicker value={configAutoTradeExec || undefined} format="HH:mm" placeholder="选择时间"
                      style={{ width: 140 }} onChange={(s) => setConfigAutoTradeExec(s)} allowClear />
                    {configAutoTradeExec && <span style={{ fontSize: 11, color: 'var(--color-text-4)' }}>每日 {configAutoTradeExec}</span>}
                  </div>
                </div>
              </div>

              <Divider style={{ margin: 0 }} />

              {/* 执行模式 */}
              <div>
                <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)', marginBottom: 10 }}>
                  <Zap size={14} style={{ marginRight: 6, verticalAlign: -2 }} />交易执行
                </div>
                <div style={{ fontSize: 11, color: 'var(--color-text-4)', marginBottom: 12 }}>
                  选择交易执行后的交易执行方式。自动下单需账户支持自动交易功能。
                </div>
                <div style={{ display: 'flex', gap: 12 }}>
                  <div
                    onClick={() => setConfigExecutionMode('manual')}
                    style={{
                      flex: 1, padding: '14px 16px', borderRadius: 8, cursor: 'pointer',
                      border: configExecutionMode === 'manual' ? '2px solid #165DFF' : '1px solid var(--color-border-2)',
                      background: configExecutionMode === 'manual' ? '#165DFF08' : 'var(--color-bg-1)',
                    }}
                  >
                    <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4 }}>✋ 手动下单</div>
                    <div style={{ fontSize: 11, color: 'var(--color-text-4)', lineHeight: 1.5 }}>
                      交易执行后生成待执行信号，需手动在信号页逐一确认交易
                    </div>
                  </div>
                  <div
                    onClick={() => setConfigExecutionMode('auto')}
                    style={{
                      flex: 1, padding: '14px 16px', borderRadius: 8, cursor: 'pointer',
                      border: configExecutionMode === 'auto' ? '2px solid #165DFF' : '1px solid var(--color-border-2)',
                      background: configExecutionMode === 'auto' ? '#165DFF08' : 'var(--color-bg-1)',
                    }}
                  >
                    <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4 }}>🤖 自动下单</div>
                    <div style={{ fontSize: 11, color: 'var(--color-text-4)', lineHeight: 1.5 }}>
                      交易执行后自动提交订单（需账户开启自动交易功能）
                    </div>
                  </div>
                </div>
                {configExecutionMode === 'auto' && (
                  <div style={{ marginTop: 10, padding: '8px 12px', background: '#FF7D0008', borderRadius: 6, border: '1px solid #FF7D0015' }}>
                    <span style={{ fontSize: 10, color: '#FF7D00' }}>
                      ⚡ 自动交易当前仅支持已配置<b>妙想（mx-moni）</b>的模拟资金账户。其他账户需龙虾代理验证后方可开启。
                      请在「交易账户」页面确认账户支持自动交易。
                    </span>
                  </div>
                )}
              </div>

              <Divider style={{ margin: 0 }} />

              {/* AI 审查 */}
              <div style={{ background: 'var(--color-fill-1)', borderRadius: 8, padding: '10px 12px' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 4 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <Cpu size={14} style={{ color: 'var(--color-text-3)' }} />
                    <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>AI 审查</span>
                  </div>
                  <Switch checked={configAiReviewEnabled} onChange={setConfigAiReviewEnabled} />
                </div>
                <div style={{ fontSize: 10, color: 'var(--color-text-4)' }}>开启后交易执行前由 AI 多智能体严格审查信号，可能否决高风险交易</div>
              </div>

              <Divider style={{ margin: 0 }} />

              {/* 通知设置 */}
              <div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}>
                  <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>
                    <Bell size={14} style={{ marginRight: 6, verticalAlign: -2 }} />通知推送
                  </span>
                  <Switch checked={configNotifyEnabled} onChange={setConfigNotifyEnabled} />
                </div>
                {configNotifyEnabled && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                    {configChannels.length === 0 && (
                      <div style={{ fontSize: 11, color: 'var(--color-text-4)', padding: '4px 0' }}>尚未添加通知渠道</div>
                    )}
                    {configChannels.map((nc, idx) => (
                      <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, background: 'var(--color-fill-1)', borderRadius: 6, padding: '6px 10px' }}>
                        <span style={{ fontSize: 11, color: 'var(--color-text-2)', minWidth: 56, fontWeight: 500 }}>
                          {nc.channel === 'dingtalk_bot' ? '🔷 钉钉' : nc.channel === 'feishu_bot' ? '🟢 飞书' : '🟢 企微'}
                        </span>
                        <span style={{ fontSize: 11, color: 'var(--color-text-3)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{nc.webhookUrl}</span>
                        <Button size="mini" type="text" status="danger" onClick={() => { if (nc.id) setRemovedNotifyIds(prev => [...prev, nc.id]); setConfigChannels(prev => prev.filter((_, i) => i !== idx)); }}>移除</Button> {nc.id && <Button size="mini" type="text" onClick={async () => { try { await testNotification(nc.id!); Message.success("测试消息已发送"); } catch(e) { Message.error("发送失败: " + ((e as any)?.response?.data?.message || String(e))); } }} style={{ padding: "0 4px", fontSize: 11 }}>测试</Button>}
                      </div>
                    ))}
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                      <div style={{ display: 'flex', gap: 8 }}>
                        <Select value={configNewChannel.channel} onChange={v => setConfigNewChannel(prev => ({ ...prev, channel: v as string }))}
                          style={{ width: 110 }}
                          options={[
                            { label: '钉钉机器人', value: 'dingtalk_bot' },
                            { label: '飞书机器人', value: 'feishu_bot' },
                            { label: '企微机器人', value: 'wecom_bot' },
                          ]} />
                        <Input value={configNewChannel.webhookUrl} placeholder="Webhook URL"
                          onChange={v => setConfigNewChannel(prev => ({ ...prev, webhookUrl: v }))} style={{ flex: 1 }} />
                        <Button size="small" type="primary" onClick={() => {
                          if (!configNewChannel.webhookUrl) { Message.warning('请输入 Webhook 地址'); return; }
                          const name = configNewChannel.channel === 'dingtalk_bot' ? '钉钉通知' : configNewChannel.channel === 'feishu_bot' ? '飞书通知' : '企业微信通知';
                          setConfigChannels(prev => [...prev, { channel: configNewChannel.channel, name, webhookUrl: configNewChannel.webhookUrl, keyword: configNewChannel.keyword, secret: configNewChannel.secret }]);
                          setConfigNewChannel({ channel: 'dingtalk_bot', webhookUrl: '', keyword: '', secret: '' });
                        }}>添加</Button>
                      </div>
                      {/* Security settings */}
                      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                        <span style={{ fontSize: 11, color: 'var(--color-text-3)', minWidth: 56 }}>安全验证</span>
                        <Input value={configNewChannel.keyword} placeholder="自定义关键词 (推荐)"
                          onChange={v => setConfigNewChannel(prev => ({ ...prev, keyword: v }))}
                          style={{ width: 160 }} size="small"
                          addAfter={
                            <span style={{ cursor: 'pointer', fontSize: 11, color: '#165DFF' }}
                              onClick={() => setConfigNewChannel(prev => ({ ...prev, keyword: '智策投研' }))}>
                              生成
                            </span>
                          } />
                        <Input value={configNewChannel.secret} placeholder="签名密钥 (可选)"
                          onChange={v => setConfigNewChannel(prev => ({ ...prev, secret: v }))}
                          style={{ width: 160 }} size="small" />
                      </div>
                      <div style={{ fontSize: 10, color: 'var(--color-text-4)', lineHeight: 1.5 }}>
                        💡 推荐使用<b>自定义关键词</b>：在钉钉/飞书机器人安全设置中添加相同关键词，消息将自动包含。点击「生成」填入默认关键词。
                      </div>
                    </div>
                  </div>
                )}
              </div>

              <Divider style={{ margin: 0 }} />

              {/* Save button */}
              <Button type="primary" onClick={handleSaveConfig} loading={configSaving} style={{ alignSelf: 'flex-start' }}>
                保存配置
              </Button>
            </div>
          </Tabs.TabPane>
        </Tabs>
      </Card>


      {/* Edit Signal Modal */}
      <Modal
        visible={editSignalModal.open}
        title="编辑交易信号"
        onCancel={() => setEditSignalModal({ open: false, signal: null })}
        onOk={handleSaveEditSignal}
        confirmLoading={savingSignal}
        okText="保存"
      >
        {editSignalModal.signal && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12, padding: '8px 0' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '8px 12px', background: 'var(--color-fill-1)', borderRadius: 8 }}>
              <Tag size="medium" color={editSignalModal.signal.actionType === 'buy' || editSignalModal.signal.actionType === 'add' ? 'green' : 'red'}>
                {editSignalModal.signal.actionType}
              </Tag>
              <span style={{ fontWeight: 600 }}>{editSignalModal.signal.stockName}({editSignalModal.signal.stockCode})</span>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
              <div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>操作类型（不可变更）</div>
                <Tag size="medium" color={editSignalModal.signal.actionType === 'buy' || editSignalModal.signal.actionType === 'add' ? 'green' : 'red'}>
                  {editSignalModal.signal.actionType === 'buy' ? '买入' : editSignalModal.signal.actionType === 'add' ? '加仓' : editSignalModal.signal.actionType === 'sell' ? '卖出' : editSignalModal.signal.actionType === 'reduce' ? '减仓' : editSignalModal.signal.actionType === 'stop' ? '止损' : editSignalModal.signal.actionType === 'hold' ? '持有' : editSignalModal.signal.actionType}
                </Tag>
              </div>
              <div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>计划价格</div>
                <Input value={editSignalForm.plannedPrice} onChange={v => setEditSignalForm(f => ({ ...f, plannedPrice: v }))}
                  placeholder={String(editSignalModal.signal.plannedPrice)} />
              </div>
              <div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>计划数量(股)</div>
                <Input value={editSignalForm.plannedQty} onChange={v => setEditSignalForm(f => ({ ...f, plannedQty: v }))}
                  placeholder={String(editSignalModal.signal.plannedQty)} />
              </div>
              <div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>计划金额</div>
                <div style={{ padding: '6px 10px', fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>
                  ¥{((parseFloat(editSignalForm.plannedPrice) || editSignalModal.signal.plannedPrice) * (parseInt(editSignalForm.plannedQty) || editSignalModal.signal.plannedQty)).toLocaleString()}
                </div>
              </div>
            </div>
            <div>
              <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>原因/备注</div>
              <Input.TextArea value={editSignalForm.reason} onChange={v => setEditSignalForm(f => ({ ...f, reason: v }))}
                placeholder={editSignalModal.signal.reason || '修改原因...'} rows={3} />
            </div>
          </div>
        )}
      </Modal>

      {/* Execute Signal Modal */}
            {/* Execute Signal Modal */}
      <Modal
        visible={executeModal.open}
        title="确认交易执行"
        onCancel={() => setExecuteModal({ open: false, signal: null })}
        footer={
          <div style={{ display: 'flex', justifyContent: 'space-between', width: '100%' }}>
            <Button type="default" status="danger" onClick={handleAbandonSignal}>放弃信号</Button>
            <Button type="primary" onClick={handleConfirmExecute}>确认执行</Button>
          </div>
        }
      >
        {executeModal.signal && (
          <div style={{ fontSize: 13 }}>
            <div style={{ display: 'flex', gap: 12, marginBottom: 16, padding: '10px 14px', background: 'var(--color-fill-1)', borderRadius: 8 }}>
              <Tag size="medium" color={executeModal.signal.actionType === 'buy' || executeModal.signal.actionType === 'add' ? 'green' : 'red'}>
                {executeModal.signal.actionType}
              </Tag>
              <span style={{ fontWeight: 600 }}>{executeModal.signal.stockName}({executeModal.signal.stockCode})</span>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, marginBottom: 16 }}>
              <div>
                <div style={{ color: 'var(--color-text-3)', fontSize: 11, marginBottom: 4 }}>计划价格</div>
                <div style={{ fontWeight: 600 }}>¥{executeModal.signal.plannedPrice?.toFixed(2) || '-'}</div>
              </div>
              <div>
                <div style={{ color: 'var(--color-text-3)', fontSize: 11, marginBottom: 4 }}>计划数量</div>
                <div style={{ fontWeight: 600 }}>{executeModal.signal.plannedQty || '-'} 股</div>
              </div>
            </div>
            <div style={{ borderTop: '1px solid var(--color-border-1)', paddingTop: 12 }}>
              <div style={{ color: 'var(--color-text-2)', fontSize: 12, fontWeight: 600, marginBottom: 8 }}>实际成交信息</div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                <div>
                  <div style={{ color: 'var(--color-text-3)', fontSize: 11, marginBottom: 4 }}>成交价格</div>
                  <input type="number" step="0.01"
                    style={{ width: '100%', padding: '6px 10px', borderRadius: 6, border: '1px solid var(--color-border-2)', fontSize: 13 }}
                    value={execForm.actualPrice} onChange={e => setExecForm(f => ({ ...f, actualPrice: e.target.value }))} />
                </div>
                <div>
                  <div style={{ color: 'var(--color-text-3)', fontSize: 11, marginBottom: 4 }}>成交数量(股)</div>
                  <input type="number" step="100"
                    style={{ width: '100%', padding: '6px 10px', borderRadius: 6, border: '1px solid var(--color-border-2)', fontSize: 13 }}
                    value={execForm.actualQty} onChange={e => setExecForm(f => ({ ...f, actualQty: e.target.value }))} />
                </div>
              </div>
            </div>
            {executeModal.signal.reason && (
              <div style={{ marginTop: 12, padding: '8px 10px', background: 'var(--color-fill-1)', borderRadius: 6, fontSize: 11, color: 'var(--color-text-2)' }}>{executeModal.signal.reason}</div>
            )}
          </div>
        )}
      </Modal>

      {/* Debate Viewer Drawer */}
      <Drawer
    visible={debateViewer.open}
    onCancel={() => setDebateViewer({ open: false, title: '', content: [] })}
    title={debateViewer.title}
    width={720}
    footer={null}
  >
    <div style={{ fontFamily: "'SF Mono', 'Inter', monospace", fontSize: 12, lineHeight: 1.9, color: '#333' }}>
      {debateViewer.content.map((entry: any, i: number) => {
        const roleLabels: Record<string, { label: string; color: string; bg: string }> = {
          bull_researcher: { label: '🐂 牛方研究员', color: '#00B42A', bg: '#f0fff4' },
          bear_researcher: { label: '🐻 熊方研究员', color: '#F53F3F', bg: '#fff0f0' },
          research_manager: { label: '⚖ 研究经理裁决', color: '#722ED1', bg: '#f9f0ff' },
          market_analyst: { label: '📊 市场分析师', color: '#165DFF', bg: '#f0f5ff' },
          sentiment_analyst: { label: '💬 情绪分析师', color: '#F7BA1E', bg: '#fffbe6' },
          news_analyst: { label: '📰 新闻分析师', color: '#0FC6C2', bg: '#f0fcfc' },
          fundamentals_analyst: { label: '📈 基本面分析师', color: '#F53F3F', bg: '#fff5f5' },
        };
        const rl = roleLabels[entry.role] || { label: entry.role, color: '#666', bg: '#f5f5f5' };
        return (
          <div key={i} style={{ marginBottom: 16, background: rl.bg, borderRadius: 10, overflow: 'hidden', border: '1px solid ' + rl.color + '20' }}>
            <div style={{ padding: '8px 14px', background: rl.color + '15', fontWeight: 700, fontSize: 13, color: rl.color, borderBottom: '1px solid ' + rl.color + '20' }}>
              {rl.label}
            </div>
            <div style={{ padding: '12px 14px', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
              {entry.content}
            </div>
          </div>
        );
      })}
    </div>
  </Drawer>

      {/* Strategy execution confirm modal */}
      <Modal
        visible={runDailyMode !== null}
        title="确认执行策略"
        okText="确认执行"
        cancelText="取消"
        onOk={() => {
          const mode = runDailyMode!;
          setRunDailyMode(null);
          handleRunDaily(mode);
        }}
        onCancel={() => setRunDailyMode(null)}
        okButtonProps={{ status: 'warning' }}
      >
        <div style={{ lineHeight: 1.8, fontSize: 13, padding: '8px 0' }}>
          <p style={{ color: 'var(--color-text-1)', marginBottom: 12 }}>
            即将执行 <b style={{ color: '#F53F3F' }}>{runDailyMode === 'after_close' ? '盘后策略执行' : runDailyMode === 'trade_exec' ? '信号刷新' : '盘中刷新'}</b>
          </p>
          <div style={{ background: '#FF7D0008', borderRadius: 6, padding: '10px 12px', border: '1px solid #FF7D0015' }}>
            <p style={{ color: '#FF7D00', fontSize: 12, margin: 0 }}>
              ⚠️ <b>注意</b>：执行策略将刷新当前日期的<b>待执行信号</b>（重新扫描生成）。
              已有委托中的信号（委托中/部分成交）<b>不会被清除或覆盖</b>，
              同一股票已有委托时<b>不会重复生成信号</b>。
            </p>
          </div>
        </div>
      </Modal>

      {/* Clear signals confirm modal */}
      <Modal
        title="清空信号"
        visible={clearModalOpen}
        onOk={doClearSignals}
        onCancel={() => setClearModalOpen(false)}
        okText="确认清空"
        okButtonProps={{ status: 'danger' }}
      >
        <div style={{ padding: '8px 0' }}>
          确认清空 <b>{signalDate}</b> 全部 <b style={{ color: '#F53F3F' }}>{clearPendingCount}</b> 条待执行信号？
        </div>
      </Modal>
    </div>
  );
}
