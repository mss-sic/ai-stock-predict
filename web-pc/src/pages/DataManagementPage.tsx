import { showToast } from '../components/Toast';
import { useState, useEffect, useRef, useCallback } from 'react';
import { Upload, Button, Table, Tag, Progress, Modal, Tooltip, Switch, Popconfirm, Message, Statistic, Badge, Descriptions } from '@arco-design/web-react';
import {
  Database, Upload as UploadIcon, RefreshCw, FileSpreadsheet, FileJson,
  CheckCircle, XCircle, Clock, Play, Terminal, Square, History, Activity, X,
  BarChart3, TrendingUp, Newspaper, FileText, PieChart, Users, Banknote,
  Timer, Bot, Sparkles, TrendingDown, AlertTriangle, Gift, Zap, Globe, Shield, Layers, ListOrdered
} from 'lucide-react';
import {
  uploadExcel, uploadKline, uploadPrediction, uploadProfile, triggerCollection, fetchCollectorProgress,
  fetchImportHistory, fetchCollectorHistory, clearCollectorHistory, fetchDataStats, fetchDataDetail,
  fetchScheduledTasks, runTaskNow, initDefaultTasks, fetchTaskLogs, resetTaskStatus, toggleTask,
  fetchSchedulerDefinitions, fetchSchedulerTasks, fetchSchedulerHealth,
  fetchSchedulerHistory, fetchSchedulerQueues, fetchSchedulerAlerts, clearSchedulerAlerts,
  triggerSchedulerTask,
} from '../services/api';
import TaskLogPage from './TaskLogPage';

const PHASE_LABELS: Record<string, string> = {
  full_sync: '股票列表同步', kline: '日K线数据', tushare_kline: '日K采集-Tushare', tushare_indicator: '技术指标-Tushare',
  industry: '行业分类', quote: '实时行情', shareholder: '股东数据',
  financial: '财务数据', news: '资讯数据', reports: '研报数据', concept: '概念板块',
  backfill_financial: '财报全量回填', backfill_shareholder: '股东全量回填',
  profile: 'AI简介+评分',
  score: 'AI六维评分',
  dragon_tiger: '龙虎榜',
  margin: '融资融券',
  block_trade: '大宗交易',
  unlock: '限售解禁',
  ths_hot: '同花顺热点',
  dividend: '分红送转',
  ths_eps: '一致预期',
  cninfo: '巨潮公告',
  macro_news: '宏观资讯',
  market_daily_agg: '市场日聚合',
  market_sentiment: '市场情绪计算',
  market_style: '市场风格计算',
  risk_scan: '风险扫描',
  concept_full: '概念全量重建',
  ai_score: 'AI评分更新',
  fund_flow: '资金流向',
  northbound: '北向资金',
  limit_stats: '涨跌停统计',
};

const PHASE_DESCRIPTIONS: Record<string, string> = {
  full_sync: '同步全市场股票代码、名称、上市日期、所属行业等基础信息',
  kline: '采集每日开/高/低/收价格、成交量、成交额等K线数据（腾讯前复权）',
  tushare_kline: 'Tushare日K采集：原始未复权行情，含昨收/涨跌额/涨跌幅，单位标准化（股/元/%）',
  tushare_indicator: 'Tushare技术指标：PE/PB/PS/股息率/换手率/量比/股本/市值（daily_basic接口）',
  industry: '同步申万行业分类标准，建立行业板块映射',
  quote: '采集实时买卖盘价格、成交量、换手率等盘中行情快照',
  shareholder: '采集股东总人数、人均持股、前十大股东、机构持股比例等筹码数据',
  financial: '采集营收、净利润、ROE、EPS、毛利率、资产负债率等财务数据',
  news: '采集个股公告、新闻资讯、行业动态等信息',
  reports: '采集券商研报、评级调整、盈利预测、目标价等机构观点',
  concept: '采集东方财富概念板块、行业板块分类及成分股关联',
  backfill_financial: '全量回溯历史财报数据，补齐所有报告期的财务指标',
  backfill_shareholder: '全量回溯历史股东数据，补齐所有报告期的股东变化',
  profile: 'AI生成结构化公司简介: 核心特征/主营业务/财报/成长/风险/展望',
  score: 'AI六维度量化评分: 基本面/成长性/估值/资金面/技术面/行业景气',
  fund_flow: '采集个股每日资金流向数据，包含主力/超大单/大单/中单/小单净流入流出',
  dragon_tiger: '采集全市场龙虎榜上榜股票+买卖席位TOP5+机构动向',
  margin: '采集全市场融资余额/融券余额/融资买入/融券卖出等两融数据',
  block_trade: '采集大宗交易成交价/折溢价率/买卖方营业部',
  unlock: '采集限售股解禁日历（历史+未来90天预告）',
  ths_hot: '采集同花顺当日强势股列表+题材归因标签',
  dividend: '采集每股分红/送股/转增历史记录',
  ths_eps: '采集同花顺机构一致预期EPS/PE预测',
  cninfo: '采集巨潮资讯网公告（年报/季报/业绩预告/重大事项）',
  macro_news: '采集东财7×24全球财经快讯（按政策/国际/产业分类）',
  market_daily_agg: '计算全市场每日涨跌比/成交额/MA20站上数等聚合指标',
  market_sentiment: '计算市场情绪指数（11项子指标综合评分）',
  market_style: '计算市场风格分类（大盘/小盘/成长/价值等）及置信度',
  risk_scan: '扫描用户持仓股风险（跌幅/破位/业绩预警等）',
  concept_full: '全量重建概念板块成分股关联（东财slist反向采集）',
  ai_score: 'AI六维度量化评分更新（基本面/成长性/估值/资金面/技术面/行业景气）',
  northbound: '采集沪股通/深股通每日北向资金净流入流出数据（分钟级）',
  limit_stats: '预计算每日涨跌停家数/炸板率/涨跌比等情绪统计指标',
};

const PHASE_COLORS: Record<string, string> = {
  full_sync: '#165dff', kline: '#ff7d00', tushare_kline: '#e8654c', tushare_indicator: '#0fc6c2',
  industry: '#722ed1', quote: '#14c9c9', shareholder: '#f53f3f',
  financial: '#0fc6c2', news: '#f77234', reports: '#e865b7', concept: '#f5319d',
  backfill_financial: '#4080ff', backfill_shareholder: '#ff4080',
  dragon_tiger: '#e8654c', margin: '#f09b38', block_trade: '#00a870', unlock: '#ed7b2f',
  northbound: '#ff5722', limit_stats: '#9c27b0',
  ths_hot: '#165dff', dividend: '#722ed1', ths_eps: '#14c9c9', cninfo: '#f5319d', macro_news: '#ff7d00',
};

// Phases that legitimately take >10 minutes
const LONG_RUNNING_PHASES: Record<string, number> = {
  concept_full: 120, // full rebuild ~30-80 min
  backfill_financial: 60,
  backfill_shareholder: 60,
};

// 支持历史数据修复的 Phase（数据源可查询任意历史日期）
const HISTORY_CAPABLE_PHASES = new Set([
  'kline',              // K线（可查询任意日期区间）
  'reports',            // 研报（可按日期范围查询）
  'market_daily_agg',   // 市场日聚合（支持日期范围/全部历史）
  'market_sentiment',   // 市场情绪（支持日期范围/全部历史）
  'market_style',       // 市场风格（支持全部历史/日期范围重算）
  'limit_stats',        // 涨跌停统计（支持 --repair --from/--to/--all）
  'backfill_financial', // 财报全量回填（无日期概念，始终全量）
  'backfill_shareholder',// 股东全量回填（无日期概念，始终全量）
]);

const RANGE_PRESETS = [
  { label: '最新', desc: '仅最新交易日', args: [] },
  { label: '最近7天', desc: '最近7个交易日', args: ['--last', '7'] },
  { label: '最近30天', desc: '最近30个交易日', args: ['--last', '30'] },
  { label: '最近60天', desc: '最近60个交易日', args: ['--last', '60'] },
  { label: '最近90天', desc: '最近90个交易日', args: ['--last', '90'] },
  { label: '全部', desc: '全量采集所有历史', args: ['--all'] },
];

interface SSELine { type: string; phase?: string; message?: string; level?: string; result?: PhaseResult; stats?: Record<string, number>; progressCurrent?: number; progressTotal?: number; }
interface PhaseResult { phase: string; total: number; new: number; skipped: number; errors: number; durationMs: number; }
interface DataStat { key: string; label: string; count: number; updatedAt?: string; }

function cronToText(expr: string): string {
  if (!expr) return '';
  const parts = expr.trim().split(/\s+/);
  if (parts.length < 6) return expr;
  const [sec, min, hour, day, month, weekday] = parts;
  // Hourly at minute M: 0 M * * * *
  if (sec === '0' && /^\d+$/.test(min) && hour === '*' && day === '*' && month === '*' && weekday === '*') {
    return `每小时第 ${min} 分`;
  }
  // Every N minutes: 0 */N * * * *
  if (sec === '0' && min.startsWith('*/') && hour === '*' && day === '*' && month === '*' && weekday === '*') {
    const n = parseInt(min.slice(2)); if (n === 30) return '每半小时'; if (n === 60) return '每小时'; return `每 ${n} 分钟`;
  }
  // Every N seconds: */N * * * * *
  if (sec.startsWith('*/') && min === '*' && hour === '*' && day === '*' && month === '*' && weekday === '*') {
    const n = sec.slice(2); return `每 ${n} 秒`;
  }
  // Daily at HH:MM: 0 MM HH * * *
  if (sec === '0' && /^\d+$/.test(min) && /^\d+$/.test(hour) && day === '*' && month === '*' && weekday === '*') {
    return `每日 ${hour.padStart(2,'0')}:${min.padStart(2,'0')}`;
  }
  // Weekly: 0 MM HH * * D
  if (sec === '0' && /^\d+$/.test(min) && /^\d+$/.test(hour) && day === '*' && month === '*' && /^\d+$/.test(weekday)) {
    const wdMap = ['日', '一', '二', '三', '四', '五', '六'];
    const wd = wdMap[parseInt(weekday)] || weekday;
    return `每周${wd} ${hour.padStart(2,'0')}:${min.padStart(2,'0')}`;
  }
  // Monthly: 0 MM HH D * *
  if (sec === '0' && /^\d+$/.test(min) && /^\d+$/.test(hour) && /^\d+$/.test(day) && month === '*' && weekday === '*') {
    return `每月${day}日 ${hour.padStart(2,'0')}:${min.padStart(2,'0')}`;
  }
  return expr;
}

export default function DataManagementPage() {
  const [tab, setTab] = useState<'overview' | 'tasks' | 'import' | 'collect' | 'history' | 'logs'>('overview');
  const [loading, setLoading] = useState(false);
  const [predLoading, setPredLoading] = useState(false);
  const [profileLoading, setProfileLoading] = useState(false);
  const [klineLoading, setKlineLoading] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [history, setHistory] = useState<any[]>([]);
  const [colHistory, setColHistory] = useState<any[]>([]);
  const [progress, setProgress] = useState<any>(null);
  const [collecting, setCollecting] = useState(false);
  const [consoleLines, setConsoleLines] = useState<{ text: string; level: string; time: string; phase?: string }[]>([]);
  const [phaseResults, setPhaseResults] = useState<PhaseResult[]>([]);
  const [phaseProgress, setPhaseProgress] = useState({ current: 0, total: 0 });
  const [totalDuration, setTotalDuration] = useState(0);
  const [dataStats, setDataStats] = useState<DataStat[]>([]);
  const [statsLoading, setStatsLoading] = useState(false);
  const [tasksLoading, setTasksLoading] = useState(false);
  const [selectedStat, setSelectedStat] = useState<string | null>(null);
  const [detailData, setDetailData] = useState<any[]>([]);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailSearch, setDetailSearch] = useState('');
  const [scheduledTasks, setScheduledTasks] = useState<any[]>([]);
  // ── v2 Scheduler state ──
  const [schedDefinitions, setSchedDefinitions] = useState<any[]>([]);
  const [schedInstances, setSchedInstances] = useState<any[]>([]);
  const [schedHealth, setSchedHealth] = useState<any>(null);
  const [schedHistory, setSchedHistory] = useState<any[]>([]);
  const [schedQueues, setSchedQueues] = useState<any>(null);
  const [schedAlerts, setSchedAlerts] = useState<any[]>([]);
  const [schedViewTab, setSchedViewTab] = useState<'tasks' | 'health'>('tasks');
  const [taskLogs, setTaskLogs] = useState<any[]>([]);
  const [taskLogsLoading, setTaskLogsLoading] = useState(false);
  const [logModalVisible, setLogModalVisible] = useState(false);
  const [logModalTaskName, setLogModalTaskName] = useState('');
  const [runModalVisible, setRunModalVisible] = useState(false);
  const [runModalTask, setRunModalTask] = useState<any>(null);
  const [selectedRange, setSelectedRange] = useState(0);
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null);
  const [selectedCollectPhase, setSelectedCollectPhase] = useState('kline');
  const [repairModalVisible, setRepairModalVisible] = useState(false);
  const [repairPhase, setRepairPhase] = useState('');
  const [repairDateRange, setRepairDateRange] = useState<string[]>([]);
  const [repairAll, setRepairAll] = useState(false);
  const repairTaskId = scheduledTasks.find((t: any) => t.phase === repairPhase)?.id;

  const handleRepair = async () => {
    if (!repairPhase) return;
    const phase = repairPhase;
    setRepairModalVisible(false);
    
    // Switch to collect console for this phase
    setSelectedCollectPhase(phase);
    setConsoleLines([]);
    setPhaseResults([]);
    setTab('collect');
    addConsoleLine(`🔧 正在触发 ${PHASE_LABELS[phase] || phase} 修复任务...`, 'system');
    
    try {
      const body: any = { all: repairAll };
      if (!repairAll && repairDateRange.length === 2) {
        body.from = repairDateRange[0];
        body.to = repairDateRange[1];
      }
      const resp = await fetch(`/api/v1/admin/scheduled-tasks/${repairTaskId}/repair`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${localStorage.getItem('aip_access_token') || ''}` },
        body: JSON.stringify(body),
      });
      const respData = await resp.json();
      addConsoleLine(`✅ 修复任务已提交: ${PHASE_LABELS[phase] || phase}`, 'success');
      if (respData.message) addConsoleLine(`📋 ${respData.message}`, 'info');
      addConsoleLine('💡 可在「定时任务」页面查看执行日志，或在采集控制台点击「采集」手动运行', 'info');
      showToast('success', `修复任务已触发: ${PHASE_LABELS[phase] || phase}`);
      setTimeout(() => { loadTasks(); loadTaskLogs(repairTaskId); }, 2000);
    } catch (e: any) {
      showToast('error', '修复失败: ' + (e?.message || '未知错误'));
      addConsoleLine(`❌ 修复失败: ${e?.message || '未知错误'}`, 'stderr');
    }
  };

  const pollRef = useRef<any>(null);
  const consoleRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const startTimeRef = useRef<number>(0);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimerRef = useRef<any>(null);
  const collectingRef = useRef(false);
  const MAX_RECONNECT = 10;

  const addConsoleLine = useCallback((text: string, level: string = 'info', phase?: string) => {
    const time = new Date().toLocaleTimeString('zh-CN', { hour12: false });
    setConsoleLines(prev => [...prev.slice(-500), { text, level, time, phase }]);
  }, []);

  useEffect(() => { if (consoleRef.current) consoleRef.current.scrollTop = consoleRef.current.scrollHeight; }, [consoleLines]);

  // ── Load functions ──
  const loadHistory = async () => { try { const res: any = await fetchImportHistory(); setHistory(res.data?.data || []); } catch { setHistory([]); } };
  const loadColHistory = async () => { try { const res: any = await fetchCollectorHistory(); setColHistory(res.data?.data || []); } catch { setColHistory([]); } };
  const handleClearStuck = async (type: string = 'stuck') => {
    try {
      const res: any = await clearCollectorHistory(type);
      showToast('success', res.data?.message || ('已清除 ' + (res.data?.deleted || 0) + ' 条记录'));
      loadColHistory();
    } catch {}
  };
  const loadProgress = async () => { try { const res: any = await fetchCollectorProgress(); setProgress(res.data?.data); } catch {} };
  const loadDataStats = async () => { setStatsLoading(true); try { const res: any = await fetchDataStats(); setDataStats(res.data?.data || []); } catch { setDataStats([]); } finally { setStatsLoading(false); } };
  const loadTasks = async () => { setTasksLoading(true); try { const res: any = await fetchScheduledTasks(); setScheduledTasks(Array.isArray(res.data?.data) ? res.data.data : []); } catch {} finally { setTasksLoading(false); } };
  const loadSchedData = async () => {
    setTasksLoading(true);
    try {
      const [defsRes, tasksRes, healthRes] = await Promise.all([
        fetchSchedulerDefinitions(),
        fetchSchedulerTasks(),
        fetchSchedulerHealth(),
      ]);
      setSchedDefinitions(Array.isArray(defsRes.data) ? defsRes.data : defsRes.data?.data || []);
      setSchedInstances(Array.isArray(tasksRes.data) ? tasksRes.data : tasksRes.data?.data || []);
      setSchedHealth(healthRes.data?.data || healthRes.data || null);
    } catch (e: any) {
      console.error('[scheduler] load failed:', e?.response?.status, e?.message);
      showToast('error', '调度数据加载失败: ' + (e?.message || '请检查后端是否已重启'));
    }
    finally { setTasksLoading(false); }
    // Load history and alerts separately (less critical)
    try {
      const [histRes, queueRes, alertRes] = await Promise.all([
        fetchSchedulerHistory(undefined, 20),
        fetchSchedulerQueues(),
        fetchSchedulerAlerts(),
      ]);
      setSchedHistory(Array.isArray(histRes.data) ? histRes.data : histRes.data?.data || []);
      setSchedQueues(queueRes.data?.data || queueRes.data || null);
      setSchedAlerts(Array.isArray(alertRes.data) ? alertRes.data : alertRes.data?.data || []);
    } catch {}
  };
  const handleSchedTrigger = async (instanceId: number) => {
    try { await triggerSchedulerTask(instanceId); showToast('success', '任务已触发'); setTimeout(loadSchedData, 2000); }
    catch { showToast('error', '触发失败'); }
  };
  const handleClearAlerts = async () => {
    try { await clearSchedulerAlerts(); setSchedAlerts([]); showToast('success', '告警已清除'); }
    catch {}
  };
  const loadDetail = async (type: string) => { if (selectedStat === type) { setSelectedStat(null); setDetailData([]); return; } setSelectedStat(type); setDetailLoading(true); setDetailSearch(''); try { const res: any = await fetchDataDetail(type); setDetailData(res.data?.data || []); } catch { setDetailData([]); } finally { setDetailLoading(false); } };

  const loadTaskLogs = async (taskId?: number) => { setTaskLogsLoading(true); setSelectedTaskId(taskId || null); try { const res: any = await fetchTaskLogs(taskId || undefined, 30); setTaskLogs(Array.isArray(res.data?.data) ? res.data.data : []); } catch { setTaskLogs([]); } finally { setTaskLogsLoading(false); } };

  const openTaskLogs = async (taskId: number, taskName: string) => { setLogModalTaskName(taskName); setSelectedTaskId(taskId); setLogModalVisible(true); setTaskLogsLoading(true); try { const res: any = await fetchTaskLogs(taskId, 30); setTaskLogs(Array.isArray(res.data?.data) ? res.data.data : []); } catch { setTaskLogs([]); } finally { setTaskLogsLoading(false); } };

  const handleRunTask = (id: number) => { const task = scheduledTasks.find((t: any) => t.id === id); if (task?.lastStatus === 'running') { showToast('info', '该任务正在运行中，请等待完成后再执行'); return; } setRunModalTask(task); setSelectedRange(0); setRunModalVisible(true); };
  const handleResetTask = async (id: number) => { try { await resetTaskStatus(id); loadTasks(); } catch {} };
  const confirmRunTask = async () => {
    if (!runModalTask) return;
    const id = runModalTask.id;
    setRunModalVisible(false);
    const preset = RANGE_PRESETS[selectedRange];
    setScheduledTasks((prev: any) => prev.map((t: any) => t.id === id ? { ...t, lastStatus: 'running', lastRun: new Date().toISOString() } : t));
    try {
      await runTaskNow(id, preset.args);
      setTimeout(async () => { await loadTasks(); loadTaskLogs(id); }, 2000);
    } catch (e: any) {
      showToast('error', '执行失败: ' + (e?.response?.data?.message || e?.message || '未知错误'));
      loadTasks();
    }
  };
  const handleInitDefaults = async () => { try { await initDefaultTasks(); loadTasks(); } catch {} };

  // ── SSE ──
  // ── SSE event handler factory ──
  const makeSSEHandler = (es: EventSource) => {
    const handleMessage = (event: MessageEvent) => {
      try {
        const line: SSELine = JSON.parse(event.data);
        if (line.type === 'connected') return;
        if (line.type === 'log' && line.message) {
          addConsoleLine(line.message, line.level || 'info', line.phase);
        } else if (line.type === 'stat') {
          // Stats are accumulated server-side; human-readable behavior logs
          // come through regular 'log' type lines from Python print() statements.
        } else if (line.type === 'progress') {
          setPhaseProgress({ current: line.progressCurrent || 0, total: line.progressTotal || 0 });
          if (line.message) {
            const pct = line.progressTotal ? Math.round((line.progressCurrent || 0) / line.progressTotal * 100) : 0;
            addConsoleLine(`⏳ 进度 ${line.message} (${pct}%)`, 'progress', line.phase);
          }
        } else if (line.type === 'phase' && line.message) {
          setPhaseProgress({ current: 0, total: 0 });
          addConsoleLine(`\n━━━ ${line.message} ━━━`, 'phase', line.phase);
        } else if (line.type === 'result' && line.result) {
          if (line.result.new === 0 && line.result.total > 0 && line.result.errors === 0) {
            addConsoleLine(`💡 ${PHASE_LABELS[line.result.phase] || line.result.phase}: 数据已是最新，共 ${line.result.total} 条无需更新`, 'info', line.phase);
          }
          setPhaseResults(prev => {
            const map = new Map(prev.map(r => [r.phase, r]));
            map.set(line.result!.phase, line.result!);
            const arr = Array.from(map.values());
            // Dedup by phase key
            const seen = new Set<string>();
            return arr.filter((r: any) => {
              if (seen.has(r.phase)) return false;
              seen.add(r.phase);
              return true;
            });
          });
          addConsoleLine(`${line.result.errors > 0 ? '⚠️' : '✅'} ${PHASE_LABELS[line.result.phase] || line.result.phase}: 总计${line.result.total} | 新增${line.result.new} | 耗时${(line.result.durationMs / 1000).toFixed(1)}s`, line.result.errors > 0 ? 'stderr' : 'success', line.phase);
        } else if (line.type === 'done') {
          addConsoleLine(`\n${line.message}`, 'success', line.phase);
          setCollecting(false);
          collectingRef.current = false;
          setTotalDuration(Date.now() - startTimeRef.current);
          es.close();
          eventSourceRef.current = null;
          reconnectAttemptRef.current = 0;
          if (reconnectTimerRef.current) { clearTimeout(reconnectTimerRef.current); reconnectTimerRef.current = null; }
          loadProgress();
          showToast('success', '采集完成');
        }
      } catch {}
    };
    return handleMessage;
  };

  // ── SSE connect with exponential backoff reconnect ──
  const connectStream = () => {
    // Stop any existing poll
    if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null; }
    setCollecting(true);
    collectingRef.current = true;
    reconnectAttemptRef.current = 0;
    const tok = localStorage.getItem('aip_access_token');
    eventSourceRef.current?.close();
    const es = new EventSource(`/api/v1/collector/stream?token=${tok || ''}`);
    eventSourceRef.current = es;
    es.onmessage = makeSSEHandler(es);
    es.onerror = () => {
      if (!collectingRef.current) {
        es.close();
        eventSourceRef.current = null;
        clearInterval(pollRef.current);
        pollRef.current = null;
        return;
      }
      // Reconnect with exponential backoff: 1s, 2s, 4s, 8s... max 60s
      const attempt = reconnectAttemptRef.current;
      if (attempt >= MAX_RECONNECT) {
        addConsoleLine('⚠️ SSE 重连失败已达上限，请手动刷新', 'stderr');
        es.close();
        eventSourceRef.current = null;
        clearInterval(pollRef.current);
        pollRef.current = null;
        return;
      }
      const delay = Math.min(1000 * Math.pow(2, attempt), 60000);
      reconnectAttemptRef.current = attempt + 1;
      addConsoleLine(`🔄 SSE 断开，${(delay / 1000).toFixed(1)}s 后重连 (第${attempt + 1}次)...`, 'warn');
      es.close();
      reconnectTimerRef.current = setTimeout(() => {
        connectStream();
      }, delay);
    };
    // Poll for progress as fallback when SSE is quiet
    pollRef.current = setInterval(async () => {
      try {
        const pr: any = await fetchCollectorProgress();
        const data = pr.data?.data;
        if (!data) return;
        setProgress(data);
        // Sync phaseProgress from server fallback
        if (data.phaseTotal && data.phaseTotal > 0) {
          setPhaseProgress({ current: data.phaseCurrent || 0, total: data.phaseTotal });
        }
        // Sync phaseResults from server state (for reconnect scenarios)
        if (Array.isArray(data.results) && data.results.length > 0) {
          setPhaseResults((prev: any[]) => {
            const map = new Map(prev.map((r: any) => [r.phase, r]));
            data.results.forEach((r: any) => map.set(r.phase, r));
            const arr = Array.from(map.values());
            const seen = new Set<string>();
            return arr.filter((r: any) => { if (seen.has(r.phase)) return false; seen.add(r.phase); return true; });
          });
        }
        // Check running from activePhases too (multi-phase support)
        const hasActive = data.activePhases && Object.values(data.activePhases).some((s: any) => s?.running);
        if (!data.running && !hasActive && collectingRef.current) {
          setCollecting(false);
          setTotalDuration(Date.now() - startTimeRef.current);
          clearInterval(pollRef.current);
          pollRef.current = null;
          eventSourceRef.current?.close();
          loadProgress();
        }
      } catch {}
    }, 3000);
  };

  const handleTrigger = async (phases: string[]) => {
    setLoading(true);
    setConsoleLines([]);
    setPhaseResults([]);
    setPhaseProgress({ current: 0, total: 0 });
    setTotalDuration(0);
    reconnectAttemptRef.current = 0;
    addConsoleLine('🚀 正在启动采集任务...', 'system');
    try {
      await triggerCollection(phases);
      setCollecting(true);
      collectingRef.current = true;
      startTimeRef.current = Date.now();
      const tok = localStorage.getItem('aip_access_token');
      const es = new EventSource(`/api/v1/collector/stream?token=${tok || ''}`);
      eventSourceRef.current = es;
      es.onmessage = makeSSEHandler(es);
      es.onerror = () => {
        if (!collectingRef.current) {
          es.close();
          eventSourceRef.current = null;
          return;
        }
        const attempt = reconnectAttemptRef.current;
        if (attempt >= MAX_RECONNECT) {
          addConsoleLine('⚠️ SSE 重连失败已达上限', 'stderr');
          es.close();
          eventSourceRef.current = null;
          return;
        }
        const delay = Math.min(1000 * Math.pow(2, attempt), 60000);
        reconnectAttemptRef.current = attempt + 1;
        addConsoleLine(`🔄 SSE 断开，${(delay / 1000).toFixed(1)}s 后重连 (第${attempt + 1}次)...`, 'warn');
        es.close();
        reconnectTimerRef.current = setTimeout(() => {
          const tok2 = localStorage.getItem('aip_access_token');
          const es2 = new EventSource(`/api/v1/collector/stream?token=${tok2 || ''}`);
          eventSourceRef.current = es2;
          es2.onmessage = makeSSEHandler(es2);
          es2.onerror = () => {
            if (!collectingRef.current) { es2.close(); eventSourceRef.current = null; }
          };
        }, delay);
      };
      // Poll for progress as fallback
      pollRef.current = setInterval(async () => {
        try {
          const pr: any = await fetchCollectorProgress();
          const data = pr.data?.data;
          setProgress(data);
          // Sync phaseProgress from server fallback (stock-level progress)
          // Always sync as long as we have valid phaseTotal > 0
          if (data?.phaseTotal && data.phaseTotal > 0) {
            setPhaseProgress({ current: data.phaseCurrent || 0, total: data.phaseTotal });
          }
          if (!data?.running && collectingRef.current) {
            setCollecting(false);
            setTotalDuration(Date.now() - startTimeRef.current);
            clearInterval(pollRef.current);
            pollRef.current = null;
            eventSourceRef.current?.close();
            loadProgress();
          }
        } catch {}
      }, 3000);
    } catch (e: any) {
      const msg = e?.message || '触发采集失败';
      showToast('error', msg);
      addConsoleLine(`❌ ${msg}`, 'stderr');
      setCollecting(false);
    }
    setLoading(false);
  };

  const handleStop = () => { eventSourceRef.current?.close(); clearInterval(pollRef.current); setCollecting(false); collectingRef.current = false; addConsoleLine('⏹ 用户停止了监控（采集进程仍在服务端运行）', 'system'); };

  const formatDuration = (ms: number) => { if (ms < 1000) return `${ms}ms`; if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`; return `${Math.floor(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`; };

  const renderConsoleLine = (line: { text: string; level: string; time: string; phase?: string }, i: number) => {
    const cm: Record<string, string> = { info: 'var(--color-border-1)', stderr: '#f76965', success: '#00b42a', phase: '#4080ff', system: '#ffb400' };
    return <div key={i} style={{ fontFamily: '"JetBrains Mono","Fira Code","SF Mono",monospace', fontSize: 12, lineHeight: '18px', color: cm[line.level] || 'var(--color-border-1)', whiteSpace: 'pre-wrap', wordBreak: 'break-all', padding: line.level === 'phase' ? '4px 0' : '0', fontWeight: line.level === 'phase' ? 600 : 400 }}><span style={{ color: '#666', marginRight: 8, userSelect: 'none' }}>{line.time}</span>{line.text}</div>;
  };

  // ── Tab effects ──
  useEffect(() => {
    if (tab === 'history') { loadHistory(); loadColHistory(); }
    if (tab === 'overview') { loadDataStats(); loadProgress(); }
    if (tab === 'tasks') { loadTasks(); loadSchedData(); }
    if (tab === 'import') { loadHistory(); }
    if (tab === 'collect') { loadTasks(); loadProgress(); loadColHistory(); }
    return () => { clearInterval(pollRef.current); eventSourceRef.current?.close(); };
  }, [tab]);

  // ── Auto-reconnect SSE on page load ──
  useEffect(() => {
    (async () => {
      try {
        const pr: any = await fetchCollectorProgress();
        if (pr.data?.data?.running) {
          const d = pr.data.data;
          setProgress(d);
          // Dedup results by phase to prevent double panels
          const rawResults = d.results || [];
          const seen = new Set<string>();
          const deduped = rawResults.filter((r: any) => {
            const key = r.phase;
            if (seen.has(key)) return false;
            seen.add(key);
            return true;
          });
          setPhaseResults(deduped);
          if (d.phaseCurrent !== undefined && d.phaseTotal && d.phaseTotal > 0) {
            setPhaseProgress({ current: d.phaseCurrent, total: d.phaseTotal });
          }
          if (d.started) startTimeRef.current = new Date(d.started).getTime();
          // Show current phase info prominently
          const phaseLabel = PHASE_LABELS[d.phase] || d.phase || '未知';
          const phaseMsg = d.message || '';
          const elapsed = d.started ? formatDuration(Date.now() - new Date(d.started).getTime()) : '';
          addConsoleLine(`🔄 检测到正在运行的采集，自动重连...`, 'system');
          addConsoleLine(`📌 当前阶段: ${phaseLabel} ${phaseMsg ? '— ' + phaseMsg : ''}`, 'phase');
          if (elapsed) addConsoleLine(`⏱ 已运行: ${elapsed}`, 'info');
          if (Array.isArray(deduped) && deduped.length > 0) {
            addConsoleLine(`✅ 已完成 ${deduped.length} 个阶段: ${deduped.map((r: any) => PHASE_LABELS[r.phase] || r.phase).join(', ')}`, 'success');
          }
          connectStream();
        }
      } catch {}
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // ══════════════════════════════════════════
  // RENDER
  // ══════════════════════════════════════════
  return (
    <div>
      <style>{`
        @keyframes collectPulse {
          0%, 100% { opacity: 1; transform: scale(1); }
          50% { opacity: 0.5; transform: scale(1.3); }
        }
        @keyframes collectBgPulse {
          0%, 100% { background-color: var(--color-warning-bg); }
          50% { background-color: var(--color-warning-text); }
        }
      `}</style>
      {/* Repair Modal */}
      <Modal
        title={`修复历史数据 — ${PHASE_LABELS[repairPhase] || repairPhase}`}
        visible={repairModalVisible}
        onOk={handleRepair}
        onCancel={() => setRepairModalVisible(false)}
        okText="开始修复"
        cancelText="取消"
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16, padding: '8px 0' }}>
          <div>
            <div style={{ fontSize: 13, fontWeight: 500, marginBottom: 6, color: 'var(--color-text-1)' }}>时间范围</div>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <input type="date" value={repairDateRange[0] || ''} onChange={e => setRepairDateRange([e.target.value, repairDateRange[1] || ''])}
                style={{ width: 160, padding: '6px 10px', border: '1px solid var(--color-border-2)', borderRadius: 4, fontSize: 13, background: 'var(--color-bg-1)', color: 'var(--color-text-1)' }} />
              <span style={{ color: 'var(--color-text-3)', fontSize: 13 }}>至</span>
              <input type="date" value={repairDateRange[1] || ''} onChange={e => setRepairDateRange([repairDateRange[0] || '', e.target.value])}
                style={{ width: 160, padding: '6px 10px', border: '1px solid var(--color-border-2)', borderRadius: 4, fontSize: 13, background: 'var(--color-bg-1)', color: 'var(--color-text-1)' }} />
            </div>
          </div>
          <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer', fontSize: 13, color: 'var(--color-text-2)' }}>
            <input type="checkbox" checked={repairAll} onChange={e => { setRepairAll(e.target.checked); if (e.target.checked) setRepairDateRange([]); }}
              style={{ width: 16, height: 16, cursor: 'pointer' }} />
            全部历史数据（忽略时间范围）
          </label>
        </div>
      </Modal>
      <div className="page-header">
        <h2><Database size={20} style={{ marginRight: 8 }} />数据管理</h2>
        <span className="muted">数据概览 · 定时任务 · 文件导入 · 采集控制台 · 采集记录</span>
      </div>

      {/* Tab bar */}
      <div style={{ display: 'flex', gap: 0, marginBottom: 16, background: 'var(--color-bg-1)', borderRadius: 6, border: '1px solid var(--color-border-1)', overflow: 'hidden' }}>
        {[
          { key: 'overview', label: '数据概览', icon: <BarChart3 size={14} /> },
          { key: 'tasks', label: '定时任务', icon: <Timer size={14} /> },
          { key: 'import', label: '文件导入', icon: <UploadIcon size={14} /> },
          { key: 'collect', label: '采集控制台', icon: <Terminal size={14} /> },
          { key: 'history', label: '采集记录', icon: <History size={14} /> },
          { key: 'logs', label: '执行历史', icon: <ListOrdered size={14} /> },
        ].map(t => (
          <button key={t.key} onClick={() => setTab(t.key as any)} style={{ padding: '10px 20px', border: 'none', cursor: 'pointer', fontSize: 13, background: tab === t.key ? 'var(--color-info-bg)' : 'transparent', color: tab === t.key ? 'var(--color-primary)' : 'var(--color-text-2)', fontWeight: tab === t.key ? 500 : 400, display: 'flex', alignItems: 'center', gap: 6, borderRight: '1px solid var(--color-border-1)' }}>{t.icon}{t.label}</button>
        ))}
      </div>

      {/* ═══ OVERVIEW TAB ═══ */}
      {tab === 'overview' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="card">
            <div className="card-header">
              <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Database size={16} color="var(--color-primary)" /><span style={{ fontSize: 15, fontWeight: 600 }}>核心数据统计</span></span>
              <Button size="small" type="text" icon={<RefreshCw size={12} />} loading={statsLoading} onClick={loadDataStats}>刷新</Button>
            </div>
            <div className="card-body" style={{ padding: '16px 20px' }}>
              {dataStats.length === 0 && !statsLoading ? (
                <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>暂无统计数据，请先采集数据</div>
              ) : (
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 12 }}>
                  {dataStats.map((stat: DataStat) => {
                    const iconDefs: Record<string, { icon: JSX.Element; color: string }> = {
                      stocks: { icon: <TrendingUp size={18} />, color: '#165dff' }, kline: { icon: <BarChart3 size={18} />, color: '#ff7d00' },
                      financial: { icon: <Banknote size={18} />, color: '#0fc6c2' }, shareholder: { icon: <Users size={18} />, color: '#f53f3f' },
                      news: { icon: <Newspaper size={18} />, color: '#f77234' }, reports: { icon: <FileText size={18} />, color: '#e865b7' },
                      concept: { icon: <PieChart size={18} />, color: '#f5319d' },
                      board_picks: { icon: <FileSpreadsheet size={18} />, color: '#722ed1' }, board_details: { icon: <FileSpreadsheet size={18} />, color: '#722ed1' },
                      signals: { icon: <Activity size={18} />, color: '#ffb400' },
                      concept: { icon: <Layers size={18} />, color: '#f5319d' },
                      dragon_tiger: { icon: <TrendingUp size={18} />, color: '#e8654c' },
                      margin: { icon: <TrendingDown size={18} />, color: '#f09b38' },
                      block_trade: { icon: <Banknote size={18} />, color: '#00a870' },
                      unlock: { icon: <AlertTriangle size={18} />, color: '#ed7b2f' },
                      ths_hot: { icon: <Zap size={18} />, color: '#f5a623' },
                      dividend: { icon: <Gift size={18} />, color: '#722ed1' },
                      ths_eps: { icon: <BarChart3 size={18} />, color: '#14c9c9' },
                      cninfo: { icon: <FileText size={18} />, color: '#4080ff' },
                      macro_news: { icon: <Globe size={18} />, color: '#0fc6c2' },
                      market_daily_agg: { icon: <Activity size={18} />, color: '#165dff' },
                      market_sentiment: { icon: <Shield size={18} />, color: '#b620e0' },
                    };
                    const def = iconDefs[stat.key] || { icon: <Database size={18} />, color: 'var(--color-text-3)' };
                    const isSelected = selectedStat === stat.key;
                    return (
                      <div key={stat.key} onClick={() => loadDetail(stat.key)} style={{ border: isSelected ? `2px solid ${def.color}` : '1px solid var(--color-border-1)', borderRadius: 8, padding: isSelected ? '13px 15px' : '14px 16px', background: isSelected ? `${def.color}08` : 'var(--color-bg-1)', cursor: 'pointer', transition: 'all 0.15s' }}
                        onMouseEnter={e => { if (!isSelected) { (e.currentTarget as HTMLElement).style.borderColor = def.color; (e.currentTarget as HTMLElement).style.boxShadow = '0 2px 8px rgba(0,0,0,0.08)'; } }}
                        onMouseLeave={e => { if (!isSelected) { (e.currentTarget as HTMLElement).style.borderColor = 'var(--color-border-1)'; (e.currentTarget as HTMLElement).style.boxShadow = 'none'; } }}>
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}><span style={{ fontSize: 13, color: 'var(--color-text-2)', fontWeight: 500 }}>{stat.label}</span><span style={{ color: def.color }}>{def.icon}</span></div>
                        <div style={{ fontSize: 28, fontWeight: 700, color: def.color, lineHeight: 1.2 }}>{stat.count.toLocaleString()}</div>
                        {stat.updatedAt && <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 8 }}>最近更新: {new Date(stat.updatedAt).toLocaleDateString('zh-CN')}</div>}
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
          {selectedStat && (
            <div className="card">
              <div className="card-header">
                <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Database size={16} color="var(--color-primary)" /><span style={{ fontSize: 15, fontWeight: 600 }}>{dataStats.find(s => s.key === selectedStat)?.label || selectedStat} 明细</span>
                  <span className="muted" style={{ marginLeft: 8 }}>{detailData.length.toLocaleString()} 条{detailData.filter((d: any) => d.count === 0).length > 0 && `（${detailData.filter((d: any) => d.count === 0).length} 条缺失）`}</span>
                </span>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <input placeholder="搜索代码或名称..." value={detailSearch} onChange={e => setDetailSearch(e.target.value)} style={{ padding: '4px 10px', border: '1px solid var(--color-border-1)', borderRadius: 4, fontSize: 12, width: 160, outline: 'none' }} />
                  <Button size="small" type="text" icon={<RefreshCw size={12} />} loading={detailLoading} onClick={() => loadDetail(selectedStat)}>刷新</Button>
                  <Button size="small" type="text" icon={<X size={12} />} onClick={() => { setSelectedStat(null); setDetailData([]); }}>关闭</Button>
                </div>
              </div>
              <div className="card-body" style={{ padding: 0, maxHeight: 500, overflow: 'auto' }}>
                {detailLoading ? <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>加载中...</div> : (
                  <Table data={detailData.filter((d: any) => { if (!detailSearch) return true; const s = detailSearch.toLowerCase(); return (d.code || '').toLowerCase().includes(s) || (d.name || '').toLowerCase().includes(s); })} rowKey={(r: any) => r.code || r.name || Math.random().toString()} size="small" pagination={{ pageSize: 50, sizeCanChange: true }}
                    columns={[
                      { title: '代码', dataIndex: 'code', width: 90, render: (v: string) => <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{v || '-'}</span> },
                      { title: '名称', dataIndex: 'name', width: 120, ellipsis: true },
                      { title: '记录数', dataIndex: 'count', width: 80, render: (v: number) => <span style={{ fontWeight: 600, color: v === 0 ? '#f53f3f' : v > 0 ? '#00b42a' : 'var(--color-text-3)' }}>{v.toLocaleString()}</span> },
                      { title: '最早', dataIndex: 'firstDate', width: 100, render: (v: string) => v ? <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>{new Date(v).toLocaleDateString('zh-CN')}</span> : <span style={{ color: 'var(--color-text-3)' }}>-</span> },
                      { title: '最晚', dataIndex: 'lastDate', width: 100, render: (v: string) => v ? <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>{new Date(v).toLocaleDateString('zh-CN')}</span> : <span style={{ color: 'var(--color-text-3)' }}>-</span> },
                    ]} border={false} stripe />
                )}
              </div>
            </div>
          )}
          <div className="card">
            <div className="card-header">
              <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Activity size={16} color={progress?.running ? '#f53f3f' : 'var(--color-text-3)'} /><span style={{ fontSize: 15, fontWeight: 600 }}>采集状态</span></span>
              {progress?.running ? <Tag color="blue">采集中</Tag> : progress?.lastRun ? <Tag color="green">就绪</Tag> : <Tag>待采集</Tag>}
            </div>
            <div className="card-body" style={{ padding: '16px 20px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 32, flexWrap: 'wrap' }}>
                <div><div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>上次采集</div><div style={{ fontSize: 14, fontWeight: 500, color: 'var(--color-text-1)' }}>{progress?.lastRun ? new Date(progress.lastRun).toLocaleString('zh-CN') : '尚未采集'}</div></div>
                <div><div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>当前状态</div><div style={{ fontSize: 14, fontWeight: 500, color: progress?.running ? '#f53f3f' : '#00b42a' }}>{progress?.running ? '正在运行' : '空闲'}</div></div>
                <div><div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>采集记录数</div><div style={{ fontSize: 14, fontWeight: 500, color: 'var(--color-text-1)' }}>{colHistory.length} 条</div></div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ═══ TASKS TAB (v2 统一调度中心) ═══ */}
      {tab === 'tasks' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>

          {/* Sub-tab bar */}
          <div style={{ display: 'flex', gap: 0, borderBottom: '1px solid var(--color-border-2)' }}>
            {[
              { key: 'tasks', label: '调度任务', icon: <Timer size={14} /> },
              { key: 'health', label: '调度监控', icon: <Activity size={14} /> },
            ].map(t => (
              <button key={t.key} onClick={() => setSchedViewTab(t.key as any)}
                style={{ padding: '10px 20px', border: 'none', cursor: 'pointer', fontSize: 13,
                  background: 'transparent',
                  color: schedViewTab === t.key ? 'var(--color-primary)' : 'var(--color-text-2)',
                  fontWeight: schedViewTab === t.key ? 600 : 400,
                  borderBottom: schedViewTab === t.key ? '2px solid var(--color-primary)' : '2px solid transparent',
                  display: 'flex', alignItems: 'center', gap: 6, transition: 'all 0.15s' }}>
                {t.icon}{t.label}
              </button>
            ))}
            <div style={{ flex: 1 }} />
            <Button size="small" icon={<RefreshCw size={12} />} loading={tasksLoading} onClick={loadSchedData}
              style={{ alignSelf: 'center', marginRight: 8 }}>刷新</Button>
          </div>

          {/* ── Sub-tab: 调度任务 ── */}
          {schedViewTab === 'tasks' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              {/* Health summary bar */}
              {schedHealth && (
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12 }}>
                  <div className="card" style={{ padding: '12px 16px', display: 'flex', alignItems: 'center', gap: 10 }}>
                    <Badge status={schedHealth.status === 'healthy' ? 'success' : schedHealth.status === 'degraded' ? 'warning' : 'danger'} />
                    <div><div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>调度状态</div>
                    <div style={{ fontSize: 14, fontWeight: 600 }}>{schedHealth.status === 'healthy' ? '健康' : schedHealth.status === 'degraded' ? '降级' : '异常'}</div></div>
                  </div>
                  <div className="card" style={{ padding: '12px 16px' }}>
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>工作线程</div>
                    <div style={{ fontSize: 14, fontWeight: 600 }}>{schedHealth.workers?.busy || 0}/{schedHealth.workers?.total || 0} 忙碌</div>
                  </div>
                  <div className="card" style={{ padding: '12px 16px' }}>
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>运行时间</div>
                    <div style={{ fontSize: 14, fontWeight: 600 }}>{schedHealth.uptime ? (schedHealth.uptime / 3600000000000).toFixed(1) + 'h' : '-'}</div>
                  </div>
                  <div className="card" style={{ padding: '12px 16px' }}>
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>活跃告警</div>
                    <div style={{ fontSize: 14, fontWeight: 600, color: schedAlerts.length > 0 ? '#f53f3f' : 'var(--color-text-1)' }}>{schedAlerts.length} 条</div>
                  </div>
                </div>
              )}

              {/* System Pipeline Tasks */}
              <div className="card">
                <div className="card-header">
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <Timer size={16} color="var(--color-primary)" />
                    <span style={{ fontSize: 15, fontWeight: 600 }}>系统调度任务</span>
                    <span className="muted" style={{ marginLeft: 8 }}>{schedInstances.length} 个实例</span>
                  </span>
                </div>
                <div className="card-body" style={{ padding: 0 }}>
                  {schedInstances.length === 0 ? (
                    <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>
                      {tasksLoading ? '加载中...' : '暂无调度任务实例'}
                    </div>
                  ) : (
                    <Table data={schedInstances} rowKey="id" size="small"
                      columns={[
                        { title: '任务名称', dataIndex: 'definitionId', width: 200, ellipsis: true,
                          render: (v: string, record: any) => {
                            const def = schedDefinitions.find((d: any) => d.id === v);
                            const label = record.label || def?.label || v;
                            return <span style={{ fontWeight: 500 }}>{label}</span>;
                          }
                        },
                        { title: '类型', dataIndex: 'definitionId', width: 80,
                          render: (v: string) => {
                            const def = schedDefinitions.find((d: any) => d.id === v);
                            const kindMap: any = { pipeline: '采集', strategy: '策略', account: '账户', portfolio: '组合', custom: '自定义' };
                            const kind = def?.kind || '';
                            const colors: any = { pipeline: 'blue', strategy: 'purple', account: 'orange', portfolio: 'green', custom: 'gray' };
                            return <Tag color={colors[kind] || 'gray'}>{kindMap[kind] || kind}</Tag>;
                          }
                        },
                        { title: '计划(Cron)', width: 170, render: (_: any, record: any) => {
                          const def = schedDefinitions.find((d: any) => d.id === record.definitionId);
                          // 优先显示实例级 cron（per-run 覆盖），其次定义级 cron，再次事件/手动
                          const cron = record.trigger?.cron || def?.trigger?.cron || '';
                          const isEvent = (record.trigger?.events?.length > 0) || (def?.trigger?.events?.length > 0);
                          return <span style={{ fontSize: 11, fontFamily: 'monospace', color: 'var(--color-text-2)' }}>
                            {cron || (isEvent ? '事件驱动' : '手动')}
                          </span>;
                        }},
                        { title: '状态', dataIndex: 'lastStatus', width: 75,
                          render: (v: string) => {
                            if (v === 'success') return <Tag color="green">成功</Tag>;
                            if (v === 'failed') return <Tag color="red">失败</Tag>;
                            if (v === 'running') return <Tag color="blue">运行中</Tag>;
                            return <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>待执行</span>;
                          }
                        },
                        { title: '上次运行', dataIndex: 'lastRunAt', width: 125,
                          render: (v: string) => <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span>
                        },
                        { title: '下次运行', dataIndex: 'nextRunAt', width: 125,
                          render: (v: string) => <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span>
                        },
                        { title: '操作', width: 80, render: (_: any, record: any) => (
                          <Button size="mini" type="outline" icon={<Play size={10} />}
                            onClick={() => handleSchedTrigger(record.id)}>触发</Button>
                        )},
                      ]} pagination={{ pageSize: 30, sizeCanChange: true }} border={false} stripe />
                  )}
                </div>
              </div>

              {/* Legacy Scheduled Tasks (deprecated, read-only) */}
              <div className="card" style={{ opacity: 0.7 }}>
                <div className="card-header">
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <Timer size={16} color="var(--color-text-3)" />
                    <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-3)' }}>旧版定时任务 (已弃用)</span>
                    <Tag color="gray" size="small">仅查看</Tag>
                  </span>
                </div>
                <div className="card-body" style={{ padding: 0 }}>
                  <Table data={scheduledTasks} rowKey="id" size="small"
                    columns={[
                      { title: '任务名称', dataIndex: 'name', width: 130, ellipsis: true },
                      { title: '类型', dataIndex: 'phase', width: 100, render: (v: string) => <span style={{ fontSize: 11, fontFamily: 'monospace', color: 'var(--color-text-3)' }}>{v}</span> },
                      { title: '上次状态', dataIndex: 'lastStatus', width: 70,
                        render: (v: string) => v === 'success' ? <Tag color="green">成功</Tag> : v === 'failed' ? <Tag color="red">失败</Tag> : <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>-</span>
                      },
                      { title: '上次运行', dataIndex: 'lastRun', width: 125,
                        render: (v: string) => <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span>
                      },
                    ]} pagination={{ pageSize: 10 }} border={false} stripe />
                </div>
              </div>
            </div>
          )}

          {/* ── Sub-tab: 调度监控 ── */}
          {schedViewTab === 'health' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              {/* Health Details */}
              {schedHealth && (
                <div className="card">
                  <div className="card-header">
                    <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                      <Activity size={16} color="var(--color-primary)" />
                      <span style={{ fontSize: 15, fontWeight: 600 }}>健康状态</span>
                      <Badge status={schedHealth.status === 'healthy' ? 'success' : 'warning'} text={schedHealth.status === 'healthy' ? '健康' : '降级'} />
                    </span>
                  </div>
                  <div className="card-body">
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                      <div><span className="muted">运行时间</span><div style={{ fontWeight: 600 }}>{schedHealth.uptime ? (schedHealth.uptime / 1000000000 / 3600).toFixed(1) + ' 小时' : '-'}</div></div>
                      <div><span className="muted">Worker 利用率</span><div style={{ fontWeight: 600 }}>{schedHealth.workers?.avgBusyPct?.toFixed(1) || 0}%</div></div>
                      <div><span className="muted">忙碌/空闲 Worker</span><div style={{ fontWeight: 600 }}>{schedHealth.workers?.busy || 0} / {schedHealth.workers?.idle || 0}</div></div>
                      <div><span className="muted">总 Worker</span><div style={{ fontWeight: 600 }}>{schedHealth.workers?.total || 0}</div></div>
                    </div>
                  </div>
                </div>
              )}

              {/* Queue Status */}
              {schedQueues && (
                <div className="card">
                  <div className="card-header">
                    <span style={{ fontSize: 15, fontWeight: 600 }}>队列状态</span>
                  </div>
                  <div className="card-body" style={{ padding: 0 }}>
                    <Table data={Object.entries(schedQueues).map(([kind, q]: [string, any]) => ({ kind, ...q }))} rowKey="kind" size="small"
                      columns={[
                        { title: '类型', dataIndex: 'kind', width: 100, render: (v: string) => <Tag>{v}</Tag> },
                        { title: '待处理', dataIndex: 'depth', width: 70 },
                        { title: '最老等待', dataIndex: 'oldest', width: 100,
                          render: (v: number) => v ? (v / 1000000000).toFixed(1) + 's' : '-'
                        },
                        { title: '平均等待', dataIndex: 'avgWait', width: 100,
                          render: (v: number) => v ? (v / 1000000).toFixed(0) + 'ms' : '-'
                        },
                      ]} pagination={false} border={false} stripe />
                  </div>
                </div>
              )}

              {/* Execution History */}
              <div className="card">
                <div className="card-header">
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <Activity size={16} color="var(--color-primary)" />
                    <span style={{ fontSize: 15, fontWeight: 600 }}>最近执行记录</span>
                  </span>
                </div>
                <div className="card-body" style={{ padding: 0 }}>
                  {schedHistory.length === 0 ? (
                    <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>暂无执行记录</div>
                  ) : (
                    <Table data={schedHistory} rowKey="id" size="small"
                      columns={[
                        { title: '任务', dataIndex: 'definitionId', width: 200, ellipsis: true,
                          render: (v: string, record: any) => {
                            const def = schedDefinitions.find((d: any) => d.id === v);
                            const label = record.label || def?.label || v;
                            return <span>{label}</span>;
                          }
                        },
                        { title: '状态', dataIndex: 'status', width: 65,
                          render: (v: string) => v === 'success' ? <Tag color="green">成功</Tag> : v === 'failed' ? <Tag color="red">失败</Tag> : <Tag>{v}</Tag>
                        },
                        { title: '耗时', dataIndex: 'duration', width: 75,
                          render: (v: number) => <span style={{ fontSize: 12, color: v > 300000000000 ? '#f53f3f' : 'var(--color-text-2)' }}>{v ? (v / 1000000000).toFixed(1) + 's' : '-'}</span>
                        },
                        { title: '开始时间', dataIndex: 'startedAt', width: 135,
                          render: (v: string) => <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span>
                        },
                        { title: '错误', dataIndex: 'errorMsg', ellipsis: true,
                          render: (v: string) => v ? <span style={{ fontSize: 11, color: '#f53f3f' }}>{v}</span> : <span style={{ color: 'var(--color-text-3)', fontSize: 11 }}>-</span>
                        },
                      ]} pagination={{ pageSize: 10 }} border={false} stripe />
                  )}
                </div>
              </div>

              {/* Alerts */}
              {schedAlerts.length > 0 && (
                <div className="card">
                  <div className="card-header">
                    <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                      <Activity size={16} color="#f53f3f" />
                      <span style={{ fontSize: 15, fontWeight: 600, color: '#f53f3f' }}>活跃告警</span>
                    </span>
                    <Button size="small" type="text" onClick={handleClearAlerts}>清除</Button>
                  </div>
                  <div className="card-body" style={{ padding: 0 }}>
                    <Table data={schedAlerts} rowKey="message" size="small"
                      columns={[
                        { title: '级别', dataIndex: 'level', width: 60,
                          render: (v: string) => <Tag color={v === 'critical' ? 'red' : 'orange'}>{v}</Tag>
                        },
                        { title: '消息', dataIndex: 'message', ellipsis: true },
                        { title: '时间', dataIndex: 'since', width: 135,
                          render: (v: string) => <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span>
                        },
                      ]} pagination={false} border={false} stripe />
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* ═══ COLLECT TAB ═══ */}
      {tab === 'collect' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="card">
            <div className="card-header">
              <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Activity size={16} color="var(--color-primary)" /><span style={{ fontSize: 15, fontWeight: 600 }}>采集控制台</span></span>
              {collecting && <Tag color="blue" style={{ display: 'flex', alignItems: 'center', gap: 4 }}><span style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--color-primary)', display: 'inline-block' }} />采集中...</Tag>}
            </div>
            <div className="card-body" style={{ padding: '14px 20px' }}>
              {/* Phase tabs */}
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap', marginBottom: 14 }}>
                {scheduledTasks.filter((t: any) => t.enabled && t.phase !== 'quote' && PHASE_LABELS[t.phase]).map(t => t.phase).filter((v, i, a) => a.indexOf(v) === i).map(phase => {
                  const isActive = collecting && progress?.phase === phase;
                  const isSelected = selectedCollectPhase === phase;
                  return (
                    <button key={phase} onClick={() => { setSelectedCollectPhase(phase); if (isActive) connectStream(); }}
                      style={{
                        padding: isActive ? '5px 13px' : '6px 14px',
                        border: isActive ? '2px solid var(--color-warning)' : isSelected ? '2px solid var(--color-primary)' : '1px solid var(--color-border-2)',
                        borderRadius: 20, cursor: 'pointer', fontSize: 12,
                        fontWeight: isActive ? 600 : isSelected ? 600 : 400,
                        background: isActive ? 'var(--color-warning-bg)' : isSelected ? 'var(--color-info-bg)' : 'var(--color-bg-1)',
                        color: isActive ? '#e65100' : isSelected ? '#165dff' : 'var(--color-text-2)',
                        display: 'flex', alignItems: 'center', gap: 5, whiteSpace: 'nowrap',
                        transition: 'all 0.15s',
                        animation: isActive ? 'collectBgPulse 2s ease-in-out infinite' : 'none',
                      }}>
                      {isActive ? (
                        <span style={{
                          width: 8, height: 8, borderRadius: '50%', background: '#ff6d00',
                          display: 'inline-block',
                          animation: 'collectPulse 1.2s ease-in-out infinite',
                        }} />
                      ) : null}
                      {PHASE_LABELS[phase]}
                      {isActive && <span style={{ fontSize: 10, opacity: 0.8 }}>采集中</span>}
                    </button>
                  );
                })}
              </div>

              {/* Selected phase detail panel */}
              <div style={{ border: '1px solid var(--color-border-1)', borderRadius: 8, overflow: 'hidden', background: 'var(--color-fill-1)' }}>
                <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--color-border-1)', display: 'flex', alignItems: 'center', justifyContent: 'space-between', background: 'var(--color-bg-1)' }}>
                  <div>
                    <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>{PHASE_LABELS[selectedCollectPhase]}</div>
                    <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 2 }}>{PHASE_DESCRIPTIONS[selectedCollectPhase] || ''}</div>
                    {(() => { const st = scheduledTasks.find((t: any) => t.phase === selectedCollectPhase && t.enabled); if (!st || collecting) return null; return (
                      <div style={{ fontSize: 11, color: 'var(--color-text-2)', marginTop: 4 }}>
                        <span style={{ color: '#00b42a' }}>⏱ {cronToText(st.cronExpr)}</span>
                        {st.nextRun && <span style={{ marginLeft: 8, color: 'var(--color-text-3)' }}>· 下次: {new Date(st.nextRun).toLocaleString('zh-CN')}</span>}
                      </div>
                    ); })()}
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    {(() => { const st = scheduledTasks.find((t: any) => t.phase === selectedCollectPhase && t.enabled); if (!st) return null; return (
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <Tag color="green" style={{ fontSize: 11 }}>{cronToText(st.cronExpr)}</Tag>
                        {!collecting && st.nextRun && <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>下次 {new Date(st.nextRun).toLocaleString('zh-CN')}</span>}
                      </div>
                    ); })()}
                    {collecting && progress?.phase === selectedCollectPhase ? (
                      <Tag color="orange" style={{ display: 'flex', alignItems: 'center', gap: 4 }}><span style={{ width: 6, height: 6, borderRadius: '50%', background: '#ff7d00', display: 'inline-block' }} />采集中</Tag>
                    ) : (
                      <>
                        <Button size="small" type="primary" icon={<Play size={12} />} onClick={() => handleTrigger([selectedCollectPhase])} disabled={collecting && progress?.phase === selectedCollectPhase}>采集</Button>
                        {HISTORY_CAPABLE_PHASES.has(selectedCollectPhase) && (
                        <Button size="small" type="outline" icon={<History size={12} />} onClick={() => { setRepairPhase(selectedCollectPhase); setRepairDateRange([]); setRepairAll(false); setRepairModalVisible(true); }} disabled={collecting && progress?.phase === selectedCollectPhase}>修复历史</Button>
                        )}
                      </>
                    )}
                  </div>
                </div>
                <div style={{ padding: '12px 16px', minHeight: 120 }}>
                  {/* 采集进度条 — 支持多 phase 并行 */}
                  {collecting && progress && (() => {
                    const activePhases: Record<string, any> = progress.activePhases || {};
                    const activeEntries = Object.entries(activePhases).filter(([, s]: [string, any]) => s?.running);
                    if (activeEntries.length === 0 && progress.running) {
                      // Fallback: single-phase display
                      activeEntries.push([progress.phase || 'unknown', { running: true, phaseCurrent: progress.phaseCurrent, phaseTotal: progress.phaseTotal }]);
                    }
                    if (activeEntries.length === 0) return null;
                    return (
                      <div style={{ marginBottom: 12, display: 'flex', flexDirection: 'column', gap: 8 }}>
                        {activeEntries.map(([phaseName, ps]: [string, any]) => (
                          <div key={phaseName} style={{
                            border: '1px solid var(--color-border-2)',
                            borderRadius: 6,
                            padding: '6px 10px',
                            background: phaseName === selectedCollectPhase ? 'var(--color-primary-light-1)' : 'var(--color-bg-1)',
                          }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                              <span style={{
                                width: 8, height: 8, borderRadius: '50%',
                                background: '#ff6d00',
                                display: 'inline-block',
                                animation: 'collectPulse 1.2s ease-in-out infinite',
                              }} />
                              <span style={{ fontSize: 13, fontWeight: 600 }}>{PHASE_LABELS[phaseName] || phaseName}</span>
                              {activeEntries.length > 1 && (
                                <Button size="mini" type="text" onClick={() => setSelectedCollectPhase(phaseName)}>
                                  {phaseName === selectedCollectPhase ? '当前' : '查看'}
                                </Button>
                              )}
                            </div>
                            {ps.phaseTotal > 0 ? (
                              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                <Progress percent={Math.round((ps.phaseCurrent / Math.max(ps.phaseTotal, 1)) * 100)} style={{ flex: 1 }} size="small" />
                                <span style={{ fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>{ps.phaseCurrent.toLocaleString()} / {ps.phaseTotal.toLocaleString()}</span>
                              </div>
                            ) : (
                              <Progress percent={0} style={{ flex: 1 }} size="small" />
                            )}
                          </div>
                        ))}
                        {progress.message && (
                          <div style={{ fontSize: 12, color: 'var(--color-text-2)' }}>{progress.message}</div>
                        )}
                      </div>
                    );
                  })()}

                  {/* 采集结果统计 */}
                  {phaseResults.filter((r: any) => r.phase === selectedCollectPhase).map((r: any) => (
                    <div key={r.phase} style={{ border: `1px solid ${r.errors > 0 ? '#f53f3f' : 'var(--color-border-1)'}`, borderRadius: 6, padding: '8px 12px', marginBottom: 8, background: r.errors > 0 ? '#ffece8' : '#f0fff4', display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 8, fontSize: 12 }}>
                      <div>总计 <b>{r.total}</b></div><div>新增 <b style={{ color: '#165dff' }}>{r.new}</b></div><div>存量 <span style={{ color: 'var(--color-text-3)' }}>{r.skipped}</span></div><div>耗时 <span>{(r.durationMs / 1000).toFixed(1)}s</span></div>
                    </div>
                  ))}

                  {/* 控制台日志 — 始终显示，采集完成后不自动关闭 */}
                  {consoleLines.length > 0 && (
                    <div>
                      <div ref={consoleRef} style={{ background: '#121215', borderRadius: 6, padding: '10px 14px', maxHeight: 300, overflow: 'auto', fontFamily: '"JetBrains Mono","Fira Code","SF Mono",monospace', fontSize: 12, lineHeight: '18px' }}>
                        {(collecting ? consoleLines.filter(l => !l.phase || l.phase === selectedCollectPhase) : consoleLines).slice(-80).map((line, i) => renderConsoleLine(line, i))}
                      </div>
                      <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
                        {collecting ? (
                          <Button size="small" type="outline" status="danger" icon={<Square size={12} />} onClick={handleStop}>断开监控</Button>
                        ) : (
                          <Button size="small" type="outline" icon={<X size={12} />} onClick={() => { setConsoleLines([]); setPhaseResults([]); }}>关闭控制台</Button>
                        )}
                      </div>
                    </div>
                  )}

                  {/* 无日志时显示提示 */}
                  {consoleLines.length === 0 && !collecting && (
                    <div style={{ padding: 32, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>点击右上角「采集」按钮开始采集</div>
                  )}
                </div>
              </div>
            </div>
          </div>
          {/* Collection history - always visible */}
            <div className="card">
              <div className="card-header">
                <span style={{ fontSize: 14, fontWeight: 600 }}>{PHASE_LABELS[selectedCollectPhase] || selectedCollectPhase} · 最近采集记录</span>
                {collecting && <Tag color="blue" style={{ fontSize: 11 }}>采集中</Tag>}
                <Button size="small" type="text" icon={<RefreshCw size={12} />} onClick={loadColHistory}>刷新</Button>
              </div>
              <div className="card-body" style={{ padding: 0 }}>
                {(() => {
                  const phaseLogs = colHistory.filter((l: any) => {
                    try { const phases = JSON.parse(l.phases || '[]'); return Array.isArray(phases) && phases.some((p: any) => (p.phase || p) === selectedCollectPhase); }
                    catch { return false; }
                  });
                  if (phaseLogs.length === 0) return <div style={{ padding: 32, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>暂无该类型的采集记录</div>;
                  return (
                    <Table data={phaseLogs.slice(0, 10)} rowKey="id" size="small"
                      columns={[
                        { title: '状态', dataIndex: 'status', width: 70, render: (v: string) => v === 'success' ? <Tag color="green">成功</Tag> : v === 'partial' ? <Tag color="orange">部分</Tag> : v === 'running' ? <Tag color="blue">运行中</Tag> : <Tag color="red">失败</Tag> },
                        { title: '新增', dataIndex: 'totalNew', width: 60, render: (v: number) => <span style={{ fontWeight: 600, color: '#165dff' }}>{v}</span> },
                        { title: '跳过', dataIndex: 'totalSkipped', width: 60, render: (v: number) => <span style={{ color: 'var(--color-text-3)' }}>{v}</span> },
                        { title: '错误', dataIndex: 'totalErrors', width: 55, render: (v: number) => v > 0 ? <span style={{ color: '#f53f3f', fontWeight: 600 }}>{v}</span> : <span style={{ color: 'var(--color-text-3)' }}>0</span> },
                        { title: '耗时', dataIndex: 'durationMs', width: 70, render: (v: number) => <span style={{ color: 'var(--color-text-2)', fontSize: 12 }}>{formatDuration(v)}</span> },
                        { title: '时间', dataIndex: 'startedAt', width: 140, render: (v: string) => <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span> },
                      ]} pagination={false} border={false} stripe />
                  );
                })()}
              </div>
            </div>
        </div>
      )}

      {/* ═══ IMPORT TAB ═══ */}
      {tab === 'import' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="card">
            <div className="card-header"><span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><FileSpreadsheet size={16} color="var(--color-primary)" /><span style={{ fontSize: 15, fontWeight: 600 }}>导入榜单数据文件</span></span><span className="muted" style={{ fontSize: 12 }}>支持 .xlsx / .xlsm</span></div>
            <div className="card-body">
              <Upload drag accept=".xlsx,.xlsm" autoUpload={false} disabled={loading} onChange={(_, file) => { setLoading(true); setResult(null); uploadExcel(file.originFile as File).then((res: any) => { setResult(res.data); showToast('success', 'Excel 导入完成'); }).catch((err: any) => showToast('error', err?.response?.data?.error || '导入失败')).finally(() => setLoading(false)); return false; }} tip="拖拽或点击上传，参考文件: MSS20260603.xlsm" />
              {loading && <div style={{ marginTop: 16, padding: '12px 16px', background: 'var(--color-info-bg)', borderRadius: 4, display: 'flex', alignItems: 'center', gap: 10, fontSize: 13, color: 'var(--color-primary)' }}><RefreshCw size={14} className="spin" />正在解析并导入数据...</div>}
            </div>
          </div>

          <div className="card">
            <div className="card-header"><span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><BarChart3 size={16} color="#00b42a" /><span style={{ fontSize: 15, fontWeight: 600 }}>导入 K 线数据 CSV</span></span><span className="muted" style={{ fontSize: 12 }}>日K线行情 .csv (GBK/UTF-8)</span></div>
            <div className="card-body">
              <Upload drag accept=".csv" autoUpload={false} disabled={klineLoading || loading} onChange={(_, file) => { setKlineLoading(true); setResult(null); uploadKline(file.originFile as File).then((res: any) => { setResult({ ...res.data, fileName: file.name, type: 'kline' }); showToast('success', 'K线数据导入完成'); }).catch((err: any) => showToast('error', err?.response?.data?.error || '导入失败')).finally(() => setKlineLoading(false)); return false; }} tip="拖拽或点击上传 CSV 文件，格式: 股票代码,交易日期,开盘价,最高价,最低价,收盘价,成交量,成交额,换手率..." />
              {klineLoading && <div style={{ marginTop: 16, padding: '12px 16px', background: 'var(--color-success-bg)', borderRadius: 4, display: 'flex', alignItems: 'center', gap: 10, fontSize: 13, color: 'var(--color-success)' }}><RefreshCw size={14} className="spin" />正在解析并导入K线数据...</div>}
            </div>
          </div>

          <div className="card">
            <div className="card-header"><span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><FileJson size={16} color="#722ed1" /><span style={{ fontSize: 15, fontWeight: 600 }}>导入预测数据 JSON</span></span><span className="muted" style={{ fontSize: 12 }}>算法预测结果文件 .json</span></div>
            <div className="card-body">
              <Upload drag accept=".json" autoUpload={false} disabled={predLoading || loading} onChange={(_, file) => { setPredLoading(true); setResult(null); uploadPrediction(file.originFile as File).then((res: any) => { setResult({ ...res.data, fileName: file.name, type: 'prediction' }); showToast('success', '预测数据导入完成'); }).catch((err: any) => showToast('error', typeof err === 'string' ? err : (err?.response?.data?.error || '导入失败'))).finally(() => setPredLoading(false)); return false; }} tip="拖拽或点击上传，JSON 格式: 算法团队预测数据" />
              {predLoading && <div style={{ marginTop: 16, padding: '12px 16px', background: 'var(--purple-1)', borderRadius: 4, display: 'flex', alignItems: 'center', gap: 10, fontSize: 13, color: 'var(--purple-6)' }}><RefreshCw size={14} className="spin" />正在解析并导入预测数据...</div>}
            </div>
          </div>

          <div className="card">
            <div className="card-header"><span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Bot size={16} color="#8b5cf6" /><span style={{ fontSize: 15, fontWeight: 600 }}>导入 AI 股票简介 JSON</span></span><span className="muted" style={{ fontSize: 12 }}>AI 分析报告文件 .json</span></div>
            <div className="card-body">
              <Upload drag accept=".json" autoUpload={false} disabled={profileLoading || loading} onChange={(_, file) => { setProfileLoading(true); setResult(null); uploadProfile(file.originFile as File).then((res: any) => { setResult({ ...res.data, fileName: file.name, type: 'profile' }); showToast('success', `简介导入完成: ${res.data.imported}只新增, ${res.data.updated}只覆盖`); }).catch((err: any) => showToast('error', err?.response?.data?.error || '导入失败')).finally(() => setProfileLoading(false)); return false; }} tip="拖拽或点击上传 JSON 文件，格式: [{stock_code, analysis_content, ...}] 导入后覆盖已有 AI 简介" />
              {profileLoading && <div style={{ marginTop: 16, padding: '12px 16px', background: 'rgba(139,92,246,0.08)', borderRadius: 4, display: 'flex', alignItems: 'center', gap: 10, fontSize: 13, color: '#8b5cf6' }}><RefreshCw size={14} className="spin" />正在导入股票简介...</div>}
            </div>
          </div>
          {result && (
            <div className="card">
              <div className="card-header"><span style={{ fontSize: 15, fontWeight: 600 }}>导入结果</span><span className="muted" style={{ fontSize: 12 }}>{result.fileName}{result.type === 'kline' ? ' (K线)' : result.type === 'prediction' ? ' (预测)' : result.type === 'profile' ? ' (简介)' : ''}</span></div>
              <div className="card-body">
                <div style={{ display: 'grid', gridTemplateColumns: result.type === 'prediction' ? 'repeat(3, 1fr)' : 'repeat(4, 1fr)', gap: 16, marginBottom: 16 }}>
                  {(result.type === 'kline' ? [{ label: '导入成功', value: result.imported || 0, color: '#00b42a' }, { label: '跳过', value: result.skipped || 0, color: 'var(--color-text-3)' }, { label: '总行数', value: result.totalRows || 0, color: '#165dff' }, { label: '交易日期', value: result.tradeDate || '-', color: '#722ed1' }] : result.type === 'profile' ? [{ label: '新增', value: result.imported || 0, color: '#8b5cf6' }, { label: '覆盖', value: result.updated || 0, color: '#f59e0b' }, { label: '错误', value: result.errors || 0, color: '#f53f3f' }, { label: '总数', value: result.total || 0, color: '#165dff' }] : result.type === 'prediction' ? [{ label: '预测记录', value: result.imported || 0, color: '#722ed1' }, { label: '跳过', value: result.skipped || 0, color: 'var(--color-text-3)' }, { label: '股票数', value: result.total || 0, color: '#165dff' }] : [{ label: '交易日数', value: result.datesImported, color: '#165dff' }, { label: '上榜记录', value: result.picksImported, color: '#f53f3f' }, { label: '信号数据', value: result.signalsImported, color: '#00b42a' }, { label: '新增个股', value: result.stocksCreated, color: '#ff7d00' }]).map(item => (
                    <div key={item.label} style={{ textAlign: 'center', padding: '12px', background: 'var(--color-fill-2)', borderRadius: 6 }}><div style={{ fontSize: 24, fontWeight: 700, color: item.color, fontFamily: 'var(--font-family-mono, monospace)' }}>{item.value}</div><div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 4 }}>{item.label}</div></div>
                  ))}
                </div>
                {result.previews?.map((p: string, i: number) => <div key={i} style={{ padding: '8px 12px', background: 'var(--color-fill-2)', borderRadius: 4, fontSize: 13, color: 'var(--color-text-2)', marginBottom: 4, display: 'flex', alignItems: 'center', gap: 6 }}><CheckCircle size={12} color="#00b42a" />{p}</div>)}
                {result.errors?.map((e: string, i: number) => <div key={i} style={{ padding: '8px 12px', background: 'var(--color-danger-bg)', borderRadius: 4, fontSize: 12, color: 'var(--red-7)', marginBottom: 4 }}><XCircle size={12} style={{ marginRight: 6 }} />{e}</div>)}
              </div>
            </div>
          )}
          {/* Import history */}
          <div className="card" style={{ marginTop: 16 }}>
            <div className="card-header">
              <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><FileSpreadsheet size={16} color="var(--color-primary)" /><span style={{ fontSize: 15, fontWeight: 600 }}>导入记录</span></span>
              <Button size="small" type="text" icon={<RefreshCw size={12} />} onClick={loadHistory}>刷新</Button>
            </div>
            <div className="card-body" style={{ padding: 0 }}>
              {history.length === 0 ? <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>暂无导入记录</div> : (
                <Table data={history} rowKey="id" size="small" columns={[
                  { title: '文件名', dataIndex: 'fileName', width: 200 }, { title: '导入条数', dataIndex: 'rowsImported', width: 100, render: (v: number) => <span style={{ fontWeight: 600 }}>{v}</span> },
                  { title: '状态', dataIndex: 'status', width: 100, render: (v: string) => v === 'success' ? <Tag color="green">成功</Tag> : v === 'partial' ? <Tag color="orange">部分</Tag> : <Tag color="red">失败</Tag> },
                  { title: '时间', dataIndex: 'importedAt', width: 180, render: (v: string) => <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span> },
                ]} pagination={false} border={false} stripe />

              )}
            </div>
          </div>
        </div>
      )}

      {/* ═══ HISTORY TAB ═══ */}
      {tab === 'history' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="card">
            <div className="card-header"><span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Activity size={16} color="var(--color-primary)" /><span style={{ fontSize: 15, fontWeight: 600 }}>采集记录</span></span><div style={{ display: 'flex', gap: 8 }}><Popconfirm title="清除超过30分钟仍处于运行中的异常记录？" onOk={() => handleClearStuck('stuck')}><Button size="small" type="text" status="warning" icon={<X size={12} />}>清除卡住</Button></Popconfirm><Popconfirm title="清除超过24小时的错误采集记录？" onOk={() => handleClearStuck('errors')}><Button size="small" type="text" status="danger" icon={<X size={12} />}>清除错误</Button></Popconfirm><Button size="small" type="text" icon={<RefreshCw size={12} />} onClick={loadColHistory}>刷新</Button></div></div>
            <div className="card-body" style={{ padding: 0 }}>
              {colHistory.length === 0 ? <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>暂无采集记录</div> : (
                <Table data={colHistory} rowKey="id" size="small"
                  columns={[
                    { title: '状态', dataIndex: 'status', width: 70, render: (v: string) => v === 'success' ? <Tag color="green">成功</Tag> : v === 'partial' ? <Tag color="orange">部分</Tag> : v === 'running' ? <Tag color="blue">运行中</Tag> : <Tag color="red">失败</Tag> },
                    { title: '采集类型', dataIndex: 'phases', width: 170, render: (v: string) => { if (!v) return <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>-</span>; try { const phases: any[] = JSON.parse(v); if (!Array.isArray(phases) || phases.length === 0) return <span style={{ fontSize: 12 }}>全量</span>; return (<div style={{ display: 'flex', flexWrap: 'wrap', gap: 3 }}>{phases.slice(0, 5).map((p: any, i: number) => <Tag key={i} color={p.errors > 0 ? 'red' : 'blue'} style={{ fontSize: 11, lineHeight: '18px', padding: '0 6px' }}>{PHASE_LABELS[p.phase] || p.phase}</Tag>)}{phases.length > 5 && <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>+{phases.length - 5}</span>}</div>); } catch { return <span style={{ fontSize: 12 }}>全量</span>; } } },
                    { title: '新增', dataIndex: 'totalNew', width: 60, render: (v: number) => <span style={{ fontWeight: 600, color: '#165dff' }}>{v}</span> },
                    { title: '跳过', dataIndex: 'totalSkipped', width: 60, render: (v: number) => <span style={{ color: 'var(--color-text-3)' }}>{v}</span> },
                    { title: '错误', dataIndex: 'totalErrors', width: 55, render: (v: number) => v > 0 ? <span style={{ color: '#f53f3f', fontWeight: 600 }}>{v}</span> : <span style={{ color: 'var(--color-text-3)' }}>0</span> },
                    { title: '耗时', dataIndex: 'durationMs', width: 70, render: (v: number) => <span style={{ color: 'var(--color-text-2)', fontSize: 12 }}>{formatDuration(v)}</span> },
                    { title: '时间', dataIndex: 'startedAt', width: 140, render: (v: string) => <span style={{ fontSize: 11 }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span> },
                  ]} pagination={false} border={false} stripe />
              )}
            </div>
          </div>
          <div className="card">
            <div className="card-header"><span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><FileSpreadsheet size={16} color="var(--color-primary)" /><span style={{ fontSize: 15, fontWeight: 600 }}>Excel 导入记录</span></span><Button size="small" type="text" icon={<RefreshCw size={12} />} onClick={loadHistory}>刷新</Button></div>
            <div className="card-body" style={{ padding: 0 }}>
              {history.length === 0 ? <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>暂无导入记录</div> : (
                <Table data={history} rowKey="id" size="small" columns={[
                  { title: '文件名', dataIndex: 'fileName', width: 200 }, { title: '导入条数', dataIndex: 'rowsImported', width: 100, render: (v: number) => <span style={{ fontWeight: 600 }}>{v}</span> },
                  { title: '状态', dataIndex: 'status', width: 100, render: (v: string) => v === 'success' ? <Tag color="green">成功</Tag> : v === 'partial' ? <Tag color="orange">部分</Tag> : <Tag color="red">失败</Tag> },
                  { title: '时间', dataIndex: 'importedAt', width: 180, render: (v: string) => <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span> },
                ]} pagination={false} border={false} stripe />

              )}
            </div>
          </div>
        </div>
      )}
      {tab === 'logs' && (
        <TaskLogPage embedded />
      )}
    </div>
  );
}
