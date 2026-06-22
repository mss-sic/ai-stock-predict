import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Tag } from '@arco-design/web-react';
import { Trophy } from 'lucide-react';
import BacktestReport from '../components/BacktestReport';
import api from '../services/api';

export default function PkEntryDetailPage() {
  const { id, entryId } = useParams<{ id: string; entryId: string }>();
  const navigate = useNavigate();
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    (async () => {
      try {
        const res = await api.get(`/pk/entries/${entryId}/detail`);
        setData(res.data.data);
      } catch {}
      setLoading(false);
    })();
  }, [entryId]);

  if (!data) return <BacktestReport result={null} loading={loading} title="加载中..." />;

  const { entry, strategy, result, logs } = data;

  return (
    <BacktestReport
      result={result}
      loading={loading}
      title={entry.username || `选手 #${entry.userId}`}
      subtitle={`${entry.strategyName || '未知策略'}${entry.finalRank > 0 ? ` · 🏅 第 ${entry.finalRank} 名` : ''}`}
      headerExtra={
        <>
          {entry.status === 'running' && <Tag color="blue" style={{ borderRadius: 6 }}>进行中</Tag>}
          {entry.status === 'completed' && <Tag color="green" style={{ borderRadius: 6 }}>已完成</Tag>}
          {entry.status === 'pending' && <Tag color="orange" style={{ borderRadius: 6 }}>等待中</Tag>}
        </>
      }
      headerIcon={<Trophy size={20} color="#fff" />}
      backPath={`/pk/${id}`}
      showEquityCurve
      strategyParams={strategy}
      strategyConditions={strategy?.conditions || null}
      logs={logs || null}
    />
  );
}
