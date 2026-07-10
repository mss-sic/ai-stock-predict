import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, Button, Table, Tag, Modal, InputNumber, Select, Input, Grid, Switch, TimePicker, Divider } from '@arco-design/web-react';
import { showToast } from '../components/Toast';
import { Play, Plus, Pause, Square, Wallet, TrendingUp, BarChart3, Target, RefreshCw, Building2, Trash2, Bell, Settings, Edit, Coins, DollarSign, Cpu, Key, Radio, Zap, Eye, Archive, RotateCcw, Activity } from 'lucide-react';
import {
  fetchLiveRuns, createLiveRun, updateLiveRunStatus, updateLiveRunConfig,
  fetchLiveAccounts, createLiveAccount, updateLiveAccount, deleteLiveAccount, restoreLiveAccount, fetchLiveAccount, fetchAccountDetail, syncFromBroker,
  generateAgentToken, revokeAgentToken, testAgentConnection, getAgentStatus,
  fetchStrategies,
  fetchNotificationConfigs, createNotificationConfig, deleteNotificationConfig, testNotification,
} from '../services/api';

interface StrategyRun {
  id: number; strategyId: number; name: string;
  status: string; startDate: string;
  initialCapital: number; availableCash?: number; positionValue?: number; currentEquity: number;
  totalReturn: number; maxDrawdown: number;
  winRate: number; tradeCount: number; lastRunDate: string;
  lastError?: string;
  autoDailyCron?: string; autoTradeExecCron?: string;
  notifyEnabled?: boolean; notifyChannels?: string;
}

interface Account {
  id: number; name: string; broker: string;
  accountType: string; accountNumber: string;
  initialCapital: number; availableCash: number;
  totalAssets?: number; totalMarketValue?: number;
  totalProfit?: number; nav?: number; frozenCash?: number;
  brokerMode?: string; mxApiKey?: string; mxAccountId?: string; agentToken?: string;
}

interface Strategy {
  id: number; name: string;
}

export default function LiveTradingPage() {
  const navigate = useNavigate();
  const [runs, setRuns] = useState<StrategyRun[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [strategies, setStrategies] = useState<Strategy[]>([]);
  const [loading, setLoading] = useState(false);

  // Create run modal
  const [createOpen, setCreateOpen] = useState(false);
  const [newRun, setNewRun] = useState<{ strategyId: number; accountId: number; name: string; initialCapital: number; startDate: string; importPositions: boolean; notifyEnabled: boolean; notifyConfigs: { channel: string; name: string; webhookUrl: string }[] }>({ strategyId: 0, accountId: 0, name: '', initialCapital: 100000, startDate: '', importPositions: false, notifyEnabled: false, notifyConfigs: [] });
  const [selectedAccountFreeCash, setSelectedAccountFreeCash] = useState(0);

  // Create account modal
  const [acctOpen, setAcctOpen] = useState(false);
  const [acctEditOpen, setAcctEditOpen] = useState(false);
  // Agent auto-trading state
  const [agentTesting, setAgentTesting] = useState<number | null>(null); // account ID under test
  const [agentTestMsg, setAgentTestMsg] = useState<string>('');
  const [editingAcct, setEditingAcct] = useState<any>(null);
  const [configOpen, setConfigOpen] = useState(false);
  const [configRun, setConfigRun] = useState<{ id: number; autoDailyCron: string; autoTradeExecCron: string; aiReviewEnabled: boolean; notifyEnabled: boolean; notifyChannels: string }>({ id: 0, autoDailyCron: '', autoTradeExecCron: '', aiReviewEnabled: false, notifyEnabled: false, notifyChannels: '[]' });
  const [removedNotifyIds, setRemovedNotifyIds] = useState<number[]>([]);
  const [configNotifyChannels, setConfigNotifyChannels] = useState<{ id?: number; channel: string; name: string; webhookUrl: string }[]>([]);
  const [configNewNotify, setConfigNewNotify] = useState({ channel: 'dingtalk_bot', webhookUrl: '' });
  const [newAcct, setNewAcct] = useState({ name: '', broker: '', accountType: 'simulated', accountNumber: '', initialCapital: 100000, mxApiKey: '', mxAccountId: '', brokerMode: 'manual' as string });
  const [archivedAccounts, setArchivedAccounts] = useState<any[]>([]);
  const [showArchived, setShowArchived] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<any>(null);
  const [deleteCheckMsg, setDeleteCheckMsg] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try { const { data: r } = await fetchLiveRuns(); setRuns(r?.data || []); } catch (e) { console.error('load runs', e); }
    try { const { data: a } = await fetchLiveAccount(); setAccounts(a?.data || []); } catch (e) { console.error('load accounts', e); }
    try { const { data: s } = await fetchStrategies(); setStrategies(s?.data || []); } catch (e) { console.error('load strategies', e); }
    setLoading(false);
  }, []);

  const loadArchived = useCallback(async () => {
    try { const { data: a } = await fetchLiveAccounts('archived'); setArchivedAccounts(a?.data || []); } catch (e) { console.error('load archived', e); }
  }, []);

  useEffect(() => { load(); }, [load]);

  const [newNotify, setNewNotify] = useState({ channel: 'dingtalk_bot', webhookUrl: '' });

  const addNotifyChannel = () => {
    if (!newNotify.webhookUrl) { showToast('warning', '请输入 Webhook 地址'); return; }
    const name = newNotify.channel === 'dingtalk_bot' ? '钉钉通知' : newNotify.channel === 'feishu_bot' ? '飞书通知' : '企业微信通知';
    setNewRun(prev => ({
      ...prev,
      notifyConfigs: [...prev.notifyConfigs, { channel: newNotify.channel, name, webhookUrl: newNotify.webhookUrl }],
    }));
    setNewNotify({ channel: 'dingtalk_bot', webhookUrl: '' });
  };

  const removeNotifyChannel = (idx: number) => {
    setNewRun(prev => ({
      ...prev,
      notifyConfigs: prev.notifyConfigs.filter((_, i) => i !== idx),
    }));
  };

  const handleCreateRun = async () => {
    if (!newRun.strategyId) { showToast('warning', '请选择策略'); return; }
    try {
      await createLiveRun(newRun);
      showToast('success', '实盘运行已创建');
      setCreateOpen(false);
      setNewRun({ strategyId: 0, accountId: 0, name: '', initialCapital: 100000, startDate: '', importPositions: false, notifyEnabled: false, notifyConfigs: [] });
      load();
    } catch (e: any) { /* toast handled by api interceptor */ }
  };

  const handleConfigSave = async () => {
    try {
      // 1. Handle notification config changes
      const origChannelIds: number[] = (() => { try { return JSON.parse(configRun.notifyChannels || "[]"); } catch { return []; } })();
      const keepIds = configNotifyChannels.filter(c => c.id).map(c => c.id as number);
      const toDelete = removedNotifyIds.filter(id => origChannelIds.includes(id));

      // Create new configs
      const newIds: number[] = [];
      for (const nc of configNotifyChannels) {
        if (!nc.id) {
          try {
            const { data: created } = await createNotificationConfig({ channel: nc.channel, name: nc.name, config: { webhook_url: nc.webhookUrl } });
            if (created?.data?.id) newIds.push(created.data.id);
          } catch (e) { console.error("create notif config failed", e); showToast('error', "通知渠道创建失败: " + ((e as any)?.message || "")); }
        }
      }
      // Delete removed configs
      for (const id of toDelete) {
        try { await deleteNotificationConfig(id); } catch (e) { console.error('delete notif config failed', e); }
      }

      const finalChannelIds = [...keepIds, ...newIds];
      const notifyChannels = JSON.stringify(finalChannelIds);

      // 2. Update run config
      await updateLiveRunConfig(configRun.id, {
        autoDailyCron: configRun.autoDailyCron,
        autoTradeExecCron: configRun.autoTradeExecCron,
        aiReviewEnabled: configRun.aiReviewEnabled,
        notifyEnabled: configRun.notifyEnabled && finalChannelIds.length > 0,
        notifyChannels: configRun.notifyEnabled ? notifyChannels : '[]',
      });
      showToast('success', '配置已保存');
      setConfigOpen(false);
      load();
    } catch (e: any) { showToast('error', '保存失败: ' + (e?.message || '未知')); }
  };

  const handleCreateAccount = async () => {
    if (!newAcct.name) { showToast('warning', '请输入账户名称'); return; }
    try {
      await createLiveAccount(newAcct);
      showToast('success', '账户已创建');
      setAcctOpen(false);
      setNewAcct({ name: '', broker: '', accountType: 'simulated', accountNumber: '', initialCapital: 100000, mxApiKey: '', mxAccountId: '', brokerMode: 'manual' });
      load();
    } catch (e: any) { /* toast handled by api interceptor */ }
  };

  const handleDeleteClick = (acct: any) => {
    // Check if account is used
    const usedBy = runs.filter(r => r.accountId === acct.id && (r.status === 'active' || r.status === 'paused'));
    if (usedBy.length > 0) {
      setDeleteCheckMsg(`该账户被 ${usedBy.length} 个运行中的策略使用：${usedBy.map(r => r.name).join('、')}。请先停止相关策略运行后再归档。`);
    } else {
      setDeleteCheckMsg('');
    }
    setDeleteTarget(acct);
  };

  const handleDeleteAccount = async () => {
    if (!deleteTarget) return;
    try {
      await deleteLiveAccount(deleteTarget.id);
      showToast('success', '账户已归档');
      setDeleteTarget(null);
      setDeleteCheckMsg('');
      load();
    } catch (e: any) { showToast('error', e?.response?.data?.message || '删除失败'); }
  };

  const handleRestoreAccount = async (id: number) => {
    try {
      await restoreLiveAccount(id);
      showToast('success', '账户已恢复');
      setShowArchived(false);
      load();
    } catch (e: any) { showToast('error', '恢复失败'); }
  };

  const handleSyncAccount = async (acct: Account) => {
    try {
      showToast('info', '正在同步...');
      const res = await syncFromBroker(acct.id);
      showToast('success', `同步完成: ¥${(res.data?.totalAssets || 0).toLocaleString()} · ${res.data?.posCount || 0}持仓`);
      load();
    } catch (e: any) { showToast('error', '同步失败: ' + (e?.response?.data?.message || '未知')); }
  };

  // Agent token handlers
  const handleGenerateToken = async (accountId: number) => {
    try {
      showToast('info', '正在生成 Token...');
      const { data } = await generateAgentToken(accountId);
      if (data?.code === 0) {
        setAccounts(prev => prev.map(a => a.id === accountId ? { ...a, agentToken: data.data.agentToken } : a));
        if (editingAcct?.id === accountId) setEditingAcct({ ...editingAcct, agentToken: data.data.agentToken });
        showToast('success', 'Agent Token 已生成，请复制并配置到本地 agent');
      }
    } catch (e: any) { showToast('error', e?.response?.data?.message || '生成失败'); }
  };

  const handleRevokeToken = async (accountId: number) => {
    try {
      showToast('info', '正在撤销 Token...');
      await revokeAgentToken(accountId);
      setAccounts(prev => prev.map(a => a.id === accountId ? { ...a, agentToken: '' } : a));
      if (editingAcct?.id === accountId) setEditingAcct({ ...editingAcct, agentToken: '' });
      showToast('success', 'Token 已撤销');
    } catch (e: any) { showToast('error', e?.response?.data?.message || '撤销失败'); }
  };

  const handleTestConnection = async (accountId: number, token: string) => {
    setAgentTesting(accountId);
    setAgentTestMsg('');
    try {
      showToast('info', '正在测试 agent 连接...');
      const { data } = await testAgentConnection(accountId, token);
      if (data?.data?.passed) {
        setAgentTestMsg('✅ 连接测试通过 — agent 响应正常');
        showToast('success', '连接测试通过！可以保存');
      } else {
        setAgentTestMsg('❌ ' + (data?.data?.message || '测试失败'));
        showToast('warning', data?.data?.message || '测试失败');
      }
    } catch (e: any) {
      const msg = e?.response?.data?.message || '测试失败，请确认 agent 已启动';
      setAgentTestMsg('❌ ' + msg);
      showToast('error', msg);
    } finally {
      setAgentTesting(null);
    }
  };

  const handleEditAccount = (acct: any) => {
    setEditingAcct({ ...acct });
    setAcctEditOpen(true);
  };

  const handleUpdateAccountSubmit = async () => {
    if (!editingAcct?.id) return;
    try {
      await updateLiveAccount(editingAcct.id, editingAcct);
      showToast('success', '账户已更新');
      setAcctEditOpen(false);
      load();
    } catch (e: any) { showToast('error', '更新失败: ' + (e?.response?.data?.message || '未知')); }
  };

  const handleStatus = async (runId: number, status: string) => {
    try {
      await updateLiveRunStatus(runId, status);
      showToast('success', status === 'active' ? '已恢复' : status === 'paused' ? '已暂停' : '已停止');
      load();
    } catch (e: any) { showToast('error', '操作失败'); }
  };


  const statusColor = (s: string) => s === 'active' ? 'green' : s === 'paused' ? 'orange' : 'gray';
  const statusLabel = (s: string) => s === 'active' ? '运行中' : s === 'paused' ? '已暂停' : s === 'stopped' ? '已停止' : s;

  const totalEquity = accounts.reduce((s, a) => s + (a.availableCash || 0), 0);
  const totalCapital = accounts.reduce((s, a) => s + (a.initialCapital || 0), 0);

  return (
    <div style={{ padding: 20 }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, display: 'flex', alignItems: 'center', gap: 8 }}>
            <Target size={22} style={{ color: '#165DFF' }} />实盘交易
          </h2>
          <p style={{ margin: '4px 0 0', color: 'var(--color-text-3)', fontSize: 13 }}>多账户管理 · 策略运行 · 信号决策</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>

          <Button icon={<Plus size={14} />} onClick={() => setCreateOpen(true)}>新建运行</Button>
        </div>
      </div>

      {/* Run Overview Panel */}
      <div style={{ marginBottom: 20 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
          <span style={{ fontSize: 14, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
            <Activity size={14} />实盘运行概览
          </span>
          <Button size="mini" icon={<Plus size={12} />} onClick={() => navigate('/holdings')}>资金账户</Button>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 12 }}>
          <Card style={{ borderRadius: 10 }} bodyStyle={{ padding: '14px 16px' }}>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>运行中 / 已暂停</div>
            <div style={{ fontSize: 22, fontWeight: 700 }}>
              {runs.filter(r => r.status === 'active').length}
              <span style={{ fontSize: 14, color: 'var(--color-text-3)', margin: '0 4px' }}>/</span>
              {runs.filter(r => r.status === 'paused').length}
            </div>
          </Card>
          <Card style={{ borderRadius: 10 }} bodyStyle={{ padding: '14px 16px' }}>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>已分配资金</div>
            <div style={{ fontSize: 18, fontWeight: 700, fontFamily: "'SF Mono', monospace" }}>
              ¥{runs.filter(r => r.status === 'active' || r.status === 'paused').reduce((s, r) => s + (r.initialCapital || 0), 0).toLocaleString()}
            </div>
          </Card>
          <Card style={{ borderRadius: 10 }} bodyStyle={{ padding: '14px 16px' }}>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>总权益</div>
            <div style={{ fontSize: 18, fontWeight: 700, fontFamily: "'SF Mono', monospace" }}>
              ¥{runs.filter(r => r.status === 'active' || r.status === 'paused').reduce((s, r) => s + ((r.availableCash || 0) + (r.positionValue || 0) || r.currentEquity || 0), 0).toLocaleString()}
            </div>
          </Card>
          <Card style={{ borderRadius: 10 }} bodyStyle={{ padding: '14px 16px' }}>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>累计盈亏</div>
            {(() => {
              const activeRuns = runs.filter(r => r.status === 'active' || r.status === 'paused');
              const totalPnl = activeRuns.reduce((s, r) => s + ((r.initialCapital || 0) * (r.totalReturn || 0) / 100), 0);
              return (
                <div style={{ fontSize: 18, fontWeight: 700, fontFamily: "'SF Mono', monospace", color: totalPnl >= 0 ? '#F53F3F' : '#00B42A' }}>
                  {totalPnl >= 0 ? '+' : ''}¥{Math.round(totalPnl).toLocaleString()}
                </div>
              );
            })()}
          </Card>
          <Card style={{ borderRadius: 10 }} bodyStyle={{ padding: '14px 16px' }}>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>今日交易</div>
            <div style={{ fontSize: 22, fontWeight: 700 }}>
              {runs.reduce((s, r) => s + (r.tradeCount || 0), 0)}
            </div>
          </Card>
        </div>
      </div>

      {/* Runs Table */}
      <Card
        title={<span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><BarChart3 size={16} />策略运行列表</span>}
        style={{ borderRadius: 10 }}
        extra={<Button size="small" icon={<RefreshCw size={12} />} onClick={load}>刷新</Button>}
      >
        <Table
          data={runs}
          loading={loading}
          rowKey="id"
          onRow={(r) => ({ onClick: () => navigate(`/live/${r.id}`), style: { cursor: 'pointer' } })}
          columns={[
            { title: '名称', dataIndex: 'name', width: 180, render: (v: string, r: StrategyRun) => (
              <div>
                <div style={{ fontWeight: 600 }}>{v}</div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>策略 #{r.strategyId} · {r.startDate}</div>
              </div>
            )},
            { title: '状态', dataIndex: 'status', width: 100, render: (v: string, r: StrategyRun) => (
              <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                <Tag color={statusColor(v)}>{statusLabel(v)}</Tag>
                {r.lastError && (
                  <Tooltip content={<div style={{ maxWidth: 300, wordBreak: 'break-all' }}>{r.lastError}</div>}>
                    <AlertCircle size={14} style={{ color: '#F53F3F', cursor: 'help', flexShrink: 0 }} />
                  </Tooltip>
                )}
              </div>
            )},
            { title: '初始资金', dataIndex: 'initialCapital', width: 110, render: (v: number) => `¥${v.toLocaleString()}` },
            { title: '当前权益', dataIndex: 'currentEquity', width: 110, render: (_: number, r: StrategyRun) => { const eq = (r.availableCash || 0) + (r.positionValue || 0) || r.currentEquity || 0; return <span style={{ fontWeight: 600 }}>¥{eq.toLocaleString()}</span>; } },
            { title: '收益率', dataIndex: 'totalReturn', width: 80, render: (v: number) => <span style={{ color: v >= 0 ? '#F53F3F' : '#00B42A', fontWeight: 600 }}>{v?.toFixed(2)}%</span> },
            { title: '最大回撤', dataIndex: 'maxDrawdown', width: 80, render: (v: number) => <span style={{ color: '#F53F3F' }}>{v?.toFixed(2)}%</span> },
            { title: '交易', dataIndex: 'tradeCount', width: 60 },
            { title: '最后运行', dataIndex: 'lastRunDate', width: 100 },
            { title: '操作', width: 160, render: (_: any, r: StrategyRun) => (
              <div onClick={e => e.stopPropagation()} style={{ display: 'flex', gap: 4 }}>
                {r.status === 'active' && <Button size="mini" status="warning" icon={<Pause size={12} />} onClick={() => handleStatus(r.id, 'paused')}>暂停</Button>}
                {r.status === 'paused' && <Button size="mini" type="primary" icon={<Play size={12} />} onClick={() => handleStatus(r.id, 'active')}>恢复</Button>}
                {r.status === 'stopped' && <Button size="mini" type="outline" icon={<Play size={12} />} onClick={() => handleStatus(r.id, 'active')}>启动</Button>}
                {r.status !== 'stopped' && <Button size="mini" status="danger" icon={<Square size={12} />} onClick={() => handleStatus(r.id, 'stopped')}>停止</Button>}
                <Button size="mini" type="text" icon={<Settings size={12} />} onClick={async () => {
                  setConfigRun({ id: r.id, autoDailyCron: r.autoDailyCron || '18:00', autoTradeExecCron: r.autoTradeExecCron || '09:00', aiReviewEnabled: r.aiReviewEnabled || false, notifyEnabled: r.notifyEnabled || false, notifyChannels: r.notifyChannels || '[]' });
                  // Load existing notification configs
                  try {
                    const { data: ncs } = await fetchNotificationConfigs();
                    const allConfigs = ncs?.data || [];
                    const runChannelIds: number[] = (() => { try { return JSON.parse(r.notifyChannels || '[]'); } catch { return []; } })();
                    const linked = allConfigs.filter((nc: any) => runChannelIds.includes(nc.id)).map((nc: any) => ({
                      id: nc.id, channel: nc.channel, name: nc.name, webhookUrl: nc.config?.webhook_url || ''
                    }));
                    setRemovedNotifyIds([]); setConfigNotifyChannels(linked);
                  } catch(e) { console.error("load notification configs failed", e); showToast('warning', "加载通知配置失败"); setConfigNotifyChannels([]); }
                  setConfigOpen(true);
                }} />
              </div>
            )},
          ]}
        />
      </Card>

      {/* Create Run Modal */}
      <Modal title="新建实盘运行" visible={createOpen} onOk={handleCreateRun} onCancel={() => setCreateOpen(false)} okText="创建">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, padding: '8px 0' }}>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>选择策略</div>
            <Select value={newRun.strategyId || undefined} placeholder="选择策略"
              onChange={(v) => setNewRun({ ...newRun, strategyId: v as number })}
              options={strategies.map(s => ({ label: s.name, value: s.id }))} style={{ width: '100%' }} />
          </div>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>绑定账户</div>
            <Select value={newRun.accountId || undefined} placeholder="选择账户（留空自动创建）" allowClear
              onChange={async (v) => {
                const aid = (v as number) || 0;
                if (aid > 0) {
                  try {
                    const res = await fetchAccountDetail(aid);
                    const freeCash = res.data?.data?.freeCash ?? 0;
                    setSelectedAccountFreeCash(freeCash);
                    setNewRun(prev => ({
                      ...prev,
                      accountId: aid,
                      initialCapital: Math.min(prev.initialCapital || freeCash, freeCash > 0 ? freeCash : 100000),
                    }));
                  } catch {
                    const acct = accounts.find(a => a.id === aid);
                    setSelectedAccountFreeCash(acct?.availableCash || 100000);
                    setNewRun(prev => ({ ...prev, accountId: aid }));
                  }
                } else {
                  setSelectedAccountFreeCash(0);
                  setNewRun(prev => ({ ...prev, accountId: 0, initialCapital: 100000 }));
                }
              }}
              options={accounts.map(a => ({ label: `${a.name} (${a.broker || '默认'}·¥${(a.availableCash || 0).toLocaleString()})`, value: a.id }))} style={{ width: '100%' }} />
            {newRun.accountId > 0 && (() => {
              const avail = selectedAccountFreeCash;
              const pct = avail > 0 ? (newRun.initialCapital / avail * 100) : 0;
              const overLimit = newRun.initialCapital > avail;
              return (
                <div style={{ marginTop: 6, fontSize: 12, color: overLimit ? '#F53F3F' : 'var(--color-text-3)' }}>
                  账户可用: <b>¥{avail.toLocaleString()}</b> · 已分配: <b>{pct.toFixed(1)}%</b>
                  {overLimit && <span style={{ marginLeft: 8, color: '#F53F3F', fontWeight: 600 }}>⚠ 超出可用资金</span>}
                </div>
              );
            })()}
          </div>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>运行名称</div>
            <Input value={newRun.name} onChange={v => setNewRun({ ...newRun, name: v })} placeholder="如：2026Q3实盘" />
          </div>
          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>分配资金</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <InputNumber value={newRun.initialCapital} onChange={v => {
                  const cap = v as number;
                  setNewRun(prev => ({ ...prev, initialCapital: cap }));
                }} min={10000} style={{ flex: 1 }}
                  status={newRun.accountId > 0 && selectedAccountFreeCash > 0 && newRun.initialCapital > selectedAccountFreeCash ? 'error' : undefined} />
                {selectedAccountFreeCash > 0 && (
                  <div style={{ display: 'flex', gap: 4, flexShrink: 0 }}>
                    {[1, 2, 3, 4].map(n => {
                      const amount = +(n === 1 ? selectedAccountFreeCash : (selectedAccountFreeCash / n)).toFixed(2);
                      const label = n === 1 ? '全部' : `1/${n}`;
                      return (
                        <Button key={n} size="mini" type="secondary"
                          onClick={() => setNewRun(prev => ({ ...prev, initialCapital: amount }))}>
                          {label}
                        </Button>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>

          </div>
          {newRun.accountId > 0 && (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '6px 10px', background: 'var(--color-fill-1)', borderRadius: 6 }}>
              <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>从券商导入当前持仓到策略</span>
              <Switch size="small" checked={newRun.importPositions} onChange={v => setNewRun(prev => ({ ...prev, importPositions: v }))} />
            </div>
          )}
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>起始日期 (可选)</div>
            <Input value={newRun.startDate} onChange={v => setNewRun({ ...newRun, startDate: v })} placeholder="YYYY-MM-DD，留空=今天" />
          </div>

          {/* ── 通知设置 ── */}
          <div style={{ borderTop: '1px solid var(--color-border-2)', paddingTop: 12, marginTop: 4 }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <Bell size={14} style={{ color: 'var(--color-text-3)' }} />
                <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>通知推送</span>
              </div>
              <Switch checked={newRun.notifyEnabled} onChange={v => setNewRun(prev => ({ ...prev, notifyEnabled: v }))} />
            </div>
            {newRun.notifyEnabled && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {/* Added channels list */}
                {newRun.notifyConfigs.map((nc, idx) => (
                  <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, background: 'var(--color-fill-1)', borderRadius: 6, padding: '6px 10px' }}>
                    <span style={{ fontSize: 11, color: 'var(--color-text-2)', minWidth: 56, fontWeight: 500 }}>
                      {nc.channel === 'dingtalk_bot' ? '🔷 钉钉' : nc.channel === 'feishu_bot' ? '🟢 飞书' : '🟢 企微'}
                    </span>
                    <span style={{ fontSize: 11, color: 'var(--color-text-3)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{nc.webhookUrl}</span>
                    <Button size="mini" type="text" status="danger" onClick={() => removeNotifyChannel(idx)} style={{ padding: '0 4px', fontSize: 11 }}>移除</Button>
                  </div>
                ))}
                {/* Add new channel */}
                <div style={{ display: 'flex', gap: 8 }}>
                  <Select value={newNotify.channel} onChange={v => setNewNotify(prev => ({ ...prev, channel: v as string }))}
                    style={{ width: 110 }}
                    options={[
                      { label: '钉钉机器人', value: 'dingtalk_bot' },
                      { label: '飞书机器人', value: 'feishu_bot' },
                      { label: '企微机器人', value: 'wecom_bot' },
                    ]} />
                  <Input value={newNotify.webhookUrl} placeholder="Webhook URL" onChange={v => setNewNotify(prev => ({ ...prev, webhookUrl: v }))}
                    style={{ flex: 1 }} />
                  <Button size="small" type="primary" onClick={addNotifyChannel} style={{ whiteSpace: 'nowrap' }}>添加</Button>
                </div>
              </div>
            )}
          </div>
        </div>
      </Modal>

      {/* Run Config Modal */}
      <Modal title="运行配置" visible={configOpen} onOk={handleConfigSave} onCancel={() => setConfigOpen(false)} okText="保存">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14, padding: '8px 0' }}>
          <div>
            <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)', marginBottom: 8 }}>⏰ 自动调度</div>
            <div style={{ fontSize: 11, color: 'var(--color-text-4)', marginBottom: 10 }}>设定每日执行时间，系统自动跳过非交易日</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              <div>
                <div style={{ marginBottom: 3, fontSize: 12, color: 'var(--color-text-3)' }}>盘后信号生成</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <TimePicker
                    value={configRun.autoDailyCron || undefined}
                    format="HH:mm"
                    placeholder="选择时间"
                    style={{ width: 140 }}
                    onChange={(s) => setConfigRun(p => ({ ...p, autoDailyCron: s }))}
                    allowClear
                  />
                  {configRun.autoDailyCron && <span style={{ fontSize: 11, color: 'var(--color-text-4)' }}>每日 {configRun.autoDailyCron} 执行</span>}
                </div>
                <div style={{ fontSize: 10, color: 'var(--color-text-4)', marginTop: 2 }}>留空=手动执行</div>
              </div>
              <div>
                <div style={{ marginBottom: 3, fontSize: 12, color: 'var(--color-text-3)' }}>交易执行</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <TimePicker
                    value={configRun.autoTradeExecCron || undefined}
                    format="HH:mm"
                    placeholder="选择时间"
                    style={{ width: 140 }}
                    onChange={(s) => setConfigRun(p => ({ ...p, autoTradeExecCron: s }))}
                    allowClear
                  />
                  {configRun.autoTradeExecCron && <span style={{ fontSize: 11, color: 'var(--color-text-4)' }}>每日 {configRun.autoTradeExecCron} 执行</span>}
                </div>
                <div style={{ fontSize: 10, color: 'var(--color-text-4)', marginTop: 2 }}>留空=手动执行</div>
              </div>
            </div>
          </div>
          <div style={{ background: 'var(--color-fill-1)', borderRadius: 8, padding: '10px 12px' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 4 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <Cpu size={14} style={{ color: 'var(--color-text-3)' }} />
                <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>AI 审查</span>
              </div>
              <Switch checked={configRun.aiReviewEnabled} onChange={v => setConfigRun(p => ({ ...p, aiReviewEnabled: v }))} />
            </div>
            <div style={{ fontSize: 10, color: 'var(--color-text-4)' }}>开启后交易执行前由 AI 多智能体严格审查信号，可能否决高风险交易</div>
          </div>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <Bell size={14} style={{ color: 'var(--color-text-3)' }} />
                <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>通知推送</span>
              </div>
              <Switch checked={configRun.notifyEnabled} onChange={v => setConfigRun(p => ({ ...p, notifyEnabled: v }))} />
            </div>
            {configRun.notifyEnabled && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {configNotifyChannels.length === 0 && (
                  <div style={{ fontSize: 11, color: 'var(--color-text-4)', padding: '4px 0' }}>尚未添加通知渠道</div>
                )}
                {configNotifyChannels.map((nc, idx) => (
                  <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, background: 'var(--color-fill-1)', borderRadius: 6, padding: '6px 10px' }}>
                    <span style={{ fontSize: 11, color: 'var(--color-text-2)', minWidth: 56, fontWeight: 500 }}>
                      {nc.channel === 'dingtalk_bot' ? '🔷 钉钉' : nc.channel === 'feishu_bot' ? '🟢 飞书' : '🟢 企微'}
                    </span>
                    <span style={{ fontSize: 11, color: 'var(--color-text-3)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{nc.webhookUrl}</span>
                    <Button size="mini" type="text" status="danger" onClick={() => { if (nc.id) setRemovedNotifyIds(prev => [...prev, nc.id]); setConfigNotifyChannels(prev => prev.filter((_, i) => i !== idx)); }} style={{ padding: "0 4px", fontSize: 11 }}>移除</Button>
                    {nc.id && <Button size="mini" type="text" onClick={async () => { try { await testNotification(nc.id!); showToast('success', "测试消息已发送"); } catch(e) { showToast('error', "发送失败: " + (e as any)?.response?.data?.message || String(e)); } }} style={{ padding: "0 4px", fontSize: 11 }}>测试</Button>}
                  </div>
                ))}
                <div style={{ display: 'flex', gap: 8 }}>
                  <Select value={configNewNotify.channel} onChange={v => setConfigNewNotify(prev => ({ ...prev, channel: v as string }))}
                    style={{ width: 110 }}
                    options={[
                      { label: '钉钉机器人', value: 'dingtalk_bot' },
                      { label: '飞书机器人', value: 'feishu_bot' },
                      { label: '企微机器人', value: 'wecom_bot' },
                    ]} />
                  <Input value={configNewNotify.webhookUrl} placeholder="Webhook URL"
                    onChange={v => setConfigNewNotify(prev => ({ ...prev, webhookUrl: v }))} style={{ flex: 1 }} />
                  <Button size="small" type="primary" onClick={() => {
                    if (!configNewNotify.webhookUrl) { showToast('warning', '请输入 Webhook 地址'); return; }
                    const name = configNewNotify.channel === 'dingtalk_bot' ? '钉钉通知' : configNewNotify.channel === 'feishu_bot' ? '飞书通知' : '企业微信通知';
                    setConfigNotifyChannels(prev => [...prev, { channel: configNewNotify.channel, name, webhookUrl: configNewNotify.webhookUrl }]);
                    setConfigNewNotify({ channel: 'dingtalk_bot', webhookUrl: '' });
                  }} style={{ whiteSpace: 'nowrap' }}>添加</Button>
                </div>
              </div>
            )}
          </div>
        </div>
      </Modal>

      {/* Create Account Modal */}
      <Modal title="添加交易账户" visible={acctOpen} onOk={handleCreateAccount} onCancel={() => setAcctOpen(false)} okText="创建">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, padding: '8px 0' }}>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>账户名称 *</div>
            <Input value={newAcct.name} onChange={v => setNewAcct({ ...newAcct, name: v })} placeholder="如：东方财富实盘" />
          </div>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>券商</div>
            <Input value={newAcct.broker} onChange={v => setNewAcct({ ...newAcct, broker: v })} placeholder="如：东方财富 / 华泰证券" />
          </div>
          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>账户类型</div>
              <Select value={newAcct.accountType} onChange={v => setNewAcct({ ...newAcct, accountType: v as string })}
                options={[{ label: '模拟账户', value: 'simulated' }, { label: '真实账户', value: 'real' }]} style={{ width: '100%' }} />
            </div>
            <div style={{ flex: 1 }}>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>资金账号</div>
              <Input value={newAcct.accountNumber} onChange={v => setNewAcct({ ...newAcct, accountNumber: v })} placeholder="选填" />
            </div>
          </div>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>初始资金</div>
            <InputNumber value={newAcct.initialCapital} onChange={v => setNewAcct({ ...newAcct, initialCapital: v as number })} min={10000} style={{ width: '100%' }} />
          </div>
          <Divider style={{ margin: '4px 0' }} orientation="left">
            <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>
              {newAcct.brokerMode === 'lobster' ? 'Agent 自动交易' : newAcct.brokerMode === 'mx_moni' ? '妙想模拟交易绑定' : '执行通道'}
            </span>
          </Divider>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>执行通道</div>
            <Select value={newAcct.brokerMode} onChange={v => setNewAcct({ ...newAcct, brokerMode: v as string })}
              options={[{ label: '手动执行', value: 'manual' }, { label: '妙想模拟盘 API', value: 'mx_moni' }, { label: 'Agent 自动交易', value: 'lobster' }]} style={{ width: '100%' }} />
          </div>
          {newAcct.brokerMode === 'mx_moni' && (<>
            <div style={{ marginTop: 4 }}>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>妙想 API Key <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>(绑定后支持同步持仓)</span></div>
              <Input.Password value={newAcct.mxApiKey} onChange={v => setNewAcct({ ...newAcct, mxApiKey: v })} placeholder="mkt_xxx...（留空则用环境变量）" />
            </div>
            <div>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>妙想账户ID</div>
              <Input value={newAcct.mxAccountId} onChange={v => setNewAcct({ ...newAcct, mxAccountId: v })} placeholder="选填" />
            </div>
          </>)}
          {newAcct.brokerMode === 'lobster' && (
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', lineHeight: 1.8, background: 'var(--color-fill-1)', borderRadius: 8, padding: 12 }}>
              <div style={{ fontWeight: 600, marginBottom: 4 }}>Agent 自动交易说明</div>
              1. 创建账户后，在账户卡片上点击 ✏️ 编辑生成 Agent Token<br />
              2. 将 Token 填入本地 trade-agent/config.yaml<br />
              3. 在 config.yaml 中配置 broker_mode（eastmoney_mac / eastmoney_web / lobster）<br />
              4. 启动本地 agent（python3 agent.py --mode daemon）<br />
              5. 在本页面点击"测试连接"，通过后点击保存
            </div>
          )}
        </div>
      </Modal>

      {/* Edit Account Modal */}
      <Modal title="编辑交易账户" visible={acctEditOpen} onOk={handleUpdateAccountSubmit} onCancel={() => setAcctEditOpen(false)} okText="保存">
        {editingAcct && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12, padding: '8px 0' }}>
            <div>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>账户名称 *</div>
              <Input value={editingAcct.name} onChange={v => setEditingAcct({ ...editingAcct, name: v })} />
            </div>
            <div>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>券商</div>
              <Input value={editingAcct.broker} onChange={v => setEditingAcct({ ...editingAcct, broker: v })} />
            </div>
            <div style={{ display: 'flex', gap: 12 }}>
              <div style={{ flex: 1 }}>
                <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>账户类型</div>
                <Select value={editingAcct.accountType} onChange={v => setEditingAcct({ ...editingAcct, accountType: v })}
                  options={[{ label: '模拟账户', value: 'simulated' }, { label: '真实账户', value: 'real' }]} style={{ width: '100%' }} />
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>资金账号</div>
                <Input value={editingAcct.accountNumber} onChange={v => setEditingAcct({ ...editingAcct, accountNumber: v })} />
              </div>
            </div>
            <div>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>初始资金</div>
              <InputNumber value={editingAcct.initialCapital} onChange={v => setEditingAcct({ ...editingAcct, initialCapital: v as number })} min={10000} style={{ width: '100%' }} />
            </div>
            <div>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>当前可用资金</div>
              <InputNumber value={editingAcct.availableCash} onChange={v => setEditingAcct({ ...editingAcct, availableCash: v as number })} min={0} style={{ width: '100%' }} />
            </div>
            <Divider style={{ margin: '4px 0' }} orientation="left">
              <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>执行通道</span>
            </Divider>
            <div>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>执行模式</div>
              <Select value={editingAcct.brokerMode || 'manual'} onChange={v => setEditingAcct({ ...editingAcct, brokerMode: v })}
                options={[{ label: '手动执行', value: 'manual' }, { label: '妙想模拟盘 API', value: 'mx_moni' }, { label: 'Agent 自动交易', value: 'lobster' }]} style={{ width: '100%' }} />
            </div>
            {/* Agent Token (龙虾自动交易) */}
            {editingAcct.brokerMode === 'lobster' && (
              <>
                <Divider style={{ margin: '4px 0' }} orientation="left">
                  <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>🤖 本地 Agent 设置</span>
                </Divider>
                <div>
                  <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)', display: 'flex', alignItems: 'center', gap: 6 }}>
                    Agent Token
                    {editingAcct.agentToken && <Tag size="small" color="green" style={{ fontSize: 10 }}>已生成</Tag>}
                  </div>
                  {editingAcct.agentToken ? (
                    <>
                      <Input.Password value={editingAcct.agentToken} readOnly style={{ fontFamily: 'monospace', fontSize: 12 }} />
                      <div style={{ display: 'flex', gap: 8, marginTop: 6 }}>
                        <Button size="small" icon={<Zap size={12} />} loading={agentTesting === editingAcct.id}
                          onClick={() => handleTestConnection(editingAcct.id, editingAcct.agentToken || '')}
                          style={{ fontSize: 12 }}>
                          测试连接
                        </Button>
                        <Button size="small" status="danger" onClick={() => handleRevokeToken(editingAcct.id)} style={{ fontSize: 12 }}>
                          撤销 Token
                        </Button>
                        <Button size="small" onClick={() => handleGenerateToken(editingAcct.id)} style={{ fontSize: 12 }}>
                          重新生成
                        </Button>
                      </div>
                      {agentTestMsg && (
                        <div style={{ marginTop: 6, fontSize: 12, color: agentTestMsg.startsWith('✅') ? '#00b42a' : '#f53f3f' }}>
                          {agentTestMsg}
                        </div>
                      )}
                      <div style={{ marginTop: 6, fontSize: 11, color: 'var(--color-text-3)', lineHeight: 1.5 }}>
                        将 Token 填入本地 trade-agent/config.yaml 的 agent_token 字段，<br/>
                        然后启动 agent.py --mode daemon，再点击"测试连接"。
                      </div>
                    </>
                  ) : (
                    <Button size="small" icon={<Key size={12} />} onClick={() => handleGenerateToken(editingAcct.id)} style={{ fontSize: 12 }}>
                      生成 Agent Token
                    </Button>
                  )}
                </div>
              </>
            )}

            {editingAcct.brokerMode !== 'lobster' && (<>
            <Divider style={{ margin: '4px 0' }} orientation="left">
              <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>妙想绑定 (同步持仓/资金)</span>
            </Divider>
            <div>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>妙想 API Key</div>
              <Input.Password value={editingAcct.mxApiKey || ''} onChange={v => setEditingAcct({ ...editingAcct, mxApiKey: v })} placeholder="留空则用环境变量" />
            </div>
            <div>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>妙想账户ID</div>
              <Input value={editingAcct.mxAccountId || ''} onChange={v => setEditingAcct({ ...editingAcct, mxAccountId: v })} />
            </div>
            </>)}
          </div>
        )}
      </Modal>

      {/* Delete Account Confirm Modal */}
      <Modal
        title="归档交易账户"
        visible={!!deleteTarget}
        onOk={deleteCheckMsg ? undefined : handleDeleteAccount}
        onCancel={() => { setDeleteTarget(null); setDeleteCheckMsg(''); }}
        okText={deleteCheckMsg ? '知道了' : '确认归档'}
        okButtonProps={deleteCheckMsg ? { type: 'default' as const } : { status: 'danger' as const }}
      >
        {deleteCheckMsg ? (
          <div style={{ padding: '8px 0' }}>
            <div style={{ fontSize: 13, color: '#f53f3f', marginBottom: 8, display: 'flex', alignItems: 'center', gap: 6 }}>
              <span style={{ fontSize: 18 }}>⚠️</span> 无法归档
            </div>
            <div style={{ fontSize: 13, color: 'var(--color-text-2)', lineHeight: 1.6 }}>
              {deleteCheckMsg}
            </div>
          </div>
        ) : (
          <div style={{ padding: '8px 0' }}>
            <div style={{ fontSize: 13, color: 'var(--color-text-1)', marginBottom: 8 }}>
              确认归档账户「{deleteTarget?.name}」？
            </div>
            <div style={{ fontSize: 12, color: 'var(--color-text-3)', lineHeight: 1.6 }}>
              归档后可随时恢复。归档后该账户将不在账户列表中显示，但历史交易记录和持仓数据仍会保留。
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
