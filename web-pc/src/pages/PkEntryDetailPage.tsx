import { useState, useEffect, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Table, Tabs, Tag, Spin, Card, Button } from '@arco-design/web-react';
import { ArrowLeft, TrendingUp, Trophy, Shield, PieChart, FileText, Activity } from 'lucide-react';
import api from '../services/api';
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
  const trades = tradesData?.data || tradesData || [];
  if (!Array.isArray(trades) || trades.length === 0) return [];
  
  const map = new Map<string, StockAnalysis>();
  for (const t of trades) {
    const code = t.code || t.stockCode || '';
    if (!code) continue;
    if (!map.has(code)) {
      map.set(code, {
        stockCode: code,
        stockName: t.name || t.stockName || code,
        totalPnl: 0,
        totalPnlPct: 0,
        buyCount: 0,
        sellCount: 0,
        trades: [],
      });
    }
    const sa = map.get(code)!;
    if (t.action === 'buy' || t.action === 'add') {
      sa.buyCount++;
    } else if (t.action === 'sell' || t.action === 'reduce') {
      sa.sellCount++;
      if (t.pnl != null) sa.totalPnl += t.pnl;
    }
    sa.trades.push(t);
  }
  
  const result = Array.from(map.values());
  for (const sa of result) {
    if (sa.totalPnl !== 0) {
      sa.totalPnlPct = sa.totalPnl / initialCapital * 100;
    }
  }
  result.sort((a, b) => Math.abs(b.totalPnl) - Math.abs(a.totalPnl));
  return result;
}

export default function PkEntryDetailPage() {
  const { id, entryId } = useParams<{ id: string; entryId: string }>();
  const navigate = useNavigate();
  const [data, setData] = useState<EntryData | null>(null);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState('overview');

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

  if (loading) return <div style={{ padding: 60, textAlign: 'center' }}><Spin size={30} /></div>;
  if (!data) return <div style={{ padding: 60, textAlign: 'center', color: 'var(--color-text-3)' }}>数据加载失败</div>;

  const { entry, strategy, result, logs } = data;
  const tradesArr = result?.trades?.data || result?.trades || [];

  return (
    <div style={{ padding: '20px 24px', maxWidth: 1200, margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24 }}>
        <span onClick={() => navigate(`/pk/${id}`)} style={{ cursor: 'pointer', display: 'flex', alignItems: 'center' }}>
          <ArrowLeft size={18} style={{ color: 'var(--color-text-2)' }} />
        </span>
        <Trophy size={22} style={{ color: 'var(--color-warning-6)' }} />
        <span style={{ fontSize: 18, fontWeight: 700 }}>{entry.username || `选手 #${entry.userId}`}</span>
        <Tag color="arcoblue" size="large">{entry.strategyName || '未知策略'}</Tag>
        {entry.finalRank > 0 && (
          <Tag color={entry.finalRank <= 3 ? 'gold' : 'gray'} size="large">
            🏅 第 {entry.finalRank} 名
          </Tag>
        )}
        {entry.status === 'running' && <Tag color="blue">进行中</Tag>}
        {entry.status === 'completed' && <Tag color="green">已完成</Tag>}
      </div>

      {/* Strategy Info */}
      {strategy && (
        <Card style={{ marginBottom: 20, background: 'var(--color-bg-1)', boxShadow: '0 1px 4px rgba(0,0,0,0.04)' }} title={
          <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <TrendingUp size={16} style={{ color: 'var(--color-primary)' }} />
            策略条件
          </span>
        }>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '10px 20px' }}>
            <div><span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>止盈</span><div style={{ fontWeight: 600 }}>{strategy.stopProfit > 0 ? `${strategy.stopProfit}%` : '未设置'}</div></div>
            <div><span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>止损</span><div style={{ fontWeight: 600, color: '#F53F3F' }}>{strategy.stopLoss < 0 ? `${strategy.stopLoss}%` : '未设置'}</div></div>
            <div><span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>最大持股</span><div style={{ fontWeight: 600 }}>{strategy.maxHoldings} 只</div></div>
            <div><span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>建仓比例</span><div style={{ fontWeight: 600 }}>{strategy.buyPct}%</div></div>
            <div><span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>加仓比例</span><div style={{ fontWeight: 600 }}>{strategy.addPct}%</div></div>
            <div><span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>初始资金</span><div style={{ fontWeight: 600 }}>¥{(strategy.initialCapital || 0).toLocaleString()}</div></div>
          </div>
          {strategy.conditions && strategy.conditions.length > 0 && (
            <>
              <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-2)', margin: '14px 0 8px', borderTop: '1px solid var(--color-border-1)', paddingTop: 12 }}>
                交易条件
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                {strategy.conditions.map((c: any, i: number) => {
                  const typeLabels: Record<string, string> = { buy: '买入', add: '加仓', sell: '卖出', reduce: '减仓' };
                  const typeColors: Record<string, string> = { buy: '#F53F3F', add: '#FF7D00', sell: '#00B42A', reduce: '#165DFF' };
                  const opLabels: Record<string, string> = { gt: '>', lt: '<', gte: '≥', lte: '≤', eq: '=', cross_up: '上穿', cross_down: '下穿' };
                  return (
                    <span key={i} style={{
                      display: 'inline-flex', alignItems: 'center', gap: 4,
                      padding: '3px 10px', borderRadius: 14,
                      fontSize: 11, fontWeight: 500,
                      background: `${typeColors[c.condType] || '#165DFF'}14`,
                      color: typeColors[c.condType] || '#165DFF',
                      border: `1px solid ${typeColors[c.condType] || '#165DFF'}30`,
                    }}>
                      <span style={{ fontWeight: 700 }}>{typeLabels[c.condType] || c.condType}</span>
                      {c.indicator} {opLabels[c.operator] || c.operator} {c.value}
                    </span>
                  );
                })}
              </div>
            </>
          )}
        </Card>
      )}

      {/* Metrics Cards */}
      {result && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 14, marginBottom: 24 }}>
          <Card style={{ background: 'var(--color-bg-1)', boxShadow: '0 1px 4px rgba(0,0,0,0.04)' }}>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>总收益率</div>
            <div style={{
              fontSize: 28, fontWeight: 800,
              color: result.totalReturn >= 0 ? '#F53F3F' : '#00B42A',
              fontFamily: 'monospace',
            }}>
              {result.totalReturn >= 0 ? '+' : ''}{result.totalReturn?.toFixed(2)}%
            </div>
          </Card>
          <Card style={{ background: 'var(--color-bg-1)', boxShadow: '0 1px 4px rgba(0,0,0,0.04)' }}>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>最终权益</div>
            <div style={{ fontSize: 24, fontWeight: 700, fontFamily: 'monospace' }}>
              ¥{result.finalEquity?.toLocaleString(undefined, { maximumFractionDigits: 0 })}
            </div>
          </Card>
          <Card style={{ background: 'var(--color-bg-1)', boxShadow: '0 1px 4px rgba(0,0,0,0.04)' }}>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>胜率</div>
            <div style={{ fontSize: 24, fontWeight: 700, fontFamily: 'monospace' }}>
              {(result.winRate * 100).toFixed(1)}%
            </div>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 2 }}>{result.tradeCount} 笔交易</div>
          </Card>
          <Card style={{ background: 'var(--color-bg-1)', boxShadow: '0 1px 4px rgba(0,0,0,0.04)' }}>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>最大回撤</div>
            <div style={{ fontSize: 24, fontWeight: 700, color: '#00B42A', fontFamily: 'monospace' }}>
              -{result.maxDrawdown}%
            </div>
          </Card>
          <Card style={{ background: 'var(--color-bg-1)', boxShadow: '0 1px 4px rgba(0,0,0,0.04)' }}>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>夏普比率</div>
            <div style={{ fontSize: 24, fontWeight: 700, fontFamily: 'monospace' }}>
              {result.sharpeRatio?.toFixed(2)}
            </div>
          </Card>
        </div>
      )}

      {/* Tabs */}
      <Card style={{ borderRadius: 12, overflow: 'hidden' }}>
        <Tabs activeTab={tab} onChange={setTab} type="card-gutter" style={{ padding: '0 20px' }}>
          <Tabs.TabPane key="overview" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Activity size={14} />概览</span>}>
            <div style={{ padding: '16px 0' }}>
              {result?.equityCurve?.data ? (
                <div style={{ height: 360 }}>
                  <KLineChart
                    data={{ dates: result.equityCurve.data.dates || [], values: result.equityCurve.data.values || [] }}
                    type="equity"
                    height={360}
                  />
                </div>
              ) : (
                <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>
                  暂无权益曲线数据
                </div>
              )}
            </div>
          </Tabs.TabPane>

          <Tabs.TabPane key="trades" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><FileText size={14} />操作记录</span>}>
            <div style={{ padding: '0 0 16px' }}>
              {!Array.isArray(tradesArr) || tradesArr.length === 0 ? (
                <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)' }}>暂无交易记录</div>
              ) : (
                <Table
                  columns={[
                    { title: '日期', dataIndex: 'date', width: 100, render: (v: string) => <span style={{ fontSize: 11, fontFamily: 'monospace' }}>{v}</span> },
                    { title: '操作', dataIndex: 'action', width: 72, render: (v: string) => {
                      const labels: Record<string, string> = { buy: '买入', add: '加仓', sell: '卖出', reduce: '减仓' };
                      const colors: Record<string, string> = { buy: '#F53F3F', add: '#FF7D00', sell: '#00B42A', reduce: '#165DFF' };
                      const bgs: Record<string, string> = { buy: 'rgba(245,63,63,0.1)', add: 'rgba(255,125,0,0.1)', sell: 'rgba(0,180,42,0.1)', reduce: 'rgba(22,93,255,0.1)' };
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
                  size="small"
                  stripe
                />
              )}
            </div>
          </Tabs.TabPane>

          <Tabs.TabPane key="analysis" title={
            <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <PieChart size={14} />收益分析
              {stockAnalysis.length > 0 && <span style={{ background: 'var(--color-primary-bg)', color: 'var(--color-primary)', fontSize: 11, fontWeight: 600, padding: '1px 8px', borderRadius: 10 }}>{stockAnalysis.length}</span>}
            </span>
          }>
            <div style={{ padding: '0 0 16px' }}>
              {stockAnalysis.length > 0 ? (
                <Table
                  columns={[
                    { title: '#', width: 40, render: (_: any, __: any, i: number) => <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{i + 1}</span> },
                    { title: '股票', dataIndex: 'stockName', width: 110, sorter: (a: any, b: any) => a.stockCode.localeCompare(b.stockCode), render: (v: string, r: any) => (
                      <div>
                        <div style={{ fontWeight: 600, fontSize: 12 }}>{v || r.stockCode}</div>
                        <div style={{ fontSize: 10, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{r.stockCode}</div>
                      </div>
                    )},
                    { title: '总盈亏', dataIndex: 'totalPnl', width: 100, sorter: (a: any, b: any) => a.totalPnl - b.totalPnl, render: (v: number) => (
                      <span style={{ color: v >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 700, fontSize: 13, fontFamily: 'monospace' }}>
                        {v >= 0 ? '+' : ''}{v?.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                      </span>
                    )},
                    { title: '收益率', dataIndex: 'totalPnlPct', width: 85, sorter: (a: any, b: any) => a.totalPnlPct - b.totalPnlPct, render: (v: number) => (
                      <span style={{ color: v >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 600, fontSize: 12, fontFamily: 'monospace' }}>
                        {v >= 0 ? '+' : ''}{v?.toFixed(2)}%
                      </span>
                    )},
                    { title: '买入', dataIndex: 'buyCount', width: 55, render: (v: number) => <span style={{ fontSize: 12, color: '#F53F3F', fontFamily: 'monospace' }}>{v}次</span> },
                    { title: '卖出', dataIndex: 'sellCount', width: 55, render: (v: number) => <span style={{ fontSize: 12, color: '#00B42A', fontFamily: 'monospace' }}>{v}次</span> },
                  ]}
                  data={stockAnalysis}
                  rowKey="stockCode"
                  pagination={{ pageSize: 15, sizeCanChange: true, showTotal: true }}
                  size="small"
                  stripe
                />
              ) : (
                <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>
                  暂无收益分析数据
                </div>
              )}
            </div>
          </Tabs.TabPane>

          <Tabs.TabPane key="logs" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Activity size={14} />执行日志</span>}>
            <div style={{ padding: '0 0 16px' }}>
              {!logs || logs.length === 0 ? (
                <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)' }}>
                  暂无执行日志
                  <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 8 }}>
                    回测完成后将自动生成详细日志
                  </div>
                </div>
              ) : (
                <div style={{ maxHeight: 520, overflow: 'auto' }}>
                  <Table
                    columns={[
                      { title: '时间', dataIndex: 'date', width: 110, render: (v: string) => <span style={{ fontSize: 11, fontFamily: 'monospace' }}>{v}</span> },
                      { title: '类型', dataIndex: 'level', width: 70, render: (v: string) => {
                        const cmap: Record<string, { color: string; bg: string; text: string }> = {
                          info: { color: '#165DFF', bg: 'rgba(22,93,255,0.08)', text: '信息' },
                          warn: { color: '#FF7D00', bg: 'rgba(255,125,0,0.08)', text: '警告' },
                          error: { color: '#F53F3F', bg: 'rgba(245,63,63,0.08)', text: '错误' },
                          success: { color: '#00B42A', bg: 'rgba(0,180,42,0.08)', text: '成功' },
                        };
                        const c = cmap[v] || cmap.info;
                        return <span style={{ padding: '2px 8px', borderRadius: 4, background: c.bg, color: c.color, fontSize: 11, fontWeight: 600 }}>{c.text}</span>;
                      }},
                      { title: '消息', dataIndex: 'msg', render: (v: string) => <span style={{ fontSize: 12, fontFamily: 'monospace', lineHeight: '20px' }}>{v}</span> },
                    ]}
                    data={logs?.map((l: any, i: number) => ({ ...l, key: i })) || []}
                    rowKey="key"
                    pagination={logs?.length > 30 ? { pageSize: 30 } : false}
                    size="small"
                    stripe
                  />
                </div>
              )}
            </div>
          </Tabs.TabPane>
        </Tabs>
      </Card>
    </div>
  );
}
