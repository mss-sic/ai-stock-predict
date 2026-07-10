import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Button, Table, Tag, Spin, Empty, Tooltip } from '@arco-design/web-react';
import { ArrowLeft, Wallet, TrendingUp, BarChart3, Layers } from 'lucide-react';
import { fetchAccountDetail, syncAccountFromBroker } from '../services/api';

interface AccountDetail {
  account: any;
  holdings: any[];
  runs: any[];
  allocations: any[];
  freeCash: number;
  holdingAllocs: any[];
}

export default function AccountDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [data, setData] = useState<AccountDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);

  useEffect(() => {
    loadData();
  }, [id]);

  const loadData = async () => {
    try {
      setLoading(true);
      const res = await fetchAccountDetail(Number(id));
      setData(res.data?.data || null);
    } catch (e) {
      console.error('load account detail failed', e);
    } finally {
      setLoading(false);
    }
  };

  const handleSync = async () => {
    setSyncing(true);
    try {
      await syncAccountFromBroker(Number(id));
      await loadData();
    } catch (e: any) {
      console.error('sync failed', e);
    } finally {
      setSyncing(false);
    }
  };

  if (loading) return <div style={{ padding: 40, textAlign: 'center' }}><Spin size={40} /></div>;
  if (!data || !data.account) return <Empty description="账户不存在" />;

  const account = data.account;
  const holdings = data.holdings || [];
  const allocations = data.allocations || [];
  const freeCash = data.freeCash ?? 0;
  const holdingAllocs = data.holdingAllocs || [];

  const formatMoney = (v: number | undefined | null) => `¥${(v || 0).toLocaleString()}`;
  const pnlColor = (v: number) => v >= 0 ? '#F53F3F' : '#00B42A';

  return (
    <div style={{ padding: 20, maxWidth: 1200, margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20 }}>
        <Button icon={<ArrowLeft size={16} />} type="text" onClick={() => navigate(-1)} />
        <h2 style={{ margin: 0, fontSize: 20, fontWeight: 700 }}>{account.name || '账户详情'}</h2>
        <Tag color={account.status === 'active' ? 'green' : 'gray'}>{account.accountType === 'real' ? '真实' : '模拟'}</Tag>
        <div style={{ flex: 1 }} />
        <Button icon={<Wallet size={14} />} loading={syncing} onClick={handleSync}>同步券商</Button>
      </div>

      {/* Overview Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 20 }}>
        {[
          { label: '总资产', value: account.totalAssets || account.availableCash, icon: <Wallet size={16} />, color: '#165DFF' },
          { label: '可用现金', value: account.availableCash, icon: <TrendingUp size={16} />, color: '#0FC6C2' },
          { label: '持仓市值', value: account.totalMarketValue || 0, icon: <BarChart3 size={16} />, color: '#722ED1' },
          { label: '自由现金', value: freeCash, icon: <Layers size={16} />, color: freeCash >= 0 ? '#00B42A' : '#F53F3F' },
        ].map((card, i) => (
          <Card key={i} style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8, color: 'var(--color-text-3)', fontSize: 12 }}>
              <span style={{ color: card.color }}>{card.icon}</span>
              {card.label}
            </div>
            <div style={{ fontSize: 18, fontWeight: 700, color: card.value < 0 ? '#F53F3F' : 'var(--color-text-1)' }}>
              {formatMoney(card.value)}
            </div>
          </Card>
        ))}
      </div>

      {/* Strategy Allocations */}
      <Card title="策略资金分配" style={{ marginBottom: 20, borderRadius: 10 }} bodyStyle={{ padding: 0 }}>
        {allocations.length === 0 ? (
          <div style={{ padding: 24, textAlign: 'center', color: 'var(--color-text-3)' }}>暂无活跃策略</div>
        ) : (
          <Table
            data={allocations}
            size="small"
            pagination={false}
            columns={[
              { title: '策略名称', dataIndex: 'runName', width: 160, render: (v: string, row: any) => (
                <a onClick={() => navigate(`/live/${row.runId}`)} style={{ cursor: 'pointer', color: '#165DFF' }}>{v}</a>
              )},
              { title: '状态', dataIndex: 'status', width: 70, render: (v: string) => <Tag size="small" color={v === 'active' ? 'green' : 'orange'}>{v}</Tag> },
              { title: '可用现金', dataIndex: 'availableCash', width: 120, render: (v: number) => <span style={{ fontFamily: 'monospace' }}>{formatMoney(v)}</span> },
              { title: '持仓市值', dataIndex: 'positionValue', width: 120, render: (v: number) => <span style={{ fontFamily: 'monospace' }}>{formatMoney(v)}</span> },
              { title: '总权益', dataIndex: 'totalEquity', width: 120, render: (v: number) => <span style={{ fontFamily: 'monospace', fontWeight: 600 }}>{formatMoney(v)}</span> },
              { title: '总成本', dataIndex: 'totalCost', width: 120, render: (v: number) => <span style={{ fontFamily: 'monospace' }}>{formatMoney(v)}</span> },
              { title: '收益率', dataIndex: 'returnPct', width: 100, render: (v: number) => (
                <span style={{ color: pnlColor(v), fontWeight: 600, fontFamily: 'monospace' }}>
                  {v >= 0 ? '+' : ''}{v?.toFixed?.(2) ?? '-'}%
                </span>
              )},
              { title: '持仓数', dataIndex: 'positionCount', width: 70, align: 'center' as const },
            ]}
          />
        )}
      </Card>

      {/* Holdings Distribution */}
      <Card title="持仓归属分布" style={{ borderRadius: 10 }} bodyStyle={{ padding: 0 }}>
        {holdingAllocs.length === 0 ? (
          <div style={{ padding: 24, textAlign: 'center', color: 'var(--color-text-3)' }}>暂无持仓</div>
        ) : (
          <Table
            data={holdingAllocs}
            size="small"
            pagination={false}
            columns={[
              { title: '股票', dataIndex: 'stockCode', width: 100, render: (v: string, row: any) => (
                <Tooltip content={row.stockName}>
                  <span style={{ fontFamily: 'monospace' }}>{v}</span>
                </Tooltip>
              )},
              { title: '名称', dataIndex: 'stockName', width: 100 },
              { title: '总持仓', dataIndex: 'totalQty', width: 80, align: 'center' as const },
              { title: '策略归属', dataIndex: 'runAllocs', render: (allocs: any[]) => (
                <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                  {(allocs || []).map((a, i) => (
                    <Tag key={i} size="small" color="arcoblue">
                      {a.runName}: {a.quantity}股
                    </Tag>
                  ))}
                  {(allocs || []).length === 0 && <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>未分配</span>}
                </div>
              )},
            ]}
          />
        )}
      </Card>
    </div>
  );
}
