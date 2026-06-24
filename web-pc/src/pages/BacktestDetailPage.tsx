import { useState, useEffect, useRef } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { Tag, Spin } from '@arco-design/web-react';
import { Activity } from 'lucide-react';
import BacktestReport from '../components/BacktestReport';
import { fetchBacktestResult, fetchBacktestTaskLogs, getBacktestStatus } from '../services/api';

export default function BacktestDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams] = useSearchParams();
  const taskIdFromUrl = searchParams.get('taskId');
  const sidFromUrl = searchParams.get('sid');

  const [loading, setLoading] = useState(true);
  const [result, setResult] = useState<any>(null);
  const [logs, setLogs] = useState<any[] | null>(null);
  const [livePhase, setLivePhase] = useState('');
  const [liveProgress, setLiveProgress] = useState('');
  const [isRunning, setIsRunning] = useState(false);
  const pollRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    return () => { if (pollRef.current) clearTimeout(pollRef.current); };
  }, []);

  useEffect(() => {
    if (!id) return;
    (async () => {
      setLoading(true);
      try {
        const { data: r } = await fetchBacktestResult(Number(id));
        const res = r.data || null;
        setResult(res);

        // If result has task info, fetch logs
        if (res?.strategyId && res?.taskId) {
          fetchLogs(res.strategyId, res.taskId);
        }

        // If taskId is in URL params, start live polling
        if (taskIdFromUrl && sidFromUrl) {
          pollTaskLive(Number(sidFromUrl), Number(taskIdFromUrl));
        }
      } catch (e) {
        console.error('[BacktestDetailPage] fetch failed:', e);
      } finally {
        setLoading(false);
      }
    })();
  }, [id, taskIdFromUrl, sidFromUrl]);

  const fetchLogs = async (sid: number, tid: number) => {
    try {
      const { data: lr } = await fetchBacktestTaskLogs(sid, tid);
      setLogs(lr.data?.logs || lr.data || null);
    } catch (e) {
      console.error('[BacktestDetailPage] logs failed:', e);
    }
  };

  const pollTaskLive = (sid: number, tid: number) => {
    setIsRunning(true);
    const poll = async () => {
      try {
        const { data: r } = await getBacktestStatus(sid, tid);
        const t = r.data;
        if (!t) { scheduleNext(); return; }
        setLivePhase(t.phase || '');
        setLiveProgress(`${t.currentDay || 0}/${t.totalDays || 0} 交易日`);

        // Refresh logs periodically
        fetchLogs(sid, tid);

        if (['completed', 'failed', 'cancelled'].includes(t.status)) {
          setIsRunning(false);
          setLivePhase(t.status === 'completed' ? '回测完成' : t.status === 'failed' ? '回测失败' : '已取消');
          fetchLogs(sid, tid);
          return;
        }
        scheduleNext();
      } catch {
        scheduleNext(2000);
      }
    };
    const scheduleNext = (d = 1000) => {
      pollRef.current = setTimeout(poll, d);
    };
    poll();
  };

  const statusTag = isRunning
    ? <Tag color="blue" style={{ borderRadius: 6 }}><Activity size={12} style={{ marginRight: 4 }} />运行中</Tag>
    : <Tag color="green" style={{ borderRadius: 6 }}>已完成</Tag>;

  return (
    <Spin loading={loading} style={{ width: '100%' }}>
      {isRunning && (
        <div style={{
          marginBottom: 12, padding: '8px 14px',
          background: 'var(--color-primary-light-1)', borderRadius: 8,
          fontSize: 13, display: 'flex', justifyContent: 'space-between',
          alignItems: 'center', color: 'var(--color-primary)'
        }}>
          <span>⏳ {livePhase}</span>
          <span style={{ fontSize: 12, opacity: 0.8 }}>{liveProgress}</span>
        </div>
      )}
      <BacktestReport
        result={result}
        loading={loading}
        title={`回测详情 #${id}`}
        subtitle={result
          ? `${result.startDate?.slice(0, 10) || '-'} ~ ${result.endDate?.slice(0, 10) || '-'} · 策略 #${result.strategyId}`
          : ''}
        headerExtra={statusTag}
        backPath="/strategy"
        showEquityCurve
        logs={logs}
      />
    </Spin>
  );
}
