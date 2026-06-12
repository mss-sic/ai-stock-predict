import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Table, Tabs, Tag, Spin } from '@arco-design/web-react';
import { ArrowLeft, TrendingUp, Trophy, Shield } from 'lucide-react';
import api from '../services/api';

interface EntryData {
  entry: any;
  result: any;
  logs: any[];
}

export default function PkEntryDetailPage() {
  const { id, entryId } = useParams<{ id: string; entryId: string }>();
  const navigate = useNavigate();
  const [data, setData] = useState<EntryData | null>(null);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState('trades');

  useEffect(() => {
    (async () => {
      try {
        const res = await api.get(`/pk/entries/${entryId}/detail`);
        setData(res.data.data);
      } catch {}
      setLoading(false);
    })();
  }, [entryId]);

  if (loading) return <div style={{ padding: 60, textAlign: 'center' }}><Spin size={30} /></div>;
  if (!data) return <div style={{ padding: 60, textAlign: 'center', color: 'var(--color-text-3)' }}>数据加载失败</div>;

  const { entry, result, logs } = data;

  return (
    <div style={{ padding: '20px 24px', maxWidth: 1100, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20 }}>
        <span onClick={() => navigate(`/pk/${id}`)} style={{ cursor: 'pointer', display: 'flex', alignItems: 'center' }}>
          <ArrowLeft size={18} style={{ color: 'var(--color-text-2)' }} />
        </span>
        <Trophy size={20} style={{ color: 'var(--color-warning)' }} />
        <span style={{ fontSize: 16, fontWeight: 600 }}>{entry.username || `选手 #${entry.userId}`}</span>
        <Tag color="arcoblue">{entry.strategyName || '未知策略'}</Tag>
        {entry.finalRank > 0 && (
          <Tag color={entry.finalRank <= 3 ? 'gold' : 'gray'}>第 {entry.finalRank} 名</Tag>
        )}
      </div>

      {/* Metrics */}
      {result && (
        <div className="stat-grid mb16">
          <div className="stat-card">
            <div className="stat-label">总收益率</div>
            <div className={`stat-value ${result.totalReturn >= 0 ? 'up' : 'down'}`}>
              {result.totalReturn >= 0 ? '+' : ''}{result.totalReturn?.toFixed(2)}%
            </div>
          </div>
          <div className="stat-card">
            <div className="stat-label">最终权益</div>
            <div className="stat-value">¥{result.finalEquity?.toLocaleString(undefined, { maximumFractionDigits: 0 })}</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">胜率 · 交易</div>
            <div className="stat-value">{(result.winRate * 100).toFixed(1)}%</div>
            <div className="stat-sub">{result.tradeCount} 笔</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">最大回撤</div>
            <div className="stat-value" style={{ color: 'var(--stock-down)' }}>-{result.maxDrawdown}%</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">夏普比率</div>
            <div className="stat-value">{result.sharpeRatio?.toFixed(2)}</div>
          </div>
        </div>
      )}

      {/* Tabs: Trades & Equity Curve */}
      <div style={{
        background: 'var(--color-bg-1)', borderRadius: 12, overflow: 'hidden',
        border: '1px solid var(--color-border-1)',
      }}>
        <Tabs activeTab={tab} onChange={setTab} style={{ padding: '16px 20px 0' }} type="card-gutter">
          <Tabs.TabPane key="trades" title="📋 操作记录">
            <div style={{ padding: '0 0 16px' }}>
              {(() => {
                const tradesArr = result?.trades?.data || result?.trades || [];
                if (!Array.isArray(tradesArr) || tradesArr.length === 0) {
                  return <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)' }}>暂无交易记录</div>;
                }
                return (
                  <Table
                    columns={[
                      { title: '日期', dataIndex: 'date', width: 100, render: (v: string) => <span style={{ fontSize: 11, fontFamily: 'monospace' }}>{v}</span> },
                      { title: '操作', dataIndex: 'action', width: 72, render: (v: string) => {
                        const labels: Record<string, string> = { buy: '买入', add: '加仓', sell: '卖出', reduce: '减仓' };
                        const colors: Record<string, string> = { buy: 'var(--stock-up)', add: 'var(--color-warning-text)', sell: 'var(--stock-down)', reduce: 'var(--color-primary)' };
                        const bgs: Record<string, string> = { buy: 'var(--color-danger-bg)', add: 'var(--color-warning-bg)', sell: 'var(--color-success-bg)', reduce: 'var(--color-info-bg)' };
                        return <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, background: bgs[v] || 'var(--color-fill-2)', color: colors[v] || 'var(--color-text-3)', fontWeight: 700, fontSize: 11 }}>{labels[v] || v}</span>;
                      }},
                      { title: '股票', dataIndex: 'name', width: 100, render: (v: string, r: any) => (
                        <div><div style={{ fontWeight: 600, fontSize: 12 }}>{v || r.code}</div><div style={{ fontSize: 10, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{r.code}</div></div>
                      )},
                      { title: '价格', dataIndex: 'price', width: 76, render: (v: number) => <span style={{ fontFamily: 'monospace', fontSize: 12 }}>¥{v?.toFixed(2)}</span> },
                      { title: '数量', dataIndex: 'quantity', width: 64, render: (v: number) => `${v}股` },
                      { title: '盈亏', dataIndex: 'pnlPct', width: 72, render: (v: number) => v ? <span style={{ color: v > 0 ? 'var(--stock-up)' : 'var(--stock-down)', fontWeight: 600, fontSize: 12 }}>{v > 0 ? '+' : ''}{v?.toFixed(1)}%</span> : <span style={{ color: 'var(--color-text-3)' }}>—</span> },
                      { title: '原因', dataIndex: 'reason', width: 120, render: (v: string) => <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{v}</span> },
                    ]}
                    data={tradesArr}
                    rowKey={(_, i) => i}
                    pagination={{ pageSize: 20, sizeCanChange: true }}
                    size="small"
                    stripe
                  />
                );
              })()}
            </div>
          </Tabs.TabPane>

          <Tabs.TabPane key="logs" title="📜 执行日志">
            <div style={{ padding: '0 0 16px', maxHeight: 500, overflow: 'auto' }}>
              {logs?.length === 0 ? (
                <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)' }}>暂无日志</div>
              ) : (
                logs?.map((l: any, i: number) => (
                  <div key={i} style={{
                    padding: '6px 12px', fontSize: 11, fontFamily: 'monospace',
                    borderBottom: '1px solid var(--color-border-1)',
                    color: l.level === 'warn' ? 'var(--color-warning-text)' : 'var(--color-text-2)',
                  }}>
                    <span style={{ color: 'var(--color-text-3)', marginRight: 8 }}>{l.date?.slice(5)}</span>
                    {l.msg}
                  </div>
                ))
              )}
            </div>
          </Tabs.TabPane>
        </Tabs>
      </div>
    </div>
  );
}
