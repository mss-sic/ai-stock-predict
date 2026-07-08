import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Table, Button, Modal, Input, InputNumber, Tag, Select, Tabs, Card, Message } from '@arco-design/web-react';
import { Briefcase, Plus, Edit, Trash2, Search, Wallet, TrendingUp, TrendingDown, DollarSign, BarChart3, Building2, RefreshCw } from 'lucide-react';
import { fetchHoldingsSummary, fetchHoldings, fetchAccountsOverview, fetchHoldingAccounts,
         createHolding, updateHolding, deleteHolding, updateAccount, searchStock, syncFromBroker } from '../services/api';

interface Holding {
  id: number; accountId: number;
  stockCode: string; stockName: string;
  costPrice: number; quantity: number; totalCost: number;
  buyDate: string; curPrice: number; priceDate: string; prevClose: number;
  dailyChg: number; dailyChgPct: number; dailyPnl: number;
  marketVal: number; pnl: number; pnlPct: number; holdDays: number;
  todayBuyQty: number; availSellQty: number;
}

interface Summary {
  initialCapital: number; availableCash: number;
  totalMarketValue: number; totalCost: number;
  totalEquity: number; totalPnl: number; totalPnlPct: number;
  totalDailyPnl: number; positionCount: number; upCount: number; downCount: number;
  accountCount: number;
}

interface AccountOv {
  accountId: number; accountName: string; broker: string; accountType: string;
  initialCapital: number; availableCash: number;
  positionValue: number; totalEquity: number;
  totalPnl: number; totalPnlPct: number; dailyPnl: number; positionCount: number;
}

interface AccountInfo {
  id: number; name: string; broker: string; accountType: string;
  initialCapital: number; availableCash: number;
  brokerMode?: string; mxApiKey?: string;
}

const pnlColor = (v: number) => v >= 0 ? '#F53F3F' : '#00B42A';
const pnlSign = (v: number) => v >= 0 ? '' : '-';

export default function HoldingsPage() {
  const navigate = useNavigate();
  const [data, setData] = useState<Holding[]>([]);
  const [summary, setSummary] = useState<Summary | null>(null);
  const [accountsOv, setAccountsOv] = useState<AccountOv[]>([]);
  const [accounts, setAccounts] = useState<AccountInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [acctType, setAcctType] = useState<string>('real'); // 'real' | 'simulated'
  const [activeTab, setActiveTab] = useState<string>('all'); // 'all' or accountId string

  // Create/Edit modal
  const [modalOpen, setModalOpen] = useState(false);
  const [fundModalOpen, setFundModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [formCode, setFormCode] = useState('');
  const [formCost, setFormCost] = useState<number>(0);
  const [formQty, setFormQty] = useState<number>(0);
  const [formBuyDate, setFormBuyDate] = useState('');
  const [formName, setFormName] = useState('');
  const [formAccountId, setFormAccountId] = useState<number>(0);
  const [searchResults, setSearchResults] = useState<any[]>([]);
  const [searching, setSearching] = useState(false);

  // Fund modal
  const [fundAction, setFundAction] = useState<'deposit' | 'withdraw'>('deposit');
  const [fundAmount, setFundAmount] = useState<number>(0);
  const [fundAccountId, setFundAccountId] = useState<number>(0);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [{ data: r }, { data: a }, { data: accts }] = await Promise.all([
        fetchHoldingsSummary(acctType || undefined),
        fetchAccountsOverview(acctType || undefined),
        fetchHoldingAccounts(),
      ]);
      setSummary(r.data || null);
      setAccountsOv(a.data || []);
      setAccounts(accts.data || []);
    } catch (e) { console.error('load holdings', e); }

    try {
      const aid = activeTab !== 'all' ? Number(activeTab) : undefined;
      const { data: holdingsRes } = await fetchHoldings(aid, activeTab === 'all' ? acctType : undefined);
      setData(holdingsRes.data || []);
    } catch (e) { console.error('load holdings list', e); }
    setLoading(false);
  }, [acctType, activeTab]);

  useEffect(() => { load(); }, [load]);

  const handleSearch = async (kw: string) => {
    setFormCode(kw);
    if (kw.length < 2) { setSearchResults([]); return; }
    setSearching(true);
    try { const { data: r } = await searchStock(kw); setSearchResults((r.data || []).slice(0, 8)); } catch { setSearchResults([]); }
    setSearching(false);
  };

  const selectStock = (s: any) => { setFormCode(s.code); setFormName(s.name); setSearchResults([]); };

  const handleSyncBroker = async (accountId: number) => {
    try {
      Message.info('正在从券商同步持仓...');
      const res = await syncFromBroker(accountId);
      Message.success(`同步完成: ${res.data?.posCount || 0}个持仓`);
      load();
    } catch (e: any) {
      Message.error('同步失败: ' + (e?.response?.data?.message || '未知错误'));
    }
  };

  const openCreate = () => {
    setEditingId(null);
    setFormCode(''); setFormName(''); setFormCost(0); setFormQty(0);
    setFormBuyDate(new Date().toISOString().slice(0, 10));
    setFormAccountId(accounts[0]?.id || 0);
    setSearchResults([]);
    setModalOpen(true);
  };

  const openEdit = (h: Holding) => {
    setEditingId(h.id); setFormCode(h.stockCode); setFormName(h.stockName);
    setFormCost(h.costPrice); setFormQty(h.quantity); setFormBuyDate(h.buyDate || '');
    setFormAccountId(h.accountId);
    setSearchResults([]);
    setModalOpen(true);
  };

  const handleSave = async () => {
    if (!formCode || formCost <= 0 || formQty <= 0) { Message.warning('请填写完整信息'); return; }
    try {
      if (editingId) {
        await updateHolding(editingId, formCost, formQty, formBuyDate);
        Message.success('修改成功');
      } else {
        await createHolding(formCode, formCost, formQty, formBuyDate, formAccountId || undefined);
        Message.success('买入成功');
      }
      setModalOpen(false); load();
    } catch (e: any) { Message.error(e?.response?.data?.message || '操作失败'); }
  };

  const handleDelete = async (id: number) => {
    try { await deleteHolding(id); Message.success('已卖出'); load(); } catch {}
  };

  const handleFundUpdate = async () => {
    if (fundAmount <= 0) return;
    try {
      await updateAccount(fundAction, fundAmount, fundAccountId || undefined);
      Message.success(fundAction === 'deposit' ? '入金成功' : '出金成功');
      setFundModalOpen(false); load();
    } catch {}
  };

  const inp: React.CSSProperties = {
    width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--color-border-1)',
    background: 'var(--color-fill-2)', color: 'var(--color-text-1)', fontSize: 14, outline: 'none', boxSizing: 'border-box',
  };

  // Build tabs
  const filteredAccounts = accounts.filter(a => a.accountType === acctType);
  const tabItems = [{ key: 'all', title: '全部' }];
  filteredAccounts.forEach(a => {
    const ov = accountsOv.find(o => o.accountId === a.id);
    tabItems.push({
      key: String(a.id),
      title: <span style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12 }}>
        {a.name}
        <Tag size="small" color={a.accountType === 'real' ? 'red' : 'arcoblue'} style={{ fontSize: 10 }}>
          {a.accountType === 'real' ? '实盘' : '模拟'}
        </Tag>
      </span>,
    });
  });

  return (
    <div style={{ padding: 20, maxWidth: 1440, margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <h2 style={{ margin: 0, fontSize: 20, fontWeight: 700, display: 'flex', alignItems: 'center', gap: 8 }}>
          <Briefcase size={22} style={{ color: '#165DFF' }} />持股管理
        </h2>
        <div style={{ display: 'flex', gap: 8 }}>
          <div style={{ display: 'flex', borderRadius: 6, overflow: 'hidden', border: '1px solid var(--color-border-2)' }}>
            <Button
              size="small"
              type={acctType === 'real' ? 'primary' : 'default'}
              onClick={() => { setAcctType('real'); setActiveTab('all'); }}
              style={{ borderRadius: 0, border: 'none' }}
            >真实资金</Button>
            <Button
              size="small"
              type={acctType === 'simulated' ? 'primary' : 'default'}
              onClick={() => { setAcctType('simulated'); setActiveTab('all'); }}
              style={{ borderRadius: 0, border: 'none' }}
            >模拟资金</Button>
          </div>
          <Button size="small" icon={<Wallet size={14} />} onClick={() => { setFundAccountId(filteredAccounts[0]?.id || 0); setFundModalOpen(true); }}>资金管理</Button>
          {filteredAccounts.length > 0 && (filteredAccounts[0]?.brokerMode === "mx_moni" || filteredAccounts[0]?.mxApiKey) && (
            <Button size="small" icon={<RefreshCw size={14} />} onClick={() => handleSyncBroker(filteredAccounts[0]?.id)}>从券商同步</Button>
          )}
          <Button size="small" type="primary" icon={<Plus size={14} />} onClick={openCreate}>买入股票</Button>
        </div>
      </div>

      {/* Overview Cards — Aggregated */}
      {summary && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 12, marginBottom: 16 }}>
          {[
            { label: '总资产', value: `¥${summary.totalEquity.toLocaleString(undefined, {maximumFractionDigits: 0})}`, icon: Wallet, color: '#165DFF' },
            { label: '可用资金', value: `¥${summary.availableCash.toLocaleString(undefined, {maximumFractionDigits: 0})}`, icon: DollarSign, color: '#0FC6C2' },
            { label: '持仓市值', value: `¥${summary.totalMarketValue.toLocaleString(undefined, {maximumFractionDigits: 0})}`, icon: BarChart3, color: '#722ED1' },
            { label: '累计盈亏', value: `${pnlSign(summary.totalPnl)}¥${Math.abs(summary.totalPnl).toLocaleString(undefined, {maximumFractionDigits: 0})}`, sub: `${summary.totalPnlPct.toFixed(2)}%`, icon: TrendingUp, color: pnlColor(summary.totalPnl) },
            { label: '日收益', value: `${pnlSign(summary.totalDailyPnl)}¥${Math.abs(summary.totalDailyPnl).toLocaleString(undefined, {maximumFractionDigits: 0})}`, icon: TrendingDown, color: pnlColor(summary.totalDailyPnl) },
          ].map((item, idx) => (
            <div key={idx} style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '14px 16px' }}>
              <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4, display: 'flex', alignItems: 'center', gap: 4 }}>
                <item.icon size={13} style={{ color: item.color }} />{item.label}
              </div>
              <div style={{ fontSize: 18, fontWeight: 700, color: item.color, fontFamily: "'SF Mono', 'Inter', monospace" }}>{item.value}</div>
              {item.sub && <div style={{ fontSize: 12, color: item.color, marginTop: 2 }}>{item.sub}</div>}
            </div>
          ))}
        </div>
      )}

      {/* Account Tabs + Holdings Table */}
      <Card style={{ borderRadius: 10 }} bodyStyle={{ padding: '0 16px 16px' }}>
        <Tabs activeTab={activeTab} onChange={setActiveTab} style={{ marginBottom: 0 }}
          tabBarExtraContent={
            <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>
              {activeTab !== 'all'
                ? (() => { const o = accountsOv.find(a => String(a.accountId) === activeTab); return o ? `${o.accountName}: 权益 ¥${o.totalEquity.toLocaleString()} · 持仓 ${o.positionCount}只` : ''; })()
                : `${summary?.positionCount || 0}只持仓 · ${summary?.accountCount || 0}个账户`
              }
            </div>
          }
        >
          {tabItems.map(tab => (
            <Tabs.TabPane key={tab.key} title={tab.title} />
          ))}
        </Tabs>

        <Table
          data={data}
          loading={loading}
          rowKey="id"
          size="small"
          pagination={{ pageSize: 30, sizeCanChange: false }}
          columns={[
            { title: '代码', dataIndex: 'stockCode', width: 80, render: (v: string, r: Holding) => (
              <span style={{ cursor: 'pointer', color: '#165DFF' }} onClick={() => navigate(`/stock/${v}`)}>{v}</span>
            )},
            { title: '名称', dataIndex: 'stockName', width: 100 },
            { title: '持仓', dataIndex: 'quantity', width: 70, render: (v: number) => `${v}股` },
            { title: '成本', dataIndex: 'costPrice', width: 85, render: (v: number) => `¥${v.toFixed(3)}` },
            { title: '现价', dataIndex: 'curPrice', width: 80, render: (v: number) => `¥${v.toFixed(2)}` },
            { title: '市值', width: 95, render: (_: any, r: Holding) => <span style={{ fontWeight: 600 }}>¥{r.marketVal.toLocaleString()}</span> },
            { title: '日涨跌', width: 90, render: (_: any, r: Holding) => (
              <span style={{ color: pnlColor(r.dailyChg), fontSize: 12 }}>
                {r.dailyChg.toFixed(2)} ({r.dailyChgPct.toFixed(2)}%)
              </span>
            )},
            { title: '浮动盈亏', width: 120, render: (_: any, r: Holding) => (
              <span style={{ color: pnlColor(r.pnl), fontWeight: 600, fontSize: 12 }}>
                {pnlSign(r.pnl)}¥{Math.abs(r.pnl).toLocaleString(undefined, {maximumFractionDigits: 0})} ({r.pnlPct.toFixed(2)}%)
              </span>
            )},
            { title: '日盈亏', width: 90, render: (_: any, r: Holding) => (
              <span style={{ color: pnlColor(r.dailyPnl), fontSize: 12 }}>{pnlSign(r.dailyPnl)}¥{Math.abs(r.dailyPnl).toFixed(0)}</span>
            )},
            { title: '可卖', dataIndex: 'availSellQty', width: 55, render: (v: number) => v + '股' },
            { title: '今买', dataIndex: 'todayBuyQty', width: 55, render: (v: number) => v > 0 ? <span style={{color:'#F53F3F'}}>{v}股</span> : '-' },
            { title: '持天数', dataIndex: 'holdDays', width: 55 },
            { title: '日期', dataIndex: 'buyDate', width: 90 },
            { title: '操作', width: 90, render: (_: any, r: Holding) => (
              <div style={{ display: 'flex', gap: 4 }}>
                <Button size="mini" type="text" icon={<Edit size={12} />} onClick={() => openEdit(r)} />
                <Button size="mini" type="text" status="danger" icon={<Trash2 size={12} />} onClick={() => handleDelete(r.id)} />
              </div>
            )},
          ]}
        />
      </Card>

      {/* Buy/Edit Modal */}
      <Modal visible={modalOpen} title={editingId ? '修改持仓' : '买入股票'} onCancel={() => setModalOpen(false)} onOk={handleSave} okText="保存" width={440}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {!editingId && (
            <div style={{ position: 'relative' }}>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>搜索股票</div>
              <Input value={formCode} onChange={handleSearch} style={{ ...inp }} placeholder="输入代码或名称搜索" />
              {searchResults.length > 0 && (
                <div style={{ position: 'absolute', zIndex: 10, top: 64, left: 0, right: 0, background: 'var(--color-bg-2)', borderRadius: 6, border: '1px solid var(--color-border-1)', maxHeight: 200, overflowY: 'auto' }}>
                  {searchResults.map((s: any) => (
                    <div key={s.code} onClick={() => selectStock(s)} style={{ padding: '8px 12px', cursor: 'pointer', borderBottom: '1px solid var(--color-border-1)', display: 'flex', justifyContent: 'space-between' }}
                      onMouseEnter={e => (e.currentTarget.style.background = 'var(--color-fill-2)')}
                      onMouseLeave={e => (e.currentTarget.style.background = '')}>
                      <span style={{ fontWeight: 600, fontFamily: 'monospace' }}>{s.code}</span><span>{s.name}</span>
                    </div>
                  ))}
                </div>
              )}
              {formName && <div style={{ marginTop: 4, fontSize: 12, color: 'var(--color-primary)' }}>已选: {formName}</div>}
            </div>
          )}
          {editingId && <div><div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>股票</div><div style={{ fontSize: 14, fontWeight: 600 }}>{formCode} {formName}</div></div>}
          {!editingId && accounts.length > 0 && (
            <div>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>选择账户</div>
              <Select value={formAccountId || undefined} onChange={v => setFormAccountId(v as number)} style={{ width: '100%' }}
                options={filteredAccounts.map(a => ({ label: `${a.name} (${a.broker || '默认'}·¥${a.availableCash})`, value: a.id }))} />
            </div>
          )}
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>买入价格 (元)</div>
            <InputNumber value={formCost} onChange={v => setFormCost(v || 0)} min={0.01} step={0.01} precision={3} style={{ width: '100%' }} />
          </div>
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>买入数量 (股)</div>
            <InputNumber value={formQty} onChange={v => setFormQty(v || 0)} min={1} step={100} precision={0} style={{ width: '100%' }} />
          </div>
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>买入日期</div>
            <Input value={formBuyDate} onChange={setFormBuyDate} style={{ ...inp }} />
          </div>
          {!editingId && (
            <div style={{ padding: '8px 12px', background: 'var(--color-info-bg)', borderRadius: 6, fontSize: 12, color: 'var(--color-primary)' }}>
              预计买入金额：<b>¥{((formCost * formQty) || 0).toLocaleString(undefined, { maximumFractionDigits: 0 })}</b>
            </div>
          )}
        </div>
      </Modal>

      {/* Fund Management Modal */}
      <Modal visible={fundModalOpen} title="资金管理" onCancel={() => setFundModalOpen(false)} onOk={handleFundUpdate} okText={fundAction === 'deposit' ? '确认入金' : '确认出金'}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {summary && (
            <div style={{ display: 'flex', gap: 16, marginBottom: 8 }}>
              <div style={{ flex: 1, padding: '10px 14px', background: 'var(--color-fill-2)', borderRadius: 8 }}>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>总资产</div>
                <div style={{ fontSize: 18, fontWeight: 700 }}>¥{summary.totalEquity.toLocaleString(undefined, {maximumFractionDigits: 0})}</div>
              </div>
              <div style={{ flex: 1, padding: '10px 14px', background: 'var(--color-fill-2)', borderRadius: 8 }}>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>可用余额</div>
                <div style={{ fontSize: 18, fontWeight: 700, color: 'var(--color-primary)' }}>¥{summary.availableCash.toLocaleString(undefined, {maximumFractionDigits: 0})}</div>
              </div>
            </div>
          )}
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 6 }}>操作类型</div>
            <div style={{ display: 'flex', gap: 8 }}>
              <Button type={fundAction === 'deposit' ? 'primary' : 'default'} onClick={() => setFundAction('deposit')}>入金</Button>
              <Button type={fundAction === 'withdraw' ? 'primary' : 'default'} onClick={() => setFundAction('withdraw')}>出金</Button>
            </div>
          </div>
          {accounts.length > 0 && (
            <div>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>选择账户</div>
              <Select
                value={fundAccountId || undefined} onChange={v => setFundAccountId(v as number)}
                style={{ width: '100%' }} placeholder="选择资金账户"
                options={filteredAccounts.map(a => ({ label: `${a.name} (${a.broker || '默认'})·余额 ¥${a.availableCash}`, value: a.id }))}
              />
            </div>
          )}
          <div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>金额 (元)</div>
            <InputNumber value={fundAmount} onChange={v => setFundAmount(v || 0)} min={1} step={1000} precision={2} style={{ width: '100%' }} />
          </div>
        </div>
      </Modal>
    </div>
  );
}
