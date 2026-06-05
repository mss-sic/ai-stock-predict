import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import {
  LayoutDashboard, History, Grid3X3, Star, Target, Briefcase, ShieldAlert, Database
} from 'lucide-react';
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

export default function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
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
            <button
              key={key}
              className={`nav-item${selectedKey === key ? ' active' : ''}`}
              onClick={() => navigate(key)}
            >
              <Icon />
              <span>{label}</span>
            </button>
          ))}
        </nav>
      </aside>
      <div className="app-main">
        <header className="app-topbar">
          <span className="title">智策投研 · 股票数据分析平台</span>
        </header>
        <div className="page-content">
          <Outlet />
        </div>
      </div>
    </div>
  );
}
