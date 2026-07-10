import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Table, Button, Modal, Input, InputNumber, Tag, Select, Card, Message, Tooltip } from '@arco-design/web-react';
import { Briefcase, Plus, Edit, Trash2, Search, Wallet, TrendingUp, TrendingDown, DollarSign, BarChart3, Building2, RefreshCw, Archive, RotateCcw, Eye, Zap, Key } from 'lucide-react';
import { fetchHoldingsSummary, fetchHoldings, fetchAccountsOverview, fetchHoldingAccounts,
         createHolding, updateHolding, deleteHolding, updateAccount, searchStock, syncFromBroker,
         createLiveAccount, updateLiveAccount, deleteLiveAccount, restoreLiveAccount, fetchLiveAccounts,
         generateAgentToken, revokeAgentToken, testAgentConnection } from '../services/api';
import { showToast } from '../components/Toast';

interface Holding {
  id: number; accountId: number; stockCode: string; stockName: string;
  costPrice: number; quantity: number; totalCost: number; buyDate: string;
  curPrice: number; priceDate: string; prevClose: number;
  dailyChg: number; dailyChgPct: number; dailyPnl: number; dailyPnlPct: number;
  marketVal: number; pnl: number; pnlPct: number; holdDays: number;
  todayBuyQty: number; availSellQty: number; updatedAt: string;
}

interface Summary {
  totalEquity: number; availableCash: number; freeCash?: number; committedToRuns?: number; totalMarketValue: number;
  totalPnl: number; totalPnlPct: number; totalDailyPnl: number; positionCount: number; accountCount: number;
}

interface AccountOv {
  accountId: number; accountName: string; broker: string; accountType: string;
  initialCapital: number; availableCash: number; freeCash?: number; committedToRuns?: number; positionValue: number;
  totalEquity: number; totalPnl: number; totalPnlPct: number; dailyPnl: number; positionCount: number;
}

interface AccountInfo {
  id: number; name: string; broker: string; accountType: string;
  initialCapital: number; availableCash: number; totalAssets: number;
  totalMarketValue: number; totalProfit: number; nav: number;
  brokerMode: string; mxApiKey: string;
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
  const [acctType, setAcctType] = useState<string>('all'); // 'all' | 'real' | 'simulated'
  const [selectedAccId, setSelectedAccId] = useState<number | 'all'>('all');

  // Account management state
  const [acctOpen, setAcctOpen] = useState(false);
  const [acctEditOpen, setAcctEditOpen] = useState(false);
  const [editingAcct, setEditingAcct] = useState<any>(null);
  const [newAcct, setNewAcct] = useState({ name: '', broker: '', accountType: 'simulated' as string, accountNumber: '', initialCapital: 100000, mxApiKey: '', mxAccountId: '', brokerMode: 'manual' as string });
  const [archivedAccounts, setArchivedAccounts] = useState<any[]>([]);
  const [showArchived, setShowArchived] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<any>(null);
  const [deleteCheckMsg, setDeleteCheckMsg] = useState('');

  const [modalOpen, setModalOpen] = useState(false);
  const [fundModalOpen, setFundModalOpen] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState<{ id: number; code: string; name: string; managed: boolean; runs: { id: number; name: string; qty: number }[] } | null>(null);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [formCode, setFormCode] = useState('');
  const [formCost, setFormCost] = useState<number>(0);
  const [formQty, setFormQty] = useState<number>(0);
  const [formBuyDate, setFormBuyDate] = useState('');
  const [formName, setFormName] = useState('');
  const [formAccountId, setFormAccountId] = useState<number>(0);
  const [searchResults, setSearchResults] = useState<any[]>([]);
  const [searching, setSearching] = useState(false);

  const [fundAction, setFundAction] = useState<'deposit' | 'withdraw'>('deposit');
  const [fundAmount, setFundAmount] = useState<number>(0);
  const [fundAccountId, setFundAccountId] = useState<number>(0);
  const [syncingAll, setSyncingAll] = useState(false);
  const [syncingAcctId, setSyncingAcctId] = useState<number | null>(null);
  const [agentTesting, setAgentTesting] = useState<number | null>(null);
  const [agentTestMsg, setAgentTestMsg] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const results = await Promise.allSettled([
        fetchHoldingsSummary(acctType !== 'all' ? acctType : undefined),
        fetchAccountsOverview(acctType !== 'all' ? acctType : undefined),
        fetchHoldingAccounts(),
      ]);
      const [r, a, accts] = results;
      const summaryRes = r.status === 'fulfilled' ? r.value?.data : null;
      const overviewRes = a.status === 'fulfilled' ? a.value?.data : null;
      const acctsRes = accts.status === 'fulfilled' ? accts.value?.data : null;
      setSummary(summaryRes?.data || null);
      setAccountsOv(overviewRes?.data || []);
      setAccounts((acctsRes?.data || []).filter((x: any) => x.status === 'active'));
      const aid = selectedAccId !== 'all' ? Number(selectedAccId) : undefined;
      const { data: holdingsRes } = await fetchHoldings(aid, selectedAccId === 'all' ? (acctType !== 'all' ? acctType : undefined) : undefined);
      setData(holdingsRes.data || []);
    } catch (e) { console.error(e); }
    finally { setLoading(false); }
  }, [acctType, selectedAccId]);

  useEffect(() => { load(); }, [load]);

  const handleSearch = async (kw: string) => {
    if (!kw || kw.length < 1) { setSearchResults([]); return; }
    setSearching(true);
    try { const { data: r } = await searchStock(kw); setSearchResults((r.data || []).slice(0, 8)); } catch { setSearchResults([]); }
    finally { setSearching(false); }
  };

  const selectStock = (s: any) => { setFormCode(s.code); setFormName(s.name); setSearchResults([]); };

  const loadArchived = useCallback(async () => {
    try { const { data: a } = await fetchLiveAccounts('archived'); setArchivedAccounts(a?.data || []); }
    catch (_) {}
  }, []);

  // ─── Account Management ───
  const handleCreateAccount = async () => {
    if (!newAcct.name) { showToast('warning', '请输入账户名称'); return; }
    try {
      await createLiveAccount(newAcct);
      showToast('success', '账户已创建');
      setAcctOpen(false);
      setNewAcct({ name: '', broker: '', accountType: 'simulated', accountNumber: '', initialCapital: 100000, mxApiKey: '', mxAccountId: '', brokerMode: 'manual' });
      load();
    } catch (e: any) { showToast('error', e?.response?.data?.message || '创建失败'); }
  };

  const handleEditAccount = (acct: AccountInfo) => {
    setEditingAcct({ ...acct });
    setAcctEditOpen(true);
  };

  const handleUpdateAccount = async () => {
    if (!editingAcct?.id) return;
    try {
      await updateLiveAccount(editingAcct.id, editingAcct);
      showToast('success', '账户已更新');
      setAcctEditOpen(false); setEditingAcct(null);
      load();
    } catch (e: any) { showToast('error', e?.response?.data?.message || '更新失败'); }
  };

  const handleDeleteClick = (acct: AccountInfo) => {
    setDeleteTarget(acct);
    setDeleteCheckMsg('');
  };

  const handleDeleteAccount = async () => {
    if (!deleteTarget) return;
    try {
      await deleteLiveAccount(deleteTarget.id);
      showToast('success', '账户已归档');
      setDeleteTarget(null);
      load();
    } catch (e: any) { showToast('error', e?.response?.data?.message || '归档失败'); }
  };

  const handleRestoreAccount = async (id: number) => {
    try {
      await restoreLiveAccount(id);
      showToast('success', '账户已恢复');
      load(); loadArchived();
    } catch (e: any) { showToast('error', e?.response?.data?.message || '恢复失败'); }
  };

  const handleSyncAccount = async (acct: AccountInfo) => {
    setSyncingAcctId(acct.id);
    try {
      await syncFromBroker(acct.id);
      showToast('success', `${acct.name} 同步成功`);
      load();
    } catch (e: any) { showToast('error', '同步失败: ' + (e?.response?.data?.message || '未知错误')); }
    finally { setSyncingAcctId(null); }
  };

  // ─── Agent Token ───
  const handleGenerateToken = async (accountId: number) => {
    try {
      const { data } = await generateAgentToken(accountId);
      if (data?.data?.agentToken) {
        setEditingAcct({ ...editingAcct, agentToken: data.data.agentToken });
        showToast('success', 'Agent Token 已生成');
      }
      load();
    } catch (_) {}
  };

  const handleRevokeToken = async (accountId: number) => {
    try {
      await revokeAgentToken(accountId);
      setEditingAcct({ ...editingAcct, agentToken: '' });
      showToast('success', 'Agent Token 已吊销');
      load();
    } catch (_) {}
  };

  const handleTestConnection = async (accountId: number, token: string) => {
    setAgentTesting(accountId);
    setAgentTestMsg('');
    try {
      const { data } = await testAgentConnection(accountId, token);
      if (data?.data?.passed) {
        setAgentTestMsg('✅ 连接测试通过 — agent 响应正常');
        showToast('success', '连接测试通过！');
      } else {
        setAgentTestMsg('❌ ' + (data?.data?.message || '测试失败'));
        showToast('warning', data?.data?.message || '测试失败，请确认 agent 已启动');
      }
    } catch (e: any) {
      const msg = e?.response?.data?.message || '测试失败，请确认 agent 已启动';
      setAgentTestMsg('❌ ' + msg);
      showToast('error', msg);
    } finally {
      setAgentTesting(null);
    }
  };

  const handleSyncAll = async () => {
    setSyncingAll(true);
    try {
      const syncable = filteredAccounts.filter(a => a.brokerMode === 'mx_moni' || a.mxApiKey);
      if (syncable.length === 0) { showToast('info', '没有可同步的账户（仅妙想/有API Key的账户支持同步）'); setSyncingAll(false); return; }
      for (const a of syncable) {
        try { await syncFromBroker(a.id); } catch {}
      }
      showToast('success', `已同步 ${syncable.length} 个账户`);
      load();
    } catch { showToast('error', '同步失败'); }
    finally { setSyncingAll(false); }
  };

  const openCreate = () => {
    setEditingId(null); setFormCode(''); setFormName(''); setFormCost(0); setFormQty(0); setFormBuyDate(''); setSearchResults([]);
    setFormAccountId(filteredAccounts[0]?.id || 0);
    setModalOpen(true);
  };
  const openEdit = (r: Holding) => {
    setEditingId(r.id); setFormCode(r.stockCode); setFormName(r.stockName);
    setFormCost(r.costPrice); setFormQty(r.quantity); setFormBuyDate(r.buyDate);
    setFormAccountId(r.accountId);
    setModalOpen(true);
  };

  const handleSave = async () => {
    if (!formCode || formQty <= 0) { Message.warning('请填写股票代码和数量'); return; }
    try {
      const body = {
        accountId: formAccountId, stockCode: formCode, stockName: formName,
        costPrice: formCost, quantity: formQty, buyDate: formBuyDate,
      };
      if (editingId) { await updateHolding(editingId, body); } else { await createHolding(body); }
      Message.success(editingId ? '修改成功' : '买入成功');
      setModalOpen(false); load();
    } catch (e: any) { Message.error(e?.response?.data?.message || '操作失败'); }
  };

  const handleDelete = async (id: number) => {
    const h = data.find((x: any) => x.id === id);
    if (!h) return;
    setDeleteConfirm({ id, code: h.stockCode || '', name: h.stockName || '', managed: false, runs: [] });
    try {
      const { fetchLiveRuns, fetchLivePositions } = await import('../services/api');
      const runsRes = await fetchLiveRuns();
      const runs = runsRes?.data?.data || [];
      const activeRuns = runs.filter((r: any) => r.status === 'active' || r.status === 'paused');
      const managed: { id: number; name: string; qty: number }[] = [];
      if (activeRuns.length > 0) {
        for (const run of activeRuns) {
          try {
            const posRes = await fetchLivePositions(run.id);
            const positions = posRes?.data?.data || [];
            const matched = positions.find((p: any) => p.stockCode === h.stockCode && p.quantity > 0);
            if (matched) managed.push({ id: run.id, name: run.name, qty: matched.quantity });
          } catch {}
        }
      }
      if (managed.length > 0) setDeleteConfirm(prev => prev ? { ...prev, managed: true, runs: managed } : null);
    } catch {}
  };

  const confirmDelete = async () => {
    if (!deleteConfirm) return;
    try { await deleteHolding(deleteConfirm.id); Message.success('已卖出'); setDeleteConfirm(null); load(); }
    catch (e: any) { Message.error(e?.response?.data?.message || '卖出失败'); }
  };

  const handleFundUpdate = async () => {
    if (fundAmount <= 0) return;
    try {
      await updateAccount(fundAction, fundAmount, fundAccountId || undefined);
      Message.success(fundAction === 'deposit' ? '入金成功' : '出金成功');
      setFundModalOpen(false); load();
    } catch {}
  };

  const filteredAccounts = accounts.filter(a => acctType === 'all' || a.accountType === acctType);
  const allAccountsOv = acctType === 'all'
    ? { totalEquity: summary?.totalEquity || 0, availableCash: summary?.availableCash || 0,
        totalPnl: summary?.totalPnl || 0, totalPnlPct: summary?.totalPnlPct || 0,
        totalDailyPnl: summary?.totalDailyPnl || 0, positionCount: summary?.positionCount || 0,
        totalMarketValue: summary?.totalMarketValue || 0, accountCount: summary?.accountCount || 0 }
    : null;

  return (
    <div style={{ padding: 20, maxWidth: 1440, margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <h2 style={{ margin: 0, fontSize: 20, fontWeight: 700, display: 'flex', alignItems: 'center', gap: 8 }}>
          <Building2 size={22} style={{ color: '#165DFF' }} />资金账户
        </h2>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', borderRadius: 6, overflow: 'hidden', border: '1px solid var(--color-border-2)' }}>
            {[{ key: 'all', label: '全部' }, { key: 'real', label: '真实' }, { key: 'simulated', label: '模拟' }].map(t => (
              <Button key={t.key} size="small" type={acctType === t.key ? 'primary' : 'default'}
                onClick={() => { setAcctType(t.key); setSelectedAccId('all'); }}
                style={{ borderRadius: 0, border: 'none' }}>{t.label}</Button>
            ))}
          </div>
          <Button size="small" icon={<RefreshCw size={14} />} loading={syncingAll} onClick={handleSyncAll}>同步券商</Button>
          <Button size="small" icon={<Plus size={14} />} onClick={() => setAcctOpen(true)}>添加账户</Button>
          <Button size="small" icon={<Wallet size={14} />} onClick={() => { setFundAccountId(filteredAccounts[0]?.id || 0); setFundModalOpen(true); }}>资金管理</Button>
          <Button size="small" type="primary" icon={<Plus size={14} />} onClick={openCreate}>买入股票</Button>
        </div>
      </div>

      {/* Account Cards Row */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))', gap: 12, marginBottom: 16 }}>
        {/* Total summary card (only when "全部") */}
        {allAccountsOv && summary && (
          <Card key="total"
            style={{ borderRadius: 10, border: selectedAccId === 'all' ? '2px solid #165DFF' : '1px solid var(--color-border-2)', cursor: 'pointer', background: selectedAccId === 'all' ? '#F0F5FF' : 'var(--color-bg-2)' }}
            bodyStyle={{ padding: '14px 16px' }}
            onClick={() => setSelectedAccId('all')}
          >
            <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8, color: '#165DFF' }}>
              全部账户 · {summary.accountCount}个
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '4px 8px' }}>
              <div><div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>总资产</div>
                <div style={{ fontSize: 14, fontWeight: 700 }}>¥{summary.totalEquity.toLocaleString(undefined, {maximumFractionDigits: 0})}</div></div>
              <div><div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>券商余额</div>
                <div style={{ fontSize: 14, fontWeight: 700 }}>¥{(summary.availableCash || 0).toLocaleString(undefined, {maximumFractionDigits: 0})}</div></div>
              <div><div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>可分配</div>
                <div style={{ fontSize: 14, fontWeight: 700, color: (summary.committedToRuns || 0) > 0 && (summary.freeCash || 0) < 0 ? '#F53F3F' : '#0FC6C2' }}>¥{(summary.freeCash ?? 0).toLocaleString(undefined, {maximumFractionDigits: 0})}</div></div>
              <div><div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>持仓市值</div>
                <div style={{ fontSize: 14, fontWeight: 700, color: '#722ED1' }}>¥{summary.totalMarketValue.toLocaleString(undefined, {maximumFractionDigits: 0})}</div></div>
              <div><div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>累计盈亏</div>
                <div style={{ fontSize: 14, fontWeight: 700, color: pnlColor(summary.totalPnl), fontFamily: "'SF Mono', monospace" }}>
                  {pnlSign(summary.totalPnl)}¥{Math.abs(summary.totalPnl).toLocaleString(undefined, {maximumFractionDigits: 0})}</div></div>
            </div>
          </Card>
        )}

        {/* Individual account cards */}
        {filteredAccounts.map(a => {
          const ov = accountsOv.find(o => o.accountId === a.id);
          const isSelected = selectedAccId === a.id;
          const canSync = a.brokerMode === 'mx_moni' || a.mxApiKey;
          return (
            <Card key={a.id}
              style={{ borderRadius: 10, border: isSelected ? '2px solid #165DFF' : '1px solid var(--color-border-2)', cursor: 'pointer', background: isSelected ? '#F0F5FF' : 'var(--color-bg-2)', position: 'relative' }}
              bodyStyle={{ padding: '14px 16px' }}
              onClick={() => setSelectedAccId(a.id)}
            >
              <div style={{ position: 'absolute', top: 4, right: 4, display: 'flex', gap: 2 }}>
                {canSync && (
                  <Tooltip content="从券商同步">
                    <span style={{ cursor: 'pointer', opacity: 0.5, padding: 2 }} onClick={(e) => { e.stopPropagation(); handleSyncAccount(a); }}>
                      <RefreshCw size={12} className={syncingAcctId === a.id ? 'spin-icon' : ''} style={syncingAcctId === a.id ? { animation: 'spin 1s linear infinite' } : {}} />
                    </span>
                  </Tooltip>
                )}
                <Tooltip content="编辑账户">
                  <span style={{ cursor: 'pointer', opacity: 0.4, padding: 2 }} onClick={(e) => { e.stopPropagation(); handleEditAccount(a); }}>
                    <Edit size={12} />
                  </span>
                </Tooltip>
                <Tooltip content="归档账户">
                  <span style={{ cursor: 'pointer', opacity: 0.4, padding: 2 }} onClick={(e) => { e.stopPropagation(); handleDeleteClick(a); }}>
                    <Trash2 size={12} />
                  </span>
                </Tooltip>
              </div>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 1, cursor: 'pointer', paddingRight: 60 }} onClick={(e) => { e.stopPropagation(); navigate(`/account/${a.id}`); }}>
                {a.name}
                {a.accountType === 'real' ? <Tag size="small" color="red" style={{ marginLeft: 6, fontSize: 10 }}>实盘</Tag>
                  : <Tag size="small" color="arcoblue" style={{ marginLeft: 6, fontSize: 10 }}>模拟</Tag>}
                {a.brokerMode === 'mx_moni' && <Tag size="small" color="green" style={{ marginLeft: 4, fontSize: 10 }}>妙想</Tag>}
                {a.brokerMode === 'lobster' && <Tag size="small" color="purple" style={{ marginLeft: 4, fontSize: 10 }}>龙虾</Tag>}
              </div>
              <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 8 }}>
                {a.broker || '默认'} · {a.accountNumber || ''} · ¥{(a.initialCapital || 0).toLocaleString()}初始
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '4px 8px' }}>
                <div><div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>总资产</div>
                  <div style={{ fontSize: 14, fontWeight: 700 }}>¥{(a.totalAssets || a.initialCapital || 0).toLocaleString()}</div></div>
                <div><div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>券商余额</div>
                  <div style={{ fontSize: 14, fontWeight: 700 }}>¥{(a.availableCash || 0).toLocaleString()}</div></div>
                <div><div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>可分配</div>
                  <div style={{ fontSize: 14, fontWeight: 700, color: (ov?.freeCash ?? 0) >= 0 ? '#0FC6C2' : '#F53F3F' }}>¥{(ov?.freeCash ?? a.availableCash ?? 0).toLocaleString()}</div></div>
                <div><div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>持仓市值</div>
                  <div style={{ fontSize: 14, fontWeight: 700, color: '#722ED1' }}>¥{(a.totalMarketValue || 0).toLocaleString()}</div></div>
                <div><div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>日盈亏</div>
                  <div style={{ fontSize: 14, fontWeight: 700, color: pnlColor(ov?.dailyPnl || 0), fontFamily: "'SF Mono', monospace" }}>{pnlSign(ov?.dailyPnl || 0)}¥{Math.abs(ov?.dailyPnl || 0).toLocaleString()}</div></div>
                <div><div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>净值</div>
                  <div style={{ fontSize: 14, fontWeight: 700, color: (a.nav || 1) >= 1 ? '#F53F3F' : '#00B42A' }}>{(a.nav || 1).toFixed(3)}</div></div>
              </div>
              {ov && (
                <div style={{ marginTop: 6, paddingTop: 6, borderTop: '1px solid var(--color-border-1)', fontSize: 11, color: 'var(--color-text-3)', display: 'flex', justifyContent: 'space-between' }}>
                  <span>{ov.positionCount}只持仓</span>
                  <span style={{ color: pnlColor(ov.totalPnl), fontWeight: 600 }}>
                    {ov.totalPnl >= 0 ? '+' : ''}¥{(ov.totalPnl || 0).toLocaleString()}
                  </span>
                </div>
              )}
            </Card>
          );
        })}
        {filteredAccounts.length === 0 && (
          <Card style={{ borderRadius: 10, borderStyle: 'dashed', display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: 80, gridColumn: '1 / -1' }}>
            <span style={{ color: 'var(--color-text-3)', fontSize: 13 }}>暂无账户</span>
          </Card>
        )}
      </div>

      {/* Holdings Table */}
      <Card style={{ borderRadius: 10 }} bodyStyle={{ padding: '0 16px 16px' }}
        title={
          <span style={{ fontSize: 14, fontWeight: 600 }}>
            {selectedAccId === 'all'
              ? `全部持仓 · ${data.length}只`
              : `${filteredAccounts.find(a => a.id === selectedAccId)?.name || ''} · ${data.length}只`}
          </span>
        }
      >
        <Table
          data={data}
          loading={loading}
          rowKey="id"
          size="small"
          scroll={{ x: 'max-content' }}
          pagination={{ pageSize: 30, sizeCanChange: false }}
          columns={[
            { title: '代码', dataIndex: 'stockCode', width: 80, render: (v: string, r: Holding) => (
              <a onClick={() => navigate(`/stock/${v}`)} style={{ cursor: 'pointer', color: '#165DFF', fontFamily: 'monospace' }}>{v}</a>
            )},
            { title: '名称', dataIndex: 'stockName', width: 90, ellipsis: true },
            { title: '持仓/可用', dataIndex: 'quantity', width: 100, render: (v: number, r: Holding) => (
              <span>{v.toLocaleString()} / <span style={{ color: 'var(--color-text-3)' }}>{r.availSellQty || 0}</span></span>
            )},
            { title: '成本价', dataIndex: 'costPrice', width: 80, render: (v: number) => v?.toFixed(3) },
            { title: '现价', dataIndex: 'curPrice', width: 80, render: (v: number) => v?.toFixed(3) },
            { title: '市值', dataIndex: 'marketVal', width: 100, render: (v: number) => `¥${v?.toFixed(2)}` },
            { title: '盈亏', dataIndex: 'pnl', width: 110, render: (v: number, r: Holding) => (
              <span style={{ color: pnlColor(v), fontWeight: 600, fontFamily: "'SF Mono', monospace" }}>
                {pnlSign(v)}¥{Math.abs(v || 0).toFixed(2)}<br />
                <span style={{ fontSize: 10 }}>{r.pnlPct?.toFixed(2)}%</span>
              </span>
            )},
            { title: '日收益', dataIndex: 'dailyPnl', width: 90, render: (v: number) => (
              <span style={{ color: pnlColor(v), fontFamily: 'monospace' }}>{pnlSign(v)}¥{Math.abs(v || 0).toFixed(2)}</span>
            )},
            { title: '操作', width: 70, fixed: 'right' as const, render: (_: any, r: Holding) => (
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
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>股票搜索</div>
            <Input value={formCode} onChange={v => { setFormCode(v); handleSearch(v); }} placeholder="输入代码搜索" style={{ width: '100%' }} />
            {searchResults.length > 0 && (
              <div style={{ marginTop: 4, maxHeight: 160, overflow: 'auto', border: '1px solid var(--color-border-1)', borderRadius: 6 }}>
                {searchResults.map((s: any) => (
                  <div key={s.code} onClick={() => selectStock(s)} style={{ padding: '6px 10px', cursor: 'pointer', fontSize: 12 }}>
                    {s.code} — {s.name}
                  </div>
                ))}
              </div>
            )}
          </div>
          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}><div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>成本价</div>
              <InputNumber value={formCost} onChange={v => setFormCost(v as number)} min={0} style={{ width: '100%' }} /></div>
            <div style={{ flex: 1 }}><div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>数量(股)</div>
              <InputNumber value={formQty} onChange={v => setFormQty(v as number)} min={100} step={100} style={{ width: '100%' }} /></div>
          </div>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>买入日期</div>
            <Input value={formBuyDate} onChange={v => setFormBuyDate(v)} placeholder="YYYY-MM-DD" />
          </div>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>账户</div>
            <Select value={formAccountId} onChange={v => setFormAccountId(v as number)}
              options={filteredAccounts.map(a => ({ label: `${a.name} (${a.broker || '默认'})·余额 ¥${a.availableCash ?? 0}`, value: a.id }))} style={{ width: '100%' }} />
          </div>
        </div>
      </Modal>

      {/* Delete Confirmation Modal */}
      <Modal visible={!!deleteConfirm} title="确认卖出" onCancel={() => setDeleteConfirm(null)} onOk={confirmDelete} okText="确认卖出" okButtonProps={{ status: 'danger' }}>
        <div style={{ lineHeight: 1.8 }}>
          <p>确认卖出 <b>{deleteConfirm?.name} ({deleteConfirm?.code})</b>？</p>
          {deleteConfirm?.managed && (
            <div style={{ marginTop: 12, padding: '10px 12px', background: '#FFF7E6', borderRadius: 8, border: '1px solid #FFD591' }}>
              <div style={{ fontSize: 13, fontWeight: 600, color: '#D46B08', marginBottom: 6 }}>⚠️ 该持仓已被以下策略管理</div>
              {deleteConfirm.runs.map((r, i) => (
                <div key={i} style={{ fontSize: 12, color: '#AD6800' }}>· 策略「{r.name}」持有 {r.qty} 股 — 卖出后将自动从该策略扣除</div>
              ))}
              <div style={{ fontSize: 11, color: '#AD6800', marginTop: 4 }}>系统会自动同步策略持仓和可用现金。</div>
            </div>
          )}
        </div>
      </Modal>

      {/* Archived Accounts Toggle */}
      <div style={{ marginTop: 0, marginBottom: 12 }}>
        <Button
          size="small"
          type="text"
          icon={showArchived ? <Eye size={12} /> : <Archive size={12} />}
          onClick={() => {
            if (!showArchived) loadArchived();
            setShowArchived(!showArchived);
          }}
        >
          {showArchived ? '隐藏已归档' : '查看已归档账户'}
        </Button>
      </div>

      {showArchived && (
        <div style={{ marginBottom: 16 }}>
          {archivedAccounts.length === 0 ? (
            <div style={{ fontSize: 12, color: 'var(--color-text-4)', padding: '12px 0' }}>暂无已归档账户</div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))', gap: 12 }}>
              {archivedAccounts.map(a => (
                <Card key={a.id} style={{ borderRadius: 10, opacity: 0.7 }} bodyStyle={{ padding: '14px 16px' }}>
                  <div style={{ position: 'absolute', top: 6, right: 6 }}>
                    <span style={{ cursor: 'pointer', opacity: 0.5, fontSize: 11, display: 'flex', alignItems: 'center', gap: 2 }}
                      onClick={() => handleRestoreAccount(a.id)}>
                      <RotateCcw size={12} /> 恢复
                    </span>
                  </div>
                  <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 1 }}>
                    {a.name}
                    {a.accountType === 'real' ? <Tag size="small" color="red" style={{ marginLeft: 6, fontSize: 10 }}>实盘</Tag>
                      : <Tag size="small" color="arcoblue" style={{ marginLeft: 6, fontSize: 10 }}>模拟</Tag>}
                  </div>
                  <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 6 }}>
                    {a.broker || '默认'} · ¥{(a.initialCapital || 0).toLocaleString()}初始
                  </div>
                  <div style={{ fontSize: 13 }}>
                    <span style={{ color: 'var(--color-text-2)' }}>总资产: </span>
                    <span style={{ fontWeight: 600 }}>¥{((a.totalAssets || 0)).toLocaleString()}</span>
                  </div>
                </Card>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Create Account Modal */}
      <Modal visible={acctOpen} title="创建交易账户" onCancel={() => setAcctOpen(false)} onOk={handleCreateAccount} okText="创建" width={480}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>账户名称 *</div>
              <Input value={newAcct.name} onChange={v => setNewAcct({ ...newAcct, name: v })} placeholder="如：东方财富实盘01" />
            </div>
            <div style={{ flex: 1 }}>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>账户类型</div>
              <Select value={newAcct.accountType} onChange={v => setNewAcct({ ...newAcct, accountType: v })} style={{ width: '100%' }}
                options={[{ label: '模拟盘', value: 'simulated' }, { label: '实盘', value: 'real' }]} />
            </div>
          </div>
          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>券商</div>
              <Input value={newAcct.broker} onChange={v => setNewAcct({ ...newAcct, broker: v })} placeholder="如：东方财富" />
            </div>
            <div style={{ flex: 1 }}>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>账号/编号</div>
              <Input value={newAcct.accountNumber} onChange={v => setNewAcct({ ...newAcct, accountNumber: v })} placeholder="可选" />
            </div>
          </div>
          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>初始资金</div>
              <InputNumber value={newAcct.initialCapital} onChange={v => setNewAcct({ ...newAcct, initialCapital: v as number })} min={10000} style={{ width: '100%' }} />
            </div>
            <div style={{ flex: 1 }}>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>接入模式</div>
              <Select value={newAcct.brokerMode || 'manual'} onChange={v => setNewAcct({ ...newAcct, brokerMode: v })} style={{ width: '100%' }}
                options={[{ label: '手动', value: 'manual' }, { label: '妙想模拟', value: 'mx_moni' }, { label: '龙虾交易', value: 'lobster' }]} />
            </div>
          </div>
          {newAcct.brokerMode === 'mx_moni' && (<>
            <div>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>妙想 API Key</div>
              <Input.Password value={newAcct.mxApiKey || ''} onChange={v => setNewAcct({ ...newAcct, mxApiKey: v })} placeholder="留空则用环境变量" />
            </div>
            <div>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>妙想 Account ID</div>
              <Input value={newAcct.mxAccountId || ''} onChange={v => setNewAcct({ ...newAcct, mxAccountId: v })} />
            </div>
          </>)}
        </div>
      </Modal>

      {/* Edit Account Modal */}
      <Modal visible={acctEditOpen} title="编辑账户" onCancel={() => { setAcctEditOpen(false); setEditingAcct(null); }} onOk={handleUpdateAccount} okText="保存" width={520}>
        {editingAcct && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{ display: 'flex', gap: 12 }}>
              <div style={{ flex: 1 }}>
                <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>账户名称</div>
                <Input value={editingAcct.name} onChange={v => setEditingAcct({ ...editingAcct, name: v })} />
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>券商</div>
                <Input value={editingAcct.broker} onChange={v => setEditingAcct({ ...editingAcct, broker: v })} />
              </div>
            </div>
            <div style={{ display: 'flex', gap: 12 }}>
              <div style={{ flex: 1 }}>
                <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>账户类型</div>
                <Select value={editingAcct.accountType} onChange={v => setEditingAcct({ ...editingAcct, accountType: v })} style={{ width: '100%' }}
                  options={[{ label: '模拟盘', value: 'simulated' }, { label: '实盘', value: 'real' }]} />
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>账号/编号</div>
                <Input value={editingAcct.accountNumber} onChange={v => setEditingAcct({ ...editingAcct, accountNumber: v })} />
              </div>
            </div>
            <div style={{ display: 'flex', gap: 12 }}>
              <div style={{ flex: 1 }}>
                <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>初始资金</div>
                <InputNumber value={editingAcct.initialCapital} onChange={v => setEditingAcct({ ...editingAcct, initialCapital: v as number })} min={10000} style={{ width: '100%' }} />
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>可用资金</div>
                <InputNumber value={editingAcct.availableCash} onChange={v => setEditingAcct({ ...editingAcct, availableCash: v as number })} min={0} style={{ width: '100%' }} />
              </div>
            </div>
            <div>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>接入模式</div>
              <Select value={editingAcct.brokerMode || 'manual'} onChange={v => setEditingAcct({ ...editingAcct, brokerMode: v })} style={{ width: '100%' }}
                options={[{ label: '手动', value: 'manual' }, { label: '妙想模拟', value: 'mx_moni' }, { label: '龙虾交易', value: 'lobster' }]} />
            </div>
            {editingAcct.brokerMode === 'lobster' && (
              <div>
                <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
                  <Zap size={14} /> Agent Token
                  {editingAcct.agentToken && <Tag size="small" color="green" style={{ fontSize: 10 }}>已生成</Tag>}
                </div>
                {editingAcct.agentToken ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                    <Input.Password value={editingAcct.agentToken} readOnly style={{ fontFamily: 'monospace', fontSize: 12 }} />
                    <div style={{ display: 'flex', gap: 8 }}>
                      <Button size="small" icon={<Zap size={12} />} loading={agentTesting === editingAcct.id}
                        onClick={() => handleTestConnection(editingAcct.id, editingAcct.agentToken || '')}>测试连接</Button>
                      <Button size="small" status="danger" onClick={() => handleRevokeToken(editingAcct.id)}>吊销 Token</Button>
                      <Button size="small" onClick={() => handleGenerateToken(editingAcct.id)}>重新生成</Button>
                    </div>
                    {agentTestMsg && <div style={{ fontSize: 12, color: 'var(--color-text-2)', padding: '4px 8px', background: 'var(--color-fill-1)', borderRadius: 4 }}>{agentTestMsg}</div>}
                  </div>
                ) : (
                  <Button size="small" icon={<Key size={12} />} onClick={() => handleGenerateToken(editingAcct.id)}>生成 Agent Token</Button>
                )}
              </div>
            )}
            {(editingAcct.brokerMode !== 'lobster' && editingAcct.brokerMode !== 'manual') && (<>
              <div>
                <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>API Key</div>
                <Input.Password value={editingAcct.mxApiKey || ''} onChange={v => setEditingAcct({ ...editingAcct, mxApiKey: v })} placeholder="留空则用环境变量" />
              </div>
              <div>
                <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>Account ID</div>
                <Input value={editingAcct.mxAccountId || ''} onChange={v => setEditingAcct({ ...editingAcct, mxAccountId: v })} />
              </div>
            </>)}
          </div>
        )}
      </Modal>

      {/* Archive Confirmation Modal */}
      <Modal visible={!!deleteTarget} title="归档账户" onCancel={() => setDeleteTarget(null)} onOk={handleDeleteAccount} okText="确认归档" okButtonProps={{ status: 'warning' }}>
        <div style={{ lineHeight: 1.8 }}>
          <p>确认归档账户 <b>{deleteTarget?.name}</b>？</p>
          <p style={{ fontSize: 12, color: 'var(--color-text-3)' }}>归档后该账户将不再显示在活跃列表中，但数据不会丢失，可随时恢复。</p>
          {deleteCheckMsg && <p style={{ fontSize: 12, color: '#F53F3F', marginTop: 8 }}>{deleteCheckMsg}</p>}
        </div>
      </Modal>

      {/* Fund Management Modal */}
      <Modal visible={fundModalOpen} title="资金管理" onCancel={() => setFundModalOpen(false)} onOk={handleFundUpdate} okText={fundAction === 'deposit' ? '确认入金' : '确认出金'}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div style={{ display: 'flex', gap: 8 }}>
            <Button type={fundAction === 'deposit' ? 'primary' : 'default'} onClick={() => setFundAction('deposit')}>入金</Button>
            <Button type={fundAction === 'withdraw' ? 'primary' : 'default'} onClick={() => setFundAction('withdraw')}>出金</Button>
          </div>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>账户</div>
            <Select value={fundAccountId} onChange={v => setFundAccountId(v as number)}
              options={filteredAccounts.map(a => ({ label: `${a.name} (${a.broker || '默认'})·余额 ¥${a.availableCash ?? 0}`, value: a.id }))} style={{ width: '100%' }} />
          </div>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>金额</div>
            <InputNumber value={fundAmount} onChange={v => setFundAmount(v as number)} min={1} style={{ width: '100%' }} placeholder="输入金额" />
          </div>
        </div>
      </Modal>
    </div>
  );
}
