import { useMemo, useState, useCallback, useRef } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Input, AutoComplete, Badge, Button, Tag } from '@arco-design/web-react';
import { Search, Bell, ChevronRight, Home, Clock, User } from 'lucide-react';
import { useAuth } from '../services/AuthContext';
import { searchStock } from '../services/api';

// ── Breadcrumb route mapping ──
const routeLabels: Record<string, { label: string; parent?: string }> = {
  '/': { label: '今日榜单' },
  '/board/history': { label: '历史榜单', parent: '算法精选' },
  '/board/heatmap': { label: '上榜热力图', parent: '算法精选' },
  '/board/concepts': { label: '概念板块', parent: '算法精选' },
  '/stocks': { label: '股票列表' },
  '/stock': { label: '股票详情', parent: '股票列表' },
  '/watchlist': { label: '自选股' },
  '/strategy': { label: '交易策略' },
  '/pk': { label: '策略PK' },
  '/holdings': { label: '持股管理' },
  '/risk': { label: '风险监控' },
  '/data': { label: '数据管理' },
  '/settings': { label: '系统设置' },
  '/admin': { label: '用户管理' },
  '/concept': { label: '概念板块详情', parent: '概念板块' },
  '/forecast': { label: '价格预测', parent: '股票详情' },
  '/ai': { label: 'AI分析', parent: '股票详情' },
  '/pk/create': { label: '创建活动', parent: '策略PK' },
  '/profile': { label: '个人设置' },
};

// ── Market time check ──
function useMarketStatus(): { isOpen: boolean; label: string } {
  return useMemo(() => {
    const now = new Date();
    const day = now.getDay();
    if (day === 0 || day === 6) return { isOpen: false, label: '休市' };
    const h = now.getHours();
    const m = now.getMinutes();
    const t = h * 60 + m;
    const morningOpen = 9 * 60 + 30;
    const morningClose = 11 * 60 + 30;
    const afternoonOpen = 13 * 60;
    const afternoonClose = 15 * 60;
    if ((t >= morningOpen && t < morningClose) || (t >= afternoonOpen && t < afternoonClose)) {
      return { isOpen: true, label: '交易中' };
    }
    if ((t >= 9 * 60 && t < morningOpen) || (t >= morningClose && t < afternoonOpen)) {
      return { isOpen: false, label: '等待开盘' };
    }
    return { isOpen: false, label: '已收盘' };
  }, []);
}

export default function AppTopbar() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const market = useMarketStatus();

  // ── Search state ──
  const [searchOptions, setSearchOptions] = useState<{ value: string; name: string }[]>([]);
  const [searching, setSearching] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();
  const optionsRef = useRef(searchOptions);
  optionsRef.current = searchOptions;

  const handleSearch = useCallback((value: string) => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    const trimmed = value.trim();
    if (!trimmed) {
      setSearchOptions([]);
      setSearching(false);
      return;
    }
    setSearching(true);
    debounceRef.current = setTimeout(async () => {
      try {
        const res = await searchStock(trimmed);
        const stocks = res.data?.data || [];
        setSearchOptions(stocks.map((s: any) => ({
          value: s.code,
          name: `${s.code}  ${s.name}`,
        })));
      } catch {
        setSearchOptions([]);
      } finally {
        setSearching(false);
      }
    }, 250);
  }, []);

  const handleSelect = useCallback((value: string) => {
    if (value) {
      navigate(`/stock/${value}`);
      setSearchOptions([]);
    }
  }, [navigate]);

  const handlePressEnter = useCallback((_e: any, activeOption?: { value: string }) => {
    const code = activeOption?.value || optionsRef.current[0]?.value;
    if (code) {
      navigate(`/stock/${code}`);
      setSearchOptions([]);
    }
  }, [navigate]);

  // ── Breadcrumb ──
  const breadcrumb = useMemo(() => {
    const parts: { label: string; path?: string }[] = [];
    const path = location.pathname;
    const exact = routeLabels[path];
    if (exact) {
      if (exact.parent) parts.push({ label: exact.parent });
      parts.push({ label: exact.label });
      return parts;
    }
    const segments = path.split('/').filter(Boolean);
    if (segments.length >= 2) {
      const prefix = '/' + segments[0];
      const base = routeLabels[prefix];
      if (base) {
        parts.push({ label: base.label, path: prefix });
        if (segments[0] === 'pk') {
          if (segments[1] && !isNaN(Number(segments[1]))) {
            parts.push({ label: '活动详情' });
            if (segments[2] === 'entry' && segments[3]) {
              parts.push({ label: '策略详情' });
            }
          }
        } else if (segments[0] === 'stock') {
          parts.push({ label: '股票详情' });
        } else if (segments[0] === 'board') {
          parts.push({ label: base.label, path: prefix });
        }
        } else if (segments[0] === 'concept') {
          parts.push({ label: segments[1] || '详情' });
        } else if (segments[0] === 'forecast') {
          parts.push({ label: '价格预测' });
        } else if (segments[0] === 'ai') {
          parts.push({ label: 'AI分析' });
        } else if (segments[0] === 'admin') {
          parts.push({ label: base.label });
        return parts;
      }
    }
    parts.push({ label: segments[segments.length - 1] || '首页', path: '/' });
    return parts;
  }, [location.pathname]);

  return (
    <div className="app-topbar" style={{ gap: 16 }}>
      {/* ── Breadcrumb ── */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexShrink: 0 }}>
        <Button
          type="text"
          size="mini"
          icon={<Home size={14} />}
          onClick={() => navigate('/')}
          style={{ color: 'var(--color-text-3)', padding: '2px 6px' }}
        />
        {breadcrumb.map((part, i) => (
          <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <ChevronRight size={12} style={{ color: 'var(--color-text-3)' }} />
            {part.path ? (
              <span
                onClick={() => navigate(part.path!)}
                style={{
                  fontSize: 13, color: 'var(--color-text-2)', cursor: 'pointer',
                  fontWeight: i === breadcrumb.length - 1 ? 600 : 400,
                  transition: 'color 0.15s',
                }}
                onMouseEnter={e => { e.currentTarget.style.color = 'var(--color-primary)'; }}
                onMouseLeave={e => { e.currentTarget.style.color = i === breadcrumb.length - 1 ? 'var(--color-text-1)' : 'var(--color-text-2)'; }}
              >
                {part.label}
              </span>
            ) : (
              <span style={{ fontSize: 13, color: 'var(--color-text-1)', fontWeight: 600 }}>
                {part.label}
              </span>
            )}
          </div>
        ))}
      </div>

      {/* ── Spacer ── */}
      <div style={{ flex: 1 }} />

      {/* ── Stock Search ── */}
      <AutoComplete
        data={searchOptions}
        filterOption={false}
        onSearch={handleSearch}
        onSelect={handleSelect}
        onPressEnter={handlePressEnter}
        placeholder="搜索股票代码/名称..."
        style={{ width: 260 }}
        triggerElement={
          <Input
            prefix={<Search size={14} style={{ color: 'var(--color-text-3)' }} />}
            style={{ borderRadius: 8 }}
            size="small"
            allowClear
          />
        }
      />

      {/* ── Market Time ── */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 6,
        background: market.isOpen ? 'rgba(0,180,42,0.08)' : 'var(--color-fill-2)',
        borderRadius: 8, padding: '4px 12px', flexShrink: 0,
      }}>
        <Clock size={13} style={{ color: market.isOpen ? '#00B42A' : 'var(--color-text-3)' }} />
        <span style={{
          fontSize: 12, fontWeight: 600,
          color: market.isOpen ? '#00B42A' : 'var(--color-text-3)',
        }}>
          {market.label}
        </span>
        {market.isOpen && (
          <span style={{
            width: 6, height: 6, borderRadius: '50%',
            background: '#00B42A',
            animation: 'pulse 2s ease-in-out infinite',
          }} />
        )}
      </div>

      {/* ── Notification Bell ── */}
      <Badge count={0} dotStyle={{ background: '#F53F3F' }}>
        <Button
          type="text"
          icon={<Bell size={16} />}
          style={{ color: 'var(--color-text-2)', padding: '4px 8px' }}
        />
      </Badge>

      {/* ── User ── */}
      <Button
        type="text"
        onClick={() => navigate('/profile')}
        style={{
          display: 'flex', alignItems: 'center', gap: 8, padding: '4px 10px',
          color: 'var(--color-text-2)', borderRadius: 8, flexShrink: 0,
        }}
      >
        <div style={{
          width: 28, height: 28, borderRadius: '50%',
          background: 'linear-gradient(135deg, var(--color-primary), #722ed1)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          color: '#fff', fontSize: 12, fontWeight: 700, flexShrink: 0,
        }}>
          {(user?.nickname || user?.username || 'U')[0].toUpperCase()}
        </div>
        <span style={{ fontSize: 13, fontWeight: 500 }}>
          {user?.nickname || user?.username || '用户'}
        </span>
      </Button>

      {/* ── Pulse animation for market dot ── */}
      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.4; }
        }
      `}</style>
    </div>
  );
}
