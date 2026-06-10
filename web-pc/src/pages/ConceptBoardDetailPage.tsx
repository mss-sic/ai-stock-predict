import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { fetchConceptBoardStocks } from '../services/api';
import { ArrowLeft, TrendingUp, TrendingDown, BarChart3, Layers } from 'lucide-react';

export default function ConceptBoardDetailPage() {
  const { code } = useParams<{ code: string }>();
  const navigate = useNavigate();
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!code) return;
    setLoading(true);
    fetchConceptBoardStocks(code)
      .then((res: any) => setData(res.data?.data))
      .finally(() => setLoading(false));
  }, [code]);

  if (loading) {
    return <div style={{ padding: 40, textAlign: 'center', color: '#86909c' }}>加载中...</div>;
  }
  if (!data) {
    return <div style={{ padding: 40, textAlign: 'center', color: '#86909c' }}>板块不存在</div>;
  }

  const { board, stocks, upCount, downCount, avgChgPct, total } = data;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <button onClick={() => navigate(-1)} style={{
          width: 32, height: 32, borderRadius: 6, border: '1px solid #e5e6eb',
          background: '#fff', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center'
        }}>
          <ArrowLeft size={16} color="#4e5969" />
        </button>
        <div style={{ width: 36, height: 36, borderRadius: 8, background: 'linear-gradient(135deg, #165dff, #722ed1)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Layers size={18} color="#fff" />
        </div>
        <div>
          <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>{board?.conceptName || code}</h2>
          <span style={{ fontSize: 12, color: '#86909c' }}>{board?.conceptType === 'industry' ? '行业板块' : '概念板块'} · {total} 只成分股</span>
        </div>
      </div>

      {/* Stats Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 10 }}>
        <div style={{ background: '#fff', borderRadius: 8, border: '1px solid #e5e6eb', padding: '14px 16px' }}>
          <div style={{ fontSize: 11, color: '#86909c', marginBottom: 4 }}>成分股数量</div>
          <div style={{ fontSize: 22, fontWeight: 600, color: '#1d2129' }}>{total}</div>
        </div>
        <div style={{ background: '#fff', borderRadius: 8, border: '1px solid #e5e6eb', padding: '14px 16px' }}>
          <div style={{ fontSize: 11, color: '#86909c', marginBottom: 4 }}>平均涨跌</div>
          <div style={{ fontSize: 22, fontWeight: 600, color: (avgChgPct || 0) >= 0 ? '#f53f3f' : '#00b42a' }}>
            {(avgChgPct || 0) >= 0 ? '+' : ''}{(avgChgPct || 0).toFixed(2)}%
          </div>
        </div>
        <div style={{ background: '#fff', borderRadius: 8, border: '1px solid #e5e6eb', padding: '14px 16px' }}>
          <div style={{ fontSize: 11, color: '#86909c', marginBottom: 4 }}>上涨</div>
          <div style={{ fontSize: 22, fontWeight: 600, color: '#f53f3f', display: 'flex', alignItems: 'center', gap: 4 }}>
            <TrendingUp size={16} />{upCount || 0}
          </div>
        </div>
        <div style={{ background: '#fff', borderRadius: 8, border: '1px solid #e5e6eb', padding: '14px 16px' }}>
          <div style={{ fontSize: 11, color: '#86909c', marginBottom: 4 }}>下跌</div>
          <div style={{ fontSize: 22, fontWeight: 600, color: '#00b42a', display: 'flex', alignItems: 'center', gap: 4 }}>
            <TrendingDown size={16} />{downCount || 0}
          </div>
        </div>
      </div>

      {/* Stock List */}
      <div style={{ background: '#fff', borderRadius: 8, border: '1px solid #e5e6eb', overflow: 'hidden' }}>
        <div style={{ padding: '12px 16px', borderBottom: '1px solid #f2f3f5', display: 'flex', alignItems: 'center', gap: 8 }}>
          <BarChart3 size={14} color="#4e5969" />
          <span style={{ fontSize: 14, fontWeight: 600 }}>成分股列表</span>
        </div>
        <div style={{ overflow: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ background: '#f7f8fa', color: '#86909c', fontSize: 11 }}>
                <th style={{ padding: '8px 12px', textAlign: 'left', fontWeight: 500 }}>代码</th>
                <th style={{ padding: '8px 12px', textAlign: 'left', fontWeight: 500 }}>名称</th>
                <th style={{ padding: '8px 12px', textAlign: 'right', fontWeight: 500 }}>最新价</th>
                <th style={{ padding: '8px 12px', textAlign: 'right', fontWeight: 500 }}>涨跌幅</th>
                <th style={{ padding: '8px 12px', textAlign: 'right', fontWeight: 500 }}>市值</th>
              </tr>
            </thead>
            <tbody>
              {(stocks || []).map((s: any, i: number) => (
                <tr key={i} onClick={() => navigate(`/stock/${s.code}`)} style={{
                  borderBottom: '1px solid #f2f3f5', cursor: 'pointer',
                  transition: 'background .15s',
                }}
                  onMouseEnter={e => (e.currentTarget.style.background = '#f7f8fa')}
                  onMouseLeave={e => (e.currentTarget.style.background = '')}>
                  <td style={{ padding: '8px 12px', fontFamily: 'monospace', color: '#86909c', fontSize: 12 }}>{s.code}</td>
                  <td style={{ padding: '8px 12px', fontWeight: 500 }}>{s.stockName || s.code}</td>
                  <td style={{ padding: '8px 12px', textAlign: 'right', fontFamily: 'monospace' }}>{s.close > 0 ? s.close.toFixed(2) : '-'}</td>
                  <td style={{ padding: '8px 12px', textAlign: 'right', fontFamily: 'monospace', color: (s.chgPct || 0) >= 0 ? '#f53f3f' : '#00b42a', fontWeight: 500 }}>
                    {(s.chgPct || 0) >= 0 ? '+' : ''}{(s.chgPct || 0).toFixed(2)}%
                  </td>
                  <td style={{ padding: '8px 12px', textAlign: 'right', color: '#86909c', fontSize: 12 }}>
                    {s.marketCap > 0 ? (s.marketCap / 1e8).toFixed(1) + '亿' : '-'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
