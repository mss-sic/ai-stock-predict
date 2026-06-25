import { useEffect, useState, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Input, Select, Table, Button, Tag, Pagination, Message, Modal, Tabs, Spin, Tooltip } from '@arco-design/web-react';
import {
  Star, Search, Eye, StarOff, TrendingUp, TrendingDown, Minus,
  BarChart3, DollarSign, Activity, Zap, AlertTriangle,
  Layers, ChevronUp, ChevronDown, ArrowUp, ArrowDown,
} from 'lucide-react';
import {
  fetchStocks, fetchMarketSnapshot, fetchStockRanking, fetchUnusualStocks,
  fetchBoardTypeCounts, fetchIndices,
  addToWatchlist, removeFromWatchlist, fetchWatchlist, fetchWatchlistGroups, createWatchlistGroup,
  fetchAppearanceStats,
} from '../services/api';

// ── Constants ──
const PAGE_SIZE = 30;

const BOARD_TABS = [
  { key: '', label: '全部' },
  { key: 'main', label: '沪深主板' },
  { key: 'cy', label: '创业板' },
  { key: 'kc', label: '科创板' },
  { key: 'bj', label: '北交所' },
  { key: 'etf-bond', label: 'ETF/国债' },
];

const VIEW_MODES = [
  { key: 'all', label: '全部', sortBy: '', asc: false },
  { key: 'gainers', label: '涨幅榜', sortBy: 'chgPct', asc: false },
  { key: 'losers', label: '跌幅榜', sortBy: 'chgPct', asc: true },
  { key: 'amount', label: '成交额榜', sortBy: 'amount', asc: false },
  { key: 'turnover', label: '换手率榜', sortBy: 'turnoverRate', asc: false },
  { key: 'unusual', label: '异动监控', sortBy: '', asc: false },
  { key: 'appearance', label: '上榜频率', sortBy: '', asc: false },
];

interface StockRow {
  code: string; name: string; industry: string; boardType: string;
  isST: boolean; close: number; chgPct: number;
  volume: number; amount: number; turnoverRate: number; tradeDate: string;
}

interface AppearanceRow {
  code: string; name: string; industry: string; boardType: string;
  appear5d: number; appear20d: number; close: number; chgPct: number;
}

interface UnusualRow extends StockRow {
  unusualTypes: string[];
  amplitude: number;
  avgVol20: number;
}

interface Snapshot {
  tradeDate: string; upCount: number; downCount: number; flatCount: number;
  totalStocks: number; limitUpCount: number; limitDownCount: number;
  amount: number; prevAmount: number; change: number; changePct: number;
  compositeScore: number;
  shAmount?: number; szAmount?: number; cyAmount?: number;
  kcAmount?: number; bjAmount?: number;
  shUp?: number; shDown?: number; shFlat?: number;
  szUp?: number; szDown?: number; szFlat?: number;
  cyUp?: number; cyDown?: number; cyFlat?: number;
}

interface IndexData { name: string; code: string; val: number; chg: number; chgPct: number; high?: number; low?: number; open?: number; amount?: number; }

// ── Helpers ──

const idxAmountMap: Record<string, keyof Snapshot> = {
  '000001': 'shAmount',
  '399001': 'szAmount',
  '399006': 'cyAmount',
};

function getBoardStats(idx: IndexData, snap: Snapshot | null): { up: number; down: number; flat: number } | null {
  if (!snap) return null;
  if (idx.code === '000001' && snap.shUp != null) return { up: snap.shUp!, down: snap.shDown!, flat: snap.shFlat! };
  if (idx.code === '399001' && snap.szUp != null) return { up: snap.szUp!, down: snap.szDown!, flat: snap.szFlat! };
  if (idx.code === '399006' && snap.cyUp != null) return { up: snap.cyUp!, down: snap.cyDown!, flat: snap.cyFlat! };
  return null;
}
function formatAmount(yi: number | null | undefined): string {
  if (yi == null || isNaN(yi) || yi === 0) return '—';
  if (yi >= 10000) return (yi / 10000).toFixed(2) + '万亿';
  return yi.toFixed(0) + '亿';
}

function formatVol(vol: number): string {
  if (vol >= 1e8) return (vol / 1e8).toFixed(2) + '亿';
  if (vol >= 1e4) return (vol / 1e4).toFixed(1) + '万';
  return String(vol);
}

function formatPct(v: number | null | undefined): string {
  if (v == null || isNaN(v)) return '—';
  const sign = v > 0 ? '+' : '';
  return sign + v.toFixed(2) + '%';
}

export default function StockListPage() {
  const navigate = useNavigate();
  const [stocks, setStocks] = useState<StockRow[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [industry, setIndustry] = useState('');
  const [page, setPage] = useState(1);
  const [boardType, setBoardType] = useState('');
  const [viewMode, setViewMode] = useState('all');
  const [watched, setWatched] = useState<Set<string>>(new Set());
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [indices, setIndices] = useState<IndexData[]>([]);
  const [unusualStocks, setUnusualStocks] = useState<UnusualRow[]>([]);
  const [appearanceStocks, setAppearanceStocks] = useState<AppearanceRow[]>([]);
  const [boardCounts, setBoardCounts] = useState<Record<string, number>>({});

  // ── Load main data ──
  const loadStocks = useCallback(async () => {
    setLoading(true);
    try {
      if (viewMode === 'unusual') {
        const res: any = await fetchUnusualStocks({ boardType, limit: 50 });
        const data = res.data?.data || [];
        setUnusualStocks(data);
        setStocks([]);
        setTotal(data.length);
      } else if (viewMode === 'appearance') {
        const res: any = await fetchAppearanceStats({ topN: 50, limit: 100 });
        const data = res.data?.data || [];
        setAppearanceStocks(data);
        setStocks([]);
        setTotal(data.length);
      } else {
        const mode = VIEW_MODES.find(m => m.key === viewMode);
        const res: any = await fetchStocks({
          page, pageSize: PAGE_SIZE, keyword, industry,
          boardType, sortBy: mode?.sortBy || '', sortDir: mode?.asc ? 'asc' : 'desc',
        });
        setStocks(res.data?.data || []);
        setTotal(res.data?.total || 0);
      }
    } catch { setStocks([]); setUnusualStocks([]); }
    setLoading(false);
  }, [page, keyword, industry, boardType, viewMode]);

  // ── Load watchlist ──
  const loadWatchlist = async () => {
    try {
      const res: any = await fetchWatchlist();
      const codes = new Set((res.data?.data || []).map((i: any) => i.stockCode));
      setWatched(codes);
    } catch {}
  };

  // ── Load snapshot + indices ──
  const loadSnapshot = useCallback(async () => {
    try {
      const [snapRes, idxRes] = await Promise.all([
        fetchMarketSnapshot(),
        fetchIndices(),
      ]);
      setSnapshot(snapRes.data?.data || null);
      setIndices((idxRes.data?.data?.indices || idxRes.data?.data || []));
    } catch {}
  }, []);

  // ── Load board counts ──
  const loadBoardCounts = useCallback(async () => {
    try {
      const res: any = await fetchBoardTypeCounts();
      setBoardCounts(res.data?.data || {});
    } catch {}
  }, []);

  useEffect(() => { loadStocks(); loadWatchlist(); }, [loadStocks]);
  useEffect(() => { loadSnapshot(); loadBoardCounts(); }, [loadSnapshot, loadBoardCounts]);

  // Poll snapshot every 10s during trading hours
  useEffect(() => {
    const timer = setInterval(loadSnapshot, 10000);
    return () => clearInterval(timer);
  }, [loadSnapshot]);

  // ── Watchlist modal ──
  const [addStockCode, setAddStockCode] = useState('');
  const [addGroupId, setAddGroupId] = useState<number>(0);
  const [groups, setGroups] = useState<any[]>([]);
  const [newGroupInput, setNewGroupInput] = useState('');
  const [showAddModal, setShowAddModal] = useState(false);

  useEffect(() => {
    fetchWatchlistGroups().then(({ data }) => setGroups(data.data || [])).catch(() => {});
  }, [watched]);

  const toggleWatch = async (code: string) => {
    if (watched.has(code)) {
      await removeFromWatchlist(code);
      watched.delete(code);
      setWatched(new Set(watched));
      Message.success('已取消自选');
    } else {
      setAddStockCode(code);
      setAddGroupId(0);
      setShowAddModal(true);
    }
  };

  const handleAddWithGroup = async () => {
    if (!addStockCode) return;
    try {
      if (newGroupInput.trim()) {
        const { data } = await createWatchlistGroup(newGroupInput.trim());
        const gid = data.data?.id || 0;
        await addToWatchlist(addStockCode, gid);
        setNewGroupInput('');
      } else {
        await addToWatchlist(addStockCode, addGroupId);
      }
      watched.add(addStockCode);
      setWatched(new Set(watched));
      setShowAddModal(false);
      Message.success('已添加自选');
    } catch (err: any) {
      Message.error(err.response?.data?.message || err.message || '添加失败');
    }
  };

  // ── Board type tab with counts ──
  const boardTabsWithCounts = useMemo(() => {
    const shCount = (boardCounts['sh'] || 0);
    const szCount = (boardCounts['sz'] || 0);
    const mainCount = shCount + szCount;
    return BOARD_TABS.map(t => {
      let count = 0;
      if (t.key === '') {
        count = Object.values(boardCounts).reduce((a, b) => a + b, 0);
      } else if (t.key === 'main') {
        count = mainCount;
      } else if (t.key === 'etf-bond') {
        count = (boardCounts['etf'] || 0) + (boardCounts['bond'] || 0);
      } else {
        count = boardCounts[t.key] || 0;
      }
      return { ...t, count };
    });
  }, [boardCounts]);

  // ── Table columns ──
  const columns = useMemo(() => [
    {
      title: '代码', dataIndex: 'code', width: 90, fixed: 'left' as const,
      render: (v: string) => (
        <span style={{ fontFamily: "'SF Mono', monospace", fontWeight: 600, fontSize: 12, color: 'var(--color-text-1)' }}>
          {v}
        </span>
      ),
    },
    {
      title: '名称', dataIndex: 'name', width: 120, fixed: 'left' as const,
      render: (v: string, r: StockRow) => (
        <span
          style={{ cursor: 'pointer', color: 'var(--color-primary)', fontWeight: 500, fontSize: 13 }}
          onClick={() => navigate(`/stock/${r.code}`)}
        >
          {v}
          {r.isST && <Tag size="small" color="red" style={{ marginLeft: 4, fontSize: 10, lineHeight: '16px' }}>ST</Tag>}
        </span>
      ),
    },
    {
      title: '现价', dataIndex: 'close', width: 80,
      render: (v: number) => v ? (
        <span style={{ fontFamily: "'SF Mono', monospace", fontWeight: 600, fontSize: 12 }}>
          {v.toFixed(2)}
        </span>
      ) : <span className="muted">—</span>,
    },
    {
      title: '涨跌幅', dataIndex: 'chgPct', width: 90,
      render: (v: number) => {
        if (v == null || isNaN(v)) return <span className="muted">—</span>;
        const color = v > 0 ? 'var(--stock-up)' : v < 0 ? 'var(--stock-down)' : 'var(--stock-flat)';
        const bg = v > 0 ? 'var(--stock-up-soft)' : v < 0 ? 'var(--stock-down-soft)' : 'transparent';
        const Icon = v > 0 ? ArrowUp : v < 0 ? ArrowDown : Minus;
        return (
          <span style={{
            display: 'inline-flex', alignItems: 'center', gap: 2,
            color, background: bg, borderRadius: 4, padding: '1px 6px',
            fontFamily: "'SF Mono', monospace", fontWeight: 600, fontSize: 12,
          }}>
            <Icon size={11} />{formatPct(v)}
          </span>
        );
      },
    },
    {
      title: '成交额', dataIndex: 'amount', width: 100,
      render: (v: number) => v ? (
        <span style={{ fontFamily: "'SF Mono', monospace", fontSize: 11 }}>
          {formatAmount(v / 1e8)}
        </span>
      ) : <span className="muted">—</span>,
    },
    {
      title: '换手率', dataIndex: 'turnoverRate', width: 80,
      render: (v: number) => v ? (
        <span style={{ fontFamily: "'SF Mono', monospace", fontSize: 11 }}>{v.toFixed(2)}%</span>
      ) : <span className="muted">—</span>,
    },
    {
      title: '行业', dataIndex: 'industry', width: 100,
      render: (v: string) => v ? (
        <Tag size="small" style={{ background: 'var(--color-fill-2)', border: 'none', fontSize: 11 }}>{v}</Tag>
      ) : <span className="muted">—</span>,
    },
    {
      title: '', width: 44, fixed: 'right' as const,
      render: (_: any, r: StockRow) => (
        <Button
          type="text" size="mini"
          icon={watched.has(r.code) ? <StarOff size={14} /> : <Star size={14} />}
          onClick={(e) => { e.stopPropagation(); toggleWatch(r.code); }}
          style={{ color: watched.has(r.code) ? '#f7ba1e' : 'var(--color-text-3)' }}
        />
      ),
    },
  ], [watched, navigate]);

  // Appearance mode columns
  const appearanceColumns = useMemo(() => [
    {
      title: '代码', dataIndex: 'code', width: 90, fixed: 'left' as const,
      render: (v: string) => (
        <span style={{ fontFamily: "'SF Mono', monospace", fontWeight: 600, fontSize: 12, color: 'var(--color-text-1)' }}>
          {v}
        </span>
      ),
    },
    {
      title: '名称', dataIndex: 'name', width: 120, fixed: 'left' as const,
      render: (v: string, r: AppearanceRow) => (
        <span
          style={{ cursor: 'pointer', color: 'var(--color-primary)', fontWeight: 500, fontSize: 13 }}
          onClick={() => navigate(`/stock/${r.code}`)}
        >
          {v}
        </span>
      ),
    },
    {
      title: '近5日上榜', dataIndex: 'appear5d', width: 100,
      render: (v: number) => (
        <span style={{
          display: 'inline-flex', alignItems: 'center', gap: 4,
          fontFamily: "'SF Mono', monospace", fontWeight: 700, fontSize: 13,
          color: v >= 3 ? '#f53f3f' : v >= 2 ? '#ff7d00' : 'var(--color-text-1)',
        }}>
          <Zap size={12} style={{ color: v >= 3 ? '#f53f3f' : v >= 2 ? '#ff7d00' : 'var(--color-text-3)' }} />
          {v}次
        </span>
      ),
      sorter: (a: AppearanceRow, b: AppearanceRow) => a.appear5d - b.appear5d,
    },
    {
      title: '近20日上榜', dataIndex: 'appear20d', width: 110,
      render: (v: number) => (
        <span style={{
          display: 'inline-flex', alignItems: 'center', gap: 4,
          fontFamily: "'SF Mono', monospace", fontWeight: 700, fontSize: 13,
          color: v >= 8 ? '#f53f3f' : v >= 5 ? '#ff7d00' : 'var(--color-text-1)',
        }}>
          <TrendingUp size={12} style={{ color: v >= 8 ? '#f53f3f' : v >= 5 ? '#ff7d00' : 'var(--color-text-3)' }} />
          {v}次
        </span>
      ),
      sorter: (a: AppearanceRow, b: AppearanceRow) => a.appear20d - b.appear20d,
    },
    {
      title: '现价', dataIndex: 'close', width: 80,
      render: (v: number) => v ? (
        <span style={{ fontFamily: "'SF Mono', monospace", fontWeight: 600, fontSize: 12 }}>
          {v.toFixed(2)}
        </span>
      ) : <span className="muted">—</span>,
    },
    {
      title: '涨跌幅', dataIndex: 'chgPct', width: 90,
      render: (v: number) => {
        if (v == null || isNaN(v)) return <span className="muted">—</span>;
        const color = v > 0 ? 'var(--stock-up)' : v < 0 ? 'var(--stock-down)' : 'var(--stock-flat)';
        const bg = v > 0 ? 'var(--stock-up-soft)' : v < 0 ? 'var(--stock-down-soft)' : 'transparent';
        const Icon = v > 0 ? ArrowUp : v < 0 ? ArrowDown : Minus;
        return (
          <span style={{
            display: 'inline-flex', alignItems: 'center', gap: 2,
            color, background: bg, borderRadius: 4, padding: '1px 6px',
            fontFamily: "'SF Mono', monospace", fontWeight: 600, fontSize: 12,
          }}>
            <Icon size={11} />{formatPct(v)}
          </span>
        );
      },
    },
    {
      title: '行业', dataIndex: 'industry', width: 100,
      render: (v: string) => v ? (
        <Tag size="small" style={{ background: 'var(--color-fill-2)', border: 'none', fontSize: 11 }}>{v}</Tag>
      ) : <span className="muted">—</span>,
    },
  ], [navigate]);

  // // Unusual mode extra columns
  const unusualColumns = useMemo(() => [
    ...columns.slice(0, 5),
    {
      title: '异动类型', dataIndex: 'unusualTypes', width: 160,
      render: (types: string[]) => (
        <div style={{ display: 'flex', gap: 3, flexWrap: 'wrap' }}>
          {types?.map((t, i) => {
            let color = 'orangered';
            if (t === '放量') color = 'arcoblue';
            if (t === '急涨') color = 'red';
            if (t === '急跌') color = 'green';
            if (t === '高振幅') color = 'purple';
            return <Tag key={i} size="small" color={color} style={{ fontSize: 10, lineHeight: '16px' }}>{t}</Tag>;
          })}
        </div>
      ),
    },
    {
      title: '振幅', dataIndex: 'amplitude', width: 70,
      render: (v: number) => (
        <span style={{ fontFamily: "'SF Mono', monospace", fontSize: 11, fontWeight: 600, color: 'var(--color-warning-text)' }}>
          {v?.toFixed(1)}%
        </span>
      ),
    },
    ...columns.slice(5),
  ], [columns]);

  return (
    <div>
      {/* ── Page Header ── */}
      <div className="page-header">
        <h2><BarChart3 size={20} style={{ marginRight: 8 }} />行情中心</h2>
        <span className="muted">
          {snapshot ? `数据日期: ${snapshot.tradeDate}` : '加载中...'}
        </span>
      </div>

      {/* ── Market Snapshot Cards ── */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginBottom: 16 }}>
        {/* Row 1: Three index cards */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12 }}>
          {indices.slice(0, 3).map((idx) => {
            const isUp = idx.chgPct >= 0;
            const color = isUp ? 'var(--stock-up)' : 'var(--stock-down)';
            const bg = isUp ? 'var(--stock-up-soft)' : 'var(--stock-down-soft)';
            const boardAmtKey = idxAmountMap[idx.code];
            const boardAmount = boardAmtKey && snapshot ? snapshot[boardAmtKey] : undefined;
            return (
              <div key={idx.code} className="card" style={{ padding: '14px 18px', display: 'flex', flexDirection: 'column', gap: 8 }}>
                {/* Header: name + change */}
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>{idx.name}</span>
                  <span style={{
                    color, background: bg, borderRadius: 5, padding: '2px 8px',
                    fontFamily: "'SF Mono', monospace", fontWeight: 700, fontSize: 13,
                    display: 'inline-flex', alignItems: 'center', gap: 3,
                  }}>
                    {isUp ? <ArrowUp size={12} /> : <ArrowDown size={12} />}
                    {formatPct(idx.chgPct)}
                  </span>
                </div>
                {/* Price + change amount */}
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
                  <span style={{ fontSize: 28, fontWeight: 800, fontFamily: "'SF Mono', monospace", color: 'var(--color-text-1)', lineHeight: 1 }}>
                    {idx.val?.toFixed(2)}
                  </span>
                  <span style={{ fontSize: 13, color, fontFamily: "'SF Mono', monospace", fontWeight: 600 }}>
                    {isUp ? '+' : ''}{idx.chg?.toFixed(2)}
                  </span>
                </div>
                {/* OHLC row */}
                {(idx.high != null || idx.low != null) && (
                  <div style={{ display: 'flex', gap: 16, fontSize: 11, color: 'var(--color-text-2)' }}>
                    {idx.open != null && <span>今开 <b style={{ color: 'var(--color-text-1)', fontFamily: "'SF Mono', monospace" }}>{idx.open.toFixed(2)}</b></span>}
                    {idx.high != null && <span>最高 <b style={{ color: 'var(--stock-up)', fontFamily: "'SF Mono', monospace" }}>{idx.high.toFixed(2)}</b></span>}
                    {idx.low != null && <span>最低 <b style={{ color: 'var(--stock-down)', fontFamily: "'SF Mono', monospace" }}>{idx.low.toFixed(2)}</b></span>}
                  </div>
                )}
                {/* Turnover */}
                {boardAmount != null && boardAmount > 0 && (
                  <div style={{ display: 'flex', gap: 4, fontSize: 11, color: 'var(--color-text-3)', borderTop: '1px solid var(--color-border-1)', paddingTop: 6 }}>
                    <DollarSign size={12} style={{ color: 'var(--color-text-3)' }} />
                    <span>成交 <b style={{ color: 'var(--color-text-1)', fontFamily: "'SF Mono', monospace" }}>{formatAmount(boardAmount)}</b></span>
                  </div>
                )}
                {(() => {
                  const bs = getBoardStats(idx, snapshot);
                  if (!bs) return null;
                  const total = bs.up + bs.down + bs.flat || 1;
                  return (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 3, borderTop: '1px solid var(--color-border-1)', paddingTop: 6 }}>
                      <div style={{ display: 'flex', gap: 10, fontSize: 11 }}>
                        <span style={{ color: 'var(--stock-up)' }}>涨 {bs.up}</span>
                        <span style={{ color: 'var(--stock-down)' }}>跌 {bs.down}</span>
                        <span style={{ color: 'var(--color-text-3)' }}>平 {bs.flat}</span>
                      </div>
                      <div style={{ height: 3, borderRadius: 2, display: 'flex', overflow: 'hidden' }}>
                        <div style={{ width: `${(bs.up / total * 100).toFixed(0)}%`, background: 'var(--stock-up)' }} />
                        <div style={{ width: `${(bs.flat / total * 100).toFixed(0)}%`, background: 'var(--gray-4)' }} />
                        <div style={{ flex: 1, background: 'var(--stock-down)' }} />
                      </div>
                    </div>
                  );
                })()}
              </div>
            );
          })}
        </div>

        {/* Row 2: Market aggregate stats */}
        {snapshot && (
          <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr 1fr', gap: 12 }}>
            {/* Total turnover card */}
            <div className="card" style={{ padding: '14px 18px', display: 'flex', alignItems: 'center', gap: 16 }}>
              <div style={{
                width: 40, height: 40, borderRadius: 10,
                background: 'linear-gradient(135deg, var(--color-primary), #722ED1)',
                display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
              }}>
                <DollarSign size={20} color="#fff" />
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 2 }}>两市成交额</div>
                <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
                  <span style={{ fontSize: 20, fontWeight: 700, fontFamily: "'SF Mono', monospace", color: 'var(--color-text-1)' }}>
                    {formatAmount(snapshot.amount)}
                  </span>
                  <span style={{
                    fontSize: 12, fontFamily: "'SF Mono', monospace",
                    color: snapshot.change >= 0 ? 'var(--stock-up)' : 'var(--stock-down)',
                  }}>
                    {snapshot.change >= 0 ? '+' : ''}{formatAmount(Math.abs(snapshot.change ?? 0))} ({snapshot.changePct >= 0 ? '+' : ''}{(snapshot.changePct ?? 0).toFixed(1)}%)
                  </span>
                </div>
              </div>
            </div>

            {/* Up/Down stats card */}
            <div className="card" style={{ padding: '14px 18px', display: 'flex', flexDirection: 'column', gap: 6 }}>
              <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>涨跌统计 · {snapshot.totalStocks}只</div>
              <div style={{ display: 'flex', gap: 14, alignItems: 'baseline' }}>
                <span style={{ fontSize: 18, fontWeight: 700, color: 'var(--stock-up)', fontFamily: "'SF Mono', monospace" }}>
                  {snapshot.upCount}<span style={{ fontSize: 11, fontWeight: 400 }}> 涨</span>
                </span>
                <span style={{ fontSize: 18, fontWeight: 700, color: 'var(--stock-down)', fontFamily: "'SF Mono', monospace" }}>
                  {snapshot.downCount}<span style={{ fontSize: 11, fontWeight: 400 }}> 跌</span>
                </span>
                <span style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-3)' }}>
                  {snapshot.flatCount}<span style={{ fontSize: 10 }}> 平</span>
                </span>
              </div>
              <div style={{ height: 4, borderRadius: 2, display: 'flex', overflow: 'hidden' }}>
                <div style={{ width: `${(snapshot.upCount / snapshot.totalStocks * 100).toFixed(0)}%`, background: 'var(--stock-up)', transition: 'width 0.3s' }} />
                <div style={{ width: `${(snapshot.flatCount / snapshot.totalStocks * 100).toFixed(0)}%`, background: 'var(--gray-4)' }} />
                <div style={{ flex: 1, background: 'var(--stock-down)' }} />
              </div>
            </div>

            {/* Limit up/down card */}
            <div className="card" style={{ padding: '14px 18px', display: 'flex', flexDirection: 'column', gap: 6 }}>
              <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>涨跌停</div>
              <div style={{ display: 'flex', gap: 14, alignItems: 'baseline' }}>
                <span style={{ fontSize: 18, fontWeight: 700, color: 'var(--stock-up)', fontFamily: "'SF Mono', monospace" }}>
                  {snapshot.limitUpCount}<span style={{ fontSize: 11, fontWeight: 400 }}> 涨停</span>
                </span>
                <span style={{ fontSize: 18, fontWeight: 700, color: 'var(--stock-down)', fontFamily: "'SF Mono', monospace" }}>
                  {snapshot.limitDownCount}<span style={{ fontSize: 11, fontWeight: 400 }}> 跌停</span>
                </span>
              </div>
            </div>
          </div>
        )}
      </div>
{/* ── Board Type Tabs ── */}
      <div className="card mb16">
        <div style={{
          display: 'flex', alignItems: 'center', gap: 4, padding: '8px 16px',
          borderBottom: '1px solid var(--color-border-1)', overflowX: 'auto',
        }}>
          {boardTabsWithCounts.map(t => (
            <div
              key={t.key}
              onClick={() => { setBoardType(t.key); setPage(1); }}
              style={{
                padding: '6px 14px', borderRadius: 6, cursor: 'pointer',
                fontSize: 13, fontWeight: boardType === t.key ? 600 : 400,
                color: boardType === t.key ? 'var(--color-primary)' : 'var(--color-text-2)',
                background: boardType === t.key ? 'var(--color-primary-light-1)' : 'transparent',
                whiteSpace: 'nowrap', transition: 'all 0.15s',
                display: 'inline-flex', alignItems: 'center', gap: 6,
              }}
            >
              {t.label}
              {t.count > 0 && (
                <span style={{
                  fontSize: 10, background: boardType === t.key ? 'var(--color-primary)' : 'var(--color-fill-2)',
                  color: boardType === t.key ? '#fff' : 'var(--color-text-3)',
                  borderRadius: 10, padding: '0 6px', lineHeight: '18px', fontWeight: 500,
                }}>
                  {t.count}
                </span>
              )}
            </div>
          ))}
        </div>

        {/* ── Search + View Mode ── */}
        <div className="card-header" style={{
          display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap',
          padding: '10px 16px',
        }}>
          <Input
            prefix={<Search size={14} />}
            placeholder="代码 / 名称搜索"
            value={keyword}
            onChange={(v) => { setKeyword(v); setPage(1); }}
            style={{ width: 220 }}
            allowClear
            size="small"
          />
          <Select
            placeholder="行业筛选"
            value={industry || undefined}
            onChange={(v) => { setIndustry(v || ''); setPage(1); }}
            style={{ width: 140 }}
            allowClear
            size="small"
            options={[
              '电子', '机械设备', '通信', '基础化工', '有色金属', '公用事业', '电力设备', '医药生物',
              '环保', '汽车', '计算机', '食品饮料', '建筑材料', '交通运输', '房地产', '银行', '商贸零售',
              '轻工制造', '国防军工', '家用电器', '农林牧渔', '石油石化', '美容护理', '社会服务',
              '传媒', '煤炭', '钢铁', '综合', '纺织服饰',
            ].map(i => ({ label: i, value: i }))}
          />

          {/* View mode switcher */}
          <div style={{ display: 'flex', gap: 2, background: 'var(--color-fill-1)', borderRadius: 6, padding: 2 }}>
            {VIEW_MODES.map(m => (
              <div
                key={m.key}
                onClick={() => { setViewMode(m.key); setPage(1); }}
                style={{
                  padding: '4px 10px', borderRadius: 4, cursor: 'pointer',
                  fontSize: 12, fontWeight: viewMode === m.key ? 600 : 400,
                  color: viewMode === m.key ? '#fff' : 'var(--color-text-2)',
                  background: viewMode === m.key ? 'var(--color-primary)' : 'transparent',
                  whiteSpace: 'nowrap', transition: 'all 0.15s',
                }}
              >
                {m.key === 'unusual' ? (
                  <span style={{ display: 'flex', alignItems: 'center', gap: 3 }}>
                    <Zap size={11} />{m.label}
                  </span>
                ) : m.key === 'appearance' ? (
                  <span style={{ display: 'flex', alignItems: 'center', gap: 3 }}>
                    <TrendingUp size={11} />{m.label}
                  </span>
                ) : m.label}
              </div>
            ))}
          </div>

          <span style={{ flex: 1 }} />
          {(viewMode !== 'unusual' && viewMode !== 'appearance') && (
            <Pagination
              current={page} total={total} pageSize={PAGE_SIZE} size="small" simple
              onChange={(p) => setPage(p)}
            />
          )}
        </div>

        {/* ── Table ── */}
        <div style={{ padding: 0 }}>
          {loading ? (
            <div style={{ padding: 40, textAlign: 'center' }}><Spin /></div>
          ) : viewMode === 'appearance' ? (
            appearanceStocks.length === 0 ? (
              <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)' }}>
                <BarChart3 size={32} style={{ marginBottom: 8 }} />
                <div>近20日无上榜记录</div>
              </div>
            ) : (
              <Table
                data={appearanceStocks} columns={appearanceColumns} rowKey="code"
                loading={false} pagination={false} border={false} stripe
                size="small" scroll={{ x: 780 }}
                onRow={(r) => ({
                  style: { cursor: 'pointer' },
                  onDoubleClick: () => navigate(`/stock/${r.code}`),
                })}
              />
            )
          ) : viewMode === 'unusual' ? (
            unusualStocks.length === 0 ? (
              <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)' }}>
                <AlertTriangle size={32} style={{ marginBottom: 8 }} />
                <div>当前无显著异动股票</div>
              </div>
            ) : (
              <Table
                data={unusualStocks} columns={unusualColumns} rowKey="code"
                loading={false} pagination={false} border={false} stripe
                size="small" scroll={{ x: 900 }}
                onRow={(r) => ({
                  style: { cursor: 'pointer' },
                  onDoubleClick: () => navigate(`/stock/${r.code}`),
                })}
              />
            )
          ) : (
            <Table
              data={stocks} columns={columns} rowKey="code"
              loading={false} pagination={false} border={false} stripe
              size="small" scroll={{ x: 780 }}
              onRow={(r) => ({
                style: { cursor: 'pointer' },
                onDoubleClick: () => navigate(`/stock/${r.code}`),
              })}
            />
          )}
        </div>
        {(viewMode !== 'unusual' && viewMode !== 'appearance') && (
          <div style={{ padding: '10px 16px', display: 'flex', justifyContent: 'flex-end' }}>
            <Pagination current={page} total={total} pageSize={PAGE_SIZE}
              onChange={(p) => setPage(p)} showTotal size="small" />
          </div>
        )}
      </div>

      {/* ── Add to Watchlist Modal ── */}
      <Modal
        title="添加到自选"
        visible={showAddModal}
        onOk={handleAddWithGroup}
        onCancel={() => setShowAddModal(false)}
        okText="确认添加"
        simple
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, paddingTop: 8 }}>
          <div>
            <span style={{ fontSize: 13, color: 'var(--color-text-2)', marginBottom: 6, display: 'block' }}>选择分组</span>
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
              {groups.map((g: any) => (
                <Tag
                  key={g.id}
                  color={addGroupId === g.id ? 'arcoblue' : undefined}
                  style={{ cursor: 'pointer' }}
                  onClick={() => setAddGroupId(g.id)}
                  checkable checked={addGroupId === g.id}
                >
                  {g.name}
                </Tag>
              ))}
            </div>
          </div>
          <div>
            <span style={{ fontSize: 13, color: 'var(--color-text-2)', marginBottom: 6, display: 'block' }}>或新建分组</span>
            <Input
              placeholder="输入新分组名称"
              value={newGroupInput}
              onChange={setNewGroupInput}
              style={{ width: '100%' }}
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}
