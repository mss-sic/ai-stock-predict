import { useState, useMemo, useRef, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Table, Tabs, Card, Spin, Button, Tag } from '@arco-design/web-react';
import { TrendingUp, Shield, PieChart, FileText, Activity, BarChart3, List, Settings, ArrowLeft } from 'lucide-react';
import ReactECharts from 'echarts-for-react';
import { fetchKLine } from '../services/api';
import { useTheme } from '../services/ThemeContext';
import KLineChart from './KLineChart';

// ── Shared types ──

export interface TradeItem {
  code?: string; stockCode?: string; name?: string; stockName?: string;
  action?: string; date?: string; price?: number; quantity?: number;
  amount?: number; pnl?: number; pnlPct?: number; reason?: string;
}

export interface StockSummary {
  stockCode: string; stockName: string;
  totalPnl: number; totalPnlPct: number;
  buyCount: number; sellCount: number;
  trades: TradeItem[];
}

export interface BacktestResultData {
  id: number;
  strategyId: number;
  startDate: string;
  endDate: string;
  initialCapital: number;
  finalEquity: number;
  totalReturn: number;
  sharpeRatio: number;
  maxDrawdown: number;
  winRate: number;
  tradeCount: number;
  trades: TradeItem[];
  equityCurve?: { data?: { date: string; equity: number }[] } | any;
  stockCode?: string;
  stockPool?: string;
  coverage?: any;
}

export interface BacktestReportProps {
  result: BacktestResultData | null;
  loading?: boolean;
  // Header
  title: string;
  subtitle?: string;
  headerExtra?: React.ReactNode;
  backPath?: string;
  headerIcon?: React.ReactNode;
  // Optional sections
  showEquityCurve?: boolean;
  strategyParams?: Record<string, any> | null;
  strategyConditions?: any[] | null;
  logs?: any[] | null;
  // Extra metric cards
  extraMetrics?: { label: string; value: string; sub?: string; color?: string; icon?: React.ReactNode }[];
}

function computeStockSummary(trades: TradeItem[], capital: number): StockSummary[] {
  if (!trades || trades.length === 0) return [];
  const arr = Array.isArray(trades) ? trades : (trades as any).data || [];
  if (arr.length === 0) return [];

  const map = new Map<string, StockSummary>();
  for (const tr of arr) {
    const code = tr.code || tr.stockCode || '';
    if (!code) continue;
    if (!map.has(code)) {
      map.set(code, { stockCode: code, stockName: tr.name || tr.stockName || code, totalPnl: 0, totalPnlPct: 0, buyCount: 0, sellCount: 0, trades: [] });
    }
    const sa = map.get(code)!;
    if (tr.action === 'buy' || tr.action === 'add') sa.buyCount++;
    else if (tr.action === 'sell' || tr.action === 'reduce') { sa.sellCount++; sa.totalPnl += tr.pnl || 0; }
    sa.trades.push(tr);
  }
  return Array.from(map.values()).map(s => ({
    ...s, totalPnlPct: capital > 0 ? (s.totalPnl / capital) * 100 : 0,
  })).sort((a, b) => b.totalPnlPct - a.totalPnlPct);
}

export default function BacktestReport({
  result, loading, title, subtitle, headerExtra, backPath, headerIcon,
  showEquityCurve = true, strategyParams, strategyConditions, logs, extraMetrics,
}: BacktestReportProps) {
  const navigate = useNavigate();
  const [tab, setTab] = useState('overview');
  const [chartMode, setChartMode] = useState<'equity' | 'return'>('equity');
  const [stockKline, setStockKline] = useState<any[]>([]);
  const [klineLoading, setKlineLoading] = useState(false);
  const [stockMarkers, setStockMarkers] = useState<any[]>([]);
  const [stockDetailVisible, setStockDetailVisible] = useState(false);
  const [selectedStock, setSelectedStock] = useState<StockSummary | null>(null);
  const tradeTableRef = useRef<HTMLDivElement>(null);
  const { isDark } = useTheme();
  const [detailTab, setDetailTab] = useState<'chart' | 'logs'>('chart');
  const handleMarkerClick = () => {
    if (tradeTableRef.current) {
      tradeTableRef.current.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  };

  const tradesArr: TradeItem[] = useMemo(() => {
    const t = result?.trades;
    if (!t) return [];
    if (Array.isArray(t)) return t;
    if ((t as any).data && Array.isArray((t as any).data)) return (t as any).data;
    return [];
  }, [result]);

  const stockAnalysis = useMemo(() => 
    computeStockSummary(tradesArr, result?.initialCapital || 100000),
  [tradesArr, result]);

  const equityCurveData = useMemo(() => {
    const ec = (result as any)?.equityCurve;
    if (!ec) return [];
    const data = ec.data || ec || [];
    return Array.isArray(data) ? data : [];
  }, [result]);

  const equityChartOption = useMemo(() => {
    if (equityCurveData.length === 0) return null;
    const dates = equityCurveData.map((d: any) => d.date || '');
    const equities = equityCurveData.map((d: any) => d.equity || 0);
    const initial = equities[0] || 1;
    const returns = equities.map((v: number) => ((v - initial) / initial * 100));
    const isEquity = chartMode === 'equity';
    const seriesData = isEquity ? equities : returns;
    const lastVal = seriesData[seriesData.length - 1] || 0;
    return {
      tooltip: { trigger: 'axis' as const, formatter: (params: any) => {
        const p = Array.isArray(params) ? params[0] : params;
        const val = p?.value ?? 0;
        return `${p?.axisValue}<br/>${isEquity ? '权益 ¥' + val.toLocaleString() : '收益率 ' + val.toFixed(2) + '%'}`;
      }},
      grid: { left: 70, right: 20, top: 10, bottom: 30 },
      xAxis: { type: 'category' as const, data: dates, axisLabel: { fontSize: 10, rotate: 30, formatter: (v: string) => v?.slice(5) } },
      yAxis: { type: 'value' as const, scale: true, name: isEquity ? '权益(万)' : '收益率(%)', axisLabel: { fontSize: 10, formatter: (v: number) => isEquity ? (v / 10000).toFixed(2) : v.toFixed(2) + '%' } },
      series: [{ type: 'line', data: seriesData, smooth: true, symbol: 'none', lineStyle: { color: isEquity ? '#165DFF' : (lastVal >= 0 ? '#F53F3F' : '#00B42A'), width: 2 }, areaStyle: { color: isEquity ? 'rgba(22,93,255,0.06)' : (lastVal >= 0 ? 'rgba(245,63,63,0.06)' : 'rgba(0,180,42,0.06)') } }],
    };
  }, [equityCurveData, chartMode]);

  const handleViewStockKLine = async (code: string) => {
    const stock = stockAnalysis.find(s => s.stockCode === code);
    if (!stock) return;
    setStockKline([]); setStockMarkers([]); setSelectedStock(stock);
    setStockDetailVisible(true);
    setKlineLoading(true);
    try {
      const { data: r } = await fetchKLine(code);
      const kdata = r.data?.data || r.data || [];
      setStockKline(kdata);
      // Generate trade markers — merge same-day buy+sell into T markers
      const stockTrades = stock.trades || [];
      const klineDates = kdata.map((d: any) => d.tradeDate?.slice(0,10) || d.date?.slice(0,10) || '');

      // Group trades by date
      const dateGroups: Record<string, TradeItem[]> = {};
      stockTrades.forEach(t => {
        const d = (t.date || '').slice(0,10);
        if (!dateGroups[d]) dateGroups[d] = [];
        dateGroups[d].push(t);
      });

      const markers: any[] = [];
      Object.entries(dateGroups).forEach(([tDate, trades]) => {
        const idx = klineDates.indexOf(tDate);
        if (idx < 0) return;

        const buys = trades.filter(t => t.action === 'buy' || t.action === 'add');
        const sells = trades.filter(t => t.action === 'sell' || t.action === 'reduce');

        if (buys.length > 0 && sells.length > 0) {
          // Same-day T trade: merge into one T marker
          const totalBuyQty = buys.reduce((s, t) => s + (t.quantity || 0), 0);
          const totalSellQty = sells.reduce((s, t) => s + (t.quantity || 0), 0);
          const avgBuyPrice = buys.reduce((s, t) => s + (t.price || 0) * (t.quantity || 0), 0) / (totalBuyQty || 1);
          const avgSellPrice = sells.reduce((s, t) => s + (t.price || 0) * (t.quantity || 0), 0) / (totalSellQty || 1);
          markers.push({
            i: idx,
            type: 't' as const,
            label: 'T',
            buyPrice: avgBuyPrice,
            sellPrice: avgSellPrice,
            buyQty: totalBuyQty,
            sellQty: totalSellQty,
          });
        } else {
          // Single-direction trades
          trades.forEach(t => {
            const isBuy = t.action === 'buy' || t.action === 'add';
            markers.push({
              i: idx,
              type: isBuy ? 'buy' as const : 'sell' as const,
              label: isBuy ? '买入' : '卖出',
              price: t.price || 0,
              quantity: t.quantity || 0,
            });
          });
        }
      });
      setStockMarkers(markers);
    } catch (err) {
      console.error('[BacktestReport] fetchKLine failed:', err);
      setStockKline([]);
      setStockMarkers([]);
    }
    setKlineLoading(false);
  };

  if (loading) return <div style={{ padding: 60, textAlign: 'center' }}><Spin size={30} /></div>;
  if (!result) return <div style={{ padding: 60, textAlign: 'center', color: 'var(--color-text-3)' }}>数据加载失败</div>;

  const buyTrades = tradesArr.filter(t => t.action === 'buy' || t.action === 'add');
  const sellTrades = tradesArr.filter(t => t.action === 'sell' || t.action === 'reduce');

  // Build metrics
  const metrics = [
    { label: '总收益率', value: `${(result.totalReturn || 0) >= 0 ? '+' : ''}${(result.totalReturn || 0)?.toFixed(2)}%`, color: (result.totalReturn || 0) >= 0 ? '#F53F3F' : '#00B42A', icon: <TrendingUp size={14} /> },
    { label: '最终权益', value: `¥${((result.finalEquity || 0) / 10000).toFixed(1)}万`, color: 'var(--color-text-1)', icon: <PieChart size={14} /> },
    { label: '胜率', value: `${(result.winRate || 0)?.toFixed(1)}%`, sub: `${result.tradeCount || 0} 笔交易`, color: 'var(--color-text-1)', icon: <Activity size={14} /> },
    { label: '最大回撤', value: `-${result.maxDrawdown || 0}%`, color: '#00B42A', icon: <Shield size={14} /> },
    { label: '夏普比率', value: `${(result.sharpeRatio || 0)?.toFixed(2)}`, color: 'var(--color-text-1)', icon: <TrendingUp size={14} /> },
    ...(extraMetrics || []),
  ];

  return (
    <div style={{ padding: '24px 32px', maxWidth: 1200, margin: '0 auto' }}>
      {/* Header */}
      <Card style={{ marginBottom: 20, borderRadius: 12, border: '1px solid var(--color-border-1)', boxShadow: '0 1px 4px rgba(0,0,0,0.03)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 14, flexWrap: 'wrap' }}>
          {backPath && (
            <span onClick={() => navigate(backPath)} style={{ cursor: 'pointer' }}>
              <ArrowLeft size={18} style={{ color: 'var(--color-text-2)' }} />
            </span>
          )}
          <div style={{ width: 40, height: 40, borderRadius: 10, background: 'linear-gradient(135deg, #165DFF, #722ED1)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            {headerIcon || <BarChart3 size={20} color="#fff" />}
          </div>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 17, fontWeight: 700 }}>{title}</div>
            {subtitle && <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 2 }}>{subtitle}</div>}
          </div>
          {headerExtra}
        </div>
      </Card>

      {/* Metrics */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(170px, 1fr))', gap: 12, marginBottom: 20 }}>
        {metrics.map((m, i) => (
          <Card key={i} style={{ borderRadius: 10, border: '1px solid var(--color-border-1)', boxShadow: 'none', background: 'var(--color-bg-1)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
              <span style={{ color: 'var(--color-text-3)' }}>{m.icon}</span>
              <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{m.label}</span>
            </div>
            <div style={{ fontSize: 22, fontWeight: 800, color: m.color || 'var(--color-text-1)', fontFamily: 'monospace', lineHeight: 1.2 }}>{m.value}</div>
            {m.sub && <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 4 }}>{m.sub}</div>}
          </Card>
        ))}
      </div>

      {/* Equity Curve */}
      {showEquityCurve && equityChartOption && (
        <Card style={{ marginBottom: 20, borderRadius: 12, border: '1px solid var(--color-border-1)', boxShadow: '0 1px 4px rgba(0,0,0,0.03)' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
            <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>📈 {chartMode === 'equity' ? '权益' : '收益'}曲线</span>
            <div style={{ display: 'flex', gap: 0, background: 'var(--color-fill-2)', borderRadius: 6, padding: 2 }}>
              <button onClick={() => setChartMode('equity')} style={{ padding: '3px 12px', borderRadius: 5, border: 'none', cursor: 'pointer', fontSize: 11, fontWeight: 600, background: chartMode === 'equity' ? 'var(--color-bg-1)' : 'transparent', color: chartMode === 'equity' ? 'var(--color-text-1)' : 'var(--color-text-3)', boxShadow: chartMode === 'equity' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none' }}>权益</button>
              <button onClick={() => setChartMode('return')} style={{ padding: '3px 12px', borderRadius: 5, border: 'none', cursor: 'pointer', fontSize: 11, fontWeight: 600, background: chartMode === 'return' ? 'var(--color-bg-1)' : 'transparent', color: chartMode === 'return' ? 'var(--color-text-1)' : 'var(--color-text-3)', boxShadow: chartMode === 'return' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none' }}>收益率</button>
            </div>
          </div>
          <ReactECharts option={equityChartOption} style={{ height: 300 }} />
        </Card>
      )}

      {/* Tabs */}
      <Card style={{ borderRadius: 12, border: '1px solid var(--color-border-1)', boxShadow: '0 1px 4px rgba(0,0,0,0.03)' }}>
        <Tabs activeTab={tab} onChange={setTab} type="line" style={{ padding: '0 4px' }}>
          {/* Overview */}
          <Tabs.TabPane key="overview" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}><PieChart size={15} />概览 & 股票汇总 ({stockAnalysis.length})</span>}>
            <div style={{ padding: '8px 0' }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: 10, marginBottom: 20 }}>
                {[
                  { label: '初始资金', value: `¥${((result.initialCapital || 0) / 10000).toFixed(1)}万` },
                  { label: '买入次数', value: `${buyTrades.length} 次` },
                  { label: '卖出次数', value: `${sellTrades.length} 次` },
                  { label: '交易股票数', value: `${stockAnalysis.length} 只` },
                  { label: '股票池', value: result.stockCode || result.stockPool || '-' },
                ].map((s, i) => (
                  <div key={i} style={{ background: 'var(--color-fill-1)', borderRadius: 8, padding: '10px 14px', border: '1px solid var(--color-border-1)' }}>
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>{s.label}</div>
                    <div style={{ fontSize: 14, fontWeight: 700, fontFamily: 'monospace' }}>{s.value}</div>
                  </div>
                ))}
              </div>
              {/* Stock Summary */}
              <div style={{ marginTop: 24 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
                  <BarChart3 size={15} style={{ color: 'var(--color-text-2)' }} />
                  <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>按股票汇总 ({stockAnalysis.length})</span>
                </div>
                <Table columns={[
                  { title: '股票', width: 130, render: (_: any, r: StockSummary) => <div><div style={{ fontWeight: 600 }}>{r.stockName}</div><div style={{ fontSize: 11, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{r.stockCode}</div></div> },
                  { title: '总盈亏', dataIndex: 'totalPnl', width: 110, sorter: (a: any, b: any) => a.totalPnl - b.totalPnl, render: (v: number) => <span style={{ color: v >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 700, fontFamily: 'monospace' }}>{v >= 0 ? '+' : ''}{v.toFixed(2)}</span> },
                  { title: '收益率', dataIndex: 'totalPnlPct', width: 90, sorter: (a: any, b: any) => a.totalPnlPct - b.totalPnlPct, render: (v: number) => <span style={{ color: v >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 600, fontFamily: 'monospace' }}>{v >= 0 ? '+' : ''}{v.toFixed(2)}%</span> },
                  { title: '买入', dataIndex: 'buyCount', width: 60, render: (v: number) => <span style={{ color: '#F53F3F', fontWeight: 500 }}>{v}次</span> },
                  { title: '卖出', dataIndex: 'sellCount', width: 60, render: (v: number) => <span style={{ color: '#00B42A', fontWeight: 500 }}>{v}次</span> },
                  { title: '', width: 60, render: (_: any, r: StockSummary) => <Button size="mini" type="text" onClick={() => handleViewStockKLine(r.stockCode)}>详情</Button> },
                ]} data={stockAnalysis} rowKey="stockCode" pagination={false} size="small" stripe />
              </div>
            </div>
          </Tabs.TabPane>

          {/* Trades */}
          <Tabs.TabPane key="trades" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}><FileText size={15} />交易记录 ({tradesArr.length})</span>}>
            <Table columns={[
              { title: '日期', dataIndex: 'date', width: 100, sorter: (a: any, b: any) => (a.date || '').localeCompare(b.date || '') },
              { title: '股票', width: 110, render: (_: any, r: TradeItem) => <span style={{ fontWeight: 600 }}>{r.code || r.stockCode} {r.name || r.stockName}</span> },
              { title: '操作', dataIndex: 'action', width: 60, render: (v: string) => <Tag color={v === 'buy' || v === 'add' ? 'red' : 'green'} style={{ borderRadius: 6, fontSize: 11 }}>{v === 'buy' ? '买入' : v === 'add' ? '加仓' : v === 'sell' ? '卖出' : '减仓'}</Tag> },
              { title: '价格', dataIndex: 'price', width: 80, render: (v: number) => <span style={{ fontFamily: 'monospace' }}>{v != null ? v.toFixed(2) : '-'}</span> },
              { title: '数量', dataIndex: 'quantity', width: 70, render: (v: number) => v || '-' },
              { title: '金额', dataIndex: 'amount', width: 90, render: (v: number) => v != null ? v.toFixed(0) : '-' },
              { title: '盈亏', dataIndex: 'pnl', width: 80, render: (v: number) => v != null ? <span style={{ color: v >= 0 ? '#F53F3F' : '#00B42A', fontFamily: 'monospace', fontWeight: 600 }}>{v >= 0 ? '+' : ''}{v.toFixed(2)}</span> : '-' },
              { title: '盈亏%', dataIndex: 'pnlPct', width: 80, render: (v: number) => v != null ? <span style={{ color: v >= 0 ? '#F53F3F' : '#00B42A', fontFamily: 'monospace' }}>{v >= 0 ? '+' : ''}{v.toFixed(2)}%</span> : '-' },
            ]} data={tradesArr} rowKey={(r: TradeItem, i: number) => `${r.date}-${r.code || r.stockCode}-${i}`} pagination={{ pageSize: 20, showTotal: true }} size="small" stripe />
          </Tabs.TabPane>



          {/* Strategy Conditions (optional) */}
          {strategyParams && (
            <Tabs.TabPane key="conditions" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}><Settings size={15} />策略参数</span>}>
              <div style={{ padding: '8px 0' }}>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: 8, marginBottom: 20 }}>
                  {[
                    { label: '止盈', value: (strategyParams.stopProfit || 0) > 0 ? `${strategyParams.stopProfit}%` : '未设置', color: '#F53F3F' },
                    { label: '止损', value: (strategyParams.stopLoss || 0) < 0 ? `${strategyParams.stopLoss}%` : '未设置', color: '#00B42A' },
                    { label: '最大持股', value: `${strategyParams.maxHoldings || '-'} 只` },
                    { label: '初始资金', value: `¥${((strategyParams.initialCapital || 0) / 10000).toFixed(1)}万` },
                  ].map((p, i) => (
                    <div key={i} style={{ background: 'var(--color-fill-1)', borderRadius: 8, padding: '10px 14px', border: '1px solid var(--color-border-1)' }}>
                      <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>{p.label}</div>
                      <div style={{ fontSize: 14, fontWeight: 700, color: p.color || 'var(--color-text-1)', fontFamily: 'monospace' }}>{p.value}</div>
                    </div>
                  ))}
                </div>
                {strategyConditions && strategyConditions.length > 0 && (
                  <Table columns={[
                    { title: '类型', dataIndex: 'condType', width: 60, render: (v: string) => ({ buy: '买入', add: '加仓', sell: '卖出', reduce: '减仓' } as any)[v] || v },
                    { title: '指标', dataIndex: 'indicator', width: 120 },
                    { title: '条件', width: 150, render: (_: any, r: any) => `${({ gte: '≥', lte: '≤', gt: '>', lt: '<' } as any)[r.operator] || r.operator} ${r.value}` },
                    { title: '组', dataIndex: 'logicGroup', width: 50 },
                  ]} data={strategyConditions} rowKey="id" pagination={false} size="small" />
                )}
              </div>
            </Tabs.TabPane>
          )}

          {/* Execution Logs (optional) — console-style */}
          {logs && logs.length > 0 && (() => {
            const typeColors: Record<string, string> = {
              trade: '#F53F3F', signal: '#165DFF', condition_eval: '#722ED1',
              system: '#86909C', error: '#F53F3F',
            };
            const typeLabels: Record<string, string> = {
              trade: 'TRADE', signal: 'SIGNAL', condition_eval: 'COND',
              system: 'SYSTEM', error: 'ERROR',
            };
            const levelColors: Record<string, string> = {
              error: '#F53F3F', warn: '#FF7D00',
            };
            const ConsoleLine = ({ log: l }: { log: any }) => {
              const [open, setOpen] = useState(false);
              const tc = typeColors[l.logType] || '#86909C';
              const tl = typeLabels[l.logType] || (l.logType || '').toUpperCase();
              const isError = l.level === 'error';
              const isWarn = l.level === 'warn';
              const timeStr = l.createdAt?.slice(0, 19) || l.date || '';
              const hasStock = !!(l.stockCode);
              const detailStr = l.detail
                ? (typeof l.detail === 'string' ? l.detail : JSON.stringify(l.detail, null, 2))
                : null;
              return (
                <div style={{
                  padding: '2px 0',
                  background: isError ? 'rgba(245,63,63,0.06)' : isWarn ? 'rgba(255,125,0,0.04)' : 'transparent',
                  borderBottom: '1px solid ' + (isDark ? 'rgba(255,255,255,0.04)' : 'rgba(0,0,0,0.04)'),
                }}>
                  <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8, cursor: detailStr ? 'pointer' : 'default' }}
                    onClick={() => detailStr && setOpen(!open)}>
                    <span style={{ color: isDark ? '#555' : '#999', flexShrink: 0, minWidth: 140 }}>{timeStr}</span>
                    {isError || isWarn ? (
                      <span style={{ display: 'inline-block', padding: '0 5px', borderRadius: 3, fontSize: 10, fontWeight: 700,
                        background: (levelColors[l.level] || '#999') + '22', color: levelColors[l.level] || '#999',
                        flexShrink: 0, minWidth: 42, textAlign: 'center', lineHeight: '18px' }}>
                        {l.level?.toUpperCase()}
                      </span>
                    ) : null}
                    <span style={{ display: 'inline-block', padding: '0 5px', borderRadius: 3, fontSize: 10, fontWeight: 600,
                      background: tc + '18', color: tc, flexShrink: 0, minWidth: 50, textAlign: 'center', lineHeight: '18px' }}>
                      {tl}
                    </span>
                    {hasStock && (
                      <span style={{ color: isDark ? '#f0c060' : '#b8860b', flexShrink: 0, fontWeight: 500 }}>
                        {l.stockCode}{l.stockName ? ' ' + l.stockName : ''}
                      </span>
                    )}
                    <span style={{ color: isDark ? '#bbb' : 'var(--color-text-1)', wordBreak: 'break-all', flex: 1 }}>
                      {l.message}
                    </span>
                    {detailStr && (
                      <span style={{ color: isDark ? '#555' : '#999', flexShrink: 0, fontSize: 10 }}>
                        {open ? '▲' : '▶'}
                      </span>
                    )}
                  </div>
                  {open && detailStr && (
                    <pre style={{
                      margin: '4px 0 4px 148px', padding: '6px 10px',
                      background: isDark ? 'rgba(255,255,255,0.03)' : 'rgba(0,0,0,0.02)',
                      borderRadius: 6, fontSize: 11, lineHeight: '18px',
                      color: isDark ? '#888' : 'var(--color-text-3)',
                      whiteSpace: 'pre-wrap', wordBreak: 'break-all',
                      borderLeft: '2px solid ' + tc,
                    }}>
                      {detailStr}
                    </pre>
                  )}
                </div>
              );
            };
            return (
              <Tabs.TabPane key="logs" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}><Activity size={15} />执行日志 ({logs.length})</span>}>
                <div style={{
                  background: isDark ? '#0d0d0f' : '#fafafa',
                  borderRadius: 10,
                  border: '1px solid var(--color-border-1)',
                  maxHeight: 520,
                  overflow: 'auto',
                  fontFamily: "'SF Mono', 'Cascadia Code', 'Consolas', monospace",
                  fontSize: 12,
                  lineHeight: '22px',
                  padding: '8px 12px',
                }}>
                  {logs.map((log: any, i: number) => (
                    <ConsoleLine key={i} log={log} />
                  ))}
                </div>
              </Tabs.TabPane>
            );
          })()}
        </Tabs>
      </Card>

      {/* ── Stock Detail Full-Screen Overlay ── */}
      {stockDetailVisible && selectedStock && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, zIndex: 1100, background: 'var(--color-fill-2)', overflow: 'auto' }}>
          <div style={{ background: 'var(--color-bg-1)', borderBottom: '1px solid var(--color-border-1)', padding: '12px 24px', display: 'flex', alignItems: 'center', gap: 16, position: 'sticky', top: 0, zIndex: 10, boxShadow: '0 1px 4px rgba(0,0,0,0.04)' }}>
            <Button type="text" onClick={() => setStockDetailVisible(false)}>← 返回</Button>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 16, fontWeight: 700 }}>{selectedStock.stockName} <span style={{ fontSize: 12, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{selectedStock.stockCode}</span></div>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 2 }}>
                累计盈亏: <span style={{ color: selectedStock.totalPnl >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 600 }}>{selectedStock.totalPnl >= 0 ? '+' : ''}{selectedStock.totalPnl?.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
                {' · '}收益率 <span style={{ color: selectedStock.totalPnlPct >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 600 }}>{selectedStock.totalPnlPct >= 0 ? '+' : ''}{selectedStock.totalPnlPct?.toFixed(2)}%</span>
                {' · '}买入 {selectedStock.buyCount} 次 · 卖出 {selectedStock.sellCount} 次
              </div>
            </div>
          </div>
          <div style={{ maxWidth: 1200, margin: '0 auto', padding: '20px 24px' }}>
            <Tabs activeTab={detailTab} onChange={(v: string) => setDetailTab(v as 'chart' | 'logs')} type="line" style={{ marginBottom: 0 }}>
              <Tabs.TabPane key="chart" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}><BarChart3 size={15} />K线 & 交易</span>}>
                {klineLoading ? (
                  <div style={{ padding: 60, textAlign: 'center' }}><Spin size={30} /></div>
                ) : (
                  <>
                    <Card style={{ marginBottom: 20, borderRadius: 12, border: '1px solid var(--color-border-1)' }}>
                      <div style={{ fontSize: 14, fontWeight: 700, marginBottom: 14 }}>📈 K线图 · 交易标记</div>
                      {stockKline.length > 0 ? (
                        <KLineChart data={stockKline} stockCode={selectedStock.stockCode} markers={stockMarkers} onMarkerClick={handleMarkerClick} height={420} />
                      ) : (
                        <div style={{ padding: 60, textAlign: 'center', color: 'var(--color-text-3)' }}>加载K线数据中...</div>
                      )}
                    </Card>
                    <Card ref={tradeTableRef} style={{ borderRadius: 12, border: '1px solid var(--color-border-1)' }}>
                      <div style={{ fontSize: 14, fontWeight: 700, marginBottom: 14 }}><List size={14} style={{ marginRight: 6 }} />交易记录</div>
                      <Table columns={[
                        { title: '日期', dataIndex: 'date', width: 100, render: (v: string) => <span style={{ fontSize: 11, fontFamily: 'monospace', color: 'var(--color-text-3)' }}>{v}</span> },
                        { title: '操作', dataIndex: 'action', width: 68, render: (v: string, record: any) => {
                          const labels: Record<string, string> = { buy: '买入', add: '加仓', sell: '卖出', reduce: '减仓', stop: '止损' };
                          const colors: Record<string, string> = { buy: '#F53F3F', add: '#FF7D00', sell: '#00B42A', reduce: '#165DFF', stop: '#7B61FF' };
                          const bgs: Record<string, string> = { buy: 'rgba(245,63,63,0.1)', add: 'rgba(255,125,0,0.1)', sell: 'rgba(0,180,42,0.1)', reduce: 'rgba(22,93,255,0.1)', stop: 'rgba(123,97,255,0.1)' };
                          if (v === 'stop') {
                            const isProfit = record.reason === '止盈' || record.pnlPct > 0;
                            return <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, background: isProfit ? 'rgba(245,63,63,0.08)' : 'rgba(0,180,42,0.08)', color: isProfit ? '#F53F3F' : '#00B42A', fontWeight: 700, fontSize: 11 }}>{isProfit ? '止盈' : '止损'}</span>;
                          }
                          return <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, background: bgs[v] || 'var(--color-fill-2)', color: colors[v] || 'var(--color-text-3)', fontWeight: 700, fontSize: 11 }}>{labels[v] || v}</span>;
                        }},
                        { title: '价格', dataIndex: 'price', width: 76, render: (v: number) => <span style={{ fontFamily: 'monospace', fontSize: 12 }}>¥{v?.toFixed(2)}</span> },
                        { title: '数量', dataIndex: 'quantity', width: 64, render: (v: number) => <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{v}股</span> },
                        { title: '盈亏%', dataIndex: 'pnlPct', width: 72, render: (v: number) => v != null ? <span style={{ color: v > 0 ? '#F53F3F' : '#00B42A', fontWeight: 600, fontSize: 12, fontFamily: 'monospace' }}>{v > 0 ? '+' : ''}{v?.toFixed(1)}%</span> : <span style={{ color: 'var(--color-text-3)', fontSize: 11 }}>—</span> },
                        { title: '原因', dataIndex: 'reason', width: 120, render: (v: string) => <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{v || '—'}</span> },
                      ]}
                      data={selectedStock.trades || []}
                      rowKey={(_, i: number) => i}
                      pagination={{ pageSize: 20, sizeCanChange: true }}
                      size="small" stripe />
                    </Card>
                  </>
                )}
              </Tabs.TabPane>
              <Tabs.TabPane key="logs" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}><Activity size={15} />操作日志 {(() => { const stockLogs = (logs || []).filter((l: any) => l.stockCode === selectedStock.stockCode); return stockLogs.length > 0 ? '(' + stockLogs.length + ')' : ''; })()}</span>}>
                {(() => {
                  const stockLogs = (logs || []).filter((l: any) => l.stockCode === selectedStock.stockCode);
                  if (stockLogs.length === 0) {
                    return <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)' }}>暂无操作日志</div>;
                  }
                  return (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                      {stockLogs.map((log: any, i: number) => {
                        const typeColors: Record<string, string> = { trade: '#F53F3F', signal: '#165DFF', condition_eval: '#722ED1', system: '#86909C', error: '#F53F3F' };
                        const typeLabels: Record<string, string> = { trade: '交易', signal: '信号', condition_eval: '条件', system: '系统', error: '错误' };
                        const levelBg: Record<string, string> = { error: 'rgba(245,63,63,0.06)', warn: 'rgba(255,125,0,0.06)', info: 'transparent', debug: 'transparent' };
                        const tc = typeColors[log.logType] || '#86909C';
                        const lb = typeLabels[log.logType] || log.logType;
                        return (
                          <div key={i} style={{
                            padding: '8px 12px', borderRadius: 8, fontSize: 12,
                            background: levelBg[log.level] || 'transparent',
                            border: '1px solid var(--color-border-1)',
                            borderLeft: '3px solid ' + tc,
                          }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 2 }}>
                              <span style={{ fontSize: 11, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{log.createdAt?.slice(0, 19) || log.date || ''}</span>
                              <span style={{ display: 'inline-block', padding: '1px 6px', borderRadius: 4, fontSize: 10, fontWeight: 600, background: tc + '18', color: tc }}>{lb}</span>
                              {log.level === 'error' && <span style={{ display: 'inline-block', padding: '1px 6px', borderRadius: 4, fontSize: 10, fontWeight: 600, background: 'rgba(245,63,63,0.12)', color: '#F53F3F' }}>ERROR</span>}
                              {log.level === 'warn' && <span style={{ display: 'inline-block', padding: '1px 6px', borderRadius: 4, fontSize: 10, fontWeight: 600, background: 'rgba(255,125,0,0.12)', color: '#FF7D00' }}>WARN</span>}
                            </div>
                            <div style={{ color: 'var(--color-text-1)', lineHeight: 1.6 }}>{log.message}</div>
                            {log.detail && <div style={{ marginTop: 4, fontSize: 11, color: 'var(--color-text-3)', fontFamily: 'monospace', whiteSpace: 'pre-wrap' }}>{typeof log.detail === 'string' ? log.detail : JSON.stringify(log.detail, null, 2)}</div>}
                          </div>
                        );
                      })}
                    </div>
                  );
                })()}
              </Tabs.TabPane>
            </Tabs>
          </div>
        </div>
      )}
    </div>

  );
}
