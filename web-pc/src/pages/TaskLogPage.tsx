import { useState, useEffect, useCallback } from 'react';
import { Table, Tag, Card, DatePicker, Select, Switch, Drawer, Empty, Message } from '@arco-design/web-react';
import { Clock, CheckCircle, XCircle, RefreshCw, AlertTriangle, ListFilter } from 'lucide-react';
import { fetchSchedulerLogs, fetchSchedulerStats } from '../services/api';

interface TaskLog {
  id: number; taskName: string; phase: string; status: string;
  errorMsg: string; durationMs: number; result: string;
  startedAt: string; finishedAt?: string;
}

interface Stats {
  total24h: number; failed24h: number; running24h: number;
  slowTasks24h: number; successRate: number;
}

const TaskLogPage = () => {
  const [logs, setLogs] = useState<TaskLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);
  const [loading, setLoading] = useState(false);
  const [statusFilter, setStatusFilter] = useState('');
  const [phaseFilter, setPhaseFilter] = useState('');
  const [dateRange, setDateRange] = useState<string[]>([]);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [stats, setStats] = useState<Stats | null>(null);
  const [selectedLog, setSelectedLog] = useState<TaskLog | null>(null);
  const [phases, setPhases] = useState<string[]>([]);

  const loadLogs = useCallback(async () => {
    setLoading(true);
    try {
      const { data: r } = await fetchSchedulerLogs({
        page, pageSize, status: statusFilter, phase: phaseFilter,
        dateFrom: dateRange[0], dateTo: dateRange[1],
      });
      setLogs(r?.data?.items || []);
      setTotal(r?.data?.total || 0);
      const allPhases = [...new Set((r?.data?.items || []).map((l: TaskLog) => l.phase))] as string[];
      setPhases(prev => [...new Set([...prev, ...allPhases])]);
    } catch (e) { console.error('load logs', e); }
    setLoading(false);
  }, [page, pageSize, statusFilter, phaseFilter, dateRange]);

  const loadStats = useCallback(async () => {
    try {
      const { data: r } = await fetchSchedulerStats();
      setStats(r?.data || null);
    } catch (e) { /* ignore */ }
  }, []);

  useEffect(() => { loadLogs(); loadStats(); }, [loadLogs, loadStats]);

  useEffect(() => {
    if (!autoRefresh) return;
    const t = setInterval(() => { loadLogs(); loadStats(); }, 10000);
    return () => clearInterval(t);
  }, [autoRefresh, loadLogs, loadStats]);

  const statusBadge = (s: string) => {
    const map: Record<string, { color: string; text: string }> = {
      success: { color: 'green', text: '成功' },
      failed: { color: 'red', text: '失败' },
      running: { color: 'blue', text: '运行中' },
    };
    const c = map[s] || { color: 'gray', text: s };
    return <Tag color={c.color}>{c.text}</Tag>;
  };

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`;
  };

  const parseTraceID = (result: string) => {
    try {
      const r = JSON.parse(result);
      return r.trace_id || '';
    } catch { return ''; }
  };

  return (
    <div style={{ padding: 20 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <div>
          <h2 style={{ margin: 0, fontSize: 20, fontWeight: 700 }}>任务执行历史</h2>
          <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 2 }}>
            所有定时任务和策略执行的运行记录与错误详情
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>自动刷新</span>
          <Switch size="small" checked={autoRefresh} onChange={setAutoRefresh} />
          <RefreshCw size={16} style={{ cursor: 'pointer', color: 'var(--color-text-2)' }} onClick={() => { loadLogs(); loadStats(); }} />
        </div>
      </div>

      {/* Stats Cards */}
      {stats && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 16 }}>
          <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <CheckCircle size={18} style={{ color: '#00B42A' }} />
              <div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>24h 成功率</div>
                <div style={{ fontSize: 18, fontWeight: 700, color: stats.successRate >= 90 ? '#00B42A' : '#F53F3F' }}>
                  {stats.successRate}%
                </div>
              </div>
            </div>
          </Card>
          <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <XCircle size={18} style={{ color: '#F53F3F' }} />
              <div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>24h 失败</div>
                <div style={{ fontSize: 18, fontWeight: 700, color: stats.failed24h > 0 ? '#F53F3F' : 'var(--color-text-1)' }}>
                  {stats.failed24h}
                </div>
              </div>
            </div>
          </Card>
          <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <AlertTriangle size={18} style={{ color: '#FF7D00' }} />
              <div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>慢任务</div>
                <div style={{ fontSize: 18, fontWeight: 700, color: stats.slowTasks24h > 0 ? '#FF7D00' : 'var(--color-text-1)' }}>
                  {stats.slowTasks24h}
                </div>
              </div>
            </div>
          </Card>
          <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <Clock size={18} style={{ color: 'var(--color-text-2)' }} />
              <div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>24h 总计</div>
                <div style={{ fontSize: 18, fontWeight: 700 }}>{stats.total24h}</div>
              </div>
            </div>
          </Card>
        </div>
      )}

      {/* Filters */}
      <Card style={{ marginBottom: 16, background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
        <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <ListFilter size={16} style={{ color: 'var(--color-text-3)' }} />
          <Select
            placeholder="全部状态" allowClear style={{ width: 130 }} size="small"
            value={statusFilter || undefined} onChange={(v) => { setStatusFilter(v || ''); setPage(1); }}
            options={[
              { value: 'success', label: '成功' },
              { value: 'failed', label: '失败' },
              { value: 'running', label: '运行中' },
            ]}
          />
          <Select
            placeholder="全部阶段" allowClear style={{ width: 180 }} size="small"
            value={phaseFilter || undefined} onChange={(v) => { setPhaseFilter(v || ''); setPage(1); }}
            options={phases.map(p => ({ value: p, label: p }))}
          />
          <DatePicker.RangePicker
            size="small" style={{ width: 240 }}
            onChange={(v) => { setDateRange(v || []); setPage(1); }}
          />
        </div>
      </Card>

      {/* Table */}
      <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
        <Table
          data={logs} loading={loading} rowKey="id" size="small"
          pagination={{
            current: page, pageSize, total,
            onChange: (p) => setPage(p),
            showTotal: true,
          }}
          columns={[
            { title: '任务名', dataIndex: 'taskName', width: 160, render: (v: string) => <span style={{ fontWeight: 500, fontSize: 12 }}>{v}</span> },
            { title: '阶段', dataIndex: 'phase', width: 140, render: (v: string) => <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>{v}</span> },
            { title: '状态', dataIndex: 'status', width: 80, render: (v: string) => statusBadge(v) },
            { title: '开始时间', dataIndex: 'startedAt', width: 160, render: (v: string) => <span style={{ fontSize: 12 }}>{v ? new Date(v).toLocaleString() : '-'}</span> },
            { title: '耗时', dataIndex: 'durationMs', width: 100, render: (v: number) => <span style={{ fontSize: 12, fontFamily: "'SF Mono', monospace" }}>{formatDuration(v)}</span> },
            {
              title: '错误摘要', dataIndex: 'errorMsg', ellipsis: true,
              render: (v: string | undefined, r: TaskLog) => {
                if (!v) return <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>-</span>;
                return (
                  <span
                    style={{ color: '#F53F3F', fontSize: 12, cursor: 'pointer' }}
                    onClick={() => setSelectedLog(r)}
                  >
                    {v.length > 60 ? v.slice(0, 60) + '...' : v}
                  </span>
                );
              },
            },
          ]}
          onRow={(r) => ({ onClick: () => setSelectedLog(r), style: { cursor: 'pointer' } })}
          locale={{ emptyText: <Empty description="暂无执行记录" /> }}
          renderPagination={() => undefined}
        />
      </Card>

      {/* Detail Drawer */}
      <Drawer
        title="执行详情" width={520} visible={!!selectedLog}
        onCancel={() => setSelectedLog(null)} footer={null}
      >
        {selectedLog && (
          <div style={{ fontSize: 13 }}>
            <div style={{ marginBottom: 12 }}>
              <div style={{ color: 'var(--color-text-3)', fontSize: 11, marginBottom: 4 }}>任务名称</div>
              <div style={{ fontWeight: 600 }}>{selectedLog.taskName}</div>
            </div>
            <div style={{ marginBottom: 12 }}>
              <div style={{ color: 'var(--color-text-3)', fontSize: 11, marginBottom: 4 }}>阶段</div>
              <div>{selectedLog.phase}</div>
            </div>
            <div style={{ marginBottom: 12 }}>
              <div style={{ color: 'var(--color-text-3)', fontSize: 11, marginBottom: 4 }}>状态</div>
              {statusBadge(selectedLog.status)}
            </div>
            <div style={{ marginBottom: 12 }}>
              <div style={{ color: 'var(--color-text-3)', fontSize: 11, marginBottom: 4 }}>开始时间</div>
              <div>{new Date(selectedLog.startedAt).toLocaleString()}</div>
            </div>
            <div style={{ marginBottom: 12 }}>
              <div style={{ color: 'var(--color-text-3)', fontSize: 11, marginBottom: 4 }}>耗时</div>
              <div style={{ fontFamily: "'SF Mono', monospace" }}>{formatDuration(selectedLog.durationMs)}</div>
            </div>
            {selectedLog.errorMsg && (
              <div style={{ marginBottom: 12 }}>
                <div style={{ color: 'var(--color-text-3)', fontSize: 11, marginBottom: 4 }}>错误信息</div>
                <div style={{
                  background: '#FFF2F0', border: '1px solid #FFCCC7',
                  borderRadius: 6, padding: 12, color: '#F53F3F',
                  whiteSpace: 'pre-wrap', wordBreak: 'break-all', fontSize: 12,
                  maxHeight: 300, overflow: 'auto',
                }}>
                  {selectedLog.errorMsg}
                </div>
              </div>
            )}
            {selectedLog.result && parseTraceID(selectedLog.result) && (
              <div style={{ marginBottom: 12 }}>
                <div style={{ color: 'var(--color-text-3)', fontSize: 11, marginBottom: 4 }}>Trace ID</div>
                <div style={{
                  fontFamily: "'SF Mono', monospace", fontSize: 11,
                  background: 'var(--color-fill-1)', borderRadius: 4, padding: '4px 8px',
                }}>
                  {parseTraceID(selectedLog.result)}
                </div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 4 }}>
                  可在服务器 <code>docker logs aip-server | grep 此ID</code> 查看完整日志
                </div>
              </div>
            )}
          </div>
        )}
      </Drawer>
    </div>
  );
};

export default TaskLogPage;
