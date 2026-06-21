import { useEffect, useState, useMemo } from 'react';
import { fetchLatestSentiment, fetchSentimentHistory, fetchIndexKLine, fetchReturnDistribution } from '../services/api';
import api from '../services/api';
import ReactECharts from 'echarts-for-react';
import {
  TrendingUp, TrendingDown, Minus, Gauge, ChevronUp, ChevronDown, ChevronRight,
  Activity, Zap, Target, Shield, DollarSign, Layers, BarChart3,
  ArrowUp, ArrowDown, Users, Flame, Skull, Unplug,
} from 'lucide-react';

interface SentimentData {
  tradeDate: string;
  compositeScore: number;
  marketBreadth: number; breadthScore: number;
  styleRiskPref: number; styleRiskScore: number;
  tradeActivity: number; activityScore: number;
  profitEffect: number; profitScore: number;
  volatility: number; volatilityScore: number;
  priceStrength: number; strengthScore: number;
  riskAppetite: number; riskAppetiteScore: number;
  limitSentiment: number; limitSentimentScore: number;
  sectorDiffusion: number; sectorDiffusionScore: number;
  northboundNet: number; northboundScore: number;
  capitalFlowNet: number; capitalFlowScore: number;
  upCount: number; downCount: number;
  limitUpCount: number; limitDownCount: number;
  boardBreakCount: number; totalStocks: number;
}

// 9 core indicators only (northbound & capital_flow excluded)
const SUB_INDICATORS = [
  { key: 'breadthScore', label: '市场广度', icon: Layers, desc: '上涨+MA20占比', weight: 18.3 },
  { key: 'styleRiskScore', label: '风格偏好', icon: Target, desc: '中证1000 vs 沪深300', weight: 17.5 },
  { key: 'activityScore', label: '成交活跃', icon: Activity, desc: '成交额/20日均', weight: 4.9 },
  { key: 'profitScore', label: '赚钱效应', icon: DollarSign, desc: '5日上涨+60日新低', weight: 6.0 },
  { key: 'volatilityScore', label: '波动率', icon: Zap, desc: '沪深300 20日波动', weight: 17.9 },
  { key: 'strengthScore', label: '价格强度', icon: TrendingUp, desc: '52周新高占比', weight: 4.9 },
  { key: 'riskAppetiteScore', label: '风险偏好', icon: Shield, desc: '股债收益差', weight: 4.9 },
  { key: 'limitSentimentScore', label: '涨跌停', icon: BarChart3, desc: '涨停比+炸板率', weight: 9.0 },
    { key: 'sectorDiffusionScore', label: '板块扩散', icon: Layers, desc: '上涨行业占比', weight: 16.8 },
];

const INDICATOR_SPECS = [
  { label: '市场广度', weight: '18.3%', formula: '0.5 × (上涨家数÷总数) + 0.5 × (MA20以上÷总数)', 
    detail: '不看指数涨跌，纯粹衡量多少股票在涨、多少站上20日均线。广度越高=上涨越真实，非权重股拉指数。', source: 'market_daily_agg.up_count / ma20_count / total_stocks' },
  { label: '风格风险偏好', weight: '17.5%', formula: '0.5 × (中证1000收益₅日 - 沪深300收益₅日) + 0.5 × (中证1000收益₂₀日 - 沪深300收益₂₀日)', 
    detail: '小盘vs大盘相对强弱。正值=小盘跑赢（risk-on），负值=大盘防御（risk-off）。', source: 'IDX000852 / IDX000300 日K线' },
  { label: '波动率情绪', weight: '17.9%', formula: 'std(沪深300日收益₂₀日) × √252 × 方向修正',
    detail: '沪深300年化波动率。方向修正：上涨时略微放大（亢奋波动）、下跌时不修正（恐慌真实）。波动越高=市场越不安。', source: 'IDX000300 日K线' },
  { label: '板块扩散', weight: '16.8%', formula: '上涨行业数 ÷ 全行业数',
    detail: '有多少行业板块当日平均涨幅>0。全面开花=可持续，少数行业独涨=结构性行情。', source: 'concept_boards (industry) → stock_concepts → stocks_daily_k' },
  { label: '涨跌停情绪', weight: '9.0%', formula: '0.4 × 涨停比 + 0.3 × (1-炸板率) + 0.3 × 昨涨得分',
    detail: '涨停比=做多热情，炸板率=封板质量（低=情绪稳），昨日涨停今日收益=追板盈亏反馈。涨跌停幅度按板块区分(主板±10%/科创±20%/北交±30%/ST±5%)。', source: 'stocks_daily_k + stocks_basic.board_type' },
  { label: '赚钱效应', weight: '6.0%', formula: '0.6 × (5日上涨数÷可比) + 0.4 × (1 - 60日新低数÷总数)',
    detail: '近期持股盈利面+惩罚创新低。5日维度衡量短线赚钱概率，60日新低=深度套牢盘。', source: 'market_daily_agg.up5_count / n60_low_count' },
  { label: '成交活跃度', weight: '4.9%', formula: '今日成交额 ÷ 过去20日均成交额',
    detail: '量能变化率。>1=放量（资金活跃），<1=缩量（观望）。', source: 'market_daily_agg.total_amount' },
  { label: '价格强度', weight: '4.9%', formula: '创52周新高家数 ÷ 总股票数',
    detail: '动量极端值。一年新高家数越多=趋势越强。', source: 'market_daily_agg.n52_high_count' },
  { label: '风险偏好', weight: '4.9%', formula: '沪深300₂₀日收益 - avg(国债ETF₁,₂,₃ ₂₀日收益)',
    detail: '股债跷跷板。正=资金从债市流入股市(风险偏好上升)，负=流回债市(避险)。国债池: 511010/511090/511520。', source: 'IDX000300 + 债券ETF K线' },
];

// For trend chart sub-indicator selection
const SUB_TREND_OPTIONS = [
  { key: 'compositeScore', label: '综合情绪' },
  ...SUB_INDICATORS.map(s => ({ key: s.key, label: s.label })),
];

const INDEX_OPTIONS = [
  { code: 'IDX000001', label: '上证指数' },
  { code: 'IDX000300', label: '沪深300' },
  { code: 'IDX000852', label: '中证1000' },
  { code: 'IDX399006', label: '创业板指' },
];

function scoreColor(score: number): string {
  if (score >= 70) return '#22c55e';
  if (score >= 50) return '#eab308';
  if (score >= 30) return '#f97316';
  return '#ef4444';
}

function scoreZone(score: number): { label: string; color: string; bg: string } {
  if (score >= 70) return { label: '乐观', color: '#22c55e', bg: '#22c55e14' };
  if (score >= 50) return { label: '中性', color: '#eab308', bg: '#eab30814' };
  if (score >= 30) return { label: '谨慎', color: '#f97316', bg: '#f9731614' };
  return { label: '悲观', color: '#ef4444', bg: '#ef444414' };
}

// ── Semi-Circular Gauge ──
function SentimentGauge({ score }: { score: number }) {
  const radius = 90;
  const strokeWidth = 14;
  const cx = 100, cy = 100;
  const startAngle = -180, endAngle = 0;
  const angleRange = endAngle - startAngle;

  const zones = [
    { start: 0, end: 30, color: '#ef4444' },
    { start: 30, end: 50, color: '#f97316' },
    { start: 50, end: 70, color: '#eab308' },
    { start: 70, end: 100, color: '#22c55e' },
  ];

  const polarToCartesian = (angle: number, r: number) => ({
    x: cx + r * Math.cos((angle * Math.PI) / 180),
    y: cy + r * Math.sin((angle * Math.PI) / 180),
  });

  const describeArc = (startA: number, endA: number, r: number) => {
    const s = polarToCartesian(endA, r), e = polarToCartesian(startA, r);
    const large = endA - startA <= 180 ? '0' : '1';
    return `M ${s.x} ${s.y} A ${r} ${r} 0 ${large} 0 ${e.x} ${e.y}`;
  };

  const needleAngle = startAngle + (score / 100) * angleRange;
  const needleTip = polarToCartesian(needleAngle, radius - 5);
  const needleBase1 = polarToCartesian(needleAngle + 90, 12);
  const needleBase2 = polarToCartesian(needleAngle - 90, 12);
  const zone = scoreZone(score);

  return (
    <div style={{ position: 'relative', width: 200, height: 130 }}>
      <svg viewBox="0 0 200 120" style={{ width: 200, height: 120 }}>
        <path d={describeArc(startAngle, endAngle, radius)} fill="none"
          stroke="var(--color-fill-2)" strokeWidth={strokeWidth} strokeLinecap="round" />
        {zones.map(z => {
          const sa = startAngle + (z.start / 100) * angleRange;
          const ea = startAngle + (z.end / 100) * angleRange;
          return <path key={z.start} d={describeArc(sa, ea, radius)} fill="none"
            stroke={z.color} strokeWidth={strokeWidth} strokeLinecap="butt" opacity={0.85} />;
        })}
        <polygon points={`${needleTip.x},${needleTip.y} ${needleBase1.x},${needleBase1.y} ${needleBase2.x},${needleBase2.y}`}
          fill="var(--color-text-1)" />
        <circle cx={cx} cy={cy} r={5} fill="var(--color-text-1)" />
      </svg>
      <div style={{
        position: 'absolute', bottom: 8, left: 0, right: 0, textAlign: 'center',
      }}>
        <div style={{ fontSize: 24, fontWeight: 700, color: zone.color, lineHeight: 1.1 }}>{score.toFixed(0)}</div>
        <div style={{ fontSize: 12, color: zone.color, fontWeight: 600 }}>{zone.label}</div>
      </div>
    </div>
  );
}

function SnapCard({ label, value, sub, icon, color, bg }: {
  label: string; value: string; sub?: string; icon: React.ReactNode; color?: string; bg?: string;
}) {
  return (
    <div style={{
      background: bg || 'var(--color-bg-2)', borderRadius: 10, padding: '12px 16px',
      border: `1px solid var(--color-border-2)`, display: 'flex', flexDirection: 'column', gap: 4,
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <span style={{ color: color || 'var(--color-text-2)' }}>{icon}</span>
        <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{label}</span>
      </div>
      <div style={{ fontSize: 20, fontWeight: 700, color: color || 'var(--color-text-1)' }}>{value}</div>
      {sub && <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{sub}</span>}
    </div>
  );
}

// ── Main Dashboard ──
export default function SentimentDashboard() {
  const [latest, setLatest] = useState<SentimentData | null>(null);
  const [history, setHistory] = useState<SentimentData[]>([]);
  const [indexData, setIndexData] = useState<Record<string, { tradeDate: string; close: number }[]>>({});
  const [selectedIndex, setSelectedIndex] = useState('IDX000300');
  const [selectedSubKey, setSelectedSubKey] = useState('compositeScore');
  const [loading, setLoading] = useState(true);
  const [distribution, setDistribution] = useState<{label:string;count:number}[]>([]);
  const [turnover, setTurnover] = useState<{amount:number;change:number;changePct:number}>({amount:0,change:0,changePct:0});
  const [showGuide, setShowGuide] = useState(false);

  const fmtAmount = (v: number) => v >= 10000 ? `${(v / 10000).toFixed(2)}万亿` : `${v.toFixed(0)}亿`;

  useEffect(() => {
    Promise.all([
      fetchLatestSentiment().then(r => r.data.data),
      fetchSentimentHistory(90).then(r => r.data.data || []),
    ]).then(([latestData, historyData]) => {
      setLatest(latestData);
      setHistory(Array.isArray(historyData) ? historyData : []);
    }).catch(console.error).finally(() => setLoading(false));
    fetchReturnDistribution().then(r => setDistribution(r.data.data || [])).catch(() => {});
    api.get('/sentiment/turnover').then(r => setTurnover(r.data.data)).catch(() => {});
  }, []);

  useEffect(() => {
    if (history.length === 0) return;
    const start = history[0]?.tradeDate?.slice(0, 10) || '';
    const end = history[history.length - 1]?.tradeDate?.slice(0, 10) || '';
    if (!start || !end) return;
    fetchIndexKLine(selectedIndex, start, end)
      .then(r => r.data?.data || [])
      .then((klines: any[]) => {
        setIndexData(prev => ({
          ...prev,
          [selectedIndex]: klines.map((k: any) => ({ tradeDate: k.tradeDate?.slice(0, 10), close: k.close })),
        }));
      }).catch(() => {});
  }, [history, selectedIndex]);

  // Load index data for all indices on mount
  useEffect(() => {
    if (history.length === 0) return;
    const start = history[0]?.tradeDate?.slice(0, 10) || '';
    const end = history[history.length - 1]?.tradeDate?.slice(0, 10) || '';
    if (!start || !end) return;
    INDEX_OPTIONS.forEach(({ code }) => {
      fetchIndexKLine(code, start, end)
        .then(r => r.data?.data || [])
        .then((klines: any[]) => {
          setIndexData(prev => ({
            ...prev,
            [code]: klines.map((k: any) => ({ tradeDate: k.tradeDate?.slice(0, 10), close: k.close })),
          }));
        }).catch(() => {});
    });
  }, [history]);

  const chartOption = useMemo(() => {
    if (history.length === 0) return {};
    const dates = history.map(d => d.tradeDate?.slice(0, 10) || '');

    // Main sentiment series — pick composite or sub-indicator
    const mainLabel = SUB_TREND_OPTIONS.find(o => o.key === selectedSubKey)?.label || '情绪';
    const mainValues = history.map(d => (d as any)[selectedSubKey] ?? null);

    const idxKlines = indexData[selectedIndex] || [];
    const idxMap: Record<string, number> = {};
    idxKlines.forEach(k => { idxMap[k.tradeDate] = k.close; });
    const idxValues = dates.map(d => idxMap[d] ?? null);
    const idxLabel = INDEX_OPTIONS.find(i => i.code === selectedIndex)?.label || '';

    return {
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(20,20,30,0.92)',
        borderColor: '#333',
        textStyle: { color: '#ccc', fontSize: 12 },
        formatter: (params: any) => {
          let html = `<b>${params[0]?.axisValue}</b><br/>`;
          params.forEach((p: any) => {
            if (p.seriesName === mainLabel) html += `${p.marker} ${mainLabel}: <b>${p.value !== null ? Number(p.value).toFixed(1) : '-'}</b><br/>`;
            else if (p.seriesName && p.value !== null) html += `${p.marker} ${p.seriesName}: <b>${Number(p.value).toFixed(2)}</b><br/>`;
          });
          return html;
        },
      },
      legend: { bottom: 0, textStyle: { color: 'var(--color-text-2)', fontSize: 11 }, data: [mainLabel, idxLabel] },
      grid: { top: 12, right: 60, bottom: 36, left: 48 },
      xAxis: { type: 'category', data: dates, axisLine: { lineStyle: { color: 'var(--color-border-2)' } },
        axisLabel: { color: 'var(--color-text-3)', fontSize: 10, rotate: 30, formatter: (v: string) => v.slice(5) } },
      yAxis: [
        { type: 'value', name: '情绪', nameTextStyle: { color: 'var(--color-text-3)', fontSize: 10 },
          min: 0, max: 100, axisLine: { show: false },
          axisLabel: { color: 'var(--color-text-3)', fontSize: 10 },
          splitLine: { lineStyle: { color: 'var(--color-border-1)' } } },
        { type: 'value', name: idxLabel, nameTextStyle: { color: '#5470c6', fontSize: 10 },
          scale: true, axisLine: { show: false }, axisLabel: { color: '#5470c6', fontSize: 10 }, splitLine: { show: false } },
      ],
      series: [
        { name: mainLabel, type: 'line', data: mainValues, smooth: true, symbol: 'circle', symbolSize: 3,
          lineStyle: { width: 2.5, color: '#165DFF' }, itemStyle: { color: '#165DFF' },
          areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [{ offset: 0, color: 'rgba(22,93,255,0.25)' }, { offset: 1, color: 'rgba(22,93,255,0.02)' }] } } },
        { name: idxLabel, type: 'line', yAxisIndex: 1, data: idxValues, smooth: true, symbol: 'none',
          lineStyle: { color: '#5470c6', width: 1.5, type: 'dashed' }, itemStyle: { color: '#5470c6' } },
      ],
    };
  }, [history, indexData, selectedIndex, selectedSubKey]);

  const stats = useMemo(() => {
    if (history.length === 0) return null;
    const scores = history.map(d => d.compositeScore);
    const latestScore = scores[scores.length - 1];
    const prevScore = scores.length > 1 ? scores[scores.length - 2] : latestScore;
    return {
      diff: latestScore - prevScore,
      ma5: scores.slice(-5).reduce((a, b) => a + b, 0) / Math.min(5, scores.length),
      ma20: scores.slice(-20).reduce((a, b) => a + b, 0) / Math.min(20, scores.length),
      max: Math.max(...scores),
      min: Math.min(...scores),
    };
  }, [history]);

  const distOption = useMemo(() => {
    if (distribution.length === 0) return {};
    const labels = distribution.map(d => d.label);
    const data = distribution.map(d => d.count);
    return {
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
      grid: { top: 10, right: 20, bottom: 40, left: 50 },
      xAxis: { type: 'category', data: labels,
        axisLabel: { color: 'var(--color-text-3)', fontSize: 10 },
        axisLine: { lineStyle: { color: 'var(--color-border-2)' } } },
      yAxis: { type: 'value', name: '股票数',
        axisLabel: { color: 'var(--color-text-3)', fontSize: 10 },
        splitLine: { lineStyle: { color: 'var(--color-border-1)' } } },
      series: [{
        type: 'bar', data: data, barWidth: '70%',
        label: { show: true, position: 'top', fontSize: 10, color: 'var(--color-text-2)', fontWeight: 500 },
        itemStyle: {
          borderRadius: [2, 2, 0, 0],
          color: (params: any) => {
            const label = labels[params.dataIndex];
            if (label === '跌停' || label === '< -8%') return '#16a34a';
            if (label === '-8%~-6%' || label === '-6%~-4%') return '#22c55e';
            if (label === '-4%~-2%' || label === '-2%~0') return '#22c55e';
            if (label === '0%') return '#9ca3af';
            if (label === '0~2%' || label === '2%~4%') return '#ef4444';
            if (label === '4%~6%' || label === '6%~8%') return '#ef4444';
            if (label === '> 8%' || label === '涨停') return '#dc2626';
            return '#888';
          },
        },
      }],
    };
  }, [distribution]);

  if (loading) return <div style={{ padding: 40, color: 'var(--color-text-3)' }}>加载中...</div>;
  if (!latest) return <div style={{ padding: 40, color: 'var(--color-text-3)' }}>暂无市场情绪数据，请先运行数据采集。</div>;

  const zone = scoreZone(latest.compositeScore);
  const upPct = (latest.upCount / Math.max(latest.totalStocks, 1) * 100).toFixed(1);
  const luRate = (latest.limitUpCount / Math.max(latest.totalStocks, 1) * 100).toFixed(2);
  const ldRate = (latest.limitDownCount / Math.max(latest.totalStocks, 1) * 100).toFixed(2);

  return (
    <div style={{ padding: '24px 28px', maxWidth: 1320, margin: '0 auto' }}>
      {/* ── Header ── */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20 }}>
        <div style={{ width: 38, height: 38, borderRadius: 10,
          background: 'linear-gradient(135deg, #165DFF, #722ED1)',
          display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Gauge size={20} color="#fff" />
        </div>
        <div>
          <h1 style={{ margin: 0, fontSize: 20, fontWeight: 700, color: 'var(--color-text-1)' }}>市场情绪指数</h1>
          <p style={{ margin: '2px 0 0', fontSize: 12, color: 'var(--color-text-3)' }}>
            {latest.tradeDate?.slice(0, 10)} · 9维度综合评估 · 回测优化权重
          </p>
        </div>
      </div>

      {/* ── Market Snapshot Cards (top) ── */}
      <div style={{
        display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
        gap: 12, marginBottom: 20,
      }}>
        <SnapCard label="上涨家数" value={String(latest.upCount)} sub={`${upPct}%`}
          icon={<ArrowUp size={14} />} color="#22c55e" bg="#22c55e0d" />
        <SnapCard label="下跌家数" value={String(latest.downCount)}
          sub={`${(latest.downCount / Math.max(latest.totalStocks, 1) * 100).toFixed(1)}%`}
          icon={<ArrowDown size={14} />} color="#ef4444" bg="#ef44440d" />
        <SnapCard label="涨停" value={String(latest.limitUpCount)} sub={`${luRate}%`}
          icon={<Flame size={14} />} color="#ef4444" bg="#ef44440d" />
        <SnapCard label="跌停" value={String(latest.limitDownCount)} sub={`${ldRate}%`}
          icon={<Skull size={14} />} color="#22c55e" bg="#22c55e0d" />
        <SnapCard label="炸板" value={String(latest.boardBreakCount)}
          icon={<Unplug size={14} />} color="#f97316" bg="#f973160d" />
        <SnapCard label="成交额" value={fmtAmount(turnover.amount)}
          sub={`${turnover.change >= 0 ? '+' : ''}${fmtAmount(turnover.change)} ${turnover.changePct >= 0 ? '+' : ''}${turnover.changePct.toFixed(1)}%`}
          icon={<Activity size={14} />} color={turnover.change >= 0 ? '#ef4444' : '#22c55e'}
          bg={turnover.change >= 0 ? '#ef44440d' : '#22c55e0d'} />
        <SnapCard label="有效股票" value={String(latest.totalStocks)}
          icon={<Users size={14} />} color="var(--color-text-2)" bg="var(--color-fill-1)" />
      </div>

      {/* ── Return Distribution Histogram ── */}
      {distribution.length > 0 && (
        <div style={{
          background: 'var(--color-bg-2)', borderRadius: 12,
          border: '1px solid var(--color-border-2)', padding: '16px 20px', marginBottom: 20,
        }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
            <h3 style={{ margin: 0, fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>市场涨跌分布</h3>
            <div style={{ display: 'flex', gap: 10, fontSize: 11, color: 'var(--color-text-3)' }}>
              <span>📈 上涨 {latest.upCount} 家</span>
              <span>📉 下跌 {latest.downCount} 家</span>
              <span style={{ color: '#ef4444' }}>🔥 涨停 {latest.limitUpCount}</span>
              <span style={{ color: '#22c55e' }}>💀 跌停 {latest.limitDownCount}</span>
            </div>
          </div>
          <ReactECharts option={distOption} style={{ height: 240 }} />
        </div>
      )}

      {/* ── Gauge + Stats Row ── */}
      <div style={{
        display: 'flex', gap: 24, marginBottom: 20,
        background: 'var(--color-bg-2)', borderRadius: 12,
        border: '1px solid var(--color-border-2)', padding: '20px 28px',
        alignItems: 'center', flexWrap: 'wrap',
      }}>
        <SentimentGauge score={latest.compositeScore} />
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '16px 32px', flex: 1 }}>
          {stats && (<>
            <StatItem label="情绪加速度" value={stats.diff > 0 ? `+${stats.diff.toFixed(1)}` : stats.diff.toFixed(1)}
              icon={stats.diff > 0 ? <ChevronUp size={14} /> : stats.diff < 0 ? <ChevronDown size={14} /> : <Minus size={14} />}
              color={stats.diff > 0 ? '#22c55e' : stats.diff < 0 ? '#ef4444' : 'var(--color-text-2)'} />
            <StatItem label="5日情绪均值" value={stats.ma5.toFixed(1)} color="var(--color-text-1)" />
            <StatItem label="20日情绪均值" value={stats.ma20.toFixed(1)} color="var(--color-text-1)" />
            <StatItem label="区间最高(亢奋)" value={stats.max.toFixed(0)} color="#22c55e" />
            <StatItem label="区间最低(恐惧)" value={stats.min.toFixed(0)} color="#ef4444" />
            <StatItem label="数据天数" value={`${history.length}天`} color="var(--color-text-2)" />
          </>)}
        </div>
      </div>

      {/* ── Sentiment + Index Trend Chart ── */}
      <div style={{
        background: 'var(--color-bg-2)', borderRadius: 12,
        border: '1px solid var(--color-border-2)', padding: '16px 20px', marginBottom: 20,
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8, flexWrap: 'wrap', gap: 8 }}>
          <h3 style={{ margin: 0, fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>情绪趋势与大盘对比</h3>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <select value={selectedSubKey} onChange={e => setSelectedSubKey(e.target.value)} style={{
              padding: '3px 8px', borderRadius: 6, border: '1px solid var(--color-border-2)',
              background: 'var(--color-bg-1)', color: 'var(--color-text-2)',
              fontSize: 11, cursor: 'pointer', outline: 'none',
            }}>
              {SUB_TREND_OPTIONS.map(o => (
                <option key={o.key} value={o.key}>{o.label}</option>
              ))}
            </select>
            <div style={{ display: 'flex', gap: 4 }}>
              {INDEX_OPTIONS.map(idx => (
                <button key={idx.code} onClick={() => setSelectedIndex(idx.code)} style={{
                  padding: '3px 10px', borderRadius: 6, border: '1px solid',
                  borderColor: selectedIndex === idx.code ? 'var(--color-primary)' : 'var(--color-border-2)',
                  background: selectedIndex === idx.code ? 'var(--color-primary-light-1)' : 'transparent',
                  color: selectedIndex === idx.code ? 'var(--color-primary)' : 'var(--color-text-2)',
                  cursor: 'pointer', fontSize: 11, fontWeight: selectedIndex === idx.code ? 600 : 400,
                }}>{idx.label}</button>
              ))}
            </div>
          </div>
        </div>
        <ReactECharts option={chartOption} style={{ height: 280 }} />
        <div style={{ display: 'flex', gap: 12, justifyContent: 'center', marginTop: 8, fontSize: 11 }}>
          {[{ color: '#ef4444', label: '悲观 0-30' }, { color: '#f97316', label: '谨慎 30-50' },
            { color: '#eab308', label: '中性 50-70' }, { color: '#22c55e', label: '乐观 70-100' }]
            .map(z => (<span key={z.label} style={{ display: 'flex', alignItems: 'center', gap: 4, color: 'var(--color-text-3)' }}>
              <span style={{ width: 10, height: 10, borderRadius: 2, background: z.color }} />{z.label}</span>))}
        </div>
      </div>

      {/* ── Sub-indicators ── */}
      <div style={{
        background: 'var(--color-bg-2)', borderRadius: 12,
        border: '1px solid var(--color-border-2)', padding: '16px 20px',
      }}>
        <h3 style={{ margin: '0 0 14px', fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>子指标详情</h3>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 10 }}>
          {SUB_INDICATORS.map(({ key, label, icon: Icon, desc, weight }) => {
            const scoreVal = (latest as any)[key] || 0;
            const c = scoreColor(scoreVal);
            return (
              <div key={key} style={{
                background: 'var(--color-fill-1)', borderRadius: 8, padding: '10px 14px',
                borderLeft: `3px solid ${c}`,
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
                  <Icon size={14} style={{ color: 'var(--color-text-2)' }} />
                  <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{desc}</span>
                </div>
                <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 4 }}>
                    <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>{label}</span>
                    <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>{weight}%</span>
                  </div>
                  <span style={{ fontSize: 16, fontWeight: 700, color: c }}>{scoreVal.toFixed(0)}</span>
                </div>
              </div>
            );
          })}
        </div>

        {/* ── Indicator Guide (collapsible) ── */}
        <div style={{ marginTop: 12, borderTop: '1px solid var(--color-border-1)', paddingTop: 12 }}>
          <button
            onClick={() => setShowGuide(!showGuide)}
            style={{
              display: 'flex', alignItems: 'center', gap: 4, background: 'none', border: 'none',
              cursor: 'pointer', padding: 0, color: 'var(--color-text-2)', fontSize: 12, fontWeight: 500,
            }}
          >
            <ChevronRight size={14} style={{ transform: showGuide ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.15s' }} />
            指标计算详解（9维度公式 + 权重 + 数据源）
          </button>
          {showGuide && (
            <div style={{ marginTop: 10, display: 'flex', flexDirection: 'column', gap: 8 }}>
              {INDICATOR_SPECS.map(s => (
                <div key={s.label} style={{
                  background: 'var(--color-fill-1)', borderRadius: 8, padding: '10px 14px',
                }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                    <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>{s.label}</span>
                    <span style={{
                      fontSize: 10, fontWeight: 600, color: '#165DFF',
                      background: 'rgba(22,93,255,0.1)', padding: '1px 6px', borderRadius: 4,
                    }}>{s.weight}</span>
                  </div>
                  <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4, fontFamily: 'SF Mono, monospace' }}>
                    {s.formula}
                  </div>
                  <div style={{ fontSize: 11, color: 'var(--color-text-2)', marginBottom: 3 }}>{s.detail}</div>
                  <div style={{ fontSize: 10, color: 'var(--color-text-3)', opacity: 0.7 }}>
                    数据源: {s.source}
                  </div>
                </div>
              ))}
              <div style={{ fontSize: 10, color: 'var(--color-text-3)', textAlign: 'center', marginTop: 4 }}>
                权重基于 119 交易日回测优化（2026-06-20），最大化与次日大盘收益相关度 · 评分采用 252 日滚动历史分位法
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function StatItem({ label, value, icon, color }: {
  label: string; value: string; icon?: React.ReactNode; color?: string;
}) {
  return (
    <div>
      <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 2 }}>{label}</div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
        {icon && <span style={{ color }}>{icon}</span>}
        <span style={{ fontSize: 16, fontWeight: 600, color: color || 'var(--color-text-1)' }}>{value}</span>
      </div>
    </div>
  );
}
