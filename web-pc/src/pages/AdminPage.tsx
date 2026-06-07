import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { listUsers, createUser, resetUserPassword, toggleUser, kickUser, fetchLoginLogs } from '../services/api';
import { useAuth } from '../services/AuthContext';
import {
  Users, Plus, Key, Ban, CheckCircle, XCircle, LogOut,
  LogIn, UserX, AlertCircle, FileText,
} from 'lucide-react';
import { Tabs, Select, Pagination } from '@arco-design/web-react';

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
  login: { label: '登录', color: '#00b42a', icon: LogIn },
  logout: { label: '退出', color: '#86909c', icon: LogOut },
  failed: { label: '失败', color: '#f53f3f', icon: AlertCircle },
  kicked: { label: '被踢', color: '#ff7d00', icon: UserX },
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
      <div style={{ padding: 40, color: '#86909c', textAlign: 'center' }}>
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
        </Tabs>
      </div>

      {msg && (
        <div style={{
          marginBottom: 12, padding: '8px 16px', borderRadius: 6, fontSize: 13,
          background: msg.startsWith('✓') ? '#e8ffea' : '#ffece8',
          color: msg.startsWith('✓') ? '#00b42a' : '#f53f3f',
        }}>{msg}</div>
      )}

      {/* ===== 用户管理 Tab ===== */}
      {activeTab === 'users' && (<>
        {/* Actions */}
        <div style={{ display: 'flex', gap: 10, marginBottom: 16, alignItems: 'center' }}>
          <button onClick={() => setShowCreate(true)} style={{
            display: 'inline-flex', alignItems: 'center', gap: 6, padding: '8px 18px',
            background: '#165dff', color: '#fff', border: 'none', borderRadius: 6, fontSize: 13, cursor: 'pointer',
          }}><Plus size={14} /> 新建用户</button>
          <span style={{ fontSize: 12, color: '#86909c' }}>共 {users.length} 个用户</span>
        </div>

        {/* Create user modal */}
        {showCreate && (
          <div style={{
            position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.35)', zIndex: 999,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }} onClick={() => setShowCreate(false)}>
            <div style={{
              background: '#fff', borderRadius: 12, padding: 24, width: 380,
              boxShadow: '0 8px 30px rgba(0,0,0,0.12)',
            }} onClick={e => e.stopPropagation()}>
              <h3 style={{ margin: '0 0 16px', fontSize: 15, color: '#1d2129' }}>新建用户</h3>
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
                <label style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 13, color: '#4e5969', cursor: 'pointer' }}>
                  <input type="radio" name="role" value="user" checked={newUser.role === 'user'} onChange={() => setNewUser(p => ({ ...p, role: 'user' }))} /> 普通用户
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 13, color: '#4e5969', cursor: 'pointer' }}>
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
              background: '#fff', borderRadius: 12, padding: 24, width: 360,
              boxShadow: '0 8px 30px rgba(0,0,0,0.12)',
            }} onClick={e => e.stopPropagation()}>
              <h3 style={{ margin: '0 0 12px', fontSize: 15, color: '#1d2129' }}>
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
        <div style={{ borderRadius: 10, overflow: 'hidden', border: '1px solid #e5e6eb' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ background: '#f7f8fa' }}>
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
                <tr key={u.id} style={{ borderBottom: '1px solid #f2f3f5' }}>
                  <td style={td}>{u.id}</td>
                  <td style={{ ...td, fontWeight: 600, color: '#1d2129' }}>{u.username}</td>
                  <td style={td}>
                    <span style={{
                      padding: '2px 8px', borderRadius: 4, fontSize: 11,
                      background: u.role === 'admin' ? '#e8f3ff' : '#f2f3f5',
                      color: u.role === 'admin' ? '#165dff' : '#86909c',
                    }}>{u.role === 'admin' ? '管理员' : '用户'}</span>
                  </td>
                  <td style={td}>
                    {u.isActive
                      ? <span style={{ color: '#00b42a', display: 'flex', alignItems: 'center', gap: 4 }}><CheckCircle size={12} /> 正常</span>
                      : <span style={{ color: '#f53f3f', display: 'flex', alignItems: 'center', gap: 4 }}><XCircle size={12} /> 停用</span>
                    }
                  </td>
                  <td style={td}>
                    {u.isOnline
                      ? <span style={{ color: '#00b42a', display: 'flex', alignItems: 'center', gap: 4 }}>
                          <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#00b42a', display: 'inline-block' }} /> 在线
                        </span>
                      : <span style={{ color: '#c9cdd4', display: 'flex', alignItems: 'center', gap: 4 }}>
                          <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#c9cdd4', display: 'inline-block' }} /> 离线
                        </span>
                    }
                    {u.sessionCount > 0 && <span style={{ fontSize: 10, color: '#86909c', marginLeft: 4 }}>({u.sessionCount})</span>}
                  </td>
                  <td style={{ ...td, fontSize: 11, color: '#86909c', maxWidth: 140, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {u.deviceInfo || '-'}
                  </td>
                  <td style={{ ...td, color: '#86909c' }}>{u.lastLoginAt ? new Date(u.lastLoginAt).toLocaleString('zh-CN') : '-'}</td>
                  <td style={{ ...td, color: '#86909c', fontFamily: 'monospace', fontSize: 11 }}>{u.lastLoginIp || '-'}</td>
                  <td style={td}>
                    <div style={{ display: 'flex', gap: 6 }}>
                      <button onClick={() => { setResetTarget({ id: u.id, name: u.username }); setNewPw(''); }}
                        style={{ ...btnSm, background: '#fff7e8', color: '#ff7d00' }}>
                        <Key size={11} /> 重置
                      </button>
                      <button onClick={() => handleKick(u.id, u.username)}
                        style={{ ...btnSm, background: '#fff7e8', color: '#ff7d00' }}>
                        <LogOut size={11} /> 踢下线
                      </button>
                      <button onClick={() => handleToggle(u.id, u.isActive)}
                        style={{
                          ...btnSm,
                          background: u.isActive ? '#ffece8' : '#e8ffea',
                          color: u.isActive ? '#f53f3f' : '#00b42a',
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
                padding: '6px 12px', borderRadius: 6, border: '1px solid #e5e6eb',
                background: '#f7f8fa', color: '#1d2129', fontSize: 13, width: 180,
                outline: 'none',
              }}
              onFocus={e => { e.target.style.borderColor = '#165dff'; e.target.style.background = '#fff'; }}
              onBlur={e => { e.target.style.borderColor = '#e5e6eb'; e.target.style.background = '#f7f8fa'; }}
            />
            <span style={{ flex: 1 }} />
            <span style={{ color: '#86909c', fontSize: 12 }}>共 {logTotal} 条</span>
            <Pagination
              current={logPage} total={logTotal} pageSize={30} size="small" simple
              onChange={(p) => setLogPage(p)}
            />
          </div>
        </div>

        <div style={{ borderRadius: 10, overflow: 'hidden', border: '1px solid #e5e6eb' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ background: '#f7f8fa' }}>
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
                const act = actionLabels[log.action] || { label: log.action, color: '#86909c', icon: AlertCircle };
                const ActIcon = act.icon;
                return (
                  <tr key={log.id} style={{ borderBottom: '1px solid #f2f3f5' }}>
                    <td style={{ ...td, color: '#86909c', whiteSpace: 'nowrap' }}>{new Date(log.createdAt).toLocaleString('zh-CN')}</td>
                    <td style={{ ...td, fontWeight: 600, color: '#1d2129' }}>{log.username}</td>
                    <td style={td}><span style={{ display: 'flex', alignItems: 'center', gap: 4, color: act.color }}><ActIcon size={13} /> {act.label}</span></td>
                    <td style={td}>{log.success
                      ? <span style={{ color: '#00b42a', display: 'flex', alignItems: 'center', gap: 4 }}><CheckCircle size={12} /> 成功</span>
                      : <span style={{ color: '#f53f3f', display: 'flex', alignItems: 'center', gap: 4 }}><XCircle size={12} /> 失败</span>}</td>
                    <td style={{ ...td, fontFamily: 'monospace', fontSize: 11, color: '#86909c' }}>{log.ipAddress}</td>
                    <td style={{ ...td, fontSize: 11, color: '#86909c', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{log.deviceInfo || '-'}</td>
                    <td style={{ ...td, color: '#f53f3f', fontSize: 12 }}>{log.failReason || '-'}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </>)}
    </div>
  );
}

const th: React.CSSProperties = { padding: '10px 14px', textAlign: 'left', color: '#86909c', fontWeight: 600, fontSize: 11, textTransform: 'uppercase' };
const td: React.CSSProperties = { padding: '10px 14px', color: '#4e5969' };
const btnSm: React.CSSProperties = { display: 'inline-flex', alignItems: 'center', gap: 4, padding: '4px 10px', border: 'none', borderRadius: 4, fontSize: 11, cursor: 'pointer' };
const inputStyle: React.CSSProperties = { width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid #e5e6eb', fontSize: 13, outline: 'none', background: '#f7f8fa', color: '#1d2129', boxSizing: 'border-box' };
const cancelBtn: React.CSSProperties = { padding: '8px 18px', background: '#f2f3f5', color: '#4e5969', border: 'none', borderRadius: 6, fontSize: 13, cursor: 'pointer' };
const primaryBtn: React.CSSProperties = { padding: '8px 18px', background: '#165dff', color: '#fff', border: 'none', borderRadius: 6, fontSize: 13, cursor: 'pointer' };
