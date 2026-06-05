import { useEffect, useState } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { LayoutDashboard, History, Grid3X3, Star, Target, Briefcase, ShieldAlert, Database } from 'lucide-react';
import '@arco-design/web-react/dist/css/arco.css';
import './styles/app.css';

const navItems = [
  { key: '/', label: '今日榜单', icon: LayoutDashboard },
  { key: '/board/history', label: '历史榜单', icon: History },
  { key: '/board/heatmap', label: '上榜热力图', icon: Grid3X3 },
  { key: '/watchlist', label: '自选股', icon: Star },
  { key: '/strategy', label: '策略中心', icon: Target },
  { key: '/holdings', label: '持仓跟踪', icon: Briefcase },
  { key: '/risk', label: '风险预警', icon: ShieldAlert },
  { key: '/data', label: '数据管理', icon: Database },
];

interface IndexData { name: string; val: number; chg: number; chgPct: number; }

export default function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const [indices, setIndices] = useState<IndexData[]>([
    { name: '上证指数', val: 3247.86, chg: 12.35, chgPct: 0.38 },
    { name: '深证成指', val: 11087.62, chg: -24.18, chgPct: -0.22 },
    { name: '创业板指', val: 2156.43, chg: 8.92, chgPct: 0.41 },
  ]);

  // Simulate live tick every 3s
  useEffect(() => {
    const timer = setInterval(() => {
      setIndices(prev => prev.map(i => {
        const jitter = (Math.random() - 0.48) * i.val * 0.002;
        const newVal = +(i.val + jitter).toFixed(2);
        const newChg = +(i.chg + jitter).toFixed(2);
        return { ...i, val: newVal, chg: newChg, chgPct: +((newChg / (newVal - newChg)) * 100).toFixed(2) };
      }));
    }, 3000);
    return () => clearInterval(timer);
  }, []);

  const selectedKey = navItems.find(
    (m) => m.key === location.pathname || (m.key !== '/' && location.pathname.startsWith(m.key))
  )?.key || '/';

  return (
    <div className="app-shell">
      <aside className="app-sidebar">
        <div className="sidebar-brand">
          <span className="brand-icon">📈</span>
          <span className="brand-text">智策投研</span>
        </div>
        <nav className="sidebar-nav">
          {navItems.map(({ key, label, icon: Icon }) => (
            <button key={key} className={`nav-item${selectedKey === key ? ' active' : ''}`} onClick={() => navigate(key)}>
              <Icon /><span>{label}</span>
            </button>
          ))}
        </nav>
      </aside>
      <div className="app-main">
        {/* Topbar */}
        <header className="app-topbar">
          <span className="title">智策投研 · 股票数据分析平台</span>
          <span className="row gap8" style={{ marginLeft: 'auto', fontSize: 12 }}>
            <span className="live-dot" /> 实时
          </span>
        </header>

        {/* Quote Ticker Bar */}
        <div className="quote-bar">
          {indices.map((idx, i) => (
            <span key={i}>
              <span className="idx">
                <span className="idx-name">{idx.name}</span>
                <span className="idx-val">{idx.val.toFixed(2)}</span>
                <span className={`idx-chg ${idx.chg >= 0 ? 'up' : 'down'}`}>
                  {idx.chg >= 0 ? '+' : ''}{idx.chg.toFixed(2)} ({idx.chgPct >= 0 ? '+' : ''}{idx.chgPct.toFixed(2)}%)
                </span>
              </span>
              {i < indices.length - 1 && <span className="vdiv" />}
            </span>
          ))}
        </div>

        <div className="page-content">
          <Outlet />
        </div>
      </div>
    </div>
  );
}
