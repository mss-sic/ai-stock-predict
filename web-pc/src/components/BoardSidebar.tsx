import { useState, useEffect, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Select, Tag, Spin, Tooltip } from '@arco-design/web-react';
import {
  BarChart3, TrendingUp, TrendingDown, Minus, ChevronRight,
  Hash, CalendarDays, Flame, Star, Briefcase, Layers,
} from 'lucide-react';
import { authFetch, fetchWatchlist, fetchWatchlistGroups, fetchHoldings, addToWatchlist, removeFromWatchlist } from '../services/api';
import { showToast } from './Toast';

/* ─── Types ─── */
interface BoardItem {
  id: number; pickDate: string; stockCode: string; stockName: string;
  rank: number; score: number; riskLevel: string; suggestion: string;
  open: number; close: number; chgPct: number;
  streakCount: number; appearanceCount: number;
}

interface WatchlistItem {
  stockCode: string; stockName: string; close: number;
  addedPrice: number; addedAt: string; yield: number; groupId: number;
}

interface HoldingItem {
  id: number; stockCode: string; stockName: string;
  costPrice: number; quantity: number; buyDate: string;
  curPrice: number; marketVal: number; pnl: number; pnlPct: number;
  holdDays: number; dailyPnl: number; dailyPnlPct: number;
}

type TabKey = 'board' | 'watchlist' | 'holdings';
type SortKey = 'rank' | 'appearanceCount' | 'streakCount';

/* ─── Constants ─── */
const SORT_OPTIONS: { key: SortKey; icon: typeof Hash; label: string }[] = [
  { key: 'rank', icon: Hash, label: '排名' },
  { key: 'appearanceCount', icon: CalendarDays, label: '上榜次数' },
  { key: 'streakCount', icon: Flame, label: '连续上榜' },
];

const TABS: { key: TabKey; icon: typeof BarChart3; label: string }[] = [
  { key: 'board', icon: BarChart3, label: '历史榜单' },
  { key: 'watchlist', icon: Star, label: '自选股' },
  { key: 'holdings', icon: Briefcase, label: '持仓股' },
];

const CHG_COLOR = (v: number) => v > 0 ? '#f53f3f' : v < 0 ? '#00b42a' : 'var(--color-text-3)';
const ChgIcon = (v: number) => v > 0 ? TrendingUp : v < 0 ? TrendingDown : Minus;

interface Props { stockCode: string; stockName: string; }

export default function BoardSidebar({ stockCode, stockName }: Props) {
  const navigate = useNavigate();
  const [tab, setTab] = useState<TabKey>('board');

  /* ── Board State ── */
  const [boardDates, setBoardDates] = useState<string[]>([]);
  const [selectedDate, setSelectedDate] = useState('');
  const [boardItems, setBoardItems] = useState<BoardItem[]>([]);
  const [boardLoading, setBoardLoading] = useState(false);
  const [sortBy, setSortBy] = useState<SortKey>('rank');

  /* ── Watchlist State ── */
  const [wlItems, setWlItems] = useState<WatchlistItem[]>([]);
  const [wlGroups, setWlGroups] = useState<{ id: number; name: string }[]>([]);
  const [wlGroupId, setWlGroupId] = useState<number>(0);
  const [wlLoading, setWlLoading] = useState(false);

  /* ── Holdings State ── */
  const [holdItems, setHoldItems] = useState<HoldingItem[]>([]);
  const [holdLoading, setHoldLoading] = useState(false);

  /* ═══════════ Fetch Board Dates ═══════════ */
  useEffect(() => {
    (async () => {
      try {
        const res = await authFetch('/api/v1/board/dates');
        const json = await res.json();
        const dates: string[] = (json.data || [])
          .map((d: any) => (typeof d === 'string' ? d : String(d || '')).slice(0, 10))
          .filter((d: string) => d.length === 10);
        setBoardDates(dates);
        if (dates.length > 0) setSelectedDate(dates[0]);
      } catch (_) {}
    })();
  }, []);

  /* ── Fetch Board ── */
  const fetchBoard = useCallback(async (date: string) => {
    setBoardLoading(true);
    try {
      const res = await authFetch(`/api/v1/board/history?date=${date}`);
      const json = await res.json();
      setBoardItems(json.data || []);
    } catch (_) { setBoardItems([]); }
    finally { setBoardLoading(false); }
  }, []);

  useEffect(() => { if (selectedDate) fetchBoard(selectedDate); }, [selectedDate, fetchBoard]);

  /* ═══════════ Fetch Watchlist ═══════════ */
  const loadWatchlist = useCallback(async () => {
    setWlLoading(true);
    try {
      const [itemsRes, groupsRes] = await Promise.all([
        fetchWatchlist(wlGroupId || undefined),
        fetchWatchlistGroups(),
      ]);
      setWlItems(itemsRes.data?.data || []);
      setWlGroups([{ id: 0, name: '默认分组' }, ...(groupsRes.data?.data || [])]);
    } catch (_) { setWlItems([]); }
    finally { setWlLoading(false); }
  }, [wlGroupId]);

  useEffect(() => { if (tab === 'watchlist') loadWatchlist(); }, [tab, loadWatchlist]);

  /* ═══════════ Fetch Holdings ═══════════ */
  const loadHoldings = useCallback(async () => {
    setHoldLoading(true);
    try {
      const res = await fetchHoldings();
      setHoldItems(res.data?.data || []);
    } catch (_) { setHoldItems([]); }
    finally { setHoldLoading(false); }
  }, []);

  useEffect(() => { if (tab === 'holdings') loadHoldings(); }, [tab, loadHoldings]);

  /* ── Sorted Board Items ── */
  const sortedBoard = useMemo(() => {
    const items = [...boardItems];
    if (sortBy === 'rank') items.sort((a, b) => a.rank - b.rank);
    else if (sortBy === 'appearanceCount') items.sort((a, b) => b.appearanceCount - a.appearanceCount || a.rank - b.rank);
    else if (sortBy === 'streakCount') items.sort((a, b) => b.streakCount - a.streakCount || a.rank - b.rank);
    return items;
  }, [boardItems, sortBy]);

  const currentStockItem = sortedBoard.find(i => i.stockCode === stockCode);
  const isWatched = wlItems.some(i => i.stockCode === stockCode);

  /* ── Watchlist toggle ── */
  const handleWlToggle = async () => {
    try {
      if (isWatched) {
        await removeFromWatchlist(stockCode);
        setWlItems(prev => prev.filter(i => i.stockCode !== stockCode));
        showToast('success', '已移出自选');
      } else {
        await addToWatchlist(stockCode, 0, 0);
        showToast('success', '已加入自选');
        loadWatchlist();
      }
    } catch { showToast('error', '操作失败'); }
  };

  /* ═══════════ RENDER ═══════════ */
  return (
    <div className="card" style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
      {/* ── Tab Bar ── */}
      <div style={{ display: 'flex', borderBottom: '1px solid var(--color-border-1)', flexShrink: 0 }}>
        {TABS.map(t => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            style={{
              flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 5,
              padding: '10px 4px', border: 'none', cursor: 'pointer',
              fontSize: 12, fontWeight: 600,
              background: tab === t.key ? 'var(--color-bg-1)' : 'transparent',
              color: tab === t.key ? 'var(--color-primary)' : 'var(--color-text-3)',
              borderBottom: tab === t.key ? '2px solid var(--color-primary)' : '2px solid transparent',
              transition: 'all 0.15s',
            }}
          >
            <t.icon size={13} />
            {t.label}
          </button>
        ))}
      </div>

      {/* ═══════════ BOARD TAB ═══════════ */}
      {tab === 'board' && (
        <>
          <div style={{ padding: '8px 14px', flexShrink: 0, borderBottom: '1px solid var(--color-border-1)', display: 'flex', alignItems: 'center', gap: 6 }}>
            {boardDates.length > 0 ? (
              <>
                <Select value={selectedDate} onChange={v => setSelectedDate(v)} style={{ flex: 1, minWidth: 0 }} size="small"
                  options={boardDates.map(d => ({ label: d, value: d }))} />
                <div style={{ display: 'flex', gap: 2, flexShrink: 0 }}>
                  {SORT_OPTIONS.map(opt => (
                    <Tooltip key={opt.key} content={opt.label}>
                      <button onClick={() => setSortBy(opt.key)}
                        style={{
                          width: 26, height: 26, display: 'flex', alignItems: 'center', justifyContent: 'center',
                          border: 'none', borderRadius: 4, cursor: 'pointer',
                          background: sortBy === opt.key ? 'var(--arcoblue-1)' : 'transparent',
                          color: sortBy === opt.key ? 'var(--arcoblue-6)' : 'var(--color-text-3)',
                        }}>
                        <opt.icon size={14} />
                      </button>
                    </Tooltip>
                  ))}
                </div>
              </>
            ) : (
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', textAlign: 'center', padding: '8px 0', flex: 1 }}>暂无榜单数据</div>
            )}
          </div>

          {currentStockItem && (
            <div style={{ padding: '8px 14px', background: 'var(--arcoblue-1)', borderBottom: '1px solid var(--color-border-1)', flexShrink: 0 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>当前排名</span>
                <span style={{ fontSize: 18, fontWeight: 700, color: '#165dff' }}>#{currentStockItem.rank}</span>
                <span style={{ fontSize: 11, color: 'var(--color-text-2)' }}>评分 {currentStockItem.score?.toFixed(1)}</span>
                {currentStockItem.chgPct !== undefined && (
                  <span style={{ fontSize: 11, fontWeight: 600, color: CHG_COLOR(currentStockItem.chgPct) }}>
                    {currentStockItem.chgPct > 0 ? '+' : ''}{currentStockItem.chgPct.toFixed(2)}%
                  </span>
                )}
              </div>
              <div style={{ display: 'flex', gap: 10, marginTop: 4 }}>
                {currentStockItem.suggestion && (
                  <Tag size="small" color={currentStockItem.suggestion.includes('买入') ? 'red' : currentStockItem.suggestion.includes('卖出') ? 'green' : 'arcoblue'} style={{ fontSize: 10 }}>
                    {currentStockItem.suggestion}
                  </Tag>
                )}
                <span style={{ fontSize: 10, color: 'var(--purple-6)' }}>近20日上榜 <b>{currentStockItem.appearanceCount}</b> 次</span>
                <span style={{ fontSize: 10, color: 'var(--purple-6)' }}>连续 <b>{currentStockItem.streakCount}</b> 天</span>
              </div>
            </div>
          )}

          <div style={{ flex: 1, overflow: 'auto', minHeight: 0 }}>
            {boardLoading ? <div style={{ textAlign: 'center', padding: 24 }}><Spin /></div>
              : sortedBoard.length === 0 ? (
                <div style={{ textAlign: 'center', padding: 20, color: 'var(--color-text-3)', fontSize: 12 }}>暂无数据</div>
              ) : (
                <div style={{ padding: '4px 0' }}>
                  {sortedBoard.map(item => {
                    const isCurrent = item.stockCode === stockCode;
                    const chgColor = CHG_COLOR(item.chgPct);
                    const CIcon = ChgIcon(item.chgPct);
                    return (
                      <div key={item.stockCode}
                        onClick={() => navigate(`/stock/${item.stockCode}`)}
                        style={{
                          display: 'flex', alignItems: 'center', gap: 6, padding: '6px 14px', cursor: 'pointer',
                          background: isCurrent ? 'var(--arcoblue-1)' : 'transparent',
                          borderLeft: isCurrent ? '3px solid #165dff' : '3px solid transparent',
                          fontSize: 12,
                        }}
                        onMouseEnter={e => { if (!isCurrent) e.currentTarget.style.background = 'var(--color-fill-2)'; }}
                        onMouseLeave={e => { if (!isCurrent) e.currentTarget.style.background = 'transparent'; }}
                      >
                        <span style={{ width: 22, height: 22, borderRadius: 4, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 10, fontWeight: 700, flexShrink: 0,
                          background: item.rank <= 3 ? '#f53f3f' : item.rank <= 10 ? 'var(--orange-6)' : 'var(--color-border-1)',
                          color: item.rank <= 10 ? 'var(--color-white)' : 'var(--color-text-3)' }}>
                          {item.rank}
                        </span>
                        <div style={{ flex: 1, minWidth: 0 }}>
                          <div style={{ fontWeight: isCurrent ? 600 : 400, color: isCurrent ? 'var(--arcoblue-6)' : 'var(--color-text-1)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            {item.stockName || item.stockCode}
                          </div>
                          <div style={{ fontSize: 10, color: 'var(--color-text-3)', display: 'flex', alignItems: 'center', gap: 4 }}>
                            <span>{item.stockCode}</span>
                            {item.appearanceCount > 0 && <span style={{ fontSize: 9, padding: '0 4px', borderRadius: 3, background: 'var(--purple-1)', color: 'var(--purple-6)', fontWeight: 500, lineHeight: '16px' }}>{item.appearanceCount}次</span>}
                            {item.streakCount > 0 && <span style={{ fontSize: 9, padding: '0 4px', borderRadius: 3, background: 'var(--red-1)', color: 'var(--red-7)', fontWeight: 500, lineHeight: '16px' }}>{item.streakCount}天</span>}
                          </div>
                        </div>
                        <div style={{ textAlign: 'right', flexShrink: 0 }}>
                          <div style={{ fontWeight: 600, fontSize: 11 }}>{item.close?.toFixed(2)}</div>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 1, fontSize: 10, color: chgColor }}>
                            <CIcon size={10} />{item.chgPct?.toFixed(1)}%
                          </div>
                        </div>
                        <ChevronRight size={12} color="var(--color-text-3)" />
                      </div>
                    );
                  })}
                </div>
              )}
          </div>
        </>
      )}

      {/* ═══════════ WATCHLIST TAB ═══════════ */}
      {tab === 'watchlist' && (
        <>
          <div style={{ padding: '8px 14px', flexShrink: 0, borderBottom: '1px solid var(--color-border-1)', display: 'flex', alignItems: 'center', gap: 8 }}>
            <Select value={wlGroupId} onChange={v => setWlGroupId(v)}
              style={{ flex: 1, minWidth: 0 }} size="small"
              options={wlGroups.map(g => ({ label: g.name, value: g.id }))} />
            <button
              onClick={handleWlToggle}
              style={{
                flexShrink: 0, padding: '3px 10px', borderRadius: 4, border: 'none', cursor: 'pointer',
                fontSize: 11, fontWeight: 600,
                background: isWatched ? 'var(--color-fill-2)' : 'var(--color-primary)',
                color: isWatched ? 'var(--color-text-2)' : '#fff',
              }}
            >
              {isWatched ? '移出' : '加入'}
            </button>
          </div>

          <div style={{ flex: 1, overflow: 'auto', minHeight: 0 }}>
            {wlLoading ? <div style={{ textAlign: 'center', padding: 24 }}><Spin /></div>
              : wlItems.length === 0 ? (
                <div style={{ textAlign: 'center', padding: 20, color: 'var(--color-text-3)', fontSize: 12 }}>暂无自选股</div>
              ) : (
                <div style={{ padding: '4px 0' }}>
                  {wlItems.map(item => {
                    const isCurrent = item.stockCode === stockCode;
                    const yieldColor = CHG_COLOR(item.yield);
                    const YIcon = ChgIcon(item.yield);
                    return (
                      <div key={item.stockCode}
                        onClick={() => navigate(`/stock/${item.stockCode}`)}
                        style={{
                          display: 'flex', alignItems: 'center', gap: 6, padding: '7px 14px', cursor: 'pointer',
                          background: isCurrent ? 'var(--arcoblue-1)' : 'transparent',
                          borderLeft: isCurrent ? '3px solid #165dff' : '3px solid transparent',
                          fontSize: 12, transition: 'background 0.15s',
                        }}
                        onMouseEnter={e => { if (!isCurrent) e.currentTarget.style.background = 'var(--color-fill-2)'; }}
                        onMouseLeave={e => { if (!isCurrent) e.currentTarget.style.background = 'transparent'; }}
                      >
                        <div style={{ flex: 1, minWidth: 0 }}>
                          <div style={{ fontWeight: isCurrent ? 600 : 400, color: isCurrent ? 'var(--arcoblue-6)' : 'var(--color-text-1)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            {item.stockName || item.stockCode}
                          </div>
                          <div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>{item.stockCode}</div>
                        </div>
                        <div style={{ textAlign: 'right', flexShrink: 0 }}>
                          <div style={{ fontWeight: 600, fontSize: 11 }}>{item.close?.toFixed(2) || '-'}</div>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 1, fontSize: 10, color: yieldColor, justifyContent: 'flex-end' }}>
                            <YIcon size={10} />{item.yield?.toFixed(2)}%
                          </div>
                        </div>
                        <ChevronRight size={12} color="var(--color-text-3)" />
                      </div>
                    );
                  })}
                </div>
              )}
          </div>
        </>
      )}

      {/* ═══════════ HOLDINGS TAB ═══════════ */}
      {tab === 'holdings' && (
        <>
          {/* Summary bar */}
          {holdItems.length > 0 && (
            <div style={{
              padding: '8px 14px', flexShrink: 0, borderBottom: '1px solid var(--color-border-1)',
              display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: 11,
            }}>
              <span style={{ color: 'var(--color-text-3)' }}>
                {holdItems.length} 只持仓
              </span>
              <span style={{ fontWeight: 700, color: CHG_COLOR(holdItems.reduce((s, i) => s + i.pnl, 0)) }}>
                总盈亏 {holdItems.reduce((s, i) => s + i.pnl, 0) >= 0 ? '+' : ''}{(holdItems.reduce((s, i) => s + i.pnl, 0) / 10000).toFixed(2)}万
              </span>
            </div>
          )}

          <div style={{ flex: 1, overflow: 'auto', minHeight: 0 }}>
            {holdLoading ? <div style={{ textAlign: 'center', padding: 24 }}><Spin /></div>
              : holdItems.length === 0 ? (
                <div style={{ textAlign: 'center', padding: 20, color: 'var(--color-text-3)', fontSize: 12 }}>暂无持仓</div>
              ) : (
                <div style={{ padding: '4px 0' }}>
                  {holdItems.map(item => {
                    const isCurrent = item.stockCode === stockCode;
                    const chgColor = CHG_COLOR(item.pnlPct);
                    const CIcon = ChgIcon(item.pnlPct);
                    return (
                      <div key={item.stockCode}
                        onClick={() => navigate(`/stock/${item.stockCode}`)}
                        style={{
                          padding: '8px 14px', cursor: 'pointer',
                          background: isCurrent ? 'var(--arcoblue-1)' : 'transparent',
                          borderLeft: isCurrent ? '3px solid #165dff' : '3px solid transparent',
                          borderBottom: '1px solid var(--color-border-1)',
                          fontSize: 12, transition: 'background 0.15s',
                        }}
                        onMouseEnter={e => { if (!isCurrent) e.currentTarget.style.background = 'var(--color-fill-2)'; }}
                        onMouseLeave={e => { if (!isCurrent) e.currentTarget.style.background = 'transparent'; }}
                      >
                        {/* Row 1: name + price */}
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                          <span style={{ fontWeight: isCurrent ? 600 : 500, color: isCurrent ? 'var(--arcoblue-6)' : 'var(--color-text-1)' }}>
                            {item.stockName || item.stockCode}
                          </span>
                          <span style={{ fontWeight: 600, fontSize: 12 }}>{item.curPrice?.toFixed(2)}</span>
                        </div>
                        {/* Row 2: code + quantity */}
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 2 }}>
                          <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>
                            {item.stockCode} · {item.quantity}股 · {item.holdDays}天
                          </span>
                          <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>
                            成本 {item.costPrice?.toFixed(2)}
                          </span>
                        </div>
                        {/* Row 3: P&L bar */}
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 3 }}>
                          <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>
                            市值 {(item.marketVal / 10000).toFixed(1)}万
                          </span>
                          <span style={{ fontSize: 11, fontWeight: 600, color: chgColor, display: 'flex', alignItems: 'center', gap: 2 }}>
                            <CIcon size={11} />
                            {item.pnlPct >= 0 ? '+' : ''}{item.pnlPct.toFixed(2)}%
                            <span style={{ fontWeight: 400 }}>
                              ({(item.pnl / 10000).toFixed(2)}万)
                            </span>
                          </span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
          </div>
        </>
      )}
    </div>
  );
}
