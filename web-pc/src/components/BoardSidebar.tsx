import { useState, useEffect, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Select, Tag, Spin, Tooltip } from '@arco-design/web-react';
import { BarChart3, TrendingUp, TrendingDown, Minus, ChevronRight, Hash, CalendarDays, Flame } from 'lucide-react';
import { authFetch } from '../services/api';

interface BoardItem {
  id: number; pickDate: string; stockCode: string; stockName: string;
  rank: number; score: number; riskLevel: string; suggestion: string;
  open: number; close: number; chgPct: number;
  streakCount: number; appearanceCount: number;
}

type SortKey = 'rank' | 'appearanceCount' | 'streakCount';

const SORT_OPTIONS: { key: SortKey; icon: typeof Hash; label: string }[] = [
  { key: 'rank', icon: Hash, label: '排名' },
  { key: 'appearanceCount', icon: CalendarDays, label: '上榜次数' },
  { key: 'streakCount', icon: Flame, label: '连续上榜' },
];

interface BoardSidebarProps {
  stockCode: string;
  stockName: string;
}

export default function BoardSidebar({ stockCode, stockName }: BoardSidebarProps) {
  const [boardDates, setBoardDates] = useState<string[]>([]);
  const [selectedDate, setSelectedDate] = useState('');
  const [boardItems, setBoardItems] = useState<BoardItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [sortBy, setSortBy] = useState<SortKey>('rank');
  const navigate = useNavigate();

  useEffect(() => {
    (async () => {
      try {
        const res = await authFetch(`/api/v1/board/heatmap/${stockCode}`);
        const json = await res.json();
        const dates: string[] = (json.data || [])
          .map((d: any) => (d.pickDate || '').slice(0, 10))
          .filter(Boolean);
        setBoardDates(dates);
        if (dates.length > 0) {
          setSelectedDate(dates[0]);
        }
      } catch (_) {}
    })();
  }, [stockCode]);

  const fetchBoard = useCallback(async (date: string) => {
    setLoading(true);
    try {
      const res = await authFetch(`/api/v1/board/history?date=${date}`);
      const json = await res.json();
      setBoardItems(json.data || []);
    } catch (_) {
      setBoardItems([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (selectedDate) fetchBoard(selectedDate);
  }, [selectedDate, fetchBoard]);

  // Sort items
  const sortedItems = useMemo(() => {
    const items = [...boardItems];
    if (sortBy === 'rank') {
      items.sort((a, b) => a.rank - b.rank);
    } else if (sortBy === 'appearanceCount') {
      items.sort((a, b) => b.appearanceCount - a.appearanceCount || a.rank - b.rank);
    } else if (sortBy === 'streakCount') {
      items.sort((a, b) => b.streakCount - a.streakCount || a.rank - b.rank);
    }
    return items;
  }, [boardItems, sortBy]);

  const currentStockItem = sortedItems.find(i => i.stockCode === stockCode);
  const hasDates = boardDates.length > 0;

  return (
    <div className="card" style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
      <div className="card-header" style={{ flexShrink: 0 }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <BarChart3 size={14} color="#722ed1" />
          <span style={{ fontSize: 13, fontWeight: 600 }}>历史榜单</span>
        </span>
      </div>

      <div style={{ padding: '8px 14px', flexShrink: 0, borderBottom: '1px solid var(--color-border-1)', display: 'flex', alignItems: 'center', gap: 6 }}>
        {hasDates ? (
          <>
            <Select
              value={selectedDate}
              onChange={(v) => setSelectedDate(v)}
              style={{ flex: 1, minWidth: 0 }}
              size="small"
              options={boardDates.map(d => ({ label: d, value: d }))}
            />
            <div style={{ display: 'flex', gap: 2, flexShrink: 0 }}>
              {SORT_OPTIONS.map(opt => (
                <Tooltip key={opt.key} content={opt.label}>
                  <button
                    onClick={() => setSortBy(opt.key)}
                    style={{
                      width: 26, height: 26, display: 'flex', alignItems: 'center', justifyContent: 'center',
                      border: 'none', borderRadius: 4, cursor: 'pointer',
                      background: sortBy === opt.key ? 'var(--arcoblue-1)' : 'transparent',
                      color: sortBy === opt.key ? 'var(--arcoblue-6)' : 'var(--color-text-3)',
                      transition: 'all 0.15s',
                    }}
                  >
                    <opt.icon size={14} />
                  </button>
                </Tooltip>
              ))}
            </div>
          </>
        ) : (
          <div style={{ fontSize: 12, color: 'var(--color-text-3)', textAlign: 'center', padding: '8px 0', flex: 1 }}>
            该股票近期未上榜
          </div>
        )}
      </div>

      {/* Current stock rank summary */}
      {currentStockItem && (
        <div style={{
          padding: '8px 14px', background: 'linear-gradient(135deg, var(--purple-1), #ede9fe)',
          borderBottom: '1px solid var(--color-border-1)', flexShrink: 0,
        }}>
          <div style={{ fontSize: 11, color: 'var(--purple-6)', marginBottom: 2 }}>当前排名</div>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 8 }}>
            <span style={{ fontSize: 22, fontWeight: 700, color: 'var(--purple-7)' }}>#{currentStockItem.rank}</span>
            <span style={{ fontSize: 12, color: 'var(--purple-6)' }}>评分 {currentStockItem.score?.toFixed(1)}</span>
          </div>
          <div style={{ display: 'flex', gap: 10, marginTop: 4 }}>
            {currentStockItem.suggestion && (
              <Tag size="small" color={currentStockItem.suggestion.includes('买入') ? 'red' : currentStockItem.suggestion.includes('卖出') ? 'green' : 'arcoblue'}
                style={{ fontSize: 10 }}>
                {currentStockItem.suggestion}
              </Tag>
            )}
            <span style={{ fontSize: 10, color: 'var(--purple-6)' }}>
              近20日上榜 <b>{currentStockItem.appearanceCount}</b> 次
            </span>
            <span style={{ fontSize: 10, color: 'var(--purple-6)' }}>
              连续 <b>{currentStockItem.streakCount}</b> 天
            </span>
          </div>
        </div>
      )}

      {/* Board ranking list */}
      <div style={{ flex: 1, overflow: 'auto', minHeight: 0 }}>
        {loading ? (
          <div style={{ textAlign: 'center', padding: 24 }}><Spin /></div>
        ) : sortedItems.length === 0 ? (
          <div style={{ textAlign: 'center', padding: 20, color: 'var(--color-text-3)', fontSize: 12 }}>
            {hasDates ? '暂无数据' : '该股票近期未上榜'}
          </div>
        ) : (
          <div style={{ padding: '4px 0' }}>
            {sortedItems.map((item) => {
              const isCurrent = item.stockCode === stockCode;
              const chgColor = item.chgPct > 0 ? '#f53f3f' : item.chgPct < 0 ? '#00b42a' : 'var(--color-text-3)';
              const ChgIcon = item.chgPct > 0 ? TrendingUp : item.chgPct < 0 ? TrendingDown : Minus;
              return (
                <div
                  key={item.stockCode}
                  onClick={() => navigate(`/stock/${item.stockCode}`)}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 6,
                    padding: '6px 14px', cursor: 'pointer',
                    background: isCurrent ? 'var(--arcoblue-1)' : 'transparent',
                    borderLeft: isCurrent ? '3px solid #165dff' : '3px solid transparent',
                    transition: 'background 0.15s',
                    fontSize: 12,
                  }}
                  onMouseEnter={e => { if (!isCurrent) e.currentTarget.style.background = 'var(--color-fill-2)'; }}
                  onMouseLeave={e => { if (!isCurrent) e.currentTarget.style.background = 'transparent'; }}
                >
                  {/* Rank badge */}
                  <span style={{
                    width: 22, height: 22, borderRadius: 4,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: 10, fontWeight: 700, flexShrink: 0,
                    background: item.rank <= 3 ? '#f53f3f' : item.rank <= 10 ? 'var(--orange-6)' : 'var(--color-border-1)',
                    color: item.rank <= 10 ? 'var(--color-white)' : 'var(--color-text-3)',
                  }}>
                    {item.rank}
                  </span>
                  {/* Stock info */}
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{
                      fontWeight: isCurrent ? 600 : 400,
                      color: isCurrent ? 'var(--arcoblue-6)' : 'var(--color-text-1)',
                      overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                    }}>
                      {item.stockName || item.stockCode}
                    </div>
                    <div style={{ fontSize: 10, color: 'var(--color-text-3)', display: 'flex', alignItems: 'center', gap: 4 }}>
                      <span>{item.stockCode}</span>
                      {item.appearanceCount > 0 && (
                        <span style={{ fontSize: 9, padding: '0 4px', borderRadius: 3, background: 'var(--purple-1)', color: 'var(--purple-6)', fontWeight: 500, lineHeight: '16px' }}>
                          {item.appearanceCount}次
                        </span>
                      )}
                      {item.streakCount > 0 && (
                        <span style={{ fontSize: 9, padding: '0 4px', borderRadius: 3, background: 'var(--red-1)', color: 'var(--red-7)', fontWeight: 500, lineHeight: '16px' }}>
                          {item.streakCount}天
                        </span>
                      )}
                    </div>
                  </div>
                  {/* Score + change */}
                  <div style={{ textAlign: 'right', flexShrink: 0 }}>
                    <div style={{ fontWeight: 600, fontSize: 11 }}>{item.close?.toFixed(2)}</div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 1, fontSize: 10, color: chgColor }}>
                      <ChgIcon size={10} />
                      {item.chgPct?.toFixed(1)}%
                    </div>
                  </div>
                  <ChevronRight size={12} color="var(--color-text-3)" />
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
