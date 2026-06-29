import { useEffect, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { FileWarning, Search, Filter, ExternalLink } from 'lucide-react';
import { authFetch } from '../services/api';

interface AnnItem {
  id: number;
  code: string;
  title: string;
  annType: string;
  annDate: string;
  annUrl: string;
}

const TYPE_TAGS = ['全部', '业绩预告', '年报', '季报', '半年报', '重大合同', '增减持', '停复牌', '股东大会', '其他'];

export default function AnnouncementsPage() {
  const navigate = useNavigate();
  const [data, setData] = useState<AnnItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('全部');
  const [dateFrom, setDateFrom] = useState('');

  const load = async () => {
    setLoading(true);
    try {
      const res = await authFetch('/api/v1/announcements?limit=500');
      const json = await res.json();
      setData(json.data || []);
    } catch { setData([]); }
    setLoading(false);
  };

  useEffect(() => { load(); }, []);

  const filtered = useMemo(() => {
    let result = data;
    if (typeFilter !== '全部') {
      result = result.filter(d => (d.annType || '').includes(typeFilter));
    }
    if (search) {
      const q = search.toLowerCase();
      result = result.filter(d => d.title.toLowerCase().includes(q) || d.code.includes(q));
    }
    if (dateFrom) {
      result = result.filter(d => d.annDate >= dateFrom);
    }
    return result.sort((a, b) => b.annDate.localeCompare(a.annDate));
  }, [data, search, typeFilter, dateFrom]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div style={{
          background: 'linear-gradient(135deg, #FF7D00, #F53F3F)', borderRadius: 10,
          width: 36, height: 36, display: 'flex', alignItems: 'center', justifyContent: 'center'
        }}>
          <FileWarning size={18} color="#fff" />
        </div>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700, color: 'var(--color-text-1)' }}>公告检索</h2>
        <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>巨潮资讯 · 全市场公告</span>
      </div>

      {/* Filters */}
      <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
        <div style={{ position: 'relative', flex: '0 1 280px' }}>
          <Search size={14} style={{ position: 'absolute', left: 10, top: 9, color: 'var(--color-text-3)' }} />
          <input placeholder="搜索公告标题或股票代码..."
            value={search} onChange={e => setSearch(e.target.value)}
            style={{ width: '100%', padding: '7px 12px 7px 30px', fontSize: 12, borderRadius: 6, border: '1px solid var(--color-border-2)', background: 'var(--color-bg-1)', color: 'var(--color-text-1)' }} />
        </div>
        <input type="date" value={dateFrom} onChange={e => setDateFrom(e.target.value)}
          style={{ padding: '6px 12px', fontSize: 12, borderRadius: 6, border: '1px solid var(--color-border-2)', background: 'var(--color-bg-1)', color: 'var(--color-text-1)' }} />
        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
          {TYPE_TAGS.map(t => (
            <button key={t} onClick={() => setTypeFilter(t)}
              style={{
                padding: '4px 12px', fontSize: 11, borderRadius: 14, cursor: 'pointer',
                border: `1px solid ${typeFilter === t ? 'var(--color-primary)' : 'var(--color-border-2)'}`,
                background: typeFilter === t ? 'var(--color-primary)' : 'var(--color-bg-1)',
                color: typeFilter === t ? '#fff' : 'var(--color-text-2)',
              }}>{t}</button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="card" style={{ textAlign: 'center', padding: 60, color: 'var(--color-text-3)', fontSize: 13 }}>加载中...</div>
      ) : (
        <div className="card">
          <div className="card-header">
            <span style={{ fontWeight: 600, fontSize: 14 }}><Filter size={14} /> 公告列表</span>
            <span style={{ fontSize: 11, color: 'var(--color-text-3)', marginLeft: 8 }}>共 {filtered.length} 条</span>
          </div>
          <div className="card-body" style={{ padding: 0 }}>
            {filtered.length === 0 ? (
              <div style={{ textAlign: 'center', padding: 48, color: 'var(--color-text-3)', fontSize: 13 }}>无匹配公告</div>
            ) : (
              filtered.map((d, i) => {
                const typeColors: Record<string, [string, string]> = {
                  '业绩预告': ['#FFF7E6', '#FF7D00'], '年报': ['#E8F3FF', '#165DFF'],
                  '季报': ['#E8F3FF', '#165DFF'], '半年报': ['#E8F3FF', '#165DFF'],
                  '重大合同': ['#E8FFEA', '#00B42A'], '增减持': ['#FFECE8', '#F53F3F'],
                  '停复牌': ['#F2F3F5', '#86909C'], '股东大会': ['#F2F3F5', '#86909C'],
                };
                const [bg, color] = typeColors[d.annType] || ['#F2F3F5', '#86909C'];
                return (
                  <div key={d.id || i} style={{
                    display: 'flex', alignItems: 'center', gap: 12, padding: '10px 16px',
                    borderBottom: '1px solid var(--color-border-1)', cursor: 'pointer'
                  }} onClick={() => navigate(`/stock/${d.code}`)}>
                    <span style={{
                      fontSize: 10, padding: '2px 8px', borderRadius: 3, background: bg, color,
                      whiteSpace: 'nowrap', flexShrink: 0
                    }}>{d.annType || '公告'}</span>
                    <code style={{ fontSize: 11, color: 'var(--color-text-3)', flexShrink: 0, cursor: 'pointer' }}
                      onClick={e => { e.stopPropagation(); navigate(`/stock/${d.code}`); }}>{d.code}</code>
                    <div style={{ flex: 1, minWidth: 0, fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {d.title}
                    </div>
                    <span style={{ fontSize: 11, color: 'var(--color-text-3)', flexShrink: 0 }}>{d.annDate?.slice(0, 10)}</span>
                    {d.annUrl && (
                      <a href={d.annUrl} target="_blank" rel="noopener noreferrer" onClick={e => e.stopPropagation()}
                        style={{ color: 'var(--color-text-3)', flexShrink: 0 }} title="查看原文">
                        <ExternalLink size={12} />
                      </a>
                    )}
                  </div>
                );
              })
            )}
          </div>
        </div>
      )}
    </div>
  );
}
