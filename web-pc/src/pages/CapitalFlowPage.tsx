import { useEffect, useState, useMemo } from 'react';
import { Card, Spin, Table, Select } from '@arco-design/web-react';
import api from '../services/api';
import ReactECharts from 'echarts-for-react';
import { DollarSign, TrendingUp, TrendingDown, ArrowUpDown, Landmark, Briefcase } from 'lucide-react';

interface CapitalSummary {
  northboundNet: number; northboundDate: string; fundFlowMain: number; fundFlowDate: string;
  marginBalance: number; marginDate: string; dragonTigerCnt: number; blockTradeCnt: number;
}

export default function CapitalFlowPage() {
  const [summary, setSummary] = useState<CapitalSummary | null>(null);
  const [fundRank, setFundRank] = useState<any[]>([]);
  const [fundDir, setFundDir] = useState<'in' | 'out'>('in');
  const [fundDaily, setFundDaily] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      api.get('/capital-flow/summary').then((r: any) => setSummary(r.data?.data)),
      api.get('/capital-flow/fund-top', { params: { limit: 20, direction: fundDir } }).then((r: any) => setFundRank(r.data?.data || [])),
      api.get('/capital-flow/daily', { params: { days: 60 } }).then((r: any) => setFundDaily(r.data?.data || [])),
    ]).finally(() => setLoading(false));
  }, [fundDir]);

  const fundChart = useMemo(() => ({
    tooltip: { trigger: 'axis' },
    legend: { bottom: 0, textStyle: { fontSize: 10 } },
    grid: { left: 60, right: 20, top: 10, bottom: 28 },
    xAxis: { type: 'category', data: [...fundDaily].reverse().map((d: any) => d.tradeDate?.slice(5)), axisLabel: { fontSize: 10 } },
    yAxis: { type: 'value', name: '万元', axisLabel: { fontSize: 10 } },
    series: [
      { name: '主力净流入', type: 'bar', data: [...fundDaily].reverse().map((d: any) => d.mainNet?.toFixed(2)), itemStyle: { color: '#ef4444' }, barMaxWidth: 14 },
      { name: '超大单', type: 'bar', data: [...fundDaily].reverse().map((d: any) => d.superNet?.toFixed(2)), itemStyle: { color: '#f97316' }, barMaxWidth: 14 },
      { name: '小单', type: 'bar', data: [...fundDaily].reverse().map((d: any) => d.smallNet?.toFixed(2)), itemStyle: { color: '#22c55e' }, barMaxWidth: 14 },
    ],
  }), [fundDaily]);

  if (loading) return <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 300 }}><Spin size={40} /></div>;

  return (
    <div style={{ padding: '20px 24px', maxWidth: 1400, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 14, marginBottom: 24 }}>
        <div style={{ width: 44, height: 44, borderRadius: 10, background: 'linear-gradient(135deg, #22c55e, #3b82f6)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <DollarSign size={22} color="#fff" />
        </div>
        <div>
          <h1 style={{ fontSize: 18, fontWeight: 700, margin: 0, color: 'var(--color-text-1)' }}>资金面综合看板</h1>
          <p style={{ margin: '2px 0 0', fontSize: 12, color: 'var(--color-text-3)' }}>北向资金 · 主力流向 · 融资融券 · 龙虎榜 · 大宗交易</p>
        </div>
      </div>

      {/* Summary Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 12, marginBottom: 16 }}>
        {[
          { icon: TrendingUp, label: '北向净流入', value: `${(summary?.northboundNet||0).toFixed(1)}亿`, color: (summary?.northboundNet||0) >= 0 ? '#ef4444' : '#22c55e', sub: summary?.northboundDate },
          { icon: DollarSign, label: '主力净流入', value: `${(summary?.fundFlowMain||0).toFixed(1)}亿`, color: (summary?.fundFlowMain||0) >= 0 ? '#ef4444' : '#22c55e', sub: summary?.fundFlowDate },
          { icon: Landmark, label: '融资余额', value: `${(summary?.marginBalance||0).toFixed(0)}亿`, color: '#f97316', sub: summary?.marginDate },
          { icon: Briefcase, label: '龙虎榜上榜', value: `${summary?.dragonTigerCnt||0} 只`, color: '#8b5cf6', sub: '最新交易日' },
          { icon: ArrowUpDown, label: '大宗交易', value: `${summary?.blockTradeCnt||0} 笔`, color: '#ec4899', sub: '最新交易日' },
        ].map((c, i) => (
          <Card key={i} style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
              <c.icon size={16} color={c.color} />
              <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{c.label}</span>
            </div>
            <div style={{ fontSize: 18, fontWeight: 700, color: c.color }}>{c.value}</div>
            {c.sub && <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 4 }}>{c.sub}</div>}
          </Card>
        ))}
      </div>

      {/* Fund Flow Chart + Ranking */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 400px', gap: 14, marginBottom: 14 }}>
        <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }} title="市场资金流向趋势（主力/超大单/小单）">
          <ReactECharts option={fundChart} style={{ height: 320 }} />
        </Card>
        <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}
          title={<div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span>资金流向排名</span><span style={{ fontSize: 11, color: 'var(--color-text-3)', marginLeft: 8 }}>{fundRank[0]?.tradeDate || ''}</span>
            <Select size="mini" value={fundDir} onChange={setFundDir} style={{ width: 80 }}>
              <Select.Option value="in">流入TOP</Select.Option>
              <Select.Option value="out">流出TOP</Select.Option>
            </Select>
          </div>}>
          <Table data={fundRank} size="small" rowKey="code" pagination={false}
            columns={[
              { title: '代码', dataIndex: 'code', width: 70 },
              { title: '名称', dataIndex: 'name', width: 70, ellipsis: true },
              { title: '主力净额', dataIndex: 'mainNet', width: 80, render: (v: number) => <span style={{ fontFamily: 'monospace', color: v >= 0 ? '#ef4444' : '#22c55e', fontSize: 12 }}>{v?.toFixed(2)}亿</span> },
              { title: '超大单', dataIndex: 'superNet', width: 70, render: (v: number) => <span style={{ fontFamily: 'monospace', fontSize: 11 }}>{v?.toFixed(2)}亿</span> },
            ]} />
        </Card>
      </div>
    </div>
  );
}
