import { useEffect, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Input, Select, Tag } from '@arco-design/web-react';
import { fetchConceptHeatmap } from '../services/api';
import { Flame, TrendingUp, TrendingDown, Search, BarChart3, Grid3X3 } from 'lucide-react';

type ViewMode = 'treemap' | 'table';

interface ConceptItem {
  conceptCode: string;
  conceptName: string;
  stockCount: number;
  avgChgPct: number;
  upCount: number;
  downCount: number;
}

export default function ConceptBoardPage() {
  const [allData, setAllData] = useState<ConceptItem[]>([]);
  const [viewMode, setViewMode] = useState<ViewMode>('treemap');
  const [filter, setFilter] = useState<'all' | 'up' | 'down' | 'significant'>('significant');
  const [search, setSearch] = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    fetchConceptHeatmap().then((res: any) => setAllData(res.data?.data || []));
  }, []);

  const filtered = useMemo(() => {
    let data = allData;
    if (filter === 'up') data = data.filter(d => d.avgChgPct > 1);
    else if (filter === 'down') data = data.filter(d => d.avgChgPct < -1);
    else if (filter === 'significant') data = data.filter(d => Math.abs(d.avgChgPct) > 0.8);
    if (search) data = data.filter(d => d.conceptName.includes(search));
    return data.sort((a, b) => Math.abs(b.avgChgPct) - Math.abs(a.avgChgPct));
  }, [allData, filter, search]);

  const stats = useMemo(() => {
    const up = allData.filter(d => d.avgChgPct > 1).length;
    const down = allData.filter(d => d.avgChgPct < -1).length;
    const hot = allData.filter(d => d.avgChgPct > 3).length;
    return { up, down, hot, total: allData.length };
  }, [allData]);

  // For treemap: limit to top 60, compute sizing
  const treemapData = useMemo(() => {
    return filtered.slice(0, 60);
  }, [filtered]);

  const maxAbs = useMemo(() => Math.max(...treemapData.map(d => Math.abs(d.avgChgPct)), 0.5), [treemapData]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {/* Header */}
      <div style={{
        padding: '12px 18px', display: 'flex', alignItems: 'center', gap: 14, flexWrap: 'wrap',
        background: 'var(--color-bg-1)', borderRadius: 10, border: '1px solid var(--color-border-1)',
      }}>
        <Flame size={18} color="#f53f3f" />
        <span style={{ fontSize: 16, fontWeight: 700, color: 'var(--color-text-1)' }}>概念板块</span>
        <div style={{ flex: 1 }} />
        <div style={{ display: 'flex', gap: 6 }}>
          {([
            { key: 'significant' as const, label: '活跃', desc: '|涨跌| > 0.8%' },
            { key: 'up' as const, label: '上涨', desc: '> 1%' },
            { key: 'down' as const, label: '下跌', desc: '< -1%' },
            { key: 'all' as const, label: '全部', desc: `${stats.total}个` },
          ]).map(f => (
            <div key={f.key} onClick={() => setFilter(f.key)} style={{
              padding: '3px 10px', borderRadius: 4, cursor: 'pointer', fontSize: 11,
              fontWeight: filter === f.key ? 600 : 400,
              color: filter === f.key ? '#fff' : 'var(--color-text-2)',
              background: filter === f.key ? 'var(--color-primary)' : 'var(--color-fill-1)',
            }}>
              {f.label} <span style={{ opacity: 0.7, fontSize: 10 }}>{f.desc}</span>
            </div>
          ))}
        </div>
        <Input prefix={<Search size={13} />} placeholder="搜索概念" value={search}
          onChange={setSearch} style={{ width: 150 }} size="small" allowClear />
        <div style={{ display: 'flex', background: 'var(--color-fill-1)', borderRadius: 6, padding: 2 }}>
          {([
            { key: 'treemap' as ViewMode, icon: <Grid3X3 size={13} />, label: '热力图' },
            { key: 'table' as ViewMode, icon: <BarChart3 size={13} />, label: '列表' },
          ]).map(v => (
            <div key={v.key} onClick={() => setViewMode(v.key)} style={{
              display: 'flex', alignItems: 'center', gap: 3, padding: '3px 8px', borderRadius: 4, cursor: 'pointer',
              fontSize: 11, color: viewMode === v.key ? '#fff' : 'var(--color-text-2)',
              background: viewMode === v.key ? 'var(--color-primary)' : 'transparent',
            }}>
              {v.icon} {v.label}
            </div>
          ))}
        </div>
      </div>

      {/* Stats bar */}
      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
        <StatBadge color="#f53f3f" icon={<TrendingUp size={12} />} label="上涨概念" value={stats.up} />
        <StatBadge color="#00b42a" icon={<TrendingDown size={12} />} label="下跌概念" value={stats.down} />
        <StatBadge color="#ff7d00" icon={<Flame size={12} />} label="大涨(>3%)" value={stats.hot} />
        <span style={{ fontSize: 11, color: 'var(--color-text-4)', alignSelf: 'center' }}>
          共 {stats.total} 个概念 · 当前显示 {filtered.length} 个
        </span>
      </div>

      {viewMode === 'treemap' ? (
        /* Heatmap — uniform grid, visual weight by color+shadow */
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(148px, 1fr))', gap: 7 }}>
          {treemapData.map((d, i) => {
            const absPct = Math.abs(d.avgChgPct);
            const t = absPct / maxAbs;
            const isUp = d.avgChgPct > 0;
            const bg = isUp
              ? `rgba(245,63,63,${(0.06 + t * 0.90).toFixed(2)})`
              : `rgba(0,180,42,${(0.06 + t * 0.90).toFixed(2)})`;
            const fg = t > 0.45 ? '#fff' : isUp ? '#cb272d' : '#008026';
            const isHot = t > 0.55;
            return (
              <div key={d.conceptCode}
                onClick={() => navigate('/concept/' + d.conceptCode)}
                title={`${d.conceptName}
涨跌: ${d.avgChgPct > 0 ? '+' : ''}${d.avgChgPct.toFixed(2)}%
${d.stockCount}只 · 涨${d.upCount}跌${d.downCount}`}
                style={{
                  background: bg, color: fg, borderRadius: 8, padding: '10px 12px',
                  cursor: 'pointer', transition: 'transform .12s, box-shadow .12s',
                  border: isHot ? `2px solid ${isUp ? 'rgba(245,63,63,0.6)' : 'rgba(0,180,42,0.6)'}`
                    : `1px solid ${t > 0.3 ? 'transparent' : 'var(--color-border-1)'}`,
                  boxShadow: isHot ? `0 3px 14px ${isUp ? 'rgba(245,63,63,0.35)' : 'rgba(0,180,42,0.35)'}` : 'none',
                }}
                onMouseEnter={e => { e.currentTarget.style.transform = 'scale(1.03)'; e.currentTarget.style.zIndex = '1'; }}
                onMouseLeave={e => { e.currentTarget.style.transform = 'scale(1)'; e.currentTarget.style.zIndex = ''; }}>
                <div style={{ fontWeight: 700, fontSize: 12, marginBottom: 4, lineHeight: 1.3 }}>
                  {isHot && '🔥 '}{d.conceptName}
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', fontSize: 10, opacity: 0.9 }}>
                  <span>{d.stockCount}只</span>
                  <span style={{ fontWeight: 700, fontFamily: "'SF Mono',monospace", fontSize: 13 }}>
                    {d.avgChgPct > 0 ? '+' : ''}{d.avgChgPct.toFixed(2)}%
                  </span>
                </div>
                <div style={{ display: 'flex', gap: 8, fontSize: 9, marginTop: 3, opacity: 0.75 }}>
                  <span>▲{d.upCount}</span>
                  <span>▼{d.downCount}</span>
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        /* Table view */
        <div style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', overflow: 'hidden' }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 80px 80px 80px 80px', padding: '8px 14px',
            background: 'var(--color-fill-1)', fontSize: 11, fontWeight: 600, color: 'var(--color-text-3)',
            borderBottom: '2px solid var(--color-border-2)' }}>
            <span>概念名称</span>
            <span style={{ textAlign: 'right' }}>股票数</span>
            <span style={{ textAlign: 'right' }}>涨跌%</span>
            <span style={{ textAlign: 'right' }}>上涨</span>
            <span style={{ textAlign: 'right' }}>下跌</span>
          </div>
          <div style={{ maxHeight: '70vh', overflowY: 'auto' }}>
            {filtered.map((d, i) => {
              const isUp = d.avgChgPct > 0;
              return (
                <div key={d.conceptCode}
                  onClick={() => navigate('/concept/' + d.conceptCode)}
                  style={{
                    display: 'grid', gridTemplateColumns: '1fr 80px 80px 80px 80px',
                    padding: '7px 14px', cursor: 'pointer', fontSize: 12,
                    borderBottom: '1px solid var(--color-border-1)',
                    background: i % 2 === 0 ? 'var(--color-bg-1)' : 'transparent',
                  }}
                  onMouseEnter={e => e.currentTarget.style.background = 'var(--color-fill-1)'}
                  onMouseLeave={e => e.currentTarget.style.background = i % 2 === 0 ? 'var(--color-bg-1)' : 'transparent'}>
                  <span style={{ fontWeight: 500 }}>{d.conceptName}</span>
                  <span style={{ textAlign: 'right', fontFamily: "'SF Mono',monospace", color: 'var(--color-text-2)' }}>{d.stockCount}</span>
                  <span style={{ textAlign: 'right', fontFamily: "'SF Mono',monospace", fontWeight: 700,
                    color: isUp ? 'var(--stock-up)' : d.avgChgPct < 0 ? 'var(--stock-down)' : 'var(--color-text-2)' }}>
                    {d.avgChgPct > 0 ? '+' : ''}{d.avgChgPct.toFixed(2)}%
                  </span>
                  <span style={{ textAlign: 'right', color: 'var(--stock-up)' }}>{d.upCount}</span>
                  <span style={{ textAlign: 'right', color: 'var(--stock-down)' }}>{d.downCount}</span>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

function StatBadge({ color, icon, label, value }: { color: string; icon: React.ReactNode; label: string; value: number }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 5, padding: '5px 10px',
      background: `${color}12`, borderRadius: 6, border: `1px solid ${color}25`,
      fontSize: 11, color: 'var(--color-text-2)',
    }}>
      <span style={{ color }}>{icon}</span>
      <span>{label}</span>
      <span style={{ fontWeight: 700, color, fontFamily: "'SF Mono',monospace" }}>{value}</span>
    </div>
  );
}
