import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { listUsers, createUser, resetUserPassword, toggleUser, kickUser, fetchLoginLogs, fetchCostLogs, fetchCostSummary, fetchModelPrices, updateModelPrice } from '../services/api';
import { useAuth } from '../services/AuthContext';
import {
  Users, Plus, Key, Ban, CheckCircle, XCircle, LogOut,
  LogIn, UserX, AlertCircle, FileText, Coins, DollarSign,
} from 'lucide-react';
import { Tabs, Select, Pagination, Button } from '@arco-design/web-react';

interface UserRecord {
  id: number; username: string; role: string; isActive: boolean;
  lastLoginAt: string; lastLoginIp: string; isOnline: boolean;
  lastHeartbeat: string; deviceInfo: string; sessionIp: string;
  sessionCount: number; createdAt: string;
}

interface LogEntry {
  id: number; userId: number; username: string; action: string;
  success: boolean; ipAddress: string; deviceInfo: string;
  failReason: string; createdAt: string;
}

const actionLabels: Record<string, { label: string; color: string; icon: any }> = {
  login: { label: '登录', color: 'var(--stock-down)', icon: LogIn },
  logout: { label: '退出', color: 'var(--color-text-3)', icon: LogOut },
  failed: { label: '失败', color: 'var(--stock-up)', icon: AlertCircle },
  kicked: { label: '被踢', color: 'var(--color-warning-text)', icon: UserX },
};

export default function AdminPage() {
  const { isAdmin } = useAuth();
  const navigate = useNavigate();
  const [users, setUsers] = useState<UserRecord[]>([]);
  const [showCreate, setShowCreate] = useState(false);
  const [newUser, setNewUser] = useState({ username: '', password: '', role: 'user' });
  const [resetTarget, setResetTarget] = useState<{ id: number; name: string } | null>(null);
  const [newPw, setNewPw] = useState('');
  const [msg, setMsg] = useState('');
  const [activeTab, setActiveTab] = useState('users');

  // 登录日志 state
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [logTotal, setLogTotal] = useState(0);
  const [logPage, setLogPage] = useState(1);
  const [logAction, setLogAction] = useState('');
  const [logKeyword, setLogKeyword] = useState('');

  const loadUsers = useCallback(async () => {
    try {
      const { data } = await listUsers();
      setUsers(data.data || []);
    } catch {}
  }, []);

  const loadLogs = useCallback(async () => {
    try {
      const { data } = await fetchLoginLogs({ page: logPage, pageSize: 30, action: logAction, username: logKeyword });
      setLogs(data.data?.data || data.data || []);
      setLogTotal(data.data?.total || data.total || 0);
    } catch {}
  }, [logPage, logAction, logKeyword]);

  useEffect(() => { loadUsers(); }, [loadUsers]);
  useEffect(() => { if (activeTab === 'logs') loadLogs(); }, [loadLogs, activeTab]);

  if (!isAdmin) {
    return (
      <div style={{ padding: 40, color: 'var(--color-text-3)', textAlign: 'center' }}>
        仅管理员可访问
      </div>
    );
  }

  const handleCreate = async () => {
    if (!newUser.username || !newUser.password) return;
    try {
      await createUser(newUser.username, newUser.password, newUser.role);
      setMsg('✓ 用户创建成功');
      setShowCreate(false);
      setNewUser({ username: '', password: '', role: 'user' });
      loadUsers();
    } catch (err: any) {
      setMsg('✗ ' + (err.response?.data?.error || '创建失败'));
    }
    setTimeout(() => setMsg(''), 3000);
  };

  const handleReset = async () => {
    if (!resetTarget || !newPw || newPw.length < 6) return;
    try {
      await resetUserPassword(resetTarget.id, newPw);
      setMsg('✓ 密码已重置');
      setResetTarget(null);
      setNewPw('');
    } catch (err: any) {
      setMsg('✗ ' + (err.response?.data?.error || '重置失败'));
    }
    setTimeout(() => setMsg(''), 3000);
  };

  const handleToggle = async (id: number, current: boolean) => {
    try {
      await toggleUser(id);
      setMsg(current ? '✓ 用户已停用' : '✓ 用户已启用');
      loadUsers();
    } catch (err: any) {
      setMsg('✗ ' + (err.response?.data?.error || '操作失败'));
    }
    setTimeout(() => setMsg(''), 3000);
  };

  const handleKick = async (id: number, name: string) => {
    if (!confirm(`确定要强制下线用户「${name}」吗？`)) return;
    try {
      await kickUser(id);
      setMsg(`✓ 已踢下线「${name}」`);
      loadUsers();
    } catch (err: any) {
      setMsg('✗ ' + (err.response?.data?.error || '操作失败'));
    }
    setTimeout(() => setMsg(''), 3000);
  };

  return (
    <div style={{ padding: '0 0 40px' }}>
      {/* Tabs */}
      <div style={{ marginBottom: 24 }}>
        <Tabs activeTab={activeTab} onChange={setActiveTab} type="rounded">
          <Tabs.TabPane key="users" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Users size={15} /> 用户管理</span>} />
          <Tabs.TabPane key="logs" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><FileText size={15} /> 登录日志</span>} />
          <Tabs.TabPane key="cost" title={<span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Coins size={15} /> AI花费明细</span>} />
        </Tabs>
      </div>

      {msg && (
        <div style={{
          marginBottom: 12, padding: '8px 16px', borderRadius: 6, fontSize: 13,
          background: msg.startsWith('✓') ? 'var(--color-success-bg)' : 'var(--color-danger-bg)',
          color: msg.startsWith('✓') ? 'var(--color-success)' : 'var(--color-danger)',
        }}>{msg}</div>
      )}

      {/* ===== 用户管理 Tab ===== */}
      {activeTab === 'users' && (<>
        {/* Actions */}
        <div style={{ display: 'flex', gap: 10, marginBottom: 16, alignItems: 'center' }}>
          <button onClick={() => setShowCreate(true)} style={{
            display: 'inline-flex', alignItems: 'center', gap: 6, padding: '8px 18px',
            background: 'var(--color-primary)', color: '#fff', border: 'none', borderRadius: 6, fontSize: 13, cursor: 'pointer',
          }}><Plus size={14} /> 新建用户</button>
          <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>共 {users.length} 个用户</span>
        </div>

        {/* Create user modal */}
        {showCreate && (
          <div style={{
            position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.35)', zIndex: 999,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }} onClick={() => setShowCreate(false)}>
            <div style={{
              background: 'var(--color-bg-1)', borderRadius: 12, padding: 24, width: 380,
              boxShadow: '0 8px 30px rgba(0,0,0,0.12)',
            }} onClick={e => e.stopPropagation()}>
              <h3 style={{ margin: '0 0 16px', fontSize: 15, color: 'var(--color-text-1)' }}>新建用户</h3>
              <input
                placeholder="用户名"
                value={newUser.username}
                onChange={e => setNewUser(p => ({ ...p, username: e.target.value }))}
                style={inputStyle}
              />
              <input
                type="password" placeholder="密码"
                value={newUser.password}
                onChange={e => setNewUser(p => ({ ...p, password: e.target.value }))}
                style={{ ...inputStyle, marginTop: 10 }}
              />
              <div style={{ marginTop: 10, display: 'flex', gap: 8 }}>
                <label style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 13, color: 'var(--color-text-2)', cursor: 'pointer' }}>
                  <input type="radio" name="role" value="user" checked={newUser.role === 'user'} onChange={() => setNewUser(p => ({ ...p, role: 'user' }))} /> 普通用户
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 13, color: 'var(--color-text-2)', cursor: 'pointer' }}>
                  <input type="radio" name="role" value="admin" checked={newUser.role === 'admin'} onChange={() => setNewUser(p => ({ ...p, role: 'admin' }))} /> 管理员
                </label>
              </div>
              <div style={{ marginTop: 16, display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                <button onClick={() => setShowCreate(false)} style={cancelBtn}>取消</button>
                <button onClick={handleCreate} style={primaryBtn}>确认创建</button>
              </div>
            </div>
          </div>
        )}

        {/* Reset password modal */}
        {resetTarget && (
          <div style={{
            position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.35)', zIndex: 999,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }} onClick={() => { setResetTarget(null); setNewPw(''); }}>
            <div style={{
              background: 'var(--color-bg-1)', borderRadius: 12, padding: 24, width: 360,
              boxShadow: '0 8px 30px rgba(0,0,0,0.12)',
            }} onClick={e => e.stopPropagation()}>
              <h3 style={{ margin: '0 0 12px', fontSize: 15, color: 'var(--color-text-1)' }}>
                重置密码 — {resetTarget.name}
              </h3>
              <input
                type="password" placeholder="新密码（至少6位）"
                value={newPw}
                onChange={e => setNewPw(e.target.value)}
                style={inputStyle}
              />
              <div style={{ marginTop: 14, display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                <button onClick={() => { setResetTarget(null); setNewPw(''); }} style={cancelBtn}>取消</button>
                <button onClick={handleReset} style={primaryBtn}>确认重置</button>
              </div>
            </div>
          </div>
        )}

        {/* User table */}
        <div style={{ borderRadius: 10, overflow: 'hidden', border: '1px solid var(--color-border-1)' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ background: 'var(--color-fill-2)' }}>
                <th style={th}>ID</th>
                <th style={th}>用户名</th>
                <th style={th}>角色</th>
                <th style={th}>状态</th>
                <th style={{ ...th, width: 80 }}>在线</th>
                <th style={{ ...th, width: 140 }}>登录设备</th>
                <th style={th}>最后登录</th>
                <th style={th}>登录IP</th>
                <th style={th}>操作</th>
              </tr>
            </thead>
            <tbody>
              {users.map(u => (
                <tr key={u.id} style={{ borderBottom: '1px solid var(--color-table-row-border)' }}>
                  <td style={td}>{u.id}</td>
                  <td style={{ ...td, fontWeight: 600, color: 'var(--color-text-1)' }}>{u.username}</td>
                  <td style={td}>
                    <span style={{
                      padding: '2px 8px', borderRadius: 4, fontSize: 11,
                      background: u.role === 'admin' ? '#e8f3ff' : 'var(--color-fill-2)',
                      color: u.role === 'admin' ? '#165dff' : 'var(--color-text-3)',
                    }}>{u.role === 'admin' ? '管理员' : '用户'}</span>
                  </td>
                  <td style={td}>
                    {u.isActive
                      ? <span style={{ color: 'var(--stock-down)', display: 'flex', alignItems: 'center', gap: 4 }}><CheckCircle size={12} /> 正常</span>
                      : <span style={{ color: 'var(--stock-up)', display: 'flex', alignItems: 'center', gap: 4 }}><XCircle size={12} /> 停用</span>
                    }
                  </td>
                  <td style={td}>
                    {u.isOnline
                      ? <span style={{ color: 'var(--stock-down)', display: 'flex', alignItems: 'center', gap: 4 }}>
                          <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#00b42a', display: 'inline-block' }} /> 在线
                        </span>
                      : <span style={{ color: 'var(--color-text-3)', display: 'flex', alignItems: 'center', gap: 4 }}>
                          <span style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--color-text-3)', display: 'inline-block' }} /> 离线
                        </span>
                    }
                    {u.sessionCount > 0 && <span style={{ fontSize: 10, color: 'var(--color-text-3)', marginLeft: 4 }}>({u.sessionCount})</span>}
                  </td>
                  <td style={{ ...td, fontSize: 11, color: 'var(--color-text-3)', maxWidth: 140, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {u.deviceInfo || '-'}
                  </td>
                  <td style={{ ...td, color: 'var(--color-text-3)' }}>{u.lastLoginAt ? new Date(u.lastLoginAt).toLocaleString('zh-CN') : '-'}</td>
                  <td style={{ ...td, color: 'var(--color-text-3)', fontFamily: 'monospace', fontSize: 11 }}>{u.lastLoginIp || '-'}</td>
                  <td style={td}>
                    <div style={{ display: 'flex', gap: 6 }}>
                      <button onClick={() => { setResetTarget({ id: u.id, name: u.username }); setNewPw(''); }}
                        style={{ ...btnSm, background: 'var(--color-warning-bg)', color: 'var(--color-warning-text)' }}>
                        <Key size={11} /> 重置
                      </button>
                      <button onClick={() => handleKick(u.id, u.username)}
                        style={{ ...btnSm, background: 'var(--color-warning-bg)', color: 'var(--color-warning-text)' }}>
                        <LogOut size={11} /> 踢下线
                      </button>
                      <button onClick={() => handleToggle(u.id, u.isActive)}
                        style={{
                          ...btnSm,
                          background: u.isActive ? 'var(--color-danger-bg)' : 'var(--color-success-bg)',
                          color: u.isActive ? 'var(--color-danger)' : 'var(--color-success)',
                        }}>
                        {u.isActive ? <><Ban size={11} /> 停用</> : <><CheckCircle size={11} /> 启用</>}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </>)}

      {/* ===== 登录日志 Tab ===== */}
      {activeTab === 'logs' && (<>
        <div className="card" style={{ marginBottom: 16 }}>
          <div style={{ padding: '12px 20px', display: 'flex', gap: 12, alignItems: 'center' }}>
            <Select
              placeholder="全部类型"
              value={logAction || undefined}
              onChange={(v: string) => { setLogAction(v || ''); setLogPage(1); }}
              style={{ width: 140 }}
              allowClear
              options={[
                { label: '全部', value: '' },
                { label: '登录成功', value: 'login' },
                { label: '退出', value: 'logout' },
                { label: '登录失败', value: 'failed' },
                { label: '被强制下线', value: 'kicked' },
              ]}
            />
            <input
              placeholder="搜索用户名..."
              value={logKeyword}
              onChange={e => { setLogKeyword(e.target.value); setLogPage(1); }}
              style={{
                padding: '6px 12px', borderRadius: 6, border: '1px solid var(--color-border-1)',
                background: 'var(--color-fill-2)', color: 'var(--color-text-1)', fontSize: 13, width: 180,
                outline: 'none',
              }}
              onFocus={e => { e.target.style.borderColor = '#165dff'; e.target.style.background = 'var(--color-bg-1)'; }}
              onBlur={e => { e.target.style.borderColor = 'var(--color-border-1)'; e.target.style.background = 'var(--color-fill-2)'; }}
            />
            <span style={{ flex: 1 }} />
            <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>共 {logTotal} 条</span>
            <Pagination
              current={logPage} total={logTotal} pageSize={30} size="small" simple
              onChange={(p) => setLogPage(p)}
            />
          </div>
        </div>

        <div style={{ borderRadius: 10, overflow: 'hidden', border: '1px solid var(--color-border-1)' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ background: 'var(--color-fill-2)' }}>
                <th style={th}>时间</th>
                <th style={th}>用户</th>
                <th style={th}>操作</th>
                <th style={th}>结果</th>
                <th style={th}>IP 地址</th>
                <th style={th}>设备信息</th>
                <th style={th}>备注</th>
              </tr>
            </thead>
            <tbody>
              {logs.map(log => {
                const act = actionLabels[log.action] || { label: log.action, color: 'var(--color-text-3)', icon: AlertCircle };
                const ActIcon = act.icon;
                return (
                  <tr key={log.id} style={{ borderBottom: '1px solid var(--color-table-row-border)' }}>
                    <td style={{ ...td, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>{new Date(log.createdAt).toLocaleString('zh-CN')}</td>
                    <td style={{ ...td, fontWeight: 600, color: 'var(--color-text-1)' }}>{log.username}</td>
                    <td style={td}><span style={{ display: 'flex', alignItems: 'center', gap: 4, color: act.color }}><ActIcon size={13} /> {act.label}</span></td>
                    <td style={td}>{log.success
                      ? <span style={{ color: 'var(--stock-down)', display: 'flex', alignItems: 'center', gap: 4 }}><CheckCircle size={12} /> 成功</span>
                      : <span style={{ color: 'var(--stock-up)', display: 'flex', alignItems: 'center', gap: 4 }}><XCircle size={12} /> 失败</span>}</td>
                    <td style={{ ...td, fontFamily: 'monospace', fontSize: 11, color: 'var(--color-text-3)' }}>{log.ipAddress}</td>
                    <td style={{ ...td, fontSize: 11, color: 'var(--color-text-3)', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{log.deviceInfo || '-'}</td>
                    <td style={{ ...td, color: 'var(--stock-up)', fontSize: 12 }}>{log.failReason || '-'}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </>)}

        {/* ===== AI花费明细 Tab ===== */}
        {activeTab === 'cost' && <CostTab />}
      </div>
  );
}

function CostTab() {
  var [logs, setLogs] = useState<any[]>([]);
  var [total, setTotal] = useState(0);
  var [page, setPage] = useState(1);
  var [module, setModule] = useState('');
  var [userId, setUserId] = useState('');
  var [start, setStart] = useState('');
  var [end, setEnd] = useState('');
  var [summary, setSummary] = useState<any>({});
  var [prices, setPrices] = useState<any[]>([]);
  var [showPrices, setShowPrices] = useState(false);
  var [editPrice, setEditPrice] = useState<any>(null);

  var loadLogsFn = useCallback(async () => {
    try {
      var res = await fetchCostLogs({ page, pageSize: 20, userId: userId || undefined, module: module || undefined, start: start || undefined, end: end || undefined });
      setLogs(res.data?.data?.list || []);
      setTotal(res.data?.data?.total || 0);
    } catch (e) { console.error(e); }
  }, [page, module, userId, start, end]);

  var loadSummaryFn = useCallback(async () => {
    try {
      var res = await fetchCostSummary({ userId: userId || undefined, module: module || undefined, start: start || undefined, end: end || undefined });
      setSummary(res.data?.data || {});
    } catch (e) { console.error(e); }
  }, [start, end, module, userId]);

  var loadPricesFn = useCallback(async () => {
    try {
      var res = await fetchModelPrices();
      setPrices(res.data?.data || []);
    } catch (e) { console.error(e); }
  }, []);

  useEffect(function() { loadLogsFn(); loadSummaryFn(); }, [loadLogsFn, loadSummaryFn]);
  useEffect(function() { if (showPrices) loadPricesFn(); }, [showPrices, loadPricesFn]);

  var moduleLabels: Record<string, string> = {
    chat: 'AI对话', stock_score: '股票评分', stock_profile: '公司简介',
    strategy_gen: '策略生成', strategy_opt: '提示词优化',
  };
  var fmtModule = function(m: string) { return moduleLabels[m] || m; };
  var fmtTokens = function(n: number) { return n >= 1000 ? (n/1000).toFixed(1)+'k' : String(n); };
  var fmtCost = function(n: number) { return n < 0.001 ? '<¥0.001' : '¥'+n.toFixed(4); };
  var fmtMs = function(n: number) { return n >= 1000 ? (n/1000).toFixed(1)+'s' : n+'ms'; };

  return (
    <div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 16 }}>
        {[
          { label: '总费用', value: fmtCost(summary.totalCost || 0), color: '#F53F3F' },
          { label: '今日费用', value: fmtCost(summary.todayCost || 0), color: '#FF7D00' },
          { label: '本月费用', value: fmtCost(summary.monthCost || 0), color: '#165DFF' },
          { label: '总调用次数', value: (summary.totalCalls || 0).toLocaleString(), color: 'var(--color-text-1)' },
        ].map(function(card, i) {
          return (
            <div key={i} style={{ background: 'var(--color-bg-2)', borderRadius: 10, padding: '14px 16px', border: '1px solid var(--color-border-2)' }}>
              <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>{card.label}</div>
              <div style={{ fontSize: 20, fontWeight: 700, color: card.color, fontFamily: "'SF Mono', monospace" }}>{card.value}</div>
            </div>
          );
        })}
      </div>

      <div style={{ display: 'flex', gap: 10, marginBottom: 12, flexWrap: 'wrap', alignItems: 'center' }}>
        <input type="date" value={start} onChange={function(e) { setStart(e.target.value); setPage(1); }}
          style={{ padding: '6px 10px', borderRadius: 6, border: '1px solid var(--color-border-2)', fontSize: 12, background: 'var(--color-bg-1)', color: 'var(--color-text-1)' }} />
        <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>至</span>
        <input type="date" value={end} onChange={function(e) { setEnd(e.target.value); setPage(1); }}
          style={{ padding: '6px 10px', borderRadius: 6, border: '1px solid var(--color-border-2)', fontSize: 12, background: 'var(--color-bg-1)', color: 'var(--color-text-1)' }} />
        <input placeholder="用户名/ID" value={userId} onChange={function(e) { setUserId(e.target.value); setPage(1); }}
          style={{ width: 100, padding: '6px 10px', borderRadius: 6, border: '1px solid var(--color-border-2)', fontSize: 12, background: 'var(--color-bg-1)', color: 'var(--color-text-1)' }} />
        <Select placeholder="模块" value={module || undefined} onChange={function(v: any) { setModule(v || ''); setPage(1); }} allowClear
          style={{ width: 130 }} size="small"
          options={[
            { label: 'AI对话', value: 'chat' }, { label: '股票评分', value: 'stock_score' },
            { label: '公司简介', value: 'stock_profile' }, { label: '策略生成', value: 'strategy_gen' },
            { label: '提示词优化', value: 'strategy_opt' },
          ]} />
        <div style={{ flex: 1 }} />
        <Button size="small" type="outline" onClick={function() { setShowPrices(!showPrices); }}>
          {showPrices ? '隐藏价格' : '价格管理'}
        </Button>
      </div>

      {showPrices && (
        <div style={{ marginBottom: 12, padding: 12, borderRadius: 8, background: 'var(--color-bg-2)', border: '1px solid var(--color-border-2)' }}>
          <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 8, color: 'var(--color-text-2)' }}>模型价格配置（元/百万tokens）</div>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
            <thead>
              <tr style={{ borderBottom: '2px solid var(--color-border-2)' }}>
                <th style={{ padding: '6px 10px', textAlign: 'left', color: 'var(--color-text-3)' }}>模型</th>
                <th style={{ padding: '6px 10px', textAlign: 'right', color: 'var(--color-text-3)' }}>输入</th>
                <th style={{ padding: '6px 10px', textAlign: 'right', color: 'var(--color-text-3)' }}>输出</th>
                <th style={{ padding: '6px 10px', textAlign: 'right', color: 'var(--color-text-3)' }}>缓存命中</th>
                <th style={{ padding: '6px 10px', textAlign: 'center', color: 'var(--color-text-3)' }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {prices.map(function(p: any) {
                var isEditing = editPrice && editPrice.modelName === p.modelName;
                return (
                  <tr key={p.modelName} style={{ borderBottom: '1px solid var(--color-border-1)' }}>
                    <td style={{ padding: '6px 10px', fontWeight: 500 }}>{p.displayName || p.modelName}</td>
                    {isEditing ? (
                      <>
                        <td style={{ padding: '4px 6px' }}><input value={editPrice.inputPrice} onChange={function(e) { setEditPrice({...editPrice, inputPrice: e.target.value}); }}
                          style={{ width: 60, padding: '3px 6px', borderRadius: 4, border: '1px solid var(--color-border-2)', fontSize: 12, textAlign: 'right' }} /></td>
                        <td style={{ padding: '4px 6px' }}><input value={editPrice.outputPrice} onChange={function(e) { setEditPrice({...editPrice, outputPrice: e.target.value}); }}
                          style={{ width: 60, padding: '3px 6px', borderRadius: 4, border: '1px solid var(--color-border-2)', fontSize: 12, textAlign: 'right' }} /></td>
                        <td style={{ padding: '4px 6px' }}><input value={editPrice.cacheHitPrice} onChange={function(e) { setEditPrice({...editPrice, cacheHitPrice: e.target.value}); }}
                          style={{ width: 60, padding: '3px 6px', borderRadius: 4, border: '1px solid var(--color-border-2)', fontSize: 12, textAlign: 'right' }} /></td>
                        <td style={{ padding: '4px 6px', textAlign: 'center' }}>
                          <button onClick={async function() {
                            await updateModelPrice(p.modelName, { inputPrice: parseFloat(editPrice.inputPrice)||0, outputPrice: parseFloat(editPrice.outputPrice)||0, cacheHitPrice: parseFloat(editPrice.cacheHitPrice)||0 });
                            setEditPrice(null); loadPricesFn();
                          }} style={{ padding: '2px 10px', fontSize: 11, borderRadius: 4, border: 'none', background: 'var(--color-primary)', color: '#fff', cursor: 'pointer' }}>保存</button>
                          <button onClick={function() { setEditPrice(null); }} style={{ marginLeft: 4, padding: '2px 8px', fontSize: 11, borderRadius: 4, border: '1px solid var(--color-border-2)', background: 'transparent', color: 'var(--color-text-2)', cursor: 'pointer' }}>取消</button>
                        </td>
                      </>
                    ) : (
                      <>
                        <td style={{ padding: '6px 10px', textAlign: 'right', fontFamily: "'SF Mono', monospace" }}>¥{p.inputPrice}</td>
                        <td style={{ padding: '6px 10px', textAlign: 'right', fontFamily: "'SF Mono', monospace" }}>¥{p.outputPrice}</td>
                        <td style={{ padding: '6px 10px', textAlign: 'right', fontFamily: "'SF Mono', monospace" }}>{p.cacheHitPrice > 0 ? '¥'+p.cacheHitPrice : '-'}</td>
                        <td style={{ padding: '4px 6px', textAlign: 'center' }}>
                          <button onClick={function() { setEditPrice({...p}); }} style={{ padding: '2px 10px', fontSize: 11, borderRadius: 4, border: '1px solid var(--color-border-2)', background: 'transparent', color: 'var(--color-text-2)', cursor: 'pointer' }}>编辑</button>
                        </td>
                      </>
                    )}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
        <thead>
          <tr style={{ background: 'var(--color-primary-bg)', borderBottom: '1px solid var(--color-border-2)' }}>
            <th colSpan={9} style={{ padding: '6px 12px', textAlign: 'left' }}>
              <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
                筛选结果：共 <b style={{ color: 'var(--color-text-1)' }}>{total.toLocaleString()}</b> 条 &nbsp;|&nbsp;
                费用合计 <b style={{ color: '#F53F3F' }}>{fmtCost(logs.reduce(function(s, l) {{ return s + (l.costAmount || 0); }}, 0))}</b> &nbsp;|&nbsp;
                Token合计 <b style={{ color: 'var(--color-text-1)' }}>{fmtTokens(logs.reduce(function(s, l) {{ return s + (l.totalTokens || 0); }}, 0))}</b>
              </span>
            </th>
          </tr>
          <tr style={{ background: 'var(--color-fill-2)', borderBottom: '2px solid var(--color-border-2)' }}>
            <th style={{ padding: '8px 12px', textAlign: 'left', fontWeight: 600, fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>时间</th>
            <th style={{ padding: '8px 12px', textAlign: 'left', fontWeight: 600, fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>用户</th>
            <th style={{ padding: '8px 12px', textAlign: 'left', fontWeight: 600, fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>模块</th>
            <th style={{ padding: '8px 12px', textAlign: 'left', fontWeight: 600, fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>模型</th>
            <th style={{ padding: '8px 12px', textAlign: 'right', fontWeight: 600, fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>Prompt</th>
            <th style={{ padding: '8px 12px', textAlign: 'right', fontWeight: 600, fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>Completion</th>
            <th style={{ padding: '8px 12px', textAlign: 'right', fontWeight: 600, fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>费用</th>
            <th style={{ padding: '8px 12px', textAlign: 'right', fontWeight: 600, fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>耗时</th>
            <th style={{ padding: '8px 12px', textAlign: 'center', fontWeight: 600, fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>状态</th>
          </tr>
        </thead>
        <tbody>
          {logs.map(function(l: any) {
            return (
              <tr key={l.id} style={{ borderBottom: '1px solid var(--color-table-row-border)' }}>
                <td style={{ padding: '7px 12px', fontSize: 11, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>
                  {new Date(l.createdAt).toLocaleString('zh-CN', { month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit', second:'2-digit' })}
                </td>
                <td style={{ padding: '7px 12px', fontSize: 12 }}>#{l.userId} {l.username}</td>
                <td style={{ padding: '7px 12px', fontSize: 12 }}>
                  <span style={{ padding: '2px 8px', borderRadius: 10, fontSize: 11, background: 'var(--color-fill-2)', color: 'var(--color-text-2)' }}>{fmtModule(l.module)}</span>
                </td>
                <td style={{ padding: '7px 12px', fontSize: 11, color: 'var(--color-text-2)', fontFamily: "'SF Mono', monospace", maxWidth: 140, overflow: 'hidden', textOverflow: 'ellipsis' }}>{l.modelName}</td>
                <td style={{ padding: '7px 12px', textAlign: 'right', fontSize: 12, fontFamily: "'SF Mono', monospace" }}>{fmtTokens(l.promptTokens)}</td>
                <td style={{ padding: '7px 12px', textAlign: 'right', fontSize: 12, fontFamily: "'SF Mono', monospace" }}>{fmtTokens(l.completionTokens)}</td>
                <td style={{ padding: '7px 12px', textAlign: 'right', fontSize: 12, fontWeight: 600, fontFamily: "'SF Mono', monospace", color: l.costAmount > 0 ? '#F53F3F' : 'var(--color-text-2)' }}>{fmtCost(l.costAmount)}</td>
                <td style={{ padding: '7px 12px', textAlign: 'right', fontSize: 11, color: 'var(--color-text-3)' }}>{fmtMs(l.durationMs)}</td>
                <td style={{ padding: '7px 12px', textAlign: 'center' }}>
                  <span style={{ padding: '2px 8px', borderRadius: 10, fontSize: 11, background: l.success ? 'rgba(0,180,42,0.1)' : 'rgba(245,63,63,0.1)', color: l.success ? '#00B42A' : '#F53F3F' }}>{l.success ? '成功' : '失败'}</span>
                </td>
              </tr>
            );
          })}
          {logs.length === 0 && (
            <tr><td colSpan={9} style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>暂无AI调用记录</td></tr>
          )}
        </tbody>
      </table>

      {total > 20 && (
        <div style={{ marginTop: 12, display: 'flex', justifyContent: 'flex-end' }}>
          <Pagination total={total} current={page} pageSize={20} onChange={function(p: number) { setPage(p); }} sizeCanChange={false} simple />
        </div>
      )}
    </div>
  );
}

const th: React.CSSProperties = { padding: '10px 14px', textAlign: 'left', color: 'var(--color-text-3)', fontWeight: 600, fontSize: 11, textTransform: 'uppercase' };
const td: React.CSSProperties = { padding: '10px 14px', color: 'var(--color-text-2)' };
const btnSm: React.CSSProperties = { display: 'inline-flex', alignItems: 'center', gap: 4, padding: '4px 10px', border: 'none', borderRadius: 4, fontSize: 11, cursor: 'pointer' };
const inputStyle: React.CSSProperties = { width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--color-border-1)', fontSize: 13, outline: 'none', background: 'var(--color-fill-2)', color: 'var(--color-text-1)', boxSizing: 'border-box' };
const cancelBtn: React.CSSProperties = { padding: '8px 18px', background: 'var(--color-fill-2)', color: 'var(--color-text-2)', border: 'none', borderRadius: 6, fontSize: 13, cursor: 'pointer' };
const primaryBtn: React.CSSProperties = { padding: '8px 18px', background: 'var(--color-primary)', color: '#fff', border: 'none', borderRadius: 6, fontSize: 13, cursor: 'pointer' };
