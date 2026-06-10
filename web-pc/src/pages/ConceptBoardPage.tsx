import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { fetchConceptHeatmap } from '../services/api';
import { Flame } from 'lucide-react';

export default function ConceptBoardPage() {
  const [conceptData, setConceptData] = useState<any[]>([]);
  const navigate = useNavigate();

  useEffect(() => {
    fetchConceptHeatmap().then((res: any) => setConceptData(res.data?.data || []));
  }, []);

  const maxChg = Math.max(...conceptData.map(d => Math.abs(d.avgChgPct || 0)), 1);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{
        padding: '12px 18px', display: 'flex', alignItems: 'center', gap: 14,
        background: '#fff', borderRadius: 6, border: '1px solid #e5e6eb',
      }}>
        <Flame size={18} color="#f53f3f" />
        <span style={{ fontSize: 16, fontWeight: 600, color: '#1d2129' }}>概念板块热力图</span>
        <span style={{ fontSize: 12, color: '#86909c' }}>行业板块涨跌分布 · 按成分股数量排序</span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 8 }}>
        {conceptData.map((item: any, idx: number) => {
          const pct = item.avgChgPct || 0;
          const t = Math.min(1, Math.abs(pct) / maxChg);
          const isUp = pct > 0.05;
          const isDown = pct < -0.05;
          const bg = isUp ? `rgba(245,63,63,${(0.15 + t * 0.8).toFixed(3)})`
                   : isDown ? `rgba(0,180,42,${(0.15 + t * 0.8).toFixed(3)})`
                   : '#f2f3f5';
          const fg = t > 0.5 ? '#fff' : isUp ? '#cb272d' : isDown ? '#008026' : '#86909c';
          return (
            <div key={idx} onClick={() => navigate('/concept/' + item.conceptCode)} style={{
              background: bg, color: fg, borderRadius: 6, padding: '8px 10px',
              cursor: 'pointer', fontSize: 12, transition: 'transform .15s',
              border: `1px solid ${t > 0.3 ? 'transparent' : '#e5e6eb'}`,
            }}
              onMouseEnter={e => (e.currentTarget.style.transform = 'scale(1.03)')}
              onMouseLeave={e => (e.currentTarget.style.transform = 'scale(1)')}>
              <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 2 }}>{item.conceptName}</div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11, opacity: 0.9 }}>
                <span>{item.stockCount}只</span>
                <span>{pct > 0 ? '+' : ''}{pct.toFixed(2)}%</span>
              </div>
              <div style={{ display: 'flex', gap: 6, fontSize: 10, marginTop: 3, opacity: 0.8 }}>
                <span>▲{item.upCount || 0}</span>
                <span>▼{item.downCount || 0}</span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
