import { useState } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { Layout, Menu } from '@arco-design/web-react';
import '@arco-design/web-react/dist/css/arco.css';

const { Sider, Header, Content } = Layout;

const menuItems = [
  { key: '/', label: '今日榜单' },
  { key: '/board/history', label: '历史榜单' },
  { key: '/board/heatmap', label: '上榜热力图' },
  { key: '/watchlist', label: '自选股' },
  { key: '/strategy', label: '策略中心' },
  { key: '/holdings', label: '持仓跟踪' },
  { key: '/risk', label: '风险预警' },
  { key: '/data', label: '数据管理' },
];

export default function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const [collapsed, setCollapsed] = useState(false);

  const selectedKey = menuItems.find(
    (m) => m.key === location.pathname || (m.key !== '/' && location.pathname.startsWith(m.key))
  )?.key || '/';

  return (
    <Layout style={{ height: '100vh' }}>
      <Sider
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        style={{ background: 'var(--color-bg-2)' }}
      >
        <div style={{ padding: '16px', textAlign: 'center', fontWeight: 700, fontSize: 16, color: 'var(--color-primary-6)' }}>
          📈 智策投研
        </div>
        <Menu
          selectedKeys={[selectedKey]}
          onClickMenuItem={(key) => navigate(key)}
          style={{ borderRadius: 0 }}
        >
          {menuItems.map((item) => (
            <Menu.Item key={item.key}>{item.label}</Menu.Item>
          ))}
        </Menu>
      </Sider>
      <Layout>
        <Header style={{ background: '#fff', padding: '0 24px', borderBottom: '1px solid var(--color-border-2)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span style={{ fontSize: 14, color: 'var(--color-text-2)' }}>智策投研 · 股票数据分析平台</span>
        </Header>
        <Content style={{ padding: 24, overflow: 'auto', background: 'var(--color-fill-1)' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
