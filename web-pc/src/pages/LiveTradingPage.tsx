import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, Button, Table, Tag, Modal, InputNumber, Select, Input, Message, Grid, Switch, TimePicker, Divider } from '@arco-design/web-react';
import { Play, Plus, Pause, Square, Wallet, TrendingUp, BarChart3, Target, RefreshCw, Building2, Trash2, Bell, Settings, Edit, Coins, DollarSign, Cpu } from 'lucide-react';
import {
  fetchLiveRuns, createLiveRun, updateLiveRunStatus, updateLiveRunConfig,
  fetchLiveAccounts, createLiveAccount, updateLiveAccount, deleteLiveAccount, fetchLiveAccount, syncFromBroker,
  fetchStrategies,
  fetchNotificationConfigs, createNotificationConfig, deleteNotificationConfig, testNotification,
} from '../services/api';

interface StrategyRun {
  id: number; strategyId: number; name: string;
  status: string; startDate: string;
  initialCapital: number; currentEquity: number;
  totalReturn: number; maxDrawdown: number;
  winRate: number; tradeCount: number; lastRunDate: string;
  autoDailyCron?: string; autoTradeExecCron?: string;
  notifyEnabled?: boolean; notifyChannels?: string;
}

interface Account {
  id: number; name: string; broker: string;
  accountType: string; accountNumber: string;
  initialCapital: number; availableCash: number;
  totalAssets?: number; totalMarketValue?: number;
  totalProfit?: number; nav?: number; frozenCash?: number;
  brokerMode?: string; mxApiKey?: string; mxAccountId?: string;
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
  const [newRun, setNewRun] = useState<{ strategyId: number; accountId: number; name: string; initialCapital: number; pctOfAccount: number; startDate: string; notifyEnabled: boolean; notifyConfigs: { channel: string; name: string; webhookUrl: string }[] }>({ strategyId: 0, accountId: 0, name: '', initialCapital: 100000, pctOfAccount: 100, startDate: '', notifyEnabled: false, notifyConfigs: [] });

  // Create account modal
  const [acctOpen, setAcctOpen] = useState(false);
  const [acctEditOpen, setAcctEditOpen] = useState(false);
  const [editingAcct, setEditingAcct] = useState<any>(null);
  const [configOpen, setConfigOpen] = useState(false);
  const [configRun, setConfigRun] = useState<{ id: number; autoDailyCron: string; autoTradeExecCron: string; aiReviewEnabled: boolean; notifyEnabled: boolean; notifyChannels: string }>({ id: 0, autoDailyCron: '', autoTradeExecCron: '', aiReviewEnabled: false, notifyEnabled: false, notifyChannels: '[]' });
  const [removedNotifyIds, setRemovedNotifyIds] = useState<number[]>([]);
  const [configNotifyChannels, setConfigNotifyChannels] = useState<{ id?: number; channel: string; name: string; webhookUrl: string }[]>([]);
  const [configNewNotify, setConfigNewNotify] = useState({ channel: 'dingtalk_bot', webhookUrl: '' });
  const [newAcct, setNewAcct] = useState({ name: '', broker: '', accountType: 'simulated', accountNumber: '', initialCapital: 100000, mxApiKey: '', mxAccountId: '', brokerMode: 'manual' as string });

  const load = useCallback(async () => {
    setLoading(true);
    try { const { data: r } = await fetchLiveRuns(); setRuns(r?.data || []); } catch (e) { console.error('load runs', e); }
    try { const { data: a } = await fetchLiveAccount(); setAccounts(a?.data || []); } catch (e) { console.error('load accounts', e); }
    try { const { data: s } = await fetchStrategies(); setStrategies(s?.data || []); } catch (e) { console.error('load strategies', e); }
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);

  const [newNotify, setNewNotify] = useState({ channel: 'dingtalk_bot', webhookUrl: '' });

  const addNotifyChannel = () => {
    if (!newNotify.webhookUrl) { Message.warning('请输入 Webhook 地址'); return; }
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
    if (!newRun.strategyId) { Message.warning('请选择策略'); return; }
    try {
      await createLiveRun(newRun);
      Message.success('实盘运行已创建');
      setCreateOpen(false);
      setNewRun({ strategyId: 0, accountId: 0, name: '', initialCapital: 100000, pctOfAccount: 100, startDate: '', notifyEnabled: false, notifyConfigs: [] });
      load();
    } catch (e: any) { Message.error(e?.response?.data?.message || '创建失败'); }
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
          } catch (e) { console.error("create notif config failed", e); Message.error("通知渠道创建失败: " + ((e as any)?.message || "")); }
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
      Message.success('配置已保存');
      setConfigOpen(false);
      load();
    } catch (e: any) { Message.error('保存失败: ' + (e?.message || '未知')); }
  };

  const handleCreateAccount = async () => {
    if (!newAcct.name) { Message.warning('请输入账户名称'); return; }
    try {
      await createLiveAccount(newAcct);
      Message.success('账户已创建');
      setAcctOpen(false);
      setNewAcct({ name: '', broker: '', accountType: 'simulated', accountNumber: '', initialCapital: 100000, mxApiKey: '', mxAccountId: '', brokerMode: 'manual' });
      load();
    } catch (e: any) { Message.error(e?.response?.data?.message || '创建失败'); }
  };

  const handleDeleteAccount = async (id: number) => {
    try {
      await deleteLiveAccount(id);
      Message.success('账户已归档');
      load();
    } catch (e: any) { Message.error('删除失败'); }
  };

  const handleSyncAccount = async (acct: Account) => {
    try {
      Message.info('正在同步...');
      const res = await syncFromBroker(acct.id);
      Message.success(`同步完成: ¥${(res.data?.totalAssets || 0).toLocaleString()} · ${res.data?.posCount || 0}持仓`);
      load();
    } catch (e: any) { Message.error('同步失败: ' + (e?.response?.data?.message || '未知')); }
  };

  const handleEditAccount = (acct: any) => {
    setEditingAcct({ ...acct });
    setAcctEditOpen(true);
  };

  const handleUpdateAccountSubmit = async () => {
    if (!editingAcct?.id) return;
    try {
      await updateLiveAccount(editingAcct.id, editingAcct);
      Message.success('账户已更新');
      setAcctEditOpen(false);
      load();
    } catch (e: any) { Message.error('更新失败: ' + (e?.response?.data?.message || '未知')); }
  };

  const handleStatus = async (runId: number, status: string) => {
    try {
      await updateLiveRunStatus(runId, status);
      Message.success(status === 'active' ? '已恢复' : status === 'paused' ? '已暂停' : '已停止');
      load();
    } catch (e: any) { Message.error('操作失败'); }
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

      {/* Account Summary Cards */}
      <div style={{ marginBottom: 20 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
          <span style={{ fontSize: 14, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
            <Building2 size={14} />交易账户
          </span>
          <Button size="mini" icon={<Plus size={12} />} onClick={() => setAcctOpen(true)}>添加账户</Button>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: `repeat(${Math.min(accounts.length + 1, 4)}, 1fr)`, gap: 12 }}>
          {accounts.map(a => (
            <Card key={a.id} style={{ borderRadius: 10, position: 'relative' }}
              bodyStyle={{ padding: '14px 16px' }}
            >
              <div style={{ position: 'absolute', top: 6, right: 6, display: 'flex', gap: 4 }}>
                {(a.brokerMode === 'mx_moni' || a.mxApiKey) && (
                  <span style={{ cursor: 'pointer', opacity: 0.5 }} onClick={() => handleSyncAccount(a)} title="从券商同步">
                    <RefreshCw size={12} />
                  </span>
                )}
                <span style={{ cursor: 'pointer', opacity: 0.4 }} onClick={() => handleEditAccount(a)}><Edit size={12} /></span>
                <span style={{ cursor: 'pointer', opacity: 0.4 }} onClick={() => handleDeleteAccount(a.id)}><Trash2 size={12} /></span>
              </div>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 1 }}>
                {a.name}
                {a.accountType === 'real'
                  ? <Tag size="small" color="red" style={{ marginLeft: 6, fontSize: 10 }}>实盘</Tag>
                  : <Tag size="small" color="arcoblue" style={{ marginLeft: 6, fontSize: 10 }}>模拟</Tag>
                }
                {a.brokerMode === 'mx_moni' && <Tag size="small" color="green" style={{ marginLeft: 4, fontSize: 10 }}>妙想</Tag>}
              </div>
              <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 10 }}>
                {a.broker || '—'}{a.accountNumber ? ` · ${a.accountNumber}` : ''}
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '6px 12px' }}>
                <div>
                  <div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>总资产</div>
                  <div style={{ fontSize: 14, fontWeight: 700 }}>¥{((a.totalAssets || a.initialCapital || 0)).toLocaleString()}</div>
                </div>
                <div>
                  <div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>净值</div>
                  <div style={{ fontSize: 14, fontWeight: 700, color: (a.nav || 1) >= 1 ? '#F53F3F' : '#00B42A' }}>{(a.nav || 1).toFixed(3)}</div>
                </div>
                <div>
                  <div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>可用资金</div>
                  <div style={{ fontSize: 14, fontWeight: 700, color: '#165DFF' }}>¥{(a.availableCash || 0).toLocaleString()}</div>
                </div>
                <div>
                  <div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>持仓市值</div>
                  <div style={{ fontSize: 14, fontWeight: 700, color: '#722ED1' }}>¥{(a.totalMarketValue || 0).toLocaleString()}</div>
                </div>
              </div>
              <div style={{ marginTop: 8, paddingTop: 8, borderTop: '1px solid var(--color-border-1)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                  <DollarSign size={11} style={{ color: 'var(--color-text-3)' }} />
                  <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>累计盈亏</span>
                </div>
                <span style={{ fontSize: 13, fontWeight: 600, fontFamily: "'SF Mono', 'Inter', monospace",
                  color: (a.totalProfit || 0) >= 0 ? '#F53F3F' : '#00B42A' }}>
                  {(a.totalProfit || 0) >= 0 ? '+' : ''}¥{(a.totalProfit || 0).toLocaleString()}
                </span>
              </div>
            </Card>
          ))}
          {accounts.length === 0 && (
            <Card style={{ borderRadius: 10, borderStyle: 'dashed', display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: 80 }}>
              <span style={{ color: 'var(--color-text-3)', fontSize: 13 }}>暂无交易账户，点击「添加账户」</span>
            </Card>
          )}
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
            { title: '状态', dataIndex: 'status', width: 80, render: (v: string) => <Tag color={statusColor(v)}>{statusLabel(v)}</Tag> },
            { title: '初始资金', dataIndex: 'initialCapital', width: 110, render: (v: number) => `¥${v.toLocaleString()}` },
            { title: '当前权益', dataIndex: 'currentEquity', width: 110, render: (v: number) => <span style={{ fontWeight: 600 }}>¥{v.toLocaleString()}</span> },
            { title: '收益率', dataIndex: 'totalReturn', width: 80, render: (v: number) => <span style={{ color: v >= 0 ? '#00B42A' : '#F53F3F', fontWeight: 600 }}>{v > 0 ? '+' : ''}{v?.toFixed(2)}%</span> },
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
                  } catch(e) { console.error("load notification configs failed", e); Message.warning("加载通知配置失败"); setConfigNotifyChannels([]); }
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
              onChange={(v) => {
                const aid = (v as number) || 0;
                const acct = accounts.find(a => a.id === aid);
                const availCash = acct?.availableCash || 100000;
                setNewRun(prev => ({
                  ...prev,
                  accountId: aid,
                  initialCapital: availCash,
                  pctOfAccount: 100,
                }));
              }}
              options={accounts.map(a => ({ label: `${a.name} (${a.broker || '默认'}·¥${(a.availableCash || 0).toLocaleString()})`, value: a.id }))} style={{ width: '100%' }} />
            {newRun.accountId > 0 && (() => {
              const acct = accounts.find(a => a.id === newRun.accountId);
              const avail = acct?.availableCash || 0;
              const pct = avail > 0 ? (newRun.initialCapital / avail * 100) : 0;
              const overLimit = newRun.initialCapital > avail;
              return (
                <div style={{ marginTop: 6, fontSize: 12, color: overLimit ? '#F53F3F' : 'var(--color-text-3)' }}>
                  账户可用: <b>¥{avail.toLocaleString()}</b>
                  {newRun.initialCapital > 0 && <> · 占比: <b style={{ color: overLimit ? '#F53F3F' : 'var(--color-text-2)' }}>{pct.toFixed(1)}%</b></>}
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
              <InputNumber value={newRun.initialCapital} onChange={v => {
                const cap = v as number;
                const acct = accounts.find(a => a.id === newRun.accountId);
                const avail = acct?.availableCash || cap;
                setNewRun(prev => ({
                  ...prev,
                  initialCapital: cap,
                  pctOfAccount: avail > 0 ? Math.round(cap / avail * 10000) / 100 : 100,
                }));
              }} min={10000} style={{ width: '100%' }}
                status={newRun.accountId > 0 && newRun.initialCapital > (accounts.find(a => a.id === newRun.accountId)?.availableCash || 0) ? 'error' : undefined} />
            </div>
            <div style={{ flex: 1 }}>
              <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>占比 (%)</div>
              <InputNumber value={newRun.pctOfAccount} onChange={v => {
                const pct = v as number;
                const acct = accounts.find(a => a.id === newRun.accountId);
                const avail = acct?.availableCash || 100000;
                setNewRun(prev => ({
                  ...prev,
                  pctOfAccount: pct,
                  initialCapital: Math.round(avail * pct / 100),
                }));
              }} min={1} max={100} style={{ width: '100%' }} />
            </div>
          </div>
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
                    {nc.id && <Button size="mini" type="text" onClick={async () => { try { await testNotification(nc.id!); Message.success("测试消息已发送"); } catch(e) { Message.error("发送失败: " + (e as any)?.response?.data?.message || String(e)); } }} style={{ padding: "0 4px", fontSize: 11 }}>测试</Button>}
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
                    if (!configNewNotify.webhookUrl) { Message.warning('请输入 Webhook 地址'); return; }
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
            <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>妙想模拟交易绑定 (可选)</span>
          </Divider>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>执行通道</div>
            <Select value={newAcct.brokerMode} onChange={v => setNewAcct({ ...newAcct, brokerMode: v as string })}
              options={[{ label: '手动执行', value: 'manual' }, { label: '东财妙想模拟盘', value: 'mx_moni' }]} style={{ width: '100%' }} />
          </div>
                    <div style={{ marginTop: 4 }}>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>妙想 API Key <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>(绑定后支持同步持仓)</span></div>
            <Input.Password value={newAcct.mxApiKey} onChange={v => setNewAcct({ ...newAcct, mxApiKey: v })} placeholder="mkt_xxx...（留空则用环境变量）" />
          </div>
          <div>
            <div style={{ marginBottom: 4, fontSize: 13, color: 'var(--color-text-2)' }}>妙想账户ID</div>
            <Input value={newAcct.mxAccountId} onChange={v => setNewAcct({ ...newAcct, mxAccountId: v })} placeholder="选填" />
          </div>
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
                options={[{ label: '手动执行', value: 'manual' }, { label: '东财妙想模拟盘', value: 'mx_moni' }]} style={{ width: '100%' }} />
            </div>
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
          </div>
        )}
      </Modal>
    </div>
  );
}
