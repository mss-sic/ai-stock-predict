import { useState, useEffect, useRef, useCallback } from 'react';
import { Upload, Button, Table, Tag, Progress } from '@arco-design/web-react';
import {
  Database, Upload as UploadIcon, RefreshCw, FileSpreadsheet,
  CheckCircle, XCircle, Clock, Play, Terminal, Square, History, Activity, X
} from 'lucide-react';
import {
  uploadExcel, triggerCollection, fetchCollectorProgress,
  fetchImportHistory, fetchCollectorHistory
} from '../services/api';

const PHASE_LABELS: Record<string, string> = {
  full_sync: '股票列表同步',
  kline: '日K线数据',
  indicator: 'PE/PB指标',
  industry: '行业分类',
  quote: '实时行情',
  shareholder: '股东数据',
  financial: '财务数据',
  news: '资讯数据',
  reports: '研报数据',
};

const PHASE_COLORS: Record<string, string> = {
  full_sync: '#165dff',
  kline: '#ff7d00',
  indicator: '#00b42a',
  industry: '#722ed1',
  quote: '#14c9c9',
  shareholder: '#f53f3f',
  financial: '#0fc6c2',
  news: '#f77234',
  reports: '#e865b7',
};

interface SSELine {
  type: string;
  phase?: string;
  message?: string;
  level?: string;
  result?: PhaseResult;
}

interface PhaseResult {
  phase: string;
  total: number;
  new: number;
  skipped: number;
  errors: number;
  durationMs: number;
}

// Custom inline notification
function Toast({ type, msg, onClose }: { type: 'success' | 'error' | 'info'; msg: string; onClose: () => void }) {
  const colors = { success: '#f0fff4', error: '#ffece8', info: '#e8f3ff' };
  const borders = { success: '#00b42a', error: '#f53f3f', info: '#165dff' };
  const icons = { success: <CheckCircle size={14} color="#00b42a" />, error: <XCircle size={14} color="#f53f3f" />, info: <Activity size={14} color="#165dff" /> };

  useEffect(() => {
    const t = setTimeout(onClose, 3000);
    return () => clearTimeout(t);
  }, [onClose]);

  return (
    <div style={{
      position: 'fixed', top: 20, right: 20, zIndex: 9999,
      padding: '10px 16px', borderRadius: 6, fontSize: 13,
      background: colors[type], border: `1px solid ${borders[type]}`,
      display: 'flex', alignItems: 'center', gap: 8, boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
      maxWidth: 400,
    }}>
      {icons[type]}
      <span style={{ flex: 1, color: '#1d2129' }}>{msg}</span>
      <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0, color: '#86909c' }}><X size={12} /></button>
    </div>
  );
}

export default function DataManagementPage() {
  const [tab, setTab] = useState<'collect' | 'import' | 'history'>('collect');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [history, setHistory] = useState<any[]>([]);
  const [colHistory, setColHistory] = useState<any[]>([]);
  const [progress, setProgress] = useState<any>(null);
  const [collecting, setCollecting] = useState(false);
  const [consoleLines, setConsoleLines] = useState<{ text: string; level: string; time: string }[]>([]);
  const [phaseResults, setPhaseResults] = useState<PhaseResult[]>([]);
  const [totalDuration, setTotalDuration] = useState(0);
  const [toast, setToast] = useState<{ type: 'success' | 'error' | 'info'; msg: string } | null>(null);
  const pollRef = useRef<any>(null);
  const consoleRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const startTimeRef = useRef<number>(0);

  const showToast = useCallback((type: 'success' | 'error' | 'info', msg: string) => {
    setToast({ type, msg });
  }, []);

  // Auto-reconnect SSE if collection is running
  const autoReconnect = useCallback(async () => {
    try {
      const pr: any = await fetchCollectorProgress();
      if (pr.data?.running) {
        setCollecting(true);
        setProgress(pr.data);
        setPhaseResults(pr.data.results || []);
        if (pr.data.started) {
          startTimeRef.current = new Date(pr.data.started).getTime();
        }
        addConsoleLine('🔄 检测到正在运行的采集，自动重连...', 'system');
        // Connect SSE
        const es = new EventSource('http://127.0.0.1:8080/api/v1/collector/stream');
        eventSourceRef.current = es;
        es.onmessage = (event) => {
          try {
            const line: SSELine = JSON.parse(event.data);
            if (line.type === 'connected') return;
            if (line.type === 'log' && line.message) {
              addConsoleLine(line.message, line.level || 'info');
            } else if (line.type === 'phase' && line.message) {
              addConsoleLine(`
━━━ ${line.message} ━━━`, 'phase');
            } else if (line.type === 'result' && line.result) {
              setPhaseResults(prev => {
                const filtered = prev.filter(p => p.phase !== line.result!.phase);
                return [...filtered, line.result!];
              });
            } else if (line.type === 'done') {
              addConsoleLine(`
${line.message}`, 'success');
              setCollecting(false);
              setTotalDuration(Date.now() - startTimeRef.current);
              es.close();
              eventSourceRef.current = null;
              loadProgress();
              showToast('success', '采集完成');
            }
          } catch {}
        };
        // Fallback polling
        pollRef.current = setInterval(async () => {
          try {
            const pr: any = await fetchCollectorProgress();
            setProgress(pr.data);
            if (!pr.data?.running) {
              setCollecting(false);
              setTotalDuration(Date.now() - startTimeRef.current);
              clearInterval(pollRef.current);
              pollRef.current = null;
              eventSourceRef.current?.close();
            }
          } catch {}
        }, 3000);
      }
    } catch {}
  }, []);

  useEffect(() => {
    if (tab === 'history') {
      loadHistory();
      loadColHistory();
    }
    if (tab === 'collect') {
      loadProgress();
      autoReconnect();
    }
    return () => {
      clearInterval(pollRef.current);
      eventSourceRef.current?.close();
    };
  }, [tab, autoReconnect]);

  const loadHistory = async () => {
    try { const res: any = await fetchImportHistory(); setHistory(res.data || []); }
    catch { setHistory([]); }
  };

  const loadColHistory = async () => {
    try { const res: any = await fetchCollectorHistory(); setColHistory(res.data || []); }
    catch { setColHistory([]); }
  };

  const loadProgress = async () => {
    try { const res: any = await fetchCollectorProgress(); setProgress(res.data); }
    catch {}
  };

  const addConsoleLine = useCallback((text: string, level: string = 'info') => {
    const time = new Date().toLocaleTimeString('zh-CN', { hour12: false });
    setConsoleLines(prev => [...prev.slice(-500), { text, level, time }]);
  }, []);

  useEffect(() => {
    if (consoleRef.current) {
      consoleRef.current.scrollTop = consoleRef.current.scrollHeight;
    }
  }, [consoleLines]);

  const handleTrigger = async (phases?: string[]) => {
    setLoading(true);
    setConsoleLines([]);
    setPhaseResults([]);
    setTotalDuration(0);
    addConsoleLine('🚀 正在启动采集任务...', 'system');

    try {
      const res: any = await triggerCollection(phases);
      const streamId = res.streamId;
      setCollecting(true);
      startTimeRef.current = Date.now();

      const es = new EventSource(`http://127.0.0.1:8080/api/v1/collector/stream?id=${streamId}`);
      eventSourceRef.current = es;

      es.onmessage = (event) => {
        try {
          const line: SSELine = JSON.parse(event.data);
          if (line.type === 'connected') return;

          if (line.type === 'log' && line.message) {
            addConsoleLine(line.message, line.level || 'info');
          } else if (line.type === 'phase' && line.message) {
            addConsoleLine(`\n━━━ ${line.message} ━━━`, 'phase');
          } else if (line.type === 'result' && line.result) {
            setPhaseResults(prev => [...prev, line.result!]);
            const r = line.result;
            const label = PHASE_LABELS[r.phase] || r.phase;
            const emoji = r.errors > 0 ? '⚠️' : '✅';
            addConsoleLine(
              `${emoji} ${label}: 总计${r.total} | 新增${r.new} | 跳过${r.skipped} | 耗时${(r.durationMs / 1000).toFixed(1)}s`,
              r.errors > 0 ? 'stderr' : 'success'
            );
          } else if (line.type === 'done') {
            addConsoleLine(`\n${line.message}`, 'success');
            setCollecting(false);
            setTotalDuration(Date.now() - startTimeRef.current);
            es.close();
            eventSourceRef.current = null;
            loadProgress();
            showToast('success', '采集完成');
          }
        } catch {}
      };

      es.onerror = () => {
        if (!collecting) {
          es.close();
          eventSourceRef.current = null;
        }
      };

      pollRef.current = setInterval(async () => {
        try {
          const pr: any = await fetchCollectorProgress();
          setProgress(pr.data);
          if (!pr.data?.running && collecting) {
            setCollecting(false);
            setTotalDuration(Date.now() - startTimeRef.current);
            clearInterval(pollRef.current);
            pollRef.current = null;
            eventSourceRef.current?.close();
            loadProgress();
          }
        } catch {}
      }, 3000);

    } catch {
      showToast('error', '触发采集失败');
      setCollecting(false);
    }
    setLoading(false);
  };

  const handleStop = () => {
    eventSourceRef.current?.close();
    clearInterval(pollRef.current);
    setCollecting(false);
    addConsoleLine('⏹ 用户停止了监控（采集进程仍在服务端运行）', 'system');
  };

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    return `${Math.floor(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`;
  };

  const renderConsoleLine = (line: { text: string; level: string; time: string }, i: number) => {
    const colorMap: Record<string, string> = {
      info: '#e5e6eb',
      stderr: '#f76965',
      success: '#00b42a',
      phase: '#4080ff',
      system: '#ffb400',
    };
    return (
      <div key={i} style={{
        fontFamily: '"JetBrains Mono", "Fira Code", "SF Mono", monospace',
        fontSize: 12,
        lineHeight: '18px',
        color: colorMap[line.level] || '#e5e6eb',
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-all',
        padding: line.level === 'phase' ? '4px 0' : '0',
        fontWeight: line.level === 'phase' ? 600 : 400,
      }}>
        <span style={{ color: '#666', marginRight: 8, userSelect: 'none' }}>{line.time}</span>
        {line.text}
      </div>
    );
  };

  return (
    <div>
      {toast && <Toast type={toast.type} msg={toast.msg} onClose={() => setToast(null)} />}

      <div className="page-header">
        <h2><Database size={20} style={{ marginRight: 8 }} />数据管理</h2>
        <span className="muted">手动采集 · Excel 导入 · 采集记录</span>
      </div>

      {/* Tab bar */}
      <div style={{ display: 'flex', gap: 0, marginBottom: 16, background: '#fff', borderRadius: 6, border: '1px solid #e5e6eb', overflow: 'hidden' }}>
        {[
          { key: 'collect', label: '手动采集', icon: <Terminal size={14} /> },
          { key: 'import', label: 'Excel 导入', icon: <UploadIcon size={14} /> },
          { key: 'history', label: '采集记录', icon: <History size={14} /> },
        ].map(t => (
          <button key={t.key} onClick={() => setTab(t.key as any)} style={{
            padding: '10px 20px', border: 'none', cursor: 'pointer', fontSize: 13,
            background: tab === t.key ? '#e8f3ff' : 'transparent',
            color: tab === t.key ? '#165dff' : '#4e5969',
            fontWeight: tab === t.key ? 500 : 400,
            display: 'flex', alignItems: 'center', gap: 6,
            borderRight: '1px solid #e5e6eb',
          }}>{t.icon}{t.label}</button>
        ))}
      </div>

      {/* === COLLECT TAB === */}
      {tab === 'collect' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {/* Control Bar */}
          <div className="card">
            <div className="card-header">
              <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <Activity size={16} color="#165dff" />
                <span style={{ fontSize: 15, fontWeight: 600 }}>采集控制台</span>
              </span>
              {collecting && (
                <Tag color="blue" style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                  <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#165dff', display: 'inline-block' }} />
                  采集中...
                </Tag>
              )}
            </div>
            <div className="card-body">
              <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: collecting ? 12 : 0 }}>
                <Button
                  type="primary"
                  loading={loading}
                  disabled={collecting}
                  icon={<Play size={14} />}
                  onClick={() => handleTrigger()}
                >
                  全量采集
                </Button>
                {['indicator', 'kline', 'quote', 'industry', 'full_sync', 'shareholder', 'financial', 'news', 'reports'].map(phase => (
                  <Button
                    key={phase}
                    size="small"
                    type="outline"
                    disabled={collecting}
                    onClick={() => handleTrigger([phase])}
                    style={{ fontSize: 12 }}
                  >
                    {PHASE_LABELS[phase] || phase}
                  </Button>
                ))}
                {collecting && (
                  <Button
                    type="outline"
                    status="danger"
                    icon={<Square size={14} />}
                    onClick={handleStop}
                  >
                    停止监控
                  </Button>
                )}
                <span className="muted" style={{ fontSize: 12 }}>
                  点击「全量采集」或单独点击各阶段按钮。已有数据自动跳过，仅采集缺失部分。
                </span>
              </div>
            </div>
          </div>

          {/* Phase Results Summary */}
          {phaseResults.length > 0 && (
            <div className="card">
              <div className="card-header">
                <span style={{ fontSize: 14, fontWeight: 600 }}>阶段结果</span>
                {totalDuration > 0 && (
                  <span className="muted" style={{ fontSize: 12 }}>总耗时: {formatDuration(totalDuration)}</span>
                )}
              </div>
              <div className="card-body" style={{ padding: '12px 16px' }}>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 10 }}>
                  {phaseResults.map((r) => (
                    <div key={r.phase} style={{
                      border: `1px solid ${r.errors > 0 ? '#f53f3f' : '#e5e6eb'}`,
                      borderRadius: 6,
                      padding: '10px 12px',
                      background: r.errors > 0 ? '#ffece8' : '#f7f8fa',
                    }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
                        <div style={{ width: 8, height: 8, borderRadius: '50%', background: PHASE_COLORS[r.phase] || '#666' }} />
                        <span style={{ fontSize: 13, fontWeight: 600 }}>{PHASE_LABELS[r.phase] || r.phase}</span>
                      </div>
                      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '2px 16px', fontSize: 12, color: '#4e5969' }}>
                        <span>总计</span><span style={{ textAlign: 'right', fontWeight: 600 }}>{r.total}</span>
                        <span>新增</span><span style={{ textAlign: 'right', color: '#165dff', fontWeight: 600 }}>{r.new}</span>
                        <span>跳过</span><span style={{ textAlign: 'right', color: '#86909c' }}>{r.skipped}</span>
                        {r.errors > 0 && <><span>错误</span><span style={{ textAlign: 'right', color: '#f53f3f' }}>{r.errors}</span></>}
                        <span>耗时</span><span style={{ textAlign: 'right' }}>{(r.durationMs / 1000).toFixed(1)}s</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}

          {/* Terminal Console */}
          {consoleLines.length > 0 && (
            <div className="card">
              <div className="card-header" style={{ background: '#1a1a2e', color: '#e5e6eb', borderBottom: '1px solid #333' }}>
                <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <Terminal size={14} color="#00b42a" />
                  <span style={{ fontSize: 13, fontWeight: 600 }}>采集输出</span>
                </span>
                <span style={{ fontSize: 11, color: '#666' }}>{consoleLines.length} 行</span>
              </div>
              <div
                ref={consoleRef}
                style={{
                  background: '#0d0d1a',
                  color: '#e5e6eb',
                  padding: '12px 16px',
                  maxHeight: 400,
                  overflow: 'auto',
                  fontFamily: '"JetBrains Mono", "Fira Code", "SF Mono", monospace',
                  fontSize: 12,
                  lineHeight: '18px',
                  borderBottomLeftRadius: 6,
                  borderBottomRightRadius: 6,
                }}
              >
                {consoleLines.map((line, i) => renderConsoleLine(line, i))}
              </div>
            </div>
          )}

          {/* Progress bar */}
          {collecting && progress?.results && (
            <div className="card">
              <div className="card-header">
                <span style={{ fontSize: 14, fontWeight: 600 }}>采集进度</span>
              </div>
              <div className="card-body">
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <Progress
                    percent={Math.round((progress.current / Math.max(progress.total, 1)) * 100)}
                    style={{ flex: 1 }}
                    status={progress.running ? 'normal' : 'success'}
                  />
                  <span style={{ fontSize: 12, color: '#86909c' }}>{progress.current}/{progress.total}</span>
                </div>
                <div style={{ fontSize: 12, color: '#4e5969', marginTop: 8 }}>
                  当前: {PHASE_LABELS[progress.phase] || progress.phase} — {progress.message}
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* === IMPORT TAB === */}
      {tab === 'import' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="card">
            <div className="card-header">
              <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <FileSpreadsheet size={16} color="#165dff" />
                <span style={{ fontSize: 15, fontWeight: 600 }}>上传 Excel 文件</span>
              </span>
              <span className="muted" style={{ fontSize: 12 }}>支持 .xlsx / .xlsm</span>
            </div>
            <div className="card-body">
              <Upload drag accept=".xlsx,.xlsm" autoUpload={false} disabled={loading}
                onChange={(_, file) => {
                  setLoading(true); setResult(null);
                  uploadExcel(file.originFile as File)
                    .then((res: any) => { setResult(res.data); showToast('success', 'Excel 导入完成'); })
                    .catch((err: any) => showToast('error', err?.response?.data?.error || '导入失败'))
                    .finally(() => setLoading(false));
                  return false;
                }}
                tip="拖拽或点击上传，参考文件: MSS20260603.xlsm" />
              {loading && (
                <div style={{ marginTop: 16, padding: '12px 16px', background: '#e8f3ff', borderRadius: 4, display: 'flex', alignItems: 'center', gap: 10, fontSize: 13, color: '#165dff' }}>
                  <RefreshCw size={14} className="spin" />正在解析并导入数据...
                </div>
              )}
            </div>
          </div>
          {result && (
            <div className="card">
              <div className="card-header">
                <span style={{ fontSize: 15, fontWeight: 600 }}>导入结果</span>
                <span className="muted" style={{ fontSize: 12 }}>{result.fileName}</span>
              </div>
              <div className="card-body">
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 16, marginBottom: 16 }}>
                  {[
                    { label: '交易日数', value: result.datesImported, color: '#165dff' },
                    { label: '上榜记录', value: result.picksImported, color: '#f53f3f' },
                    { label: '信号数据', value: result.signalsImported, color: '#00b42a' },
                    { label: '新增个股', value: result.stocksCreated, color: '#ff7d00' },
                  ].map(item => (
                    <div key={item.label} style={{ textAlign: 'center', padding: '12px', background: '#f7f8fa', borderRadius: 6 }}>
                      <div style={{ fontSize: 24, fontWeight: 700, color: item.color, fontFamily: 'var(--font-family-mono, monospace)' }}>{item.value}</div>
                      <div style={{ fontSize: 12, color: '#86909c', marginTop: 4 }}>{item.label}</div>
                    </div>
                  ))}
                </div>
                {result.previews?.map((p: string, i: number) => (
                  <div key={i} style={{ padding: '8px 12px', background: '#f7f8fa', borderRadius: 4, fontSize: 13, color: '#4e5969', marginBottom: 4, display: 'flex', alignItems: 'center', gap: 6 }}>
                    <CheckCircle size={12} color="#00b42a" />{p}
                  </div>
                ))}
                {result.errors?.map((e: string, i: number) => (
                  <div key={i} style={{ padding: '8px 12px', background: '#ffece8', borderRadius: 4, fontSize: 12, color: '#cb272d', marginBottom: 4 }}>
                    <XCircle size={12} style={{ marginRight: 6 }} />{e}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* === HISTORY TAB === */}
      {tab === 'history' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {/* Collection History */}
          <div className="card">
            <div className="card-header">
              <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <Activity size={16} color="#165dff" />
                <span style={{ fontSize: 15, fontWeight: 600 }}>采集记录</span>
              </span>
              <Button size="small" type="text" icon={<RefreshCw size={12} />} onClick={loadColHistory}>刷新</Button>
            </div>
            <div className="card-body" style={{ padding: 0 }}>
              {colHistory.length === 0 ? (
                <div style={{ padding: 40, textAlign: 'center', color: '#86909c', fontSize: 13 }}>暂无采集记录</div>
              ) : (
                <Table
                  data={colHistory}
                  rowKey="id"
                  size="small"
                  columns={[
                    {
                      title: '状态',
                      dataIndex: 'status',
                      width: 80,
                      render: (v: string) =>
                        v === 'success' ? <Tag color="green">成功</Tag> :
                        v === 'partial' ? <Tag color="orange">部分</Tag> :
                        v === 'running' ? <Tag color="blue">运行中</Tag> :
                        <Tag color="red">失败</Tag>
                    },
                    { title: '新增', dataIndex: 'totalNew', width: 70, render: (v: number) => <span style={{ fontWeight: 600, color: '#165dff' }}>{v}</span> },
                    { title: '跳过', dataIndex: 'totalSkipped', width: 70, render: (v: number) => <span style={{ color: '#86909c' }}>{v}</span> },
                    {
                      title: '错误',
                      dataIndex: 'totalErrors',
                      width: 60,
                      render: (v: number) => v > 0 ? <span style={{ color: '#f53f3f', fontWeight: 600 }}>{v}</span> : <span style={{ color: '#86909c' }}>0</span>
                    },
                    {
                      title: '耗时',
                      dataIndex: 'durationMs',
                      width: 80,
                      render: (v: number) => <span style={{ color: '#4e5969', fontSize: 12 }}>{formatDuration(v)}</span>
                    },
                    { title: '开始时间', dataIndex: 'startedAt', width: 160, render: (v: string) => <span style={{ fontSize: 12 }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span> },
                  ]}
                  pagination={false}
                  border={false}
                  stripe
                />
              )}
            </div>
          </div>

          {/* Import History (Excel) */}
          <div className="card">
            <div className="card-header">
              <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <FileSpreadsheet size={16} color="#165dff" />
                <span style={{ fontSize: 15, fontWeight: 600 }}>Excel 导入记录</span>
              </span>
              <Button size="small" type="text" icon={<RefreshCw size={12} />} onClick={loadHistory}>刷新</Button>
            </div>
            <div className="card-body" style={{ padding: 0 }}>
              {history.length === 0 ? (
                <div style={{ padding: 40, textAlign: 'center', color: '#86909c', fontSize: 13 }}>暂无导入记录</div>
              ) : (
                <Table data={history} rowKey="id" size="small" columns={[
                  { title: '文件名', dataIndex: 'fileName', width: 200 },
                  { title: '导入条数', dataIndex: 'rowsImported', width: 100, render: (v: number) => <span style={{ fontWeight: 600 }}>{v}</span> },
                  { title: '状态', dataIndex: 'status', width: 100, render: (v: string) => v === 'success' ? <Tag color="green">成功</Tag> : v === 'partial' ? <Tag color="orange">部分</Tag> : <Tag color="red">失败</Tag> },
                  { title: '时间', dataIndex: 'importedAt', width: 180, render: (v: string) => <span style={{ color: '#86909c', fontSize: 12 }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span> },
                ]} pagination={false} border={false} stripe />
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
