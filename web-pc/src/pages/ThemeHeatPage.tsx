import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { TrendingUp, Tag, Hash } from 'lucide-react';
import { fetchThsHotConcepts } from '../services/api';

interface ConceptStat {
  tag: string;
  stock_count: number;
  appear_count: number;
}

export default function ThemeHeatPage() {
  const navigate = useNavigate();
  const [data, setData] = useState<ConceptStat[]>([]);
  const [loading, setLoading] = useState(true);
  const [days, setDays] = useState(7);

  useEffect(() => {
    setLoading(true);
    fetchThsHotConcepts(days)
      .then((r: any) => setData(r.data?.data || []))
      .catch(() => setData([]))
      .finally(() => setLoading(false));
  }, [days]);

  const maxCount = data.length > 0 ? Math.max(...data.map(d => d.stock_count)) : 1;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div style={{
          background: 'linear-gradient(135deg, #165DFF, #722ED1)', borderRadius: 10,
          width: 36, height: 36, display: 'flex', alignItems: 'center', justifyContent: 'center'
        }}>
          <TrendingUp size={18} color="#fff" />
        </div>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700, color: 'var(--color-text-1)' }}>题材热度</h2>
        <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>基于同花顺每日强势股题材归因</span>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 8 }}>
          {[1, 3, 7, 30].map(d => (
            <button key={d} onClick={() => setDays(d)}
              style={{
                padding: '4px 12px', fontSize: 12, borderRadius: 6, border: '1px solid var(--color-border-2)',
                background: days === d ? 'var(--color-primary)' : 'transparent',
                color: days === d ? '#fff' : 'var(--color-text-2)', cursor: 'pointer',
              }}>
              {d === 1 ? '今日' : `近${d}天`}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="card" style={{ textAlign: 'center', padding: 60, color: 'var(--color-text-3)', fontSize: 13 }}>加载中...</div>
      ) : data.length === 0 ? (
        <div className="card" style={{ textAlign: 'center', padding: 60 }}>
          <Tag size={40} style={{ color: 'var(--color-text-3)', marginBottom: 12 }} />
          <div style={{ color: 'var(--color-text-3)', fontSize: 13 }}>暂无题材数据，请先运行同花顺热点采集</div>
        </div>
      ) : (
        <div className="card">
          <div className="card-header">
            <span style={{ fontWeight: 600, fontSize: 14 }}><Hash size={14} /> 热门题材覆盖</span>
            <span style={{ fontSize: 11, color: 'var(--color-text-3)', marginLeft: 8 }}>
              共 {data.length} 个题材 · 最大覆盖 {maxCount} 只股票
            </span>
          </div>
          <div className="card-body" style={{ padding: '12px 20px' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {data.map((d, i) => {
                const pct = (d.stock_count / maxCount) * 100;
                const colors = ['#165DFF', '#14C9C9', '#F77234', '#722ED1', '#F53F3F', '#FF7D00', '#00B42A', '#3491FA'];
                const color = colors[i % colors.length];
                return (
                  <div key={d.tag} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <span style={{ width: 80, fontSize: 12, fontWeight: 500, color: 'var(--color-text-1)', textAlign: 'right', flexShrink: 0 }}>
                      {d.tag}
                    </span>
                    <div style={{ flex: 1, height: 22, background: 'var(--color-fill-2)', borderRadius: 4, overflow: 'hidden' }}>
                      <div style={{
                        width: `${Math.max(pct, 2)}%`, height: '100%',
                        background: `linear-gradient(90deg, ${color}, ${color}88)`,
                        borderRadius: 4, transition: 'width 0.3s ease',
                        display: 'flex', alignItems: 'center', paddingLeft: 8
                      }}>
                        <span style={{ fontSize: 11, color: '#fff', fontWeight: 600 }}>{d.stock_count}只</span>
                      </div>
                    </div>
                    <span style={{ width: 50, fontSize: 10, color: 'var(--color-text-3)', flexShrink: 0 }}>
                      {d.appear_count > d.stock_count ? `出现${d.appear_count}次` : ''}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
