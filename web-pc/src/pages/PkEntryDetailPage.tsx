import { useState, useEffect, useMemo, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Table, Tabs, Tag, Spin, Card, Button } from '@arco-design/web-react';
import { ArrowLeft, TrendingUp, Trophy, Shield, PieChart, FileText, Activity, List, Settings } from 'lucide-react';
import api, { fetchKLine } from '../services/api';
import KLineChart from '../components/KLineChart';

interface EntryData {
  entry: any;
  strategy?: any;
  result: any;
  logs: any[];
}

interface StockAnalysis {
  stockCode: string;
  stockName: string;
  totalPnl: number;
  totalPnlPct: number;
  buyCount: number;
  sellCount: number;
  trades: any[];
}

function computeStockAnalysis(tradesData: any, initialCapital: number): StockAnalysis[] {
  const t = tradesData;
  const trades = (() => {
    if (!t) return [];
    if (Array.isArray(t)) return t;
    if (t.data && Array.isArray(t.data)) return t.data;
    return [];
  })();
  if (trades.length === 0) return [];

  const map = new Map<string, StockAnalysis>();
  for (const tr of trades) {
    const code = tr.code || tr.stockCode || '';
    if (!code) continue;
    if (!map.has(code)) {
      map.set(code, {
        stockCode: code,
        stockName: tr.name || tr.stockName || code,
        totalPnl: 0, totalPnlPct: 0, buyCount: 0, sellCount: 0, trades: [],
      });
    }
    const sa = map.get(code)!;
    if (tr.action === 'buy' || tr.action === 'add') sa.buyCount++;
    else if (tr.action === 'sell' || tr.action === 'reduce') { sa.sellCount++; if (tr.pnl != null) sa.totalPnl += tr.pnl; }
    sa.trades.push(tr);
  }
  const result = Array.from(map.values());
  for (const sa of result) { if (sa.totalPnl !== 0) sa.totalPnlPct = sa.totalPnl / initialCapital * 100; }
  result.sort((a, b) => Math.abs(b.totalPnl) - Math.abs(a.totalPnl));
  return result;
}

export default function PkEntryDetailPage() {
  const { id, entryId } = useParams<{ id: string; entryId: string }>();
  const navigate = useNavigate();
  const [data, setData] = useState<EntryData | null>(null);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState('overview');

  // Stock detail state
  const [stockDetailVisible, setStockDetailVisible] = useState(false);
  const [selectedStock, setSelectedStock] = useState<StockAnalysis | null>(null);
  const [stockKline, setStockKline] = useState<any[]>([]);
  const [stockMarkers, setStockMarkers] = useState<any[]>([]);
  const [stockLoading, setStockLoading] = useState(false);
  const tradeTableRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    (async () => {
      try {
        const res = await api.get(`/pk/entries/${entryId}/detail`);
        setData(res.data.data);
      } catch {}
      setLoading(false);
    })();
  }, [entryId]);

  const stockAnalysis = useMemo(() => {
    if (!data?.result) return [];
    const trades = data.result.trades;
    const capital = data.entry?.initialCapital || data.result?.initialCapital || 100000;
    return computeStockAnalysis(trades, capital);
  }, [data]);

  const handleMarkerClick = (klineIdx: number) => {
    if (tradeTableRef.current) {
      tradeTableRef.current.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  };
  const handleViewStockDetail = async (stock: StockAnalysis) => {
    setSelectedStock(stock);
    setStockKline([]);
    setStockMarkers([]);
    setStockDetailVisible(true);
    setStockLoading(true);
    try {
      const { data: r }: any = await fetchKLine(stock.stockCode);
      const kl = r.data || r || [];
      const cleaned = Array.isArray(kl) ? kl : [];
      setStockKline(cleaned);
      const markers: any[] = [];
      stock.trades.forEach((t: any) => {
        const date = (t.date || t.execDate || '').slice(0, 10);
        const idx = cleaned.findIndex((k: any) => {
          const d = (k.tradeDate || k.date || '').slice(0, 10);
          return d === date;
        });
        if (idx >= 0 && (t.action === 'buy' || t.action === 'add')) {
          markers.push({ i: idx, type: 'buy' as const, label: `¥${(t.price || t.execPrice || 0).toFixed(1)}` });
        } else if (idx >= 0 && (t.action === 'sell' || t.action === 'reduce' || t.action === 'stop')) {
          const pnl = t.pnlPct != null ? `${t.pnlPct > 0 ? '+' : ''}${t.pnlPct?.toFixed(1)}%` : '';
          markers.push({ i: idx, type: 'sell' as const, label: `¥${(t.price || t.execPrice || 0).toFixed(1)} ${pnl}` });
        }
      });
      setStockMarkers(markers);
    } catch {} finally { setStockLoading(false); }
  };

  if (loading) return <div style={{ padding: 60, textAlign: 'center' }}><Spin size={30} /></div>;
  if (!data) return <div style={{ padding: 60, textAlign: 'center', color: 'var(--color-text-3)' }}>数据加载失败</div>;

  const { entry, strategy, result, logs } = data;
  const tradesArr = (() => {
    const t = result?.trades;
    if (!t) return [];
    if (Array.isArray(t)) return t;
    if (t.data && Array.isArray(t.data)) return t.data;
    return [];
  })();

  return (
    <div style={{ padding: '24px 32px', maxWidth: 1200, margin: '0 auto' }}>
      {/* ── Header Card ── */}
      <Card style={{ marginBottom: 20, borderRadius: 12, border: '1px solid var(--color-border-1)', boxShadow: '0 1px 4px rgba(0,0,0,0.03)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 14, flexWrap: 'wrap' }}>
          <span onClick={() => navigate(`/pk/${id}`)} style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', flexShrink: 0 }}>
            <ArrowLeft size={18} style={{ color: 'var(--color-text-2)' }} />
          </span>
          <div style={{ width: 40, height: 40, borderRadius: 10, background: 'linear-gradient(135deg, var(--color-primary), #722ed1)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
            <Trophy size={20} color="#fff" />
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 17, fontWeight: 700, color: 'var(--color-text-1)' }}>{entry.username || `选手 #${entry.userId}`}</div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 2 }}>
              {entry.strategyName || '未知策略'}
              {entry.finalRank > 0 && <span style={{ marginLeft: 8, color: entry.finalRank <= 3 ? '#f5a623' : 'var(--color-text-3)', fontWeight: 600 }}>🏅 第 {entry.finalRank} 名</span>}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 6, flexShrink: 0 }}>
            {entry.status === 'running' && <Tag color="blue" style={{ borderRadius: 6 }}>进行中</Tag>}
            {entry.status === 'completed' && <Tag color="green" style={{ borderRadius: 6 }}>已完成</Tag>}
            {entry.status === 'pending' && <Tag color="orange" style={{ borderRadius: 6 }}>等待中</Tag>}
          </div>
        </div>
      </Card>

      {/* ── Metrics Grid ── */}
      {result && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(170px, 1fr))', gap: 12, marginBottom: 20 }}>
          {[
            { label: '总收益率', value: `${result.totalReturn >= 0 ? '+' : ''}${result.totalReturn?.toFixed(2)}%`, color: result.totalReturn >= 0 ? '#F53F3F' : '#00B42A', icon: <TrendingUp size={14} /> },
            { label: '最终权益', value: `¥${(result.finalEquity || 0).toLocaleString(undefined, { maximumFractionDigits: 0 })}`, color: 'var(--color-text-1)', icon: <PieChart size={14} /> },
            { label: '胜率', value: `${result.winRate?.toFixed(1)}%`, sub: `${result.tradeCount || 0} 笔交易`, color: 'var(--color-text-1)', icon: <Trophy size={14} /> },
            { label: '最大回撤', value: `-${result.maxDrawdown || 0}%`, color: '#00B42A', icon: <Shield size={14} /> },
            { label: '夏普比率', value: `${result.sharpeRatio?.toFixed(2) || '-'}`, color: 'var(--color-text-1)', icon: <Activity size={14} /> },
          ].map((m, i) => (
            <Card key={i} style={{ borderRadius: 10, border: '1px solid var(--color-border-1)', boxShadow: 'none', background: 'var(--color-bg-1)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
                <span style={{ color: 'var(--color-text-3)' }}>{m.icon}</span>
                <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{m.label}</span>
              </div>
              <div style={{ fontSize: 22, fontWeight: 800, color: m.color, fontFamily: 'monospace', lineHeight: 1.2 }}>
                {m.value}
              </div>
              {m.sub && <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 4 }}>{m.sub}</div>}
            </Card>
          ))}
        </div>
      )}

      {/* ── Tabs ── */}
      <Card style={{ borderRadius: 12, border: '1px solid var(--color-border-1)', boxShadow: '0 1px 4px rgba(0,0,0,0.03)', overflow: 'visible' }}>
        <Tabs activeTab={tab} onChange={setTab} type="line" style={{ padding: '0 4px' }}>
          <Tabs.TabPane key="overview" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}><PieChart size={15} />概览 & 收益分析</span>}>
            <div style={{ padding: '8px 0 0' }}>
              {result ? (
                <>
                  {/* Overview stats */}
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(170px, 1fr))', gap: 10, marginBottom: 24 }}>
                    {[
                      { label: '初始资金', value: `¥${(result.initialCapital || 100000).toLocaleString()}` },
                      { label: '最终权益', value: `¥${(result.finalEquity || 0).toLocaleString()}`, color: (result.finalEquity || 0) >= (result.initialCapital || 100000) ? '#F53F3F' : '#00B42A' },
                      { label: '交易天数', value: result.tradingDays || result.totalDays || '-' },
                      { label: '日均收益率', value: result.dailyReturn ? `${(result.dailyReturn * 100).toFixed(2)}%` : '-', color: (result.dailyReturn || 0) >= 0 ? '#F53F3F' : '#00B42A' },
                    ].map((s, i) => (
                      <div key={i} style={{ background: 'var(--color-fill-1)', borderRadius: 8, padding: '12px 16px', border: '1px solid var(--color-border-1)' }}>
                        <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>{s.label}</div>
                        <div style={{ fontSize: 18, fontWeight: 700, color: s.color || 'var(--color-text-1)', fontFamily: 'monospace' }}>{s.value}</div>
                      </div>
                    ))}
                  </div>

                  {/* Profit Analysis */}
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 14 }}>
                    <TrendingUp size={15} style={{ color: 'var(--color-text-2)' }} />
                    <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>收益分析</span>
                    {stockAnalysis.length > 0 && <span style={{ background: 'var(--color-primary-bg)', color: 'var(--color-primary)', fontSize: 11, fontWeight: 600, padding: '1px 8px', borderRadius: 10 }}>{stockAnalysis.length} 只股票</span>}
                  </div>
                  {stockAnalysis.length > 0 ? (
                    <Table
                      columns={[
                        { title: '#', width: 36, render: (_: any, __: any, i: number) => <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{i + 1}</span> },
                        { title: '股票', dataIndex: 'stockName', width: 120, render: (v: string, r: any) => (
                          <div><div style={{ fontWeight: 600, fontSize: 13 }}>{v || r.stockCode}</div><div style={{ fontSize: 10, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{r.stockCode}</div></div>
                        )},
                        { title: '总盈亏', dataIndex: 'totalPnl', width: 110, sorter: (a: any, b: any) => a.totalPnl - b.totalPnl, render: (v: number) => (
                          <span style={{ color: v >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 700, fontSize: 13, fontFamily: 'monospace' }}>{v >= 0 ? '+' : ''}{v?.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
                        )},
                        { title: '收益率', dataIndex: 'totalPnlPct', width: 90, sorter: (a: any, b: any) => a.totalPnlPct - b.totalPnlPct, render: (v: number) => (
                          <span style={{ color: v >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 600, fontSize: 12, fontFamily: 'monospace' }}>{v >= 0 ? '+' : ''}{v?.toFixed(2)}%</span>
                        )},
                        { title: '买入', dataIndex: 'buyCount', width: 60, render: (v: number) => <span style={{ fontSize: 12, color: '#F53F3F', fontWeight: 500 }}>{v}次</span> },
                        { title: '卖出', dataIndex: 'sellCount', width: 60, render: (v: number) => <span style={{ fontSize: 12, color: '#00B42A', fontWeight: 500 }}>{v}次</span> },
                        { title: '', width: 60, render: (_: any, r: StockAnalysis) => (
                          <Button size="mini" type="text" onClick={() => handleViewStockDetail(r)}>详情 →</Button>
                        )},
                      ]}
                      data={stockAnalysis}
                      rowKey="stockCode"
                      pagination={{ pageSize: 15, sizeCanChange: true, showTotal: true }}
                      size="small" stripe
                      style={{ borderRadius: 8 }}
                    />
                  ) : (
                    <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13, background: 'var(--color-fill-1)', borderRadius: 8, border: '1px dashed var(--color-border-1)' }}>暂无收益分析数据</div>
                  )}
                </>
              ) : (
                <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)', background: 'var(--color-fill-1)', borderRadius: 8, border: '1px dashed var(--color-border-1)' }}>回测未完成</div>
              )}
            </div>
          </Tabs.TabPane>

          <Tabs.TabPane key="conditions" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}><Settings size={15} />策略条件</span>}>
            <div style={{ padding: '8px 0 0' }}>
              {strategy ? (
                <>
                  {/* Strategy Parameters */}
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 14 }}>
                    <Settings size={15} style={{ color: 'var(--color-text-2)' }} />
                    <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>策略参数</span>
                  </div>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: 8, marginBottom: 28 }}>
                    {[
                      { label: '止盈', value: strategy.stopProfit > 0 ? `${strategy.stopProfit}%` : '未设置', color: '#F53F3F' },
                      { label: '止损', value: strategy.stopLoss < 0 ? `${strategy.stopLoss}%` : '未设置', color: '#00B42A' },
                      { label: '最大持股', value: `${strategy.maxHoldings} 只` },
                      { label: '建仓比例', value: `${strategy.buyPct || strategy.buyPositionPct || '-'}%` },
                      { label: '加仓比例', value: `${strategy.addPct || strategy.addPositionPct || '-'}%` },
                      { label: '初始资金', value: `¥${(strategy.initialCapital || 0).toLocaleString()}` },
                    ].map((p, i) => (
                      <div key={i} style={{ background: 'var(--color-fill-1)', borderRadius: 8, padding: '10px 14px', border: '1px solid var(--color-border-1)' }}>
                        <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>{p.label}</div>
                        <div style={{ fontSize: 14, fontWeight: 700, color: p.color || 'var(--color-text-1)', fontFamily: 'monospace' }}>{p.value}</div>
                      </div>
                    ))}
                  </div>

                  {/* Condition Blocks */}
                  {strategy.conditions && strategy.conditions.length > 0 ? (
                    <>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 14 }}>
                        <List size={15} style={{ color: 'var(--color-text-2)' }} />
                        <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>交易条件</span>
                      </div>
                      {(() => {
                        const typeConfig: Record<string, { label: string; color: string; bg: string; border: string }> = {
                          buy: { label: '买入条件', color: '#F53F3F', bg: 'rgba(245,63,63,0.03)', border: 'rgba(245,63,63,0.15)' },
                          add: { label: '加仓条件', color: '#FF7D00', bg: 'rgba(255,125,0,0.03)', border: 'rgba(255,125,0,0.15)' },
                          sell: { label: '卖出条件', color: '#00B42A', bg: 'rgba(0,180,42,0.03)', border: 'rgba(0,180,42,0.15)' },
                          reduce: { label: '减仓条件', color: '#165DFF', bg: 'rgba(22,93,255,0.03)', border: 'rgba(22,93,255,0.15)' },
                        };
                        const indicatorNames: Record<string, string> = {
                          streak_count: '连涨/连跌天数', volume_ratio: '量比', algo_score: '算法评分',
                          ai_score: 'AI评分', ma_cross: '均线金叉/死叉', rsi: 'RSI', macd: 'MACD',
                          change_pct: '涨跌幅', turnover_rate: '换手率', amplitude: '振幅',
                          score: '综合评分', signal_score: '信号评分', pe_ratio: '市盈率',
                        };
                        const opLabels: Record<string, string> = {
                          gt: '>', lt: '<', gte: '≥', lte: '≤', eq: '=',
                          cross_up: '金叉', cross_down: '死叉',
                        };
                        const grouped: Record<string, any[]> = { buy: [], add: [], sell: [], reduce: [] };
                        strategy.conditions.forEach((c: any) => {
                          if (grouped[c.condType]) grouped[c.condType].push(c);
                        });
                        return (Object.keys(typeConfig) as string[]).map(ct => {
                          const conds = grouped[ct] || [];
                          if (conds.length === 0) return null;
                          const cfg = typeConfig[ct];
                          return (
                            <div key={ct} style={{
                              marginBottom: 16, borderRadius: 10, overflow: 'hidden',
                              border: `1px solid ${cfg.border}`, background: cfg.bg,
                            }}>
                              <div style={{
                                padding: '8px 16px', background: `${cfg.color}0D`,
                                display: 'flex', alignItems: 'center', gap: 8,
                                borderBottom: `1px solid ${cfg.border}`,
                              }}>
                                <div style={{ width: 8, height: 8, borderRadius: '50%', background: cfg.color, flexShrink: 0 }} />
                                <span style={{ fontSize: 13, fontWeight: 700, color: cfg.color }}>{cfg.label}</span>
                                <span style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--color-text-3)', background: 'var(--color-fill-2)', padding: '2px 8px', borderRadius: 10 }}>{conds.length} 条 · AND</span>
                              </div>
                              <div style={{ padding: '10px 16px' }}>
                                {conds.map((c: any, i: number) => {
                                  const indicatorName = indicatorNames[c.indicator] || c.indicator;
                                  const opSymbol = opLabels[c.operator] || c.operator;
                                  return (
                                    <div key={i} style={{
                                      display: 'flex', alignItems: 'center', gap: 10,
                                      padding: i > 0 ? '8px 0 0' : '0',
                                      borderTop: i > 0 ? '1px solid var(--color-border-1)' : 'none',
                                      marginTop: i > 0 ? 8 : 0,
                                    }}>
                                      <span style={{
                                        flexShrink: 0, padding: '2px 10px', borderRadius: 8,
                                        fontSize: 11, fontWeight: 700,
                                        background: i === 0 ? `${cfg.color}15` : 'var(--color-fill-2)',
                                        color: i === 0 ? cfg.color : 'var(--color-text-3)',
                                      }}>{i === 0 ? 'IF' : 'AND'}</span>
                                      <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-1)', flex: 1 }}>{indicatorName}</span>
                                      <span style={{
                                        flexShrink: 0, padding: '3px 10px', borderRadius: 6,
                                        fontSize: 12, fontWeight: 700, fontFamily: 'monospace',
                                        background: `${cfg.color}10`, color: cfg.color,
                                        border: `1px solid ${cfg.color}25`,
                                      }}>{opSymbol} {c.value}</span>
                                    </div>
                                  );
                                })}
                              </div>
                            </div>
                          );
                        });
                      })()}
                    </>
                  ) : (
                    <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)', border: '1.5px dashed var(--color-border-1)', borderRadius: 10, background: 'var(--color-fill-1)' }}>
                      暂无交易条件配置
                    </div>
                  )}
                </>
              ) : (
                <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)', background: 'var(--color-fill-1)', borderRadius: 8, border: '1px dashed var(--color-border-1)' }}>暂无策略信息</div>
              )}
            </div>
          </Tabs.TabPane>

          <Tabs.TabPane key="trades" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}><FileText size={15} />操作记录</span>}>
            <div style={{ padding: '8px 0 0' }}>
              {tradesArr.length === 0 ? (
                <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)', background: 'var(--color-fill-1)', borderRadius: 8, border: '1px dashed var(--color-border-1)' }}>暂无交易记录</div>
              ) : (
                <Table
                  columns={[
                    { title: '日期', dataIndex: 'date', width: 100, render: (v: string) => <span style={{ fontSize: 11, fontFamily: 'monospace', color: 'var(--color-text-3)' }}>{v}</span> },
                    { title: '操作', dataIndex: 'action', width: 72, render: (v: string, record: any) => {
                        const labels: Record<string, string> = { buy: '买入', add: '加仓', sell: '卖出', reduce: '减仓', stop: '止损' };
                        const colors: Record<string, string> = { buy: '#F53F3F', add: '#FF7D00', sell: '#00B42A', reduce: '#165DFF', stop: '#7B61FF' };
                        const bgs: Record<string, string> = { buy: 'rgba(245,63,63,0.1)', add: 'rgba(255,125,0,0.1)', sell: 'rgba(0,180,42,0.1)', reduce: 'rgba(22,93,255,0.1)', stop: 'rgba(123,97,255,0.1)' };
                        if (v === 'stop') {
                          const isProfit = (record as any).reason === '止盈' || (record as any).pnlPct > 0;
                          return <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, background: isProfit ? 'rgba(245,63,63,0.08)' : 'rgba(0,180,42,0.08)', color: isProfit ? '#F53F3F' : '#00B42A', fontWeight: 700, fontSize: 11 }}>{isProfit ? '止盈' : '止损'}</span>;
                        }
                        return <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, background: bgs[v] || 'var(--color-fill-2)', color: colors[v] || 'var(--color-text-3)', fontWeight: 700, fontSize: 11 }}>{labels[v] || v}</span>;
                      }},
                    { title: '股票', dataIndex: 'name', width: 110, render: (v: string, r: any) => (
                      <div><div style={{ fontWeight: 600, fontSize: 12 }}>{v || r.code}</div><div style={{ fontSize: 10, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{r.code}</div></div>
                    )},
                    { title: '价格', dataIndex: 'price', width: 80, render: (v: number) => <span style={{ fontFamily: 'monospace', fontSize: 12 }}>¥{v?.toFixed(2)}</span> },
                    { title: '数量', dataIndex: 'quantity', width: 64, render: (v: number) => `${v}股` },
                    { title: '盈亏', dataIndex: 'pnlPct', width: 80, render: (v: number) => v != null ? <span style={{ color: v > 0 ? '#F53F3F' : '#00B42A', fontWeight: 600, fontSize: 12 }}>{v > 0 ? '+' : ''}{v?.toFixed(1)}%</span> : <span style={{ color: 'var(--color-text-3)' }}>—</span> },
                    { title: '原因', dataIndex: 'reason', width: 140, render: (v: string) => <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{v || '—'}</span> },
                  ]}
                  data={tradesArr}
                  rowKey={(_, i) => String(i)}
                  pagination={{ pageSize: 20, sizeCanChange: true }}
                  size="small" stripe
                  style={{ borderRadius: 8 }}
                />
              )}
            </div>
          </Tabs.TabPane>

          <Tabs.TabPane key="logs" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}><Activity size={15} />执行日志</span>}>
            <div style={{ padding: '8px 0 0' }}>
              {!logs || logs.length === 0 ? (
                <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)', background: 'var(--color-fill-1)', borderRadius: 8, border: '1px dashed var(--color-border-1)' }}>
                  暂无执行日志
                  <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 8 }}>回测完成后将自动生成详细日志</div>
                </div>
              ) : (
                <div style={{ maxHeight: 520, overflow: 'auto', borderRadius: 8, border: '1px solid var(--color-border-1)' }}>
                  <Table
                    columns={[
                      { title: '序号', dataIndex: 'seq', width: 50 },
                      { title: '日期', dataIndex: 'date', width: 90 },
                      { title: '类型', dataIndex: 'logType', width: 80, render: (v: string) => {
                        const typeColor: Record<string, string> = { info: 'blue', warn: 'orange', error: 'red', debug: 'gray' };
                        return <Tag color={typeColor[v] || 'gray'} size="small">{v}</Tag>;
                      }},
                      { title: '股票', dataIndex: 'stockCode', width: 70 },
                      { title: '信息', dataIndex: 'message', render: (v: string) => <span style={{ fontSize: 12 }}>{v}</span> },
                    ]}
                    data={logs}
                    rowKey="seq"
                    pagination={{ pageSize: 30, sizeCanChange: true }}
                    size="small" stripe
                  />
                </div>
              )}
            </div>
          </Tabs.TabPane>
        </Tabs>
      </Card>

      {stockDetailVisible && selectedStock && (
        <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, zIndex: 1100, background: 'var(--color-fill-2)', overflow: 'auto' }}>
          <div style={{ background: 'var(--color-bg-1)', borderBottom: '1px solid var(--color-border-1)', padding: '12px 24px', display: 'flex', alignItems: 'center', gap: 16, position: 'sticky', top: 0, zIndex: 10, boxShadow: '0 1px 4px rgba(0,0,0,0.04)' }}>
            <Button type="text" onClick={() => setStockDetailVisible(false)}>← 返回</Button>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 16, fontWeight: 700 }}>{selectedStock.stockName} <span style={{ fontSize: 12, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{selectedStock.stockCode}</span></div>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 2 }}>
                累计盈亏: <span style={{ color: selectedStock.totalPnl >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 600 }}>{selectedStock.totalPnl >= 0 ? '+' : ''}{selectedStock.totalPnl?.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
                {' · '}买入 {selectedStock.buyCount} 次 · 卖出 {selectedStock.sellCount} 次
              </div>
            </div>
          </div>
          <div style={{ maxWidth: 1200, margin: '0 auto', padding: '20px 24px' }}>
            {stockLoading ? (
              <div style={{ padding: 60, textAlign: 'center' }}><Spin size={30} /></div>
            ) : (
              <>
                <div style={{ background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 24px', marginBottom: 20, boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)' }}>
                  <div style={{ fontSize: 14, fontWeight: 700, marginBottom: 14 }}>📈 K线图 · 交易标记</div>
                  {stockKline.length > 0 ? (
                    <KLineChart data={stockKline} markers={stockMarkers} height={420} onMarkerClick={handleMarkerClick} />
                  ) : (
                    <div style={{ padding: 60, textAlign: 'center', color: 'var(--color-text-3)' }}>加载K线数据中...</div>
                  )}
                </div>
                <div ref={tradeTableRef} style={{ background: 'var(--color-bg-1)', borderRadius: 12, padding: '20px 24px', boxShadow: '0 1px 3px rgba(0,0,0,0.04)', border: '1px solid var(--color-border-1)' }}>
                  <div style={{ fontSize: 14, fontWeight: 700, marginBottom: 14 }}><List size={14} style={{ marginRight: 6 }} />交易记录</div>
                  <Table
                    columns={[
                      { title: '日期', dataIndex: 'date', width: 100, render: (v: string) => <span style={{ fontSize: 11, fontFamily: 'monospace', color: 'var(--color-text-3)' }}>{v}</span> },
                      { title: '操作', dataIndex: 'action', width: 68, render: (v: string) => {
                        const labels: Record<string, string> = { buy: '买入', add: '加仓', sell: '卖出', reduce: '减仓', stop: '止损' };
                        const colors: Record<string, string> = { buy: '#F53F3F', add: '#FF7D00', sell: '#00B42A', reduce: '#165DFF', stop: '#7B61FF' };
                        const bgs: Record<string, string> = { buy: 'rgba(245,63,63,0.1)', add: 'rgba(255,125,0,0.1)', sell: 'rgba(0,180,42,0.1)', reduce: 'rgba(22,93,255,0.1)', stop: 'rgba(123,97,255,0.1)' };
                        if (v === 'stop') {
                          const isProfit = (record as any).reason === '止盈' || (record as any).pnlPct > 0;
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
                    size="small" stripe
                  />
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
