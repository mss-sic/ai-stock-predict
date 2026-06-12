import { useEffect, useState } from 'react';
import ToastContainer, { showToast } from './components/Toast';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from './services/AuthContext';
import { fetchIndices, logout as logoutApi, getAccessToken, heartbeat } from './services/api';
import Logo from './components/Logo';
import { LayoutDashboard, History, Grid3X3, Layers, Star, Target, Briefcase, ShieldAlert, Database, Search, Settings, LogOut, UserCog, Shield, Sun, Moon, Trophy } from 'lucide-react';
import { useTheme } from './services/ThemeContext';
import '@arco-design/web-react/dist/css/arco.css';
import './styles/app.css';
import PkNotice from './components/PkNotice';

const navItems = [
  { key: '/', label: '今日榜单', icon: LayoutDashboard },
  { key: '/board/history', label: '历史榜单', icon: History },
  { key: '/board/heatmap', label: '上榜热力图', icon: Grid3X3 },
  { key: '/board/concepts', label: '概念板块', icon: Layers },
  { key: '/stocks', label: '股票列表', icon: Search },
  { key: '/watchlist', label: '自选股', icon: Star },
  { key: '/strategy', label: '交易策略', icon: Target },
  { key: '/pk', label: '策略PK', icon: Trophy },
  { key: '/holdings', label: '持股管理', icon: Briefcase },
  { key: '/risk', label: '风险监控', icon: ShieldAlert },
  { key: '/data', label: '数据管理', icon: Database },
  { key: '/settings', label: '系统设置', icon: Settings },
];

interface IndexData { name: string; code: string; val: number; chg: number; chgPct: number; }

export default function AppLayout() {
  const { user, logout } = useAuth();
  const { theme, toggle, isDark } = useTheme();
  const navigate = useNavigate();
  const location = useLocation();
  const [indices, setIndices] = useState<IndexData[]>([]);
  const [showUserMenu, setShowUserMenu] = useState(false);

  useEffect(() => {
    const token = getAccessToken();
    if (!token) return;
    const fetch = () => fetchIndices().then((r: any) => r.data && setIndices((r.data?.data?.indices || r.data?.data || []))).catch(() => {});
    fetch();
    const timer = setInterval(fetch, 4000);
    return () => clearInterval(timer);
  }, []);

  // Heartbeat
  useEffect(() => {
    if (!user) return;
    const hb = setInterval(() => { heartbeat().catch(() => {}); }, 60000);
    return () => clearInterval(hb);
  }, [user]);

  // Toast event listener
  useEffect(() => {
    const handler = (e: Event) => {
      const { type, message } = (e as CustomEvent).detail || {};
      if (message) showToast(type || 'error', message);
    };
    window.addEventListener('app:toast', handler);
    return () => window.removeEventListener('app:toast', handler);
  }, []);

  const selectedKey = navItems.find(
    (m) => m.key === location.pathname || (m.key !== '/' && location.pathname.startsWith(m.key))
  )?.key || '/';

  const handleLogout = async () => {
    try { await logoutApi(); } catch (_) {}
    logout();
    navigate('/login');
  };

  return (
    <>
      <ToastContainer />
      <PkNotice />
      <div className="app-shell">
      <aside className="app-sidebar">
        <div className="sidebar-brand" style={{ borderColor: 'var(--color-border-1)' }}>
          <span className="brand-icon"><Logo size={24} /></span>
          <span className="brand-text">智策投研</span>
          <button
            onClick={toggle}
            title={isDark ? '切换亮色主题' : '切换暗色主题'}
            style={{
              marginLeft: 'auto', background: 'var(--color-fill-2)', border: '1px solid var(--color-border-1)',
              color: 'var(--color-text-2)', cursor: 'pointer',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              width: 36, height: 36, padding: 0, borderRadius: 8,
              fontSize: 18, transition: 'all 0.2s'
            }}
            onMouseEnter={e => { e.currentTarget.style.background = 'var(--color-border-1)'; e.currentTarget.style.color = 'var(--color-text-1)'; }}
            onMouseLeave={e => { e.currentTarget.style.background = 'var(--color-fill-2)'; e.currentTarget.style.color = 'var(--color-text-2)'; }}
          >
            {isDark ? <Sun size={18} /> : <Moon size={18} />}
          </button>
        </div>
        <nav className="sidebar-nav">
          {navItems.map(({ key, label, icon: Icon }) => (
            <button
              key={key}
              className={`nav-item${key === selectedKey ? ' active' : ''}`}
              onClick={() => navigate(key)}
            >
              <Icon size={16} />
              <span>{label}</span>
            </button>
          ))}
        </nav>
        {/* User area */}
        <div style={{ marginTop: 'auto', borderTop: '1px solid var(--color-border-1)', position: 'relative' }}>
          {user?.role === 'admin' && (
            <button
              className={`nav-item${location.pathname === '/admin' ? ' active' : ''}`}
              onClick={() => navigate('/admin')}
              style={{ width: '100%' }}
            >
              <Shield size={16} />
              <span>用户管理</span>
            </button>
          )}
          <button
            className="nav-item"
            onClick={() => setShowUserMenu(!showUserMenu)}
            style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 8, padding: '8px 14px' }}
          >
            <UserCog size={16} />
            <span style={{ flex: 1, textAlign: 'left', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {user?.nickname || user?.username || '用户'}
            </span>
            <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>▼</span>
          </button>
          {showUserMenu && (
            <>
              <div style={{ position: 'fixed', inset: 0, zIndex: 99 }} onClick={() => setShowUserMenu(false)} />
              <div style={{
                position: 'absolute', bottom: '100%', left: 8, right: 8,
                background: 'var(--color-bg-1)', border: '1px solid var(--color-border-1)',
                borderRadius: 8, padding: 4, zIndex: 100,
                boxShadow: '0 4px 16px rgba(0,0,0,0.1)',
              }}>
                <button
                  onClick={() => { setShowUserMenu(false); navigate('/profile'); }}
                  style={{ display: 'flex', alignItems: 'center', gap: 8, width: '100%', padding: '8px 12px', border: 'none', background: 'none', color: '#4e5969', fontSize: 13, cursor: 'pointer', borderRadius: 6 }}
                  onMouseEnter={e => e.currentTarget.style.background = 'var(--color-fill-2)'}
                  onMouseLeave={e => e.currentTarget.style.background = 'none'}
                >
                  <Settings size={14} /> 个人设置
                </button>
                <button
                  onClick={() => { setShowUserMenu(false); handleLogout(); }}
                  style={{ display: 'flex', alignItems: 'center', gap: 8, width: '100%', padding: '8px 12px', border: 'none', background: 'none', color: '#f53f3f', fontSize: 13, cursor: 'pointer', borderRadius: 6 }}
                  onMouseEnter={e => e.currentTarget.style.background = 'var(--red-1)'}
                  onMouseLeave={e => e.currentTarget.style.background = 'none'}
                >
                  <LogOut size={14} /> 退出登录
                </button>
              </div>
            </>
          )}
        </div>
      </aside>

      <main className="app-main">
        {/* Market indices */}
        <div style={{ display: 'flex', gap: 0, marginBottom: 16, background: 'var(--color-bg-1)', borderRadius: 10, padding: '10px 20px', border: '1px solid var(--color-border-1)' }}>
          {indices.map((idx, i) => (
            <span key={idx.code} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12 }}>
              <span style={{ color: 'var(--color-text-3)', fontWeight: 500 }}>{idx.name}</span>
              <span style={{ color: 'var(--color-text-1)', fontWeight: 600, fontFamily: 'monospace' }}>{idx.val.toFixed(2)}</span>
              <span style={{ color: idx.chg >= 0 ? '#f53f3f' : '#00b42a', fontFamily: 'monospace', fontSize: 11 }}>
                {idx.chg >= 0 ? '+' : ''}{idx.chg.toFixed(2)} ({idx.chg >= 0 ? '+' : ''}{idx.chgPct.toFixed(2)}%)
              </span>
              {i < indices.length - 1 && <span style={{ margin: '0 12px', width: 1, height: 14, background: 'var(--color-border-1)' }} />}
            </span>
          ))}
        </div>

        <div className="page-content">
          <Outlet />
        </div>
      </main>
    </div>
    </>
  );
}
