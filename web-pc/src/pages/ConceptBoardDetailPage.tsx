import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Tag, Select } from '@arco-design/web-react';
import ReactECharts from 'echarts-for-react';
import { fetchConceptBoardStocks, fetchConceptBoardKline } from '../services/api';
import {
  ArrowLeft, TrendingUp, TrendingDown, Layers, BarChart3, Star,
  Target, Shield, Zap, Calendar
} from 'lucide-react';

export default function ConceptBoardDetailPage() {
  const { code } = useParams<{ code: string }>();
  const navigate = useNavigate();
  const [data, setData] = useState<any>(null);
  const [kline, setKline] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [klineDays, setKlineDays] = useState(60);

  useEffect(() => {
    if (!code) return;
    setLoading(true);
    Promise.all([
      fetchConceptBoardStocks(code).then((res: any) => setData(res.data?.data)),
      fetchConceptBoardKline(code, klineDays).then((res: any) => setKline(res.data?.data || [])),
    ]).finally(() => setLoading(false));
  }, [code, klineDays]);

  if (loading) {
    return <div style={{ padding: 60, textAlign: 'center', color: 'var(--color-text-3)' }}>加载中...</div>;
  }
  if (!data) {
    return <div style={{ padding: 60, textAlign: 'center', color: 'var(--color-text-3)' }}>板块不存在</div>;
  }

  const { board, stocks, upCount, downCount, avgChgPct, total } = data;
  const isUp = (avgChgPct || 0) >= 0;

  // K-line chart options
  const klineOption = kline.length > 0 ? {
    grid: { top: 40, right: 20, bottom: 30, left: 50 },
    tooltip: { trigger: 'axis' },
    xAxis: {
      type: 'category',
      data: kline.map((p: any) => p.tradeDate.slice(5)),
      axisLabel: { fontSize: 10, color: 'var(--color-text-3)' },
    },
    yAxis: {
      type: 'value',
      axisLabel: { fontSize: 10, color: 'var(--color-text-3)', formatter: (v: number) => v.toFixed(0) },
      splitLine: { lineStyle: { color: 'var(--color-border-1)' } },
    },
    series: [
      {
        name: '概念指数',
        type: 'line',
        data: kline.map((p: any) => p.indexValue),
        smooth: true,
        lineStyle: { color: isUp ? '#f53f3f' : '#00b42a', width: 2 },
        areaStyle: {
          color: {
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: isUp ? 'rgba(245,63,63,0.15)' : 'rgba(0,180,42,0.15)' },
              { offset: 1, color: 'rgba(255,255,255,0)' },
            ],
          },
        },
        symbol: 'none',
      },
    ],
  } : null;

  // Count today's picks
  const todayPicks = (stocks || []).filter((s: any) => s.todayPick).length;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
        <button onClick={() => navigate('/board/concepts')} style={{
          width: 32, height: 32, borderRadius: 6, border: '1px solid var(--color-border-1)',
          background: 'var(--color-bg-1)', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center'
        }}>
          <ArrowLeft size={16} color="var(--color-text-2)" />
        </button>
        <div style={{ width: 36, height: 36, borderRadius: 8, background: 'linear-gradient(135deg, var(--color-primary), #722ED1)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Layers size={18} color="#fff" />
        </div>
        <div>
          <h2 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>{board?.conceptName || code}</h2>
          <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>{board?.conceptType === 'industry' ? '行业板块' : '概念板块'} · {total} 只成分股</span>
        </div>
        <div style={{ flex: 1 }} />
        {todayPicks > 0 && (
          <Tag color="red" style={{ display: 'flex', alignItems: 'center', gap: 3 }}>
            <Star size={10} /> 今日上榜 {todayPicks} 只
          </Tag>
        )}
      </div>

      {/* Stats Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 10 }}>
        <StatCard label="成分股数量" value={total} />
        <StatCard label="平均涨跌" value={`${(avgChgPct || 0) >= 0 ? "+" : ""}${(avgChgPct || 0).toFixed(2)}%`} valueColor={isUp ? '#f53f3f' : '#00b42a'} />
        <StatCard label="上涨" value={upCount || 0} icon={<TrendingUp size={14} />} valueColor="#f53f3f" />
        <StatCard label="下跌" value={downCount || 0} icon={<TrendingDown size={14} />} valueColor="#00b42a" />
        <StatCard label="今日上榜" value={todayPicks} icon={<Star size={14} />} valueColor="#ff7d00" />
      </div>

      {/* K-line Chart */}
      {klineOption && (
        <div style={{ background: 'var(--color-bg-1)', borderRadius: 8, border: '1px solid var(--color-border-1)' }}>
          <div style={{ padding: '10px 16px', borderBottom: '1px solid var(--color-border-1)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <BarChart3 size={14} color="var(--color-text-2)" />
              <span style={{ fontSize: 14, fontWeight: 600 }}>概念指数走势</span>
              <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
                (成分股等权合成 · 基准1000)
              </span>
            </div>
            <Select value={klineDays} onChange={setKlineDays} size="small" style={{ width: 90 }}
              options={[
                { label: '30天', value: 30 },
                { label: '60天', value: 60 },
                { label: '120天', value: 120 },
              ]} />
          </div>
          <div style={{ padding: '8px 0' }}>
            <ReactECharts option={klineOption} style={{ height: 280 }} />
          </div>
        </div>
      )}

      {/* Stock List — Trading View */}
      <div style={{ background: 'var(--color-bg-1)', borderRadius: 8, border: '1px solid var(--color-border-1)', overflow: 'hidden' }}>
        <div style={{ padding: '10px 16px', borderBottom: '1px solid var(--color-border-1)', display: 'flex', alignItems: 'center', gap: 8 }}>
          <Target size={14} color="var(--color-text-2)" />
          <span style={{ fontSize: 14, fontWeight: 600 }}>成分股 · 交易视角</span>
          <span style={{ fontSize: 11, color: 'var(--color-text-3)', marginLeft: 8 }}>
            按上榜排名 → 涨跌幅排序
          </span>
        </div>
        <div style={{ overflow: 'auto', maxHeight: '60vh' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
            <thead>
              <tr style={{ background: 'var(--color-fill-2)', color: 'var(--color-text-3)', fontSize: 11, position: 'sticky', top: 0 }}>
                <th style={thStyle}>代码</th>
                <th style={thStyle}>名称</th>
                <th style={{ ...thStyle, textAlign: 'right' }}>最新价</th>
                <th style={{ ...thStyle, textAlign: 'right' }}>涨跌幅</th>
                <th style={{ ...thStyle, textAlign: 'right' }}>市值</th>
                <th style={{ ...thStyle, textAlign: 'right' }}>AI评分</th>
                <th style={{ ...thStyle, textAlign: 'center' }}>上榜</th>
                <th style={{ ...thStyle, textAlign: 'center' }}>5日</th>
                <th style={{ ...thStyle, textAlign: 'center' }}>20日</th>
              </tr>
            </thead>
            <tbody>
              {(stocks || []).map((s: any, i: number) => {
                const up = (s.chgPct || 0) > 0;
                const down = (s.chgPct || 0) < 0;
                return (
                  <tr key={i} onClick={() => navigate('/stock/' + s.code)} style={{
                    borderBottom: '1px solid var(--color-border-1)', cursor: 'pointer',
                    background: s.todayPick ? 'rgba(255,125,0,0.04)' : (i % 2 === 0 ? 'var(--color-bg-1)' : 'transparent'),
                  }}
                    onMouseEnter={e => (e.currentTarget.style.background = 'var(--color-fill-2)')}
                    onMouseLeave={e => (e.currentTarget.style.background = s.todayPick ? 'rgba(255,125,0,0.04)' : (i % 2 === 0 ? 'var(--color-bg-1)' : 'transparent'))}>
                    <td style={{ padding: '7px 12px', fontFamily: 'monospace', color: 'var(--color-text-3)', fontSize: 11 }}>{s.code}</td>
                    <td style={{ padding: '7px 12px', fontWeight: 500, display: 'flex', alignItems: 'center', gap: 6 }}>
                      {s.todayPick && <Star size={11} color="#ff7d00" style={{ flexShrink: 0 }} />}
                      {s.stockName || s.code}
                      {s.riskLevel === 'high' && <Shield size={10} color="#f53f3f" title="高风险" />}
                    </td>
                    <td style={{ padding: '7px 12px', textAlign: 'right', fontFamily: 'monospace' }}>{s.close > 0 ? s.close.toFixed(2) : '-'}</td>
                    <td style={{ padding: '7px 12px', textAlign: 'right', fontFamily: 'monospace',
                      color: up ? '#f53f3f' : down ? '#00b42a' : 'var(--color-text-2)', fontWeight: 500 }}>
                      {up ? '+' : ''}{(s.chgPct || 0).toFixed(2)}%
                    </td>
                    <td style={{ padding: '7px 12px', textAlign: 'right', color: 'var(--color-text-3)', fontSize: 11 }}>
                      {s.marketCap > 0 ? (s.marketCap / 1e8).toFixed(1) + '亿' : '-'}
                    </td>
                    <td style={{ padding: '7px 12px', textAlign: 'right', fontFamily: 'monospace', fontSize: 11 }}>
                      <span style={{ color: (s.aiScore || 0) >= 60 ? '#00b42a' : (s.aiScore || 0) >= 30 ? '#ff7d00' : 'var(--color-text-3)' }}>
                        {(s.aiScore || 0).toFixed(0)}
                      </span>
                    </td>
                    <td style={{ padding: '7px 12px', textAlign: 'center' }}>
                      {s.todayPick
                        ? <Tag color="red" size="small" style={{ fontSize: 10, padding: '0 4px' }}>#{s.pickRank || '?'}</Tag>
                        : <span style={{ color: 'var(--color-text-4)', fontSize: 10 }}>-</span>}
                    </td>
                    <td style={{ padding: '7px 12px', textAlign: 'center', fontFamily: 'monospace', fontSize: 11,
                      color: (s.pick5Day || 0) >= 3 ? '#ff7d00' : 'var(--color-text-3)', fontWeight: (s.pick5Day || 0) >= 3 ? 600 : 400 }}>
                      {s.pick5Day || 0}
                    </td>
                    <td style={{ padding: '7px 12px', textAlign: 'center', fontFamily: 'monospace', fontSize: 11,
                      color: (s.pick20Day || 0) >= 8 ? '#ff7d00' : 'var(--color-text-3)', fontWeight: (s.pick20Day || 0) >= 8 ? 600 : 400 }}>
                      {s.pick20Day || 0}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

const thStyle: React.CSSProperties = {
  padding: '8px 12px', textAlign: 'left', fontWeight: 500,
  background: 'var(--color-fill-2)', whiteSpace: 'nowrap',
};

function StatCard({ label, value, icon, valueColor }: {
  label: string; value: any; icon?: React.ReactNode; valueColor?: string;
}) {
  return (
    <div style={{ background: 'var(--color-bg-1)', borderRadius: 8, border: '1px solid var(--color-border-1)', padding: '14px 16px' }}>
      <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 20, fontWeight: 600, color: valueColor || 'var(--color-text-1)', display: 'flex', alignItems: 'center', gap: 4 }}>
        {icon || null} {value}
      </div>
    </div>
  );
}
