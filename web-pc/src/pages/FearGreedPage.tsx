import { useEffect, useState, useMemo } from 'react';
import { Card, Spin } from '@arco-design/web-react';
import api from '../services/api';
import ReactECharts from 'echarts-for-react';
import { Gauge, TrendingUp, TrendingDown, BarChart3, DollarSign, Activity, Zap } from 'lucide-react';

interface FearGreedFactor {
  name: string;
  raw: number;
  score: number;
  label: string;
}

interface FearGreedData {
  tradeDate: string;
  score: number;
  zone: string;
  factors: FearGreedFactor[];
}

const ZONE_COLORS: Record<string, string> = {
  '极度恐惧': '#3b82f6',
  '恐惧': '#60a5fa',
  '中性': '#eab308',
  '贪婪': '#f97316',
  '极度贪婪': '#ef4444',
};

const FACTOR_ICONS: Record<string, any> = {
  '涨跌停比': TrendingUp,
  '涨跌比': BarChart3,
  '量能偏离': Activity,
  '北向资金': DollarSign,
  '波动率': Gauge,
  '炸板率': Zap,
};

export default function FearGreedPage() {
  const [data, setData] = useState<FearGreedData | null>(null);
  const [history, setHistory] = useState<FearGreedData[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      api.get('/sentiment/fear-greed').then((r: any) => r.data?.data),
      api.get('/sentiment/fear-greed/history', { params: { days: 60 } }).then((r: any) => r.data?.data || []),
    ])
      .then(([latest, hist]) => {
        setData(latest || null);
        setHistory(hist);
      })
      .catch((err) => console.error('[FearGreed] fetch failed:', err))
      .finally(() => setLoading(false));
  }, []);

  const zoneColor = ZONE_COLORS[data?.zone || ''] || '#eab308';

  const gaugeOption = useMemo(() => ({
    series: [{
      type: 'gauge',
      startAngle: 210,
      endAngle: -30,
      center: ['50%', '58%'],
      radius: '90%',
      min: 0,
      max: 100,
      splitNumber: 10,
      axisLine: {
        show: true,
        lineStyle: {
          width: 20,
          color: [
            [0.25, '#3b82f6'],
            [0.45, '#60a5fa'],
            [0.55, '#eab308'],
            [0.75, '#f97316'],
            [1, '#ef4444'],
          ],
        },
      },
      pointer: { icon: 'path://M12.8,0.7l12,40.1H0.7L12.8,0.7z', length: '70%', width: 8, offsetCenter: [0, '-10%'], itemStyle: { color: 'auto' } },
      axisTick: { distance: -20, length: 8, lineStyle: { width: 1, color: '#999' } },
      splitLine: { distance: -24, length: 18, lineStyle: { width: 3, color: '#999' } },
      axisLabel: { color: 'var(--color-text-3)', distance: 35, fontSize: 10 },
      anchor: { show: true, showAbove: true, size: 16, itemStyle: { borderWidth: 2 } },
      title: { show: false },
      detail: {
        valueAnimation: true,
        fontSize: 40,
        fontWeight: 700,
        offsetCenter: [0, '60%'],
        formatter: (v: number) => `${v}`,
        color: zoneColor,
      },
      data: [{ value: data?.score ?? 50, name: data?.zone || '-' }],
    }],
  }), [data, zoneColor]);

  const historyOption = useMemo(() => ({
    tooltip: { trigger: 'axis' },
    grid: { left: 50, right: 50, top: 10, bottom: 44 },
    xAxis: {
      type: 'category',
      data: history.map(d => d.tradeDate.slice(5)),
      axisLabel: { fontSize: 10 },
    },
    yAxis: {
      type: 'value', min: 0, max: 100,
      axisLabel: { fontSize: 10 },
      splitLine: { lineStyle: { color: 'var(--color-border-1)' } },
    },
    dataZoom: [{ type: 'slider', start: history.length > 30 ? 40 : 0, end: 100, height: 20, bottom: 4 }],
    visualMap: {
      show: false,
      pieces: [
        { lte: 25, color: '#3b82f6' },
        { gt: 25, lte: 45, color: '#60a5fa' },
        { gt: 45, lte: 55, color: '#eab308' },
        { gt: 55, lte: 75, color: '#f97316' },
        { gt: 75, color: '#ef4444' },
      ],
      dimension: 1,
    },
    series: [{
      type: 'bar',
      data: history.map(d => d.score),
      barMaxWidth: 14,
      markLine: {
        silent: true,
        data: [
          { yAxis: 25, label: { formatter: '极度恐惧', fontSize: 9 }, lineStyle: { color: '#3b82f6', type: 'dashed' } },
          { yAxis: 45, label: { formatter: '恐惧', fontSize: 9 }, lineStyle: { color: '#60a5fa', type: 'dashed' } },
          { yAxis: 55, label: { formatter: '中性', fontSize: 9 }, lineStyle: { color: '#eab308', type: 'dashed' } },
          { yAxis: 75, label: { formatter: '贪婪', fontSize: 9 }, lineStyle: { color: '#f97316', type: 'dashed' } },
        ],
      },
    }],
  }), [history]);

  if (loading) {
    return <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 300 }}><Spin size={40} /></div>;
  }

  return (
    <div style={{ padding: '20px 24px', maxWidth: 1400, margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 14, marginBottom: 24 }}>
        <div style={{ width: 44, height: 44, borderRadius: 10, background: 'linear-gradient(135deg, #ef4444, #f97316)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Gauge size={22} color="#fff" />
        </div>
        <div>
          <h1 style={{ fontSize: 18, fontWeight: 700, margin: 0, color: 'var(--color-text-1)' }}>恐慌贪婪指数</h1>
          <p style={{ margin: '2px 0 0', fontSize: 12, color: 'var(--color-text-3)' }}>{data?.tradeDate?.slice(0, 10) || '—'} · 6因子综合：涨跌停比 · 涨跌家数 · 量能 · 北向 · 波动 · 炸板</p>
        </div>
      </div>

      {/* Gauge + Factors Row */}
      <div style={{ display: 'grid', gridTemplateColumns: '380px 1fr', gap: 14, marginBottom: 14 }}>
        {/* Gauge Card */}
        <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
          <div style={{ textAlign: 'center', paddingBottom: 8 }}>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>{data?.tradeDate}</div>
            <div style={{ fontSize: 24, fontWeight: 700, color: zoneColor, marginTop: 4 }}>{data?.zone || '-'}</div>
          </div>
          <ReactECharts option={gaugeOption} style={{ height: 260 }} />
        </Card>

        {/* 6 Factors Grid */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 10 }}>
          {(data?.factors || []).map((f, i) => {
            const Icon = FACTOR_ICONS[f.name] || Activity;
            const barColor = f.score >= 60 ? '#ef4444' : f.score >= 45 ? '#f97316' : f.score >= 35 ? '#eab308' : '#3b82f6';
            return (
              <Card key={i} style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
                  <Icon size={16} color="var(--color-text-2)" />
                  <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>{f.name}</span>
                </div>
                <div style={{ fontSize: 24, fontWeight: 700, color: barColor, marginBottom: 4 }}>{Math.round(f.score)}</div>
                <div style={{ height: 6, background: 'var(--color-fill-1)', borderRadius: 3, overflow: 'hidden', marginBottom: 6 }}>
                  <div style={{ height: '100%', width: `${f.score}%`, background: barColor, borderRadius: 3, transition: 'width 0.5s' }} />
                </div>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11, color: 'var(--color-text-3)' }}>
                  <span>{f.label}</span>
                  <span style={{ fontFamily: "'SF Mono', 'Inter', monospace" }}>{f.raw}</span>
                </div>
              </Card>
            );
          })}
        </div>
      </div>

      {/* History Chart */}
      <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }} title="恐慌贪婪指数 · 历史走势">
        <ReactECharts option={historyOption} style={{ height: 280 }} />
        <div style={{ display: 'flex', gap: 18, justifyContent: 'center', paddingBottom: 8 }}>
          {Object.entries(ZONE_COLORS).map(([zone, color]) => (
            <div key={zone} style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11, color: 'var(--color-text-3)' }}>
              <div style={{ width: 12, height: 12, borderRadius: 3, background: color }} />
              {zone}
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}
