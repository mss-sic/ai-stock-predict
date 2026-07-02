import { useEffect, useState, useMemo, useCallback } from 'react';
import { Card, Spin, Radio } from '@arco-design/web-react';
import api from '../services/api';
import ReactECharts from 'echarts-for-react';
import { DollarSign, TrendingUp, TrendingDown, Landmark, Briefcase, BarChart3, ArrowUp, ArrowDown } from 'lucide-react';

interface CapitalSummary {
  northboundNet: number; northboundDate: string;
  fundFlowMain: number; fundFlowDate: string;
  marginBalance: number; marginDate: string;
  dragonTigerCnt: number; blockTradeCnt: number;
}

interface StockRank {
  code: string; name: string;
  netFlow: number; rzye: number; rqye: number;
  tradeDate: string;
}

type TabKey = 'northbound' | 'fundflow' | 'margin_rz' | 'margin_rq';
type SortCol = 'netFlow' | 'rzye' | 'rqye';

const TAB_OPTIONS: { value: TabKey; label: string }[] = [
  { value: 'northbound', label: '北向资金' },
  { value: 'fundflow', label: '主力流向(内外盘)' },
  { value: 'margin_rz', label: '融资余额' },
  { value: 'margin_rq', label: '融券余额' },
];

const COL_DEFS: { key: SortCol; label: string; width: number }[] = [
  { key: 'netFlow', label: '净流入', width: 88 },
  { key: 'rzye', label: '融资余额', width: 88 },
  { key: 'rqye', label: '融券余额', width: 80 },
];

const FMT = (v: number | undefined, d = 2): string => {
  if (v === undefined || v === null || v === 0) return '-';
  const abs = Math.abs(v);
  if (abs >= 10000) return (v / 10000).toFixed(1) + '万亿';
  if (abs >= 1) return v.toFixed(d) + '亿';
  return (v * 10000).toFixed(0) + '万';
};

export default function CapitalFlowPage() {
  const [summary, setSummary] = useState<CapitalSummary | null>(null);
  const [chartLoading, setChartLoading] = useState(true);
  const [rankLoading, setRankLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<TabKey>('northbound');
  const [sortCol, setSortCol] = useState<SortCol>('netFlow');
  const [sortAsc, setSortAsc] = useState(false);

  const [northboundTrend, setNorthboundTrend] = useState<any[]>([]);
  const [fundFlowDaily, setFundFlowDaily] = useState<any[]>([]);
  const [marginTrend, setMarginTrend] = useState<any[]>([]);
  const [stockRank, setStockRank] = useState<StockRank[]>([]);

  useEffect(() => { api.get('/capital-flow/summary').then((r: any) => setSummary(r.data?.data)); }, []);

  // Chart — only depends on activeTab
  useEffect(() => {
    setChartLoading(true);
    switch (activeTab) {
      case 'northbound':
        api.get('/capital-flow/northbound-trend', { params: { days: 60 } }).then((r: any) => setNorthboundTrend(r.data?.data || [])).finally(() => setChartLoading(false));
        break;
      case 'fundflow':
        api.get('/capital-flow/daily', { params: { days: 60 } }).then((r: any) => setFundFlowDaily(r.data?.data || [])).finally(() => setChartLoading(false));
        break;
      case 'margin_rz':
      case 'margin_rq':
        api.get('/capital-flow/margin-trend', { params: { days: 60 } }).then((r: any) => setMarginTrend(r.data?.data || [])).finally(() => setChartLoading(false));
        break;
    }
  }, [activeTab]);

  // Ranking — only depends on sort
  useEffect(() => {
    setRankLoading(true);
    api.get('/capital-flow/stock-rank', { params: { limit: 20, sort: sortCol, order: sortAsc ? 'asc' : 'desc' } })
      .then((r: any) => setStockRank(r.data?.data || []))
      .finally(() => setRankLoading(false));
  }, [sortCol, sortAsc]);

  const handleSort = useCallback((col: SortCol) => {
    if (sortCol === col) setSortAsc(!sortAsc);
    else { setSortCol(col); setSortAsc(false); }
  }, [sortCol, sortAsc]);

  const chartOption = useMemo(() => {
    const g = { left: 60, right: 20, top: 10, bottom: 28 };
    const xA = { type: 'category' as const, axisLabel: { fontSize: 10 } };
    const yA = { type: 'value' as const, axisLabel: { fontSize: 10, formatter: (v: number) => v >= 10000 ? (v/10000).toFixed(0)+'万' : v.toFixed(0) } };

    switch (activeTab) {
      case 'northbound': {
        const d = [...northboundTrend].reverse();
        return {
          tooltip: { trigger: 'axis', valueFormatter: (v: number) => v?.toFixed(2) + '亿' },
          grid: g, xAxis: { ...xA, data: d.map((x: any) => x.tradeDate?.slice(5)) },
          yAxis: { ...yA, name: '亿元' },
          series: [{ name: '北向净流入', type: 'bar', data: d.map((x: any) => +x.totalNet?.toFixed(2) || 0), itemStyle: { color: (p: any) => p.value >= 0 ? '#ef4444' : '#22c55e' }, barMaxWidth: 14 }],
        };
      }
      case 'fundflow': {
        const d = [...fundFlowDaily].reverse();
        return {
          tooltip: { trigger: 'axis', valueFormatter: (v: number) => v?.toFixed(2) + '亿' },
          legend: { bottom: 0, textStyle: { fontSize: 10 } },
          grid: g, xAxis: { ...xA, data: d.map((x: any) => x.tradeDate?.slice(5)) },
          yAxis: { ...yA, name: '亿元' },
          series: [
            { name: '外盘', type: 'bar', data: d.map((x: any) => +x.buyFlow?.toFixed(2) || 0), itemStyle: { color: '#ef4444' }, barMaxWidth: 14, stack: 'flow' },
            { name: '内盘', type: 'bar', data: d.map((x: any) => +x.sellFlow?.toFixed(2) || 0), itemStyle: { color: '#22c55e' }, barMaxWidth: 14, stack: 'flow' },
            { name: '净流入', type: 'line', data: d.map((x: any) => +x.netFlow?.toFixed(2) || 0), itemStyle: { color: '#3b82f6' }, symbol: 'none', smooth: true },
          ],
        };
      }
      case 'margin_rz': {
        const d = [...marginTrend].reverse();
        return {
          tooltip: { trigger: 'axis', valueFormatter: (v: number) => v?.toFixed(0) + '亿' },
          grid: g, xAxis: { ...xA, data: d.map((x: any) => x.tradeDate?.slice(5)) },
          yAxis: { ...yA, name: '亿元' },
          series: [{ name: '融资余额', type: 'line', data: d.map((x: any) => +x.rzye?.toFixed(0) || 0), itemStyle: { color: '#f97316' }, areaStyle: { color: 'rgba(249,115,22,0.1)' }, symbol: 'none', smooth: true }],
        };
      }
      case 'margin_rq': {
        const d = [...marginTrend].reverse();
        return {
          tooltip: { trigger: 'axis', valueFormatter: (v: number) => v?.toFixed(2) + '亿' },
          grid: g, xAxis: { ...xA, data: d.map((x: any) => x.tradeDate?.slice(5)) },
          yAxis: { ...yA, name: '亿元' },
          series: [{ name: '融券余额', type: 'line', data: d.map((x: any) => +x.rqye?.toFixed(0) || 0), itemStyle: { color: '#8b5cf6' }, areaStyle: { color: 'rgba(139,92,246,0.1)' }, symbol: 'none', smooth: true }],
        };
      }
      default: return {};
    }
  }, [activeTab, northboundTrend, fundFlowDaily, marginTrend]);

  const chartTitle: Record<TabKey, string> = {
    northbound: '资金趋势 · 北向',
    fundflow: '资金趋势 · 主力流向(内外盘)',
    margin_rz: '资金趋势 · 融资余额',
    margin_rq: '资金趋势 · 融券余额',
  };

  const SortIcon = ({ col }: { col: SortCol }) => {
    if (sortCol !== col) return null;
    return sortAsc ? <ArrowUp size={10} color="#165DFF" /> : <ArrowDown size={10} color="#165DFF" />;
  };

  if (!summary) return <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 300 }}><Spin size={40} /></div>;

  return (
    <div style={{ padding: '20px 24px', maxWidth: 1400, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 14, marginBottom: 24 }}>
        <div style={{ width: 44, height: 44, borderRadius: 10, background: 'linear-gradient(135deg, #22c55e, #3b82f6)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <DollarSign size={22} color="#fff" />
        </div>
        <div>
          <h1 style={{ fontSize: 18, fontWeight: 700, margin: 0, color: 'var(--color-text-1)' }}>资金面综合看板</h1>
          <p style={{ margin: '2px 0 0', fontSize: 12, color: 'var(--color-text-3)' }}>北向资金 · 主力流向(内外盘) · 融资融券 · 龙虎榜 · 大宗交易</p>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 12, marginBottom: 16 }}>
        {[
          { icon: TrendingUp, label: '北向净流入', value: (summary?.northboundNet || 0) >= 0 ? `+${(summary?.northboundNet||0).toFixed(1)}亿` : `${(summary?.northboundNet||0).toFixed(1)}亿`, color: (summary?.northboundNet||0) >= 0 ? '#ef4444' : '#22c55e', sub: summary?.northboundDate },
          { icon: BarChart3, label: '内外盘净差', value: (summary?.fundFlowMain||0) >= 0 ? `+${(summary?.fundFlowMain||0).toFixed(1)}亿` : `${(summary?.fundFlowMain||0).toFixed(1)}亿`, color: (summary?.fundFlowMain||0) >= 0 ? '#ef4444' : '#22c55e', sub: summary?.fundFlowDate },
          { icon: Landmark, label: '融资余额', value: `${(summary?.marginBalance||0).toFixed(0)}亿`, color: '#f97316', sub: summary?.marginDate },
          { icon: Briefcase, label: '龙虎榜上榜', value: `${summary?.dragonTigerCnt||0} 只`, color: '#8b5cf6', sub: '最新交易日' },
          { icon: TrendingDown, label: '大宗交易', value: `${summary?.blockTradeCnt||0} 笔`, color: '#ec4899', sub: '最新交易日' },
        ].map((c, i) => (
          <Card key={i} style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' , }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
              <c.icon size={16} color={c.color} />
              <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{c.label}</span>
            </div>
            <div style={{ fontSize: 18, fontWeight: 700, color: c.color }}>{c.value}</div>
            {c.sub && <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 4 }}>{c.sub}</div>}
          </Card>
        ))}
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 480px', gap: 14 }}>
        <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}
          title={
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <span>{chartTitle[activeTab]}</span>
              <Radio.Group size="small" type="button" value={activeTab} onChange={(v) => setActiveTab(v as TabKey)}>
                {TAB_OPTIONS.map((o) => <Radio key={o.value} value={o.value}>{o.label}</Radio>)}
              </Radio.Group>
            </div>
          }>
          {chartLoading ? <div style={{ display: 'flex', justifyContent: 'center', padding: 120 }}><Spin /></div> : <ReactECharts option={chartOption} style={{ height: 340 }} />}
        </Card>

        <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}
          title={
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span>个股资金排名</span>
              <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{stockRank[0]?.tradeDate || ''}</span>
            </div>
          }>
          <div style={{ display: 'grid', gridTemplateColumns: '64px 72px 95px 95px 85px', padding: '7px 8px', borderBottom: '2px solid var(--color-border-2)', fontSize: 11, color: 'var(--color-text-3)', fontWeight: 600 }}>
            <span>代码</span><span>名称</span>
            {COL_DEFS.map((c) => (
              <span key={c.key} onClick={() => handleSort(c.key)}
                style={{ cursor: 'pointer', userSelect: 'none', textAlign: 'right', display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 2,
                  color: sortCol === c.key ? '#165DFF' : 'var(--color-text-3)', fontWeight: sortCol === c.key ? 700 : 600 }}>
                {c.label}<SortIcon col={c.key} />
              </span>
            ))}
          </div>
          {rankLoading ? <div style={{ display: 'flex', justifyContent: 'center', padding: 80 }}><Spin /></div> :
            stockRank.map((r, i) => (
              <div key={r.code} style={{ display: 'grid', gridTemplateColumns: '64px 72px 95px 95px 85px', padding: '6px 8px',
                borderBottom: '1px solid var(--color-border-1)', fontSize: 11, background: i % 2 === 0 ? 'var(--color-fill-1)' : 'transparent' }}>
                <span style={{ fontFamily: 'monospace', color: 'var(--color-text-2)' }}>{r.code}</span>
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{r.name}</span>
                <span style={{ fontFamily: "'SF Mono','Inter',monospace", textAlign: 'right', fontWeight: 600, color: (r.netFlow || 0) >= 0 ? '#ef4444' : '#22c55e' }}>{FMT(r.netFlow)}</span>
                <span style={{ fontFamily: "'SF Mono','Inter',monospace", textAlign: 'right', color: r.rzye > 0 ? 'var(--color-text-1)' : 'var(--color-text-3)' }}>{FMT(r.rzye, 0)}</span>
                <span style={{ fontFamily: "'SF Mono','Inter',monospace", textAlign: 'right', color: r.rqye > 0 ? 'var(--color-text-1)' : 'var(--color-text-3)' }}>{FMT(r.rqye, 0)}</span>
              </div>
            ))}
        </Card>
      </div>
    </div>
  );
}
