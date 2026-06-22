import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { Tag } from '@arco-design/web-react';
import BacktestReport from '../components/BacktestReport';
import { fetchBacktestResult, fetchBacktestTaskLogs } from '../services/api';

export default function BacktestDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [loading, setLoading] = useState(true);
  const [result, setResult] = useState<any>(null);
  const [logs, setLogs] = useState<any[] | null>(null);

  useEffect(() => {
    if (!id) return;
    (async () => {
      try {
        const { data: r } = await fetchBacktestResult(Number(id));
        const res = r.data || null;
        setResult(res);
        // Fetch logs if result has taskId
        if (res?.taskId && res?.strategyId) {
          try {
            const { data: lr } = await fetchBacktestTaskLogs(res.strategyId, res.taskId);
            setLogs(lr.data?.logs || lr.data || null);
          } catch (e) {
            console.error('[BacktestDetailPage] fetch logs failed:', e);
          }
        }
      } catch (e) {
        console.error('[BacktestDetailPage] fetch result failed:', e);
      } finally {
        setLoading(false);
      }
    })();
  }, [id]);

  return (
    <BacktestReport
      result={result}
      loading={loading}
      title={`回测详情 #${id}`}
      subtitle={result ? `${result.startDate?.slice(0, 10) || '-'} ~ ${result.endDate?.slice(0, 10) || '-'} · 策略 #${result.strategyId}` : ''}
      headerExtra={<Tag color="green" style={{ borderRadius: 6 }}>已完成</Tag>}
      backPath="/strategy"
      showEquityCurve
      logs={logs}
    />
  );
}
