import { useState, type FormEvent } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '../services/AuthContext';
import { login as loginApi } from '../services/api';
import { LogoLarge } from '../components/Logo';
import { Shield, User, Lock } from 'lucide-react';

export default function LoginPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [deviceWarning, setDeviceWarning] = useState('');
  const { login } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!username || !password) { setError('请输入用户名和密码'); return; }
    setSubmitting(true);
    setError('');
    setDeviceWarning('');
    try {
      const { data } = await loginApi(username, password);
      login(data.data.accessToken, data.data.refreshToken, data.data.user);
      if (data.data.deviceChanged) {
        setDeviceWarning('检测到登录设备变化，如非本人操作请立即修改密码');
      }
      const redirect = params.get('redirect') || '/';
      navigate(redirect, { replace: true });
    } catch (err: any) {
      setError(err.response?.data?.error || '登录失败，请检查网络');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div style={{
      minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center',
      background: 'linear-gradient(135deg, #f0f4ff 0%, #f5f7fa 30%, #fafbfc 60%, #f0f4ff 100%)',
      fontFamily: 'inherit',
    }}>
      <div style={{
        width: 420, padding: '44px 40px 36px', borderRadius: 16,
        background: '#fff', border: '1px solid #e5e6eb',
        boxShadow: '0 2px 12px rgba(0,0,0,0.04), 0 8px 40px rgba(0,0,0,0.06)',
      }}>
        {/* Logo */}
        <div style={{ textAlign: 'center', marginBottom: 28 }}>
          <LogoLarge />
          <h1 style={{ color: '#1d2129', fontSize: 22, fontWeight: 700, marginTop: 12, marginBottom: 0 }}>
            智策投研
          </h1>
          <p style={{ color: '#86909c', fontSize: 13, marginTop: 6 }}>
            股票数据分析平台 · 团队版
          </p>
        </div>

        <form onSubmit={handleSubmit}>
          {/* Username */}
          <div style={{ marginBottom: 16 }}>
            <label style={{ color: '#4e5969', fontSize: 13, fontWeight: 600, display: 'block', marginBottom: 6 }}>
              用户名
            </label>
            <div style={{ position: 'relative' }}>
              <span style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: '#c9cdd4', display: 'flex' }}>
                <User size={16} />
              </span>
              <input
                type="text" value={username} onChange={e => setUsername(e.target.value)}
                autoFocus autoComplete="username"
                style={{
                  width: '100%', padding: '10px 14px 10px 38px', borderRadius: 8,
                  border: '1px solid #e5e6eb', background: '#f7f8fa', color: '#1d2129',
                  fontSize: 15, outline: 'none', boxSizing: 'border-box',
                  transition: 'border-color 0.2s, box-shadow 0.2s',
                }}
                onFocus={e => { e.target.style.borderColor = '#165dff'; e.target.style.boxShadow = '0 0 0 2px rgba(22,93,255,0.1)'; e.target.style.background = '#fff'; }}
                onBlur={e => { e.target.style.borderColor = '#e5e6eb'; e.target.style.boxShadow = 'none'; e.target.style.background = '#f7f8fa'; }}
              />
            </div>
          </div>

          {/* Password */}
          <div style={{ marginBottom: 8 }}>
            <label style={{ color: '#4e5969', fontSize: 13, fontWeight: 600, display: 'block', marginBottom: 6 }}>
              密码
            </label>
            <div style={{ position: 'relative' }}>
              <span style={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: '#c9cdd4', display: 'flex' }}>
                <Lock size={16} />
              </span>
              <input
                type="password" value={password} onChange={e => setPassword(e.target.value)}
                autoComplete="current-password"
                style={{
                  width: '100%', padding: '10px 14px 10px 38px', borderRadius: 8,
                  border: '1px solid #e5e6eb', background: '#f7f8fa', color: '#1d2129',
                  fontSize: 15, outline: 'none', boxSizing: 'border-box',
                  transition: 'border-color 0.2s, box-shadow 0.2s',
                }}
                onFocus={e => { e.target.style.borderColor = '#165dff'; e.target.style.boxShadow = '0 0 0 2px rgba(22,93,255,0.1)'; e.target.style.background = '#fff'; }}
                onBlur={e => { e.target.style.borderColor = '#e5e6eb'; e.target.style.boxShadow = 'none'; e.target.style.background = '#f7f8fa'; }}
              />
            </div>
          </div>

          {/* Error */}
          {error && (
            <div style={{
              color: '#f53f3f', fontSize: 13, marginBottom: 12,
              padding: '8px 12px', background: '#ffece8', borderRadius: 6,
              border: '1px solid #ffccc7',
            }}>
              {error}
            </div>
          )}

          {/* Device warning */}
          {deviceWarning && (
            <div style={{
              color: '#ff7d00', fontSize: 12, marginBottom: 12,
              padding: '8px 12px', background: '#fff7e8', borderRadius: 6,
              border: '1px solid #ffe4ba',
            }}>
              {deviceWarning}
            </div>
          )}

          {/* Submit */}
          <button
            type="submit" disabled={submitting}
            style={{
              width: '100%', padding: '12px', borderRadius: 8, border: 'none',
              background: submitting ? '#94bfff' : '#165dff',
              color: '#fff', fontSize: 15, fontWeight: 600, cursor: submitting ? 'wait' : 'pointer',
              marginTop: 8, transition: 'background 0.2s',
            }}
            onMouseEnter={e => { if (!submitting) e.currentTarget.style.background = '#4080ff'; }}
            onMouseLeave={e => { if (!submitting) e.currentTarget.style.background = '#165dff'; }}
          >
            {submitting ? '登录中...' : '登 录'}
          </button>
        </form>

        {/* Footer */}
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
          marginTop: 24, color: '#c9cdd4', fontSize: 12,
        }}>
          <Shield size={12} />
          <span>仅限团队内部使用 · 登录即表示同意保密协议</span>
        </div>
      </div>
    </div>
  );
}
