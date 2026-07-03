import { useEffect, useState, useMemo } from 'react';
import { Card, Spin } from '@arco-design/web-react';
import api from '../services/api';
import ReactECharts from 'echarts-for-react';
import { TrendingUp, TrendingDown, BarChart3, Activity, Zap, Flame, Skull } from 'lucide-react';

interface LimitStats {
  tradeDate: string;
  upCount: number;
  downCount: number;
  riseCount: number;
  fallCount: number;
  boardBreak: number;
  maxStreak: number;
  totalStocks: number;
}

function sentimentZone(upRatio: number): { label: string; color: string; bg: string } {
  if (upRatio >= 3.0) return { label: '极度亢奋', color: '#ef4444', bg: '#ef444408' };
  if (upRatio >= 2.0) return { label: '偏热', color: '#f97316', bg: '#f9731608' };
  if (upRatio >= 1.0) return { label: '温和', color: '#22c55e', bg: '#22c55e08' };
  if (upRatio >= 0.5) return { label: '偏冷', color: '#eab308', bg: '#eab30808' };
  return { label: '冰点', color: '#3b82f6', bg: '#3b82f608' };
}

export default function LimitStatsPage() {
  const [data, setData] = useState<LimitStats[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    api.get('/sentiment/limit-stats', { params: { days: 60 } })
      .then((r: any) => setData(r.data?.data || []))
      .catch((err) => console.error('[LimitStats] fetch failed:', err))
      .finally(() => setLoading(false));
  }, []);

  const latest = data[data.length - 1];
  const zone = useMemo(() => {
    if (!latest) return { label: '-', color: 'var(--color-text-3)', bg: '' };
    return sentimentZone(latest.upCount / Math.max(latest.downCount, 1));
  }, [latest]);

  const upDownChart = useMemo(() => ({
    tooltip: { trigger: 'axis' },
    legend: { bottom: 0, textStyle: { fontSize: 11, color: 'var(--color-text-2)' } },
    grid: { left: 50, right: 50, top: 10, bottom: 44 },
    xAxis: { type: 'category', data: data.map(d => d.tradeDate.slice(5)), axisLabel: { fontSize: 10 } },
    yAxis: [
      { type: 'value', name: '涨停/跌停', axisLabel: { fontSize: 10 } },
      { type: 'value', name: '炸板', axisLabel: { fontSize: 10 } },
    ],
    dataZoom: [{ type: 'slider', start: data.length > 30 ? 40 : 0, end: 100, height: 20, bottom: 4 }],
    series: [
      { name: '涨停', type: 'bar', data: data.map(d => d.upCount), itemStyle: { color: '#ef4444' }, barMaxWidth: 14, barCategoryGap: '30%' },
      { name: '跌停', type: 'bar', data: data.map(d => d.downCount), itemStyle: { color: '#22c55e' }, barMaxWidth: 14, barCategoryGap: '30%' },
      { name: '炸板', type: 'line', yAxisIndex: 1, data: data.map(d => d.boardBreak), smooth: true, lineStyle: { color: '#f59e0b', width: 1.5 }, symbol: 'none' },
    ],
  }), [data]);

  const advDeclChart = useMemo(() => ({
    tooltip: { trigger: 'axis' },
    legend: { bottom: 0, textStyle: { fontSize: 11 } },
    grid: { left: 50, right: 50, top: 10, bottom: 44 },
    xAxis: { type: 'category', data: data.map(d => d.tradeDate.slice(5)), axisLabel: { fontSize: 10 } },
    yAxis: [
      { type: 'value', name: '家数', axisLabel: { fontSize: 10 } },
      { type: 'value', name: '涨跌比', axisLabel: { fontSize: 10 } },
    ],
    dataZoom: [{ type: 'slider', start: data.length > 30 ? 40 : 0, end: 100, height: 20, bottom: 4 }],
    series: [
      { name: '上涨', type: 'bar', data: data.map(d => d.riseCount), stack: 'x', itemStyle: { color: '#ef4444' }, barMaxWidth: 20 },
      {
        name: '涨跌比', type: 'line', yAxisIndex: 1,
        data: data.map(d => {
          const r = d.downCount > 0 ? d.riseCount / d.downCount : 5;
          return Math.min(r, 5);
        }),
        smooth: true, lineStyle: { color: '#8b5cf6', width: 2 }, symbol: 'none',
        markLine: {
          silent: true,
          data: [
            { yAxis: 1, label: { formatter: '1:1', fontSize: 10 }, lineStyle: { color: 'var(--color-text-3)', type: 'dashed' } },
          ],
        },
      },
    ],
  }), [data]);

  const upDownRatioChart = useMemo(() => ({
    tooltip: { trigger: 'axis' },
    grid: { left: 50, right: 20, top: 10, bottom: 44 },
    xAxis: { type: 'category', data: data.map(d => d.tradeDate.slice(5)), axisLabel: { fontSize: 10 } },
    yAxis: { type: 'value', name: '涨停/跌停比', axisLabel: { fontSize: 10 }, min: 0 },
    visualMap: {
      show: false, dimension: 0,
      pieces: [
        { lt: 0.5, color: '#3b82f6' },
        { gte: 0.5, lt: 1, color: '#eab308' },
        { gte: 1, lt: 2, color: '#22c55e' },
        { gte: 2, lt: 3, color: '#f97316' },
        { gte: 3, color: '#ef4444' },
      ],
    },
    dataZoom: [{ type: 'slider', start: data.length > 30 ? 40 : 0, end: 100, height: 20, bottom: 4 }],
    series: [{
      type: 'bar',
      data: data.map(d => {
        const r = d.downCount > 0 ? d.upCount / d.downCount : 10;
        return Math.min(r, 10);
      }),
      barMaxWidth: 18,
      markLine: {
        silent: true,
        data: [
          { yAxis: 1, label: { formatter: '均衡', fontSize: 10 }, lineStyle: { color: 'var(--color-text-3)', type: 'dashed' } },
          { yAxis: 2, label: { formatter: '过热', fontSize: 10 }, lineStyle: { color: '#f97316', type: 'dashed' } },
        ],
      },
    }],
  }), [data]);

  if (loading) {
    return <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 300 }}><Spin size={40} /></div>;
  }

  return (
    <div style={{ padding: '20px 24px', maxWidth: 1400, margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 14, marginBottom: 24 }}>
        <div style={{ width: 44, height: 44, borderRadius: 10, background: 'linear-gradient(135deg, #165DFF, #722ED1)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <BarChart3 size={22} color="#fff" />
        </div>
        <div>
          <h1 style={{ fontSize: 18, fontWeight: 700, margin: 0, color: 'var(--color-text-1)' }}>涨跌停情绪</h1>
          <p style={{ margin: '2px 0 0', fontSize: 12, color: 'var(--color-text-3)' }}>{latest?.tradeDate?.slice(0, 10) || '—'} · 涨停/跌停家数 · 炸板率 · 涨跌比 · 情绪带</p>
        </div>
      </div>

      {/* Stats Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 14, marginBottom: 20 }}>
        {[
          { icon: Flame, label: '涨停家数', value: latest?.upCount ?? '-', color: '#ef4444' },
          { icon: Skull, label: '跌停家数', value: latest?.downCount ?? '-', color: '#22c55e' },
          { icon: TrendingUp, label: '涨跌比', value: latest ? (latest.downCount > 0 ? (latest.riseCount / latest.downCount).toFixed(2) : '∞') : '-', color: '#8b5cf6' },
          { icon: Activity, label: `情绪带 · ${zone.label}`, value: latest ? `${latest.boardBreak} 炸板` : '-', color: zone.color },
        ].map((c, i) => (
          <Card key={i} style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <c.icon size={18} color={c.color} />
              <div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{c.label}</div>
                <div style={{ fontSize: 16, fontWeight: 700, color: 'var(--color-text-1)' }}>{c.value}</div>
              </div>
            </div>
          </Card>
        ))}
      </div>

      {/* Charts Row 1 */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14, marginBottom: 14 }}>
        <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }} title="涨停 / 跌停 / 炸板">
          <ReactECharts option={upDownChart} style={{ height: 280 }} />
        </Card>
        <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }} title="上涨 / 下跌 家数 & 涨跌比">
          <ReactECharts option={advDeclChart} style={{ height: 280 }} />
        </Card>
      </div>

      {/* Charts Row 2 */}
      <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }} title="涨停/跌停 比（情绪带）">
        <ReactECharts option={upDownRatioChart} style={{ height: 200 }} />
        <div style={{ display: 'flex', gap: 18, justifyContent: 'center', paddingBottom: 12 }}>
          {[
            { color: '#3b82f6', label: '冰点 (<0.5)' },
            { color: '#eab308', label: '偏冷 (0.5-1)' },
            { color: '#22c55e', label: '温和 (1-2)' },
            { color: '#f97316', label: '偏热 (2-3)' },
            { color: '#ef4444', label: '亢奋 (>3)' },
          ].map((z, i) => (
            <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11, color: 'var(--color-text-3)' }}>
              <div style={{ width: 12, height: 12, borderRadius: 3, background: z.color }} />
              {z.label}
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}
