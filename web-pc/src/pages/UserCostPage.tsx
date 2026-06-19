import { useState, useEffect, useCallback, useRef } from 'react';
import { Select, Pagination, Spin } from '@arco-design/web-react';
import {
  DollarSign, Coins, Calendar, Clock, Cpu,
  TrendingUp, BarChart3, ListFilter, CheckCircle, XCircle,
} from 'lucide-react';
import * as echarts from 'echarts';
import { authFetch } from '../services/api';

const MODULE_COLORS: Record<string, string> = {
  chat: '#165DFF',
  stock_score: '#00B42A',
  stock_profile: '#FF7D00',
  strategy_gen: '#722ED1',
  strategy_opt: '#F53F3F',
};

const MODULE_LABELS: Record<string, string> = {
  chat: 'AI对话', stock_score: '股票评分', stock_profile: '公司简介',
  strategy_gen: '策略生成', strategy_opt: '提示词优化',
};

interface Summary {
  totalCost: number; totalCalls: number; totalTokens: number;
  todayCost: number; todayTokens: number; modelCalls: number;
}
interface LogItem {
  id: number; userId: number; username: string; module: string;
  modelName: string; promptTokens: number; completionTokens: number;
  totalTokens: number; costAmount: number; durationMs: number;
  success: boolean; createdAt: string;
}
interface DailyItem { date: string; module: string; cost: number; tokens: number; }

export default function UserCostPage() {
  const [summary, setSummary] = useState<Summary>({} as Summary);
  const [logs, setLogs] = useState<LogItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [module, setModule] = useState('');
  const [month, setMonth] = useState(new Date().toISOString().slice(0, 7));
  const [daily, setDaily] = useState<DailyItem[]>([]);
  const [loading, setLoading] = useState(true);
  const chartRef = useRef<HTMLDivElement>(null);
  const chartInstance = useRef<echarts.ECharts>();

  const fmtCost = (n: number) => n < 0.001 ? '<¥0.001' : '¥' + n.toFixed(4);
  const fmtTokens = (n: number) => n >= 1000 ? (n / 1000).toFixed(1) + 'k' : String(n);
  const fmtMs = (n: number) => n >= 1000 ? (n / 1000).toFixed(1) + 's' : n + 'ms';

  const fetchSummary = useCallback(async () => {
    try {
      const r = await authFetch('/api/v1/cost/summary');
      const j = await r.json();
      if (j.data) setSummary(j.data);
    } catch (e) { console.error(e); }
  }, []);

  const fetchLogs = useCallback(async () => {
    try {
      const params: any = { page, pageSize: 20 };
      if (module) params.module = module;
      const r = await authFetch('/api/v1/cost/logs?' + new URLSearchParams(params));
      const j = await r.json();
      if (j.data) { setLogs(j.data.list || []); setTotal(j.data.total || 0); }
    } catch (e) { console.error(e); }
  }, [page, module]);

  const fetchDaily = useCallback(async () => {
    try {
      const r = await authFetch('/api/v1/cost/daily?month=' + month);
      const j = await r.json();
      if (j.data) setDaily(j.data);
    } catch (e) { console.error(e); }
  }, [month]);

  useEffect(() => { setLoading(true); Promise.all([fetchSummary(), fetchLogs(), fetchDaily()]).finally(() => setLoading(false)); }, [fetchSummary, fetchLogs, fetchDaily]);

  // Render chart
  useEffect(() => {
    if (!chartRef.current || daily.length === 0) {
      // Even with no data, show empty chart with dates
      if (chartRef.current) {
        if (!chartInstance.current) chartInstance.current = echarts.init(chartRef.current);
        const [year, m] = month.split('-').map(Number);
        const daysInMonth = new Date(year, m, 0).getDate();
        const allDates: string[] = [];
        for (let d = 1; d <= daysInMonth; d++) allDates.push(String(d));
        chartInstance.current.setOption({
          tooltip: { trigger: 'axis' },
          legend: { bottom: 0, itemWidth: 12, itemHeight: 12, textStyle: { fontSize: 11 } },
          grid: { left: 50, right: 10, top: 10, bottom: 35 },
          xAxis: { type: 'category', data: allDates, axisLabel: { fontSize: 10, rotate: allDates.length > 15 ? 45 : 0 } },
          yAxis: { type: 'value', axisLabel: { fontSize: 10, formatter: (v: number) => '¥' + v.toFixed(2) } },
          series: [{ type: 'bar', data: allDates.map(() => 0), itemStyle: { color: '#e5e6eb' } }],
        }, true);
      }
      return;
    }
    if (!chartInstance.current) {
      chartInstance.current = echarts.init(chartRef.current);
    }

    // Generate all days in the selected month
    const [year, m] = month.split('-').map(Number);
    const daysInMonth = new Date(year, m, 0).getDate();
    const allDates: string[] = [];
    for (let d = 1; d <= daysInMonth; d++) {
      allDates.push(`${month}-${String(d).padStart(2, '0')}`);
    }

    // Group by date → { module → cost }
    const dateMap: Record<string, Record<string, number>> = {};
    const allModules = new Set<string>();
    for (const d of daily) {
      if (!dateMap[d.date]) dateMap[d.date] = {};
      dateMap[d.date][d.module] = (dateMap[d.date][d.module] || 0) + d.cost;
      allModules.add(d.module);
    }
    const modules = ['chat', 'stock_score', 'stock_profile', 'strategy_gen', 'strategy_opt'].filter(m => allModules.has(m));
    if (modules.length === 0 && allModules.size > 0) Array.from(allModules).forEach(m => modules.push(m));
    if (modules.length === 0) modules.push('chat');

    const series = modules.map(m => ({
      name: MODULE_LABELS[m] || m,
      type: 'bar' as const,
      stack: 'total',
      emphasis: { focus: 'series' as const },
      barWidth: allDates.length > 15 ? '60%' : '40%',
      itemStyle: {
        color: MODULE_COLORS[m] || '#86909C',
        borderRadius: allDates.length <= 7 ? [3, 3, 0, 0] : 0,
      },
      data: allDates.map(d => dateMap[d]?.[m] || 0),
    }));

    const hasData = daily.length > 0;

    chartInstance.current.setOption({
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
        backgroundColor: 'var(--color-bg-2)',
        borderColor: 'var(--color-border-2)',
        textStyle: { color: 'var(--color-text-1)', fontSize: 12 },
        formatter: (params: any) => {
          let s = '<b>' + params[0]?.axisValue + '</b><br/>';
          let total = 0;
          params.forEach((p: any) => {
            if (p.value > 0) {
              s += `<span style="display:inline-block;width:10px;height:10px;border-radius:2px;background:${p.color};margin-right:6px"></span>${p.seriesName}: ¥${p.value.toFixed(4)}<br/>`;
              total += p.value;
            }
          });
          if (total > 0) s += `<b>合计: ¥${total.toFixed(4)}</b>`;
          else s += '<span style="color:var(--color-text-3)">无调用</span>';
          return s;
        },
      },
      legend: {
        bottom: 4, itemWidth: 10, itemHeight: 10,
        textStyle: { fontSize: 11, color: 'var(--color-text-2)' },
        itemGap: 16,
      },
      grid: { left: 52, right: 14, top: 12, bottom: 36 },
      xAxis: {
        type: 'category',
        data: hasData ? allDates.map(d => d.slice(8)) : allDates.map(d => d),
        axisLabel: {
          fontSize: 10, color: 'var(--color-text-3)',
          rotate: allDates.length > 15 ? 45 : 0,
        },
        axisLine: { lineStyle: { color: 'var(--color-border-2)' } },
        axisTick: { show: false },
      },
      yAxis: {
        type: 'value',
        axisLabel: {
          fontSize: 10, color: 'var(--color-text-3)',
          formatter: (v: number) => '¥' + (v >= 0.01 ? v.toFixed(2) : v.toFixed(4)),
        },
        splitLine: { lineStyle: { color: 'var(--color-border-1)', type: 'dashed' } },
      },
      series,
    }, true);

    const handleResize = () => chartInstance.current?.resize();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [daily, month]);

  const pageTitle = (
    <div style={{ display: 'flex', alignItems: 'center', gap: 10, margin: '16px 0 16px' }}>
      <div style={{
        width: 36, height: 36, borderRadius: 10,
        background: 'linear-gradient(135deg, #165DFF, #722ED1)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}>
        <BarChart3 size={18} color="#fff" />
      </div>
      <div>
        <h2 style={{ fontSize: 18, fontWeight: 700, margin: 0, color: 'var(--color-text-1)' }}>AI 调用分析</h2>
        <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 2 }}>查看你的 AI 调用花费统计与明细</div>
      </div>
    </div>
  );

  if (loading) {
    return (
      <div style={{ padding: '0 24px 24px', maxWidth: 1200, margin: '0 auto' }}>
        {pageTitle}
        <div style={{ display: 'flex', justifyContent: 'center', padding: 80 }}><Spin /></div>
      </div>
    );
  }

  const summaryCards = [
    { icon: DollarSign, label: '总花费', value: fmtCost(summary.totalCost || 0), color: '#F53F3F', bg: 'rgba(245,63,63,0.06)' },
    { icon: Coins, label: '总 Token', value: fmtTokens(summary.totalTokens || 0), color: '#722ED1', bg: 'rgba(114,46,209,0.06)' },
    { icon: Calendar, label: '今日花费', value: fmtCost(summary.todayCost || 0), color: '#FF7D00', bg: 'rgba(255,125,0,0.06)' },
    { icon: Clock, label: '今日 Token', value: fmtTokens(summary.todayTokens || 0), color: '#165DFF', bg: 'rgba(22,93,255,0.06)' },
    { icon: Cpu, label: '调用模型', value: String(summary.modelCalls || 0) + ' 个', color: '#00B42A', bg: 'rgba(0,180,42,0.06)' },
  ];

  return (
    <div style={{ padding: '0 24px 24px', maxWidth: 1200, margin: '0 auto' }}>
      {pageTitle}

      {/* ── Summary Cards ── */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 10, marginBottom: 16 }}>
        {summaryCards.map((card, i) => {
          const Icon = card.icon;
          return (
            <div key={i} style={{
              background: 'var(--color-bg-2)', borderRadius: 10, padding: '14px 16px',
              border: '1px solid var(--color-border-2)', transition: 'box-shadow 0.2s',
            }}
              onMouseEnter={e => { e.currentTarget.style.boxShadow = '0 2px 12px rgba(0,0,0,0.06)'; }}
              onMouseLeave={e => { e.currentTarget.style.boxShadow = ''; }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                <div style={{
                  width: 32, height: 32, borderRadius: 8,
                  background: card.bg,
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>
                  <Icon size={16} color={card.color} />
                </div>
                <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>{card.label}</span>
              </div>
              <div style={{ fontSize: 22, fontWeight: 700, color: card.color, fontFamily: "'SF Mono', 'Inter', monospace", letterSpacing: -0.5 }}>
                {card.value}
              </div>
            </div>
          );
        })}
      </div>

      {/* ── Daily Bar Chart ── */}
      <div style={{
        background: 'var(--color-bg-2)', borderRadius: 10, padding: '16px 18px',
        border: '1px solid var(--color-border-2)', marginBottom: 16,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <TrendingUp size={16} style={{ color: 'var(--color-primary)' }} />
            <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>每日花费趋势</span>
          </div>
          <input
            type="month"
            value={month}
            onChange={e => setMonth(e.target.value)}
            style={{
              padding: '5px 12px', borderRadius: 6, border: '1px solid var(--color-border-2)',
              fontSize: 12, background: 'var(--color-bg-1)', color: 'var(--color-text-1)',
              outline: 'none',
            }}
          />
        </div>
        <div ref={chartRef} style={{ width: '100%', height: 280 }} />
      </div>

      {/* ── Detail Table ── */}
      <div style={{
        background: 'var(--color-bg-2)', borderRadius: 10, padding: '16px 18px',
        border: '1px solid var(--color-border-2)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <ListFilter size={16} style={{ color: 'var(--color-primary)' }} />
            <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>调用明细</span>
            {total > 0 && (
              <span style={{ fontSize: 11, color: 'var(--color-text-3)', marginLeft: 4 }}>共 {total} 条</span>
            )}
          </div>
          <Select
            placeholder="全部模块"
            value={module || undefined}
            onChange={(v: any) => { setModule(v || ''); setPage(1); }}
            allowClear
            style={{ width: 130 }}
            size="small"
            options={Object.entries(MODULE_LABELS).map(([k, v]) => ({ label: v, value: k }))}
          />
        </div>

        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
          <thead>
            <tr style={{ background: 'var(--color-fill-1)', borderBottom: '2px solid var(--color-border-2)' }}>
              {['时间', '模块', '模型', 'Prompt', 'Completion', '费用', '耗时', '状态'].map(h => (
                <th key={h} style={{
                  padding: '9px 12px', textAlign: h === '时间' || h === '模块' || h === '模型' ? 'left' : h === '状态' ? 'center' : 'right',
                  fontWeight: 600, fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap',
                }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {logs.map((l: LogItem) => (
              <tr key={l.id} style={{
                borderBottom: '1px solid var(--color-border-1)',
                transition: 'background 0.15s',
              }}
                onMouseEnter={e => { e.currentTarget.style.background = 'var(--color-fill-1)'; }}
                onMouseLeave={e => { e.currentTarget.style.background = ''; }}
              >
                <td style={{ padding: '8px 12px', fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>
                  {new Date(l.createdAt).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}
                </td>
                <td style={{ padding: '8px 12px', fontSize: 12 }}>
                  <span style={{
                    padding: '2px 10px', borderRadius: 10, fontSize: 11, fontWeight: 500,
                    background: (MODULE_COLORS[l.module] || '#86909C') + '15',
                    color: MODULE_COLORS[l.module] || 'var(--color-text-2)',
                  }}>
                    {MODULE_LABELS[l.module] || l.module}
                  </span>
                </td>
                <td style={{ padding: '8px 12px', fontSize: 11, color: 'var(--color-text-2)', fontFamily: "'SF Mono', monospace", maxWidth: 140, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {l.modelName}
                </td>
                <td style={{ padding: '8px 12px', textAlign: 'right', fontSize: 12, fontFamily: "'SF Mono', monospace" }}>{fmtTokens(l.promptTokens)}</td>
                <td style={{ padding: '8px 12px', textAlign: 'right', fontSize: 12, fontFamily: "'SF Mono', monospace" }}>{fmtTokens(l.completionTokens)}</td>
                <td style={{ padding: '8px 12px', textAlign: 'right', fontSize: 12, fontWeight: 600, fontFamily: "'SF Mono', monospace", color: l.costAmount > 0 ? '#F53F3F' : 'var(--color-text-2)' }}>
                  {fmtCost(l.costAmount)}
                </td>
                <td style={{ padding: '8px 12px', textAlign: 'right', fontSize: 11, color: 'var(--color-text-3)', fontFamily: "'SF Mono', monospace" }}>{fmtMs(l.durationMs)}</td>
                <td style={{ padding: '8px 12px', textAlign: 'center' }}>
                  {l.success ? (
                    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                      <CheckCircle size={12} color="#00B42A" />
                      <span style={{ fontSize: 11, color: '#00B42A' }}>成功</span>
                    </span>
                  ) : (
                    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                      <XCircle size={12} color="#F53F3F" />
                      <span style={{ fontSize: 11, color: '#F53F3F' }}>{l.errorMsg ? '失败' : '失败'}</span>
                    </span>
                  )}
                </td>
              </tr>
            ))}
            {logs.length === 0 && (
              <tr><td colSpan={8} style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>暂无调用记录</td></tr>
            )}
          </tbody>
        </table>

        {total > 20 && (
          <div style={{ marginTop: 14, display: 'flex', justifyContent: 'flex-end' }}>
            <Pagination total={total} current={page} pageSize={20} onChange={(p: number) => setPage(p)} sizeCanChange={false} simple />
          </div>
        )}
      </div>
    </div>
  );
}
