import { useEffect, useState } from 'react';
import { Globe, Filter, RefreshCw, Clock } from 'lucide-react';
import { fetchMacroNews, fetchMacroCategories } from '../services/api';

interface MacroItem {
  id: number;
  title: string;
  summary: string;
  newsTime: string;
  category: string;
}

const CAT_COLORS: Record<string, string> = {
  '货币政策': '#F53F3F', '财政政策': '#FF7D00', '行业政策': '#165DFF',
  '国际宏观': '#722ED1', '地缘政治': '#F5319D', '央行政策': '#F53F3F',
};

export default function MacroNewsPage() {
  const [data, setData] = useState<MacroItem[]>([]);
  const [categories, setCategories] = useState<string[]>([]);
  const [selectedCat, setSelectedCat] = useState('');
  const [loading, setLoading] = useState(true);

  const load = (cat: string = selectedCat) => {
    setLoading(true);
    fetchMacroNews(cat, 100)
      .then((r: any) => setData(r.data?.data || []))
      .catch(() => setData([]))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchMacroCategories()
      .then((r: any) => setCategories(r.data?.data || []))
      .catch(() => {});
    load('');
  }, []);

  const handleCatChange = (cat: string) => {
    setSelectedCat(cat);
    load(cat);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div style={{
          background: 'linear-gradient(135deg, #722ED1, #F5319D)', borderRadius: 10,
          width: 36, height: 36, display: 'flex', alignItems: 'center', justifyContent: 'center'
        }}>
          <Globe size={18} color="#fff" />
        </div>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700, color: 'var(--color-text-1)' }}>全球宏观资讯</h2>
        <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>7×24 实时滚动</span>
        <div style={{ marginLeft: 'auto' }}>
          <button onClick={() => load(selectedCat)} disabled={loading}
            style={{ padding: '6px 12px', fontSize: 11, borderRadius: 6, border: '1px solid var(--color-border-2)', background: 'var(--color-bg-1)', color: 'var(--color-text-2)', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 4 }}>
            <RefreshCw size={12} className={loading ? 'spin' : ''} />刷新
          </button>
        </div>
      </div>

      {/* Category filter */}
      {categories.length > 0 && (
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button onClick={() => handleCatChange('')}
            style={{
              padding: '4px 14px', fontSize: 12, borderRadius: 16, cursor: 'pointer',
              border: '1px solid var(--color-border-2)',
              background: selectedCat === '' ? 'var(--color-primary)' : 'var(--color-bg-1)',
              color: selectedCat === '' ? '#fff' : 'var(--color-text-2)',
            }}>全部</button>
          {categories.map(cat => (
            <button key={cat} onClick={() => handleCatChange(cat)}
              style={{
                padding: '4px 14px', fontSize: 12, borderRadius: 16, cursor: 'pointer',
                border: `1px solid ${CAT_COLORS[cat] || 'var(--color-border-2)'}`,
                background: selectedCat === cat ? (CAT_COLORS[cat] || 'var(--color-primary)') : 'var(--color-bg-1)',
                color: selectedCat === cat ? '#fff' : 'var(--color-text-2)',
              }}>{cat}</button>
          ))}
        </div>
      )}

      {loading ? (
        <div className="card" style={{ textAlign: 'center', padding: 60, color: 'var(--color-text-3)', fontSize: 13 }}>加载中...</div>
      ) : data.length === 0 ? (
        <div className="card" style={{ textAlign: 'center', padding: 60 }}>
          <Globe size={40} style={{ color: 'var(--color-text-3)', marginBottom: 12 }} />
          <div style={{ color: 'var(--color-text-3)', fontSize: 13 }}>暂无宏观资讯，等待定时采集更新</div>
        </div>
      ) : (
        <div className="card">
          <div className="card-body" style={{ padding: 0 }}>
            {data.map((item, i) => (
              <div key={item.id || i} style={{
                display: 'flex', gap: 14, padding: '12px 18px',
                borderBottom: i < data.length - 1 ? '1px solid var(--color-border-1)' : 'none',
              }}>
                <div style={{ flexShrink: 0, minWidth: 60 }}>
                  <span style={{
                    fontSize: 10, padding: '2px 8px', borderRadius: 10,
                    background: (CAT_COLORS[item.category] || '#86909C') + '18',
                    color: CAT_COLORS[item.category] || '#86909C',
                    whiteSpace: 'nowrap',
                  }}>{item.category || '宏观'}</span>
                </div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-1)', lineHeight: 1.5, marginBottom: 4 }}>
                    {item.title}
                  </div>
                  {item.summary && (
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)', lineHeight: 1.5 }}>
                      {item.summary.slice(0, 200)}{item.summary.length > 200 ? '...' : ''}
                    </div>
                  )}
                </div>
                <div style={{ flexShrink: 0, display: 'flex', alignItems: 'flex-start', gap: 4, color: 'var(--color-text-3)', fontSize: 11 }}>
                  <Clock size={11} />
                  {item.newsTime?.slice(0, 16) || '-'}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
