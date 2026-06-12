import { useState, useEffect, useCallback } from 'react';
import { Select, Pagination } from '@arco-design/web-react';
import { useAuth } from '../services/AuthContext';
import { fetchLoginLogs } from '../services/api';
import { History, CheckCircle, XCircle, LogIn, LogOut, UserX, AlertCircle, Search } from 'lucide-react';

interface LogEntry {
  id: number;
  userId: number;
  username: string;
  action: string;
  success: boolean;
  ipAddress: string;
  deviceInfo: string;
  failReason: string;
  createdAt: string;
}

const actionLabels: Record<string, { label: string; color: string; icon: any }> = {
  login: { label: '登录', color: '#00b42a', icon: LogIn },
  logout: { label: '退出', color: 'var(--color-text-3)', icon: LogOut },
  failed: { label: '失败', color: '#f53f3f', icon: AlertCircle },
  kicked: { label: '被踢', color: '#ff7d00', icon: UserX },
};

export default function LoginLogsPage() {
  const { isAdmin } = useAuth();
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [action, setAction] = useState('');
  const [keyword, setKeyword] = useState('');

  const load = useCallback(async () => {
    try {
      const { data } = await fetchLoginLogs({ page, pageSize: 30, action, username: keyword });
      setLogs(data.data || []);
      setTotal(data.total || 0);
    } catch {}
  }, [page, action, keyword]);

  useEffect(() => { load(); }, [load]);

  if (!isAdmin) {
    return <div style={{ padding: 40, color: 'var(--color-text-3)', textAlign: 'center' }}>仅管理员可访问</div>;
  }

  return (
    <div>
      <h2 style={{ color: 'var(--color-text-1)', fontSize: 18, fontWeight: 700, marginBottom: 20, display: 'flex', alignItems: 'center', gap: 8 }}>
        <History size={20} color="#165dff" /> 登录日志
      </h2>

      {/* Filters */}
      <div className="card" style={{ marginBottom: 16 }}>
        <div style={{ padding: '12px 20px', display: 'flex', gap: 12, alignItems: 'center' }}>
          <Select
            placeholder="全部类型"
            value={action || undefined}
            onChange={(v: string) => { setAction(v || ''); setPage(1); }}
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
            value={keyword}
            onChange={e => { setKeyword(e.target.value); setPage(1); }}
            style={{
              padding: '6px 12px', borderRadius: 6, border: '1px solid #e5e6eb',
              background: 'var(--color-fill-2)', color: 'var(--color-text-1)', fontSize: 13, width: 180,
              outline: 'none',
            }}
            onFocus={e => { e.target.style.borderColor = '#165dff'; e.target.style.background = '#fff'; }}
            onBlur={e => { e.target.style.borderColor = 'var(--color-border-1)'; e.target.style.background = 'var(--color-fill-2)'; }}
          />
          <span style={{ flex: 1 }} />
          <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>共 {total} 条</span>
          <Pagination
            current={page} total={total} pageSize={30} size="small" simple
            onChange={(p) => setPage(p)}
          />
        </div>
      </div>

      {/* Log table */}
      <div style={{ borderRadius: 10, overflow: 'hidden', border: '1px solid #e5e6eb' }}>
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
                <tr key={log.id} style={{ borderBottom: '1px solid #f2f3f5' }}>
                  <td style={{ ...td, color: 'var(--color-text-3)', whiteSpace: 'nowrap' }}>
                    {new Date(log.createdAt).toLocaleString('zh-CN')}
                  </td>
                  <td style={{ ...td, fontWeight: 600, color: 'var(--color-text-1)' }}>{log.username}</td>
                  <td style={td}>
                    <span style={{ display: 'flex', alignItems: 'center', gap: 4, color: act.color }}>
                      <ActIcon size={13} /> {act.label}
                    </span>
                  </td>
                  <td style={td}>
                    {log.success
                      ? <span style={{ color: '#00b42a', display: 'flex', alignItems: 'center', gap: 4 }}><CheckCircle size={12} /> 成功</span>
                      : <span style={{ color: '#f53f3f', display: 'flex', alignItems: 'center', gap: 4 }}><XCircle size={12} /> 失败</span>
                    }
                  </td>
                  <td style={{ ...td, fontFamily: 'monospace', fontSize: 11, color: 'var(--color-text-3)' }}>{log.ipAddress}</td>
                  <td style={{ ...td, fontSize: 11, color: 'var(--color-text-3)', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {log.deviceInfo || '-'}
                  </td>
                  <td style={{ ...td, color: '#f53f3f', fontSize: 12 }}>{log.failReason || '-'}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

const th: React.CSSProperties = { padding: '10px 14px', textAlign: 'left', color: 'var(--color-text-3)', fontWeight: 600, fontSize: 11, textTransform: 'uppercase' };
const td: React.CSSProperties = { padding: '10px 14px', color: 'var(--color-text-2)' };
