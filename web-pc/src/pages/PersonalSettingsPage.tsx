import { useState, useEffect } from 'react';
import { Button, Message } from '@arco-design/web-react';
import { User, Key, Shield, Monitor, Smartphone, Globe } from 'lucide-react';
import { changePassword, getMySessions, revokeSession, updateProfile } from '../services/api';
import { useAuth } from '../services/AuthContext';

export default function PersonalSettingsPage() {
  const { user, refreshUser } = useAuth();
  const [nickname, setNickname] = useState(user?.nickname || '');
  const [nickMsg, setNickMsg] = useState('');
  const [nickSaving, setNickSaving] = useState(false);

  const [oldPw, setOldPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [pwMsg, setPwMsg] = useState('');
  const [pwChanging, setPwChanging] = useState(false);

  const [sessions, setSessions] = useState<any[]>([]);

  useEffect(() => {
    getMySessions().then(({ data }) => setSessions(data.data || [])).catch(() => {});
  }, []);

  const handleUpdateNickname = async () => {
    if (!nickname.trim()) { setNickMsg('昵称不能为空'); return; }
    setNickSaving(true);
    try {
      await updateProfile(nickname.trim());
      setNickMsg('✓ 昵称已更新');
      refreshUser?.();
    } catch (err: any) {
      setNickMsg('✗ ' + (err.response?.data?.error || '更新失败'));
    }
    setNickSaving(false);
  };

  const handleChangePassword = async () => {
    if (!oldPw || !newPw) { setPwMsg('请填写新旧密码'); return; }
    if (newPw.length < 6) { setPwMsg('新密码至少6位'); return; }
    setPwChanging(true);
    try {
      const { data } = await changePassword(oldPw, newPw);
      setPwMsg('✓ ' + data.data);
      setOldPw(''); setNewPw('');
    } catch (err: any) {
      setPwMsg('✗ ' + (err.response?.data?.error || '修改失败'));
    }
    setPwChanging(false);
  };

  const handleRevoke = async (id: number) => {
    try {
      await revokeSession(id);
      setSessions(sessions.filter(s => s.id !== id));
      Message.success('会话已撤销');
    } catch { Message.error('撤销失败'); }
  };

  const inp: React.CSSProperties = {
    width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid #e5e6eb',
    background: 'var(--color-fill-2)', color: 'var(--color-text-1)', fontSize: 14, outline: 'none', boxSizing: 'border-box',
  };

  return (
    <div style={{ padding: '0 0 40px', maxWidth: 720 }}>
      {/* 个人信息 */}
      <div className="card" style={{ marginBottom: 16 }}>
        <div className="card-header" style={{ fontSize: 14, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
          <User size={16} color="#165dff" /> 个人信息
        </div>
        <div className="card-body">
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div>
              <label style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4, display: 'block' }}>用户名</label>
              <input disabled value={user?.username || ''} style={{ ...inp, background: '#f2f3f5', color: 'var(--color-text-3)' }} />
            </div>
            <div>
              <label style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4, display: 'block' }}>昵称</label>
              <div style={{ display: 'flex', gap: 8 }}>
                <input
                  value={nickname}
                  onChange={e => { setNickname(e.target.value); setNickMsg(''); }}
                  placeholder="输入昵称"
                  style={{ ...inp, flex: 1 }}
                />
                <Button onClick={handleUpdateNickname} loading={nickSaving} type="primary" size="small">保存</Button>
              </div>
              {nickMsg && (
                <div style={{ fontSize: 12, marginTop: 4, color: nickMsg.startsWith('✓') ? '#00b42a' : '#f53f3f' }}>{nickMsg}</div>
              )}
            </div>
            <div>
              <label style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4, display: 'block' }}>角色</label>
              <span style={{
                display: 'inline-block', padding: '2px 10px', borderRadius: 4, fontSize: 12,
                background: user?.role === 'admin' ? '#e8f3ff' : '#f2f3f5',
                color: user?.role === 'admin' ? '#165dff' : 'var(--color-text-3)',
              }}>{user?.role === 'admin' ? '管理员' : '普通用户'}</span>
            </div>
          </div>
        </div>
      </div>

      {/* 修改密码 */}
      <div className="card" style={{ marginBottom: 16 }}>
        <div className="card-header" style={{ fontSize: 14, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
          <Key size={16} color="#165dff" /> 修改密码
        </div>
        <div className="card-body">
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10, maxWidth: 360 }}>
            <input type="password" value={oldPw} onChange={e => setOldPw(e.target.value)} placeholder="当前密码" style={inp} />
            <input type="password" value={newPw} onChange={e => setNewPw(e.target.value)} placeholder="新密码（至少6位）" style={inp} />
            {pwMsg && <div style={{ fontSize: 12, color: pwMsg.startsWith('✓') ? '#00b42a' : '#f53f3f' }}>{pwMsg}</div>}
            <Button onClick={handleChangePassword} loading={pwChanging} type="primary" size="small" style={{ width: 100 }}>修改密码</Button>
          </div>
        </div>
      </div>

      {/* 登录设备 */}
      <div className="card">
        <div className="card-header" style={{ fontSize: 14, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
          <Monitor size={16} color="#165dff" /> 登录设备
        </div>
        <div className="card-body" style={{ padding: 0 }}>
          {sessions.length === 0 ? (
            <div style={{ padding: 20, color: 'var(--color-text-3)', fontSize: 13, textAlign: 'center' }}>暂无活跃会话</div>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
              <thead>
                <tr style={{ background: 'var(--color-fill-2)' }}>
                  <th style={th}>状态</th>
                  <th style={th}>设备信息</th>
                  <th style={th}>IP 地址</th>
                  <th style={th}>登录时间</th>
                  <th style={th}>最后心跳</th>
                  <th style={{ ...th, textAlign: 'center' }}>操作</th>
                </tr>
              </thead>
              <tbody>
                {sessions.map((s: any) => (
                  <tr key={s.id} style={{ borderBottom: '1px solid #f2f3f5' }}>
                    <td style={td}>
                      {s.isOnline
                        ? <span style={{ color: '#00b42a', display: 'flex', alignItems: 'center', gap: 4 }}>
                            <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#00b42a', display: 'inline-block' }} /> 在线
                          </span>
                        : <span style={{ color: '#c9cdd4' }}>离线</span>
                      }
                    </td>
                    <td style={{ ...td, fontSize: 11, maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{s.deviceInfo || '未知设备'}</td>
                    <td style={{ ...td, fontFamily: 'monospace', fontSize: 11 }}>{s.ipAddress}</td>
                    <td style={{ ...td, color: 'var(--color-text-3)' }}>{new Date(s.createdAt).toLocaleString('zh-CN')}</td>
                    <td style={{ ...td, color: 'var(--color-text-3)', fontSize: 11 }}>{s.lastHeartbeat ? new Date(s.lastHeartbeat).toLocaleTimeString('zh-CN') : '-'}</td>
                    <td style={{ ...td, textAlign: 'center' }}>
                      <button onClick={() => handleRevoke(s.id)}
                        style={{ ...btnSm, background: '#ffece8', color: '#f53f3f', cursor: 'pointer' }}>
                        下线
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
}

const th: React.CSSProperties = { padding: '10px 14px', textAlign: 'left', color: 'var(--color-text-3)', fontWeight: 600, fontSize: 11 };
const td: React.CSSProperties = { padding: '10px 14px', color: 'var(--color-text-2)' };
const btnSm: React.CSSProperties = { display: 'inline-flex', alignItems: 'center', gap: 4, padding: '4px 10px', border: 'none', borderRadius: 4, fontSize: 11 };
