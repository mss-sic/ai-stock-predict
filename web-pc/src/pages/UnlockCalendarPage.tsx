import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Calendar, AlertTriangle, TrendingUp, DollarSign, BarChart3 } from 'lucide-react';
import { fetchAllFutureUnlocks } from '../services/api';

interface UnlockItem {
  id: number;
  code: string;
  name: string;
  freeDate: string;
  stockType: string;
  shares: number;
  ratio: number;
}

export default function UnlockCalendarPage() {
  const navigate = useNavigate();
  const [data, setData] = useState<UnlockItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [days, setDays] = useState(90);

  useEffect(() => {
    setLoading(true);
    fetchAllFutureUnlocks(days)
      .then((r: any) => setData(r.data?.data || []))
      .catch(() => setData([]))
      .finally(() => setLoading(false));
  }, [days]);

  // Group by month
  const grouped: Record<string, UnlockItem[]> = {};
  data.forEach((d) => {
    const month = d.freeDate?.slice(0, 7) || '未知';
    if (!grouped[month]) grouped[month] = [];
    grouped[month].push(d);
  });

  const months = Object.keys(grouped).sort();
  const totalShares = data.reduce((s, d) => s + (d.shares || 0), 0);
  const highRatio = data.filter(d => d.ratio > 5).length;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div style={{
          background: 'linear-gradient(135deg, #F53F3F, #FF7D00)', borderRadius: 10,
          width: 36, height: 36, display: 'flex', alignItems: 'center', justifyContent: 'center'
        }}>
          <AlertTriangle size={18} color="#fff" />
        </div>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700, color: 'var(--color-text-1)' }}>解禁日历</h2>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 8 }}>
          {[30, 60, 90, 180].map(d => (
            <button key={d} onClick={() => setDays(d)}
              style={{
                padding: '4px 12px', fontSize: 12, borderRadius: 6, border: '1px solid var(--color-border-2)',
                background: days === d ? 'var(--color-primary)' : 'transparent',
                color: days === d ? '#fff' : 'var(--color-text-2)', cursor: 'pointer',
              }}>
              未来{d}天
            </button>
          ))}
        </div>
      </div>

      {/* Summary cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12 }}>
        <div className="card" style={{ padding: '16px 20px' }}>
          <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>待解禁股票数</div>
          <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--color-text-1)' }}>{loading ? '-' : data.length}<span style={{ fontSize: 12, fontWeight: 400, color: 'var(--color-text-3)' }}>只</span></div>
        </div>
        <div className="card" style={{ padding: '16px 20px' }}>
          <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>解禁总股数</div>
          <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--color-text-1)' }}>{loading ? '-' : (totalShares / 1e8).toFixed(2)}<span style={{ fontSize: 12, fontWeight: 400, color: 'var(--color-text-3)' }}>亿股</span></div>
        </div>
        <div className="card" style={{ padding: '16px 20px' }}>
          <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>高比例解禁(&gt;5%)</div>
          <div style={{ fontSize: 24, fontWeight: 700, color: highRatio > 0 ? '#F53F3F' : 'var(--color-text-1)' }}>{loading ? '-' : highRatio}<span style={{ fontSize: 12, fontWeight: 400, color: 'var(--color-text-3)' }}>只</span></div>
        </div>
      </div>

      {loading ? (
        <div className="card" style={{ textAlign: 'center', padding: 60, color: 'var(--color-text-3)', fontSize: 13 }}>加载中...</div>
      ) : months.length === 0 ? (
        <div className="card" style={{ textAlign: 'center', padding: 60 }}>
          <Calendar size={40} style={{ color: 'var(--color-text-3)', marginBottom: 12 }} />
          <div style={{ color: 'var(--color-text-3)', fontSize: 13 }}>暂无解禁数据，请先在股票详情页采集解禁数据</div>
        </div>
      ) : (
        months.map(month => {
          const items = grouped[month];
          const monthTotal = items.reduce((s, d) => s + (d.shares || 0), 0);
          return (
            <div key={month} className="card">
              <div className="card-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <span style={{ fontWeight: 600, fontSize: 14, color: 'var(--color-text-1)' }}>
                  <Calendar size={14} style={{ marginRight: 6 }} />{month}
                </span>
                <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
                  {items.length} 只 · 解禁 {(monthTotal / 1e8).toFixed(2)} 亿股
                </span>
              </div>
              <div className="card-body" style={{ padding: 0 }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
                  <thead>
                    <tr style={{ background: 'var(--color-fill-1)', borderBottom: '2px solid var(--color-border-2)' }}>
                      {['解禁日期','股票代码','股票名称','解禁股类型','解禁数量(万股)','占总股本','操作'].map(h => (
                        <th key={h} style={{ padding: '8px 12px', textAlign: h === '解禁股类型' || h === '股票名称' ? 'left' : h === '操作' ? 'center' : 'right', fontSize: 11, color: 'var(--color-text-3)', fontWeight: 500 }}>{h}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {items.map((d, i) => (
                      <tr key={i} style={{ borderBottom: '1px solid var(--color-border-1)', cursor: 'pointer' }}
                        onClick={() => navigate(`/stock/${d.code}`)}>
                        <td style={{ padding: '6px 12px', fontSize: 12, color: '#F53F3F', fontWeight: 600 }}>{d.freeDate?.slice(0, 10)}</td>
                        <td style={{ padding: '6px 12px', fontFamily: 'monospace', fontSize: 12 }}>{d.code}</td>
                        <td style={{ padding: '6px 12px', fontSize: 12 }}>{d.name || d.code}</td>
                        <td style={{ padding: '6px 12px', fontSize: 11, color: 'var(--color-text-2)' }}>{d.stockType || '-'}</td>
                        <td style={{ padding: '6px 12px', textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>{(d.shares / 10000).toFixed(2)}</td>
                        <td style={{ padding: '6px 12px', textAlign: 'right', fontVariantNumeric: 'tabular-nums', color: d.ratio > 5 ? '#F53F3F' : 'var(--color-text-1)', fontWeight: d.ratio > 5 ? 600 : 400 }}>
                          {(d.ratio || 0).toFixed(2)}%
                        </td>
                        <td style={{ padding: '6px 12px', textAlign: 'center' }}>
                          <span style={{ fontSize: 10, padding: '2px 8px', borderRadius: 10, background: 'rgba(245,63,63,0.1)', color: '#F53F3F' }}>详情</span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          );
        })
      )}
    </div>
  );
}
