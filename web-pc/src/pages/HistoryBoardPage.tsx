import { useEffect, useState, useCallback } from 'react';
import { Table, Tag, Spin, Tooltip } from '@arco-design/web-react';
import { useNavigate } from 'react-router-dom';
import {
  CalendarDays, TrendingUp, TrendingDown, Minus,
  Star, Shield, AlertTriangle,
  ArrowUpRight, ArrowDownRight, Zap,
} from 'lucide-react';
import { authFetch } from '../services/api';

interface BoardItem {
  id: number; pickDate: string; stockCode: string; stockName: string;
  rank: number; riskLevel: string; suggestion: string;
  open: number; close: number; chgPct: number;
  pe: number; pb: number; industry: string;
  nextDate: string; nextOpen: number; nextClose: number; nextChgPct: number;
  cumuChgPct: number;
}

const SUGGEST_COLORS: Record<string,string>={'强烈买入':'#F53F3F','买入':'#F77234','增持':'#FF7D00','持有':'#86909C','减持':'#3491FA','卖出':'#00B42A','强烈卖出':'#009A29'};
const SUGGEST_BG: Record<string,string>={'强烈买入':'#FFECE8','买入':'#FFF3E8','增持':'#FFF7E8','持有':'#F2F3F5','减持':'#E8F3FF','卖出':'#E8FFEA','强烈卖出':'#DBF5DF'};
const RISK_COLORS: Record<string,string>={'高风险':'#F53F3F','中高风险':'#F77234','中风险':'#FF7D00','中低风险':'#3491FA','低风险':'#00B42A'};
const RISK_BG: Record<string,string>={'高风险':'#FFECE8','中高风险':'#FFF3E8','中风险':'#FFF7E8','中低风险':'#E8F3FF','低风险':'#E8FFEA'};

const fmtDateChip = (d: string) => {
  const dt = new Date(d + 'T00:00:00');
  const mm = dt.getMonth() + 1;
  const dd = dt.getDate();
  const dayMap = ['日', '一', '二', '三', '四', '五', '六'];
  return { md: `${mm}/${dd}`, day: `周${dayMap[dt.getDay()]}` };
};

export default function HistoryBoardPage() {
  const [dates, setDates] = useState<string[]>([]);
  const [selectedDate, setSelectedDate] = useState('');
  const [data, setData] = useState<BoardItem[]>([]);
  const [newEntries, setNewEntries] = useState<BoardItem[]>([]);
  const [removedEntries, setRemovedEntries] = useState<BoardItem[]>([]);
  const [prevDate, setPrevDate] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    (async () => {
      try {
        const res = await authFetch('/api/v1/board/dates');
        const json = await res.json();
        const rawDates: string[] = (json.data || []).map((d: string) => d.slice(0, 10));
        const recentDates = rawDates.slice(0, 20);
        setDates(recentDates);
        if (recentDates.length > 0) {
          setSelectedDate(recentDates[0]);
          fetchBoardData(recentDates[0], recentDates);
        }
      } catch (_) {}
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const fetchBoardData = useCallback(async (d: string, allDates: string[]) => {
    setLoading(true);
    try {
      const res = await authFetch(`/api/v1/board/history?date=${d}`);
      const json = await res.json();
      const items: BoardItem[] = json.data || [];
      setData(items);

      const idx = allDates.indexOf(d);
      let prev = '';
      let fetchedPrev: BoardItem[] = [];
      if (idx >= 0 && idx + 1 < allDates.length) {
        prev = allDates[idx + 1];
        try {
          const pr = await authFetch(`/api/v1/board/history?date=${prev}`);
          const pj = await pr.json();
          fetchedPrev = pj.data || [];
        } catch (_) {}
      }
      setPrevDate(prev);

      if (fetchedPrev.length > 0) {
        const prevCodes = new Set(fetchedPrev.map((i: BoardItem) => i.stockCode));
        const currCodes = new Set(items.map((i: BoardItem) => i.stockCode));
        setNewEntries(items.filter(i => !prevCodes.has(i.stockCode)));
        setRemovedEntries(fetchedPrev.filter(i => !currCodes.has(i.stockCode)));
      } else {
        setNewEntries([]);
        setRemovedEntries([]);
      }
    } catch (_) {
      setData([]);
    }
    setLoading(false);
  }, []);

  const handleDateClick = (d: string) => {
    setSelectedDate(d);
    fetchBoardData(d, dates);
  };

  // Stats: next-day
  const nextUp = data.filter(s => (s.nextChgPct ?? 0) > 0).length;
  const nextDown = data.filter(s => (s.nextChgPct ?? 0) < 0).length;
  const nextAvg = data.length > 0
    ? data.reduce((sum, s) => sum + (s.nextChgPct || 0), 0) / data.length : 0;

  // Stats: cumulative to present
  const cumuUp = data.filter(s => (s.cumuChgPct ?? 0) > 0).length;
  const cumuDown = data.filter(s => (s.cumuChgPct ?? 0) < 0).length;
  const cumuAvg = data.length > 0
    ? data.reduce((sum, s) => sum + (s.cumuChgPct || 0), 0) / data.length : 0;

  // Next day header label
  const nextDateLabel = data.length > 0 ? data[0].nextDate : '';

  const columns = [
    {
      title: '#', dataIndex: 'rank', width: 48, align: 'center' as const,
      sorter: (a: BoardItem, b: BoardItem) => a.rank - b.rank,
      render: (v: number) => {
        if (v <= 3) return <span style={{ fontSize: 18 }}>{['🥇', '🥈', '🥉'][v - 1]}</span>;
        return <span style={{ color: '#86909c', fontWeight: 500 }}>{v}</span>;
      },
    },
    {
      title: '股票', dataIndex: 'stockCode', width: 150,
      render: (_: string, r: BoardItem) => (
        <div style={{ cursor: 'pointer' }} onClick={() => navigate(`/stock/${r.stockCode}`)}>
          <div style={{ fontWeight: 600, fontSize: 14 }}>{r.stockName || r.stockCode}</div>
          <div style={{ fontSize: 11, color: '#86909c' }}>{r.stockCode}{r.industry ? ` · ${r.industry}` : ''}</div>
        </div>
      ),
    },
    {
      title: '收盘', dataIndex: 'close', width: 68, align: 'right' as const,
      sorter: (a: BoardItem, b: BoardItem) => (a.close || 0) - (b.close || 0),
      render: (v: number) => v > 0 ? <span style={{ fontWeight: 600, fontSize: 13 }}>{v.toFixed(2)}</span> : <span className="muted">-</span>,
    },
    {
      title: '当日涨跌', dataIndex: 'chgPct', width: 88, align: 'right' as const,
      sorter: (a: BoardItem, b: BoardItem) => (a.chgPct || 0) - (b.chgPct || 0),
      render: (v: number) => {
        if (v === undefined || v === null) return <span className="muted">-</span>;
        const color = v > 0 ? '#f53f3f' : v < 0 ? '#00b42a' : '#86909c';
        const icon = v > 0 ? <TrendingUp size={12} /> : v < 0 ? <TrendingDown size={12} /> : <Minus size={12} />;
        return (
          <span style={{ color, fontWeight: 600, fontSize: 13, display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 2 }}>
            {icon}{v > 0 ? '+' : ''}{v.toFixed(2)}%
          </span>
        );
      },
    },
    {
      title: '次日表现', dataIndex: 'nextChgPct', width: 100, align: 'right' as const,
      sorter: (a: BoardItem, b: BoardItem) => (a.nextChgPct || 0) - (b.nextChgPct || 0),
      render: (v: number, r: BoardItem) => {
        if (!r.nextDate) return <span className="muted" style={{ fontSize: 12 }}>-</span>;
        const color = v > 0 ? '#f53f3f' : v < 0 ? '#00b42a' : '#86909c';
        const icon = v > 0 ? <TrendingUp size={12} /> : v < 0 ? <TrendingDown size={12} /> : <Minus size={12} />;
        return (
          <Tooltip content={`${r.nextDate} 开 ${r.nextOpen?.toFixed(2) ?? '-'} 收 ${r.nextClose?.toFixed(2) ?? '-'}`}>
            <span style={{ color, fontWeight: 600, fontSize: 13, display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 2, cursor: 'default' }}>
              {icon}{v > 0 ? '+' : ''}{v.toFixed(2)}%
            </span>
          </Tooltip>
        );
      },
    },
    {
      title: '至今涨幅', dataIndex: 'cumuChgPct', width: 100, align: 'right' as const,
      sorter: (a: BoardItem, b: BoardItem) => (a.cumuChgPct || 0) - (b.cumuChgPct || 0),
      render: (v: number) => {
        if (v === undefined || v === null) return <span className="muted" style={{ fontSize: 12 }}>-</span>;
        const color = v > 0 ? '#f53f3f' : v < 0 ? '#00b42a' : '#86909c';
        const icon = v > 0 ? <TrendingUp size={12} /> : v < 0 ? <TrendingDown size={12} /> : <Minus size={12} />;
        return (
          <Tooltip content="从上榜日收盘至今的累计涨跌幅">
            <span style={{ color, fontWeight: 600, fontSize: 13, display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 2, cursor: 'default' }}>
              {icon}{v > 0 ? '+' : ''}{v.toFixed(2)}%
            </span>
          </Tooltip>
        );
      },
    },
    {
      title: 'PE', dataIndex: 'pe', width: 58, align: 'right' as const,
      sorter: (a: BoardItem, b: BoardItem) => (a.pe || 0) - (b.pe || 0),
      render: (v: number) => v > 0 ? <span style={{ fontWeight: 500, fontSize: 12 }}>{v.toFixed(1)}</span> : <span className="muted">-</span>,
    },
    {
      title: 'PB', dataIndex: 'pb', width: 58, align: 'right' as const,
      sorter: (a: BoardItem, b: BoardItem) => (a.pb || 0) - (b.pb || 0),
      render: (v: number) => v > 0 ? <span style={{ fontWeight: 500, fontSize: 12 }}>{v.toFixed(2)}</span> : <span className="muted">-</span>,
    },
    {
      title: '建议', dataIndex: 'suggestion', width: 62, align: 'center' as const,
      render: (v: string) => {
        if (!v) return <span style={{ color: '#C9CDD4', fontSize: 12 }}>—</span>;
        const color = SUGGEST_COLORS[v] || '#86909C';
        const bg = SUGGEST_BG[v] || '#F2F3F5';
        return (
          <span style={{
            display: 'inline-block', padding: '2px 8px', borderRadius: 3,
            background: bg, color, fontWeight: 600, fontSize: 11,
            border: '1px solid ' + color,
          }}>{v}</span>
        );
      },
    },
    {
      title: '风险', dataIndex: 'riskLevel', width: 85, align: 'center' as const,
      render: (v: string) => {
        if (!v) return <span style={{ color: '#C9CDD4', fontSize: 12 }}>—</span>;
        const color = RISK_COLORS[v] || '#86909C';
        const bg = RISK_BG[v] || '#F2F3F5';
        return (
          <span style={{
            display: 'inline-block', padding: '1px 8px', borderRadius: 3,
            background: bg, color, fontWeight: 600, fontSize: 12,
            border: '1px solid ' + color,
          }}>{v}</span>
        );
      },
    },
  ];

  const renderStockCard = (item: BoardItem, type: 'new' | 'removed') => {
    const chgColor = (item.chgPct ?? 0) > 0 ? '#f53f3f' : (item.chgPct ?? 0) < 0 ? '#00b42a' : '#86909c';
    return (
      <div
        key={item.id}
        onClick={() => navigate(`/stock/${item.stockCode}`)}
        style={{
          display: 'flex', alignItems: 'center', gap: 10,
          padding: '10px 16px', cursor: 'pointer',
          borderBottom: '1px solid #f5f5f5', transition: 'background 0.12s',
        }}
        onMouseEnter={e => (e.currentTarget.style.background = '#f7f8fa')}
        onMouseLeave={e => (e.currentTarget.style.background = '')}
      >
        <div style={{
          width: 24, height: 24, borderRadius: 6,
          background: type === 'new' ? '#ffece8' : '#f2f3f5',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontSize: 11, fontWeight: 700, flexShrink: 0,
          color: type === 'new' ? '#f53f3f' : '#86909c',
        }}>
          {item.rank}
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontWeight: 600, fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {item.stockName || item.stockCode}
          </div>
          <div style={{ fontSize: 11, color: '#86909c' }}>{item.stockCode}</div>
        </div>
        <div style={{ textAlign: 'right', flexShrink: 0 }}>
          <div style={{ fontWeight: 600, fontSize: 13, color: chgColor }}>
            {item.chgPct != null ? `${item.chgPct > 0 ? '+' : ''}${item.chgPct.toFixed(1)}%` : '-'}
          </div>
          <div style={{ fontSize: 11, color: '#86909c' }}>
            {item.close > 0 ? item.close.toFixed(2) : '-'}
          </div>
        </div>
      </div>
    );
  };

  return (
    <div>
      <div className="page-header">
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <h2 style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <CalendarDays size={20} color="#165dff" />
            <span style={{ fontSize: 16, fontWeight: 700 }}>历史榜单</span>
          </h2>
          {selectedDate && <Tag color="blue" style={{ fontSize: 12 }}>{selectedDate}</Tag>}
          <span className="muted" style={{ fontSize: 13 }}>回看任一交易日的算法榜单与次日表现</span>
        </div>
      </div>

      {/* Date chips */}
      <div style={{ marginBottom: 20 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap', padding: '4px 0' }}>
          <span style={{ fontSize: 12, color: '#86909c', marginRight: 4, flexShrink: 0 }}>交易日</span>
          {dates.map((d) => {
            const { md, day } = fmtDateChip(d);
            const isSelected = d === selectedDate;
            const isToday = d === new Date().toISOString().slice(0, 10);
            return (
              <Tooltip key={d} content={`${d} 榜单数据`}>
                <button
                  onClick={() => handleDateClick(d)}
                  style={{
                    flexShrink: 0, display: 'flex', flexDirection: 'column',
                    alignItems: 'center', gap: 0, padding: '5px 13px',
                    borderRadius: 8,
                    border: isSelected ? '2px solid #165dff' : '1px solid #e5e6eb',
                    background: isSelected ? '#e8f3ff' : '#fff',
                    cursor: 'pointer', minWidth: 54, transition: 'all 0.15s',
                    color: isSelected ? '#165dff' : '#1d2129', fontFamily: 'inherit',
                  }}
                >
                  <span style={{ fontSize: 12, fontWeight: isSelected ? 700 : 500, lineHeight: '18px' }}>
                    {isToday ? '今' : md}
                  </span>
                  <span style={{ fontSize: 10, color: isSelected ? '#165dff' : '#86909c', lineHeight: '16px' }}>
                    {isToday ? '今天' : day}
                  </span>
                </button>
              </Tooltip>
            );
          })}
        </div>
      </div>

      {/* Overview stats */}
      {selectedDate && data.length > 0 && (
        <div style={{ display: 'flex', gap: 12, marginBottom: 20, flexWrap: 'wrap' }}>
          {[
            { label: '上榜股数', value: data.length, unit: '只', color: '#165dff', icon: Star },
            { label: '次日上涨', value: nextUp, unit: '只', color: '#f53f3f', icon: TrendingUp, sub: `${((nextUp / data.length) * 100).toFixed(0)}%` },
            { label: '次日下跌', value: nextDown, unit: '只', color: '#00b42a', icon: TrendingDown, sub: `${((nextDown / data.length) * 100).toFixed(0)}%` },
            { label: '平均次日', value: nextAvg >= 0 ? `+${nextAvg.toFixed(2)}` : nextAvg.toFixed(2), unit: '%', color: nextAvg >= 0 ? '#f53f3f' : '#00b42a', icon: Zap },
            { label: '至今上涨', value: cumuUp, unit: '只', color: '#f53f3f', icon: TrendingUp, sub: `${((cumuUp / data.length) * 100).toFixed(0)}%` },
            { label: '至今下跌', value: cumuDown, unit: '只', color: '#00b42a', icon: TrendingDown, sub: `${((cumuDown / data.length) * 100).toFixed(0)}%` },
          ].map((card, i) => {
            const Icon = card.icon;
            return (
              <div key={i} style={{
                background: '#fff', borderRadius: 8, border: '1px solid #e5e6eb',
                padding: '12px 16px', display: 'flex', alignItems: 'center', gap: 10, minWidth: 130,
              }}>
                <div style={{
                  width: 36, height: 36, borderRadius: 8,
                  background: `${card.color}15`, display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>
                  <Icon size={17} color={card.color} />
                </div>
                <div>
                  <div style={{ fontSize: 11, color: '#86909c', marginBottom: 1 }}>{card.label}</div>
                  <div style={{ fontSize: 19, fontWeight: 700, color: '#1d2129', lineHeight: 1.2 }}>
                    {card.value}<span style={{ fontSize: 11, fontWeight: 400, color: '#86909c', marginLeft: 2 }}>{card.unit}</span>
                  </div>
                  {card.sub && <div style={{ fontSize: 10, color: '#86909c' }}>{card.sub}</div>}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Main layout */}
      <div style={{ display: 'flex', gap: 16 }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div className="card" style={{ overflow: 'hidden' }}>
            {loading ? (
              <div style={{ padding: 60, textAlign: 'center' }}>
                <Spin size={20} /><div style={{ marginTop: 10, color: '#86909c', fontSize: 13 }}>加载中...</div>
              </div>
            ) : data.length === 0 ? (
              <div style={{ padding: 80, textAlign: 'center' }}>
                <div style={{ fontSize: 40, marginBottom: 12 }}>📅</div>
                <div style={{ fontSize: 15, fontWeight: 600, color: '#1d2129' }}>暂无数据</div>
                <div style={{ fontSize: 13, color: '#86909c', marginTop: 4 }}>选择上方日期查看历史榜单</div>
              </div>
            ) : (
              <>
                <div className="card-header">
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <Star size={14} color="#165dff" />
                    <span style={{ fontSize: 14, fontWeight: 600 }}>{selectedDate} 榜单</span>
                    <Tag color="blue" style={{ fontSize: 11 }}>{data.length} 只</Tag>
                  </span>
                  {prevDate && <span className="muted" style={{ fontSize: 12 }}>对比 {prevDate}</span>}
                </div>
                <Table
                  columns={columns} data={data || []} rowKey="id" pagination={false}
                  scroll={{ x: 850 }} stripe size="small"
                  onRow={r => ({ onClick: () => navigate(`/stock/${r.stockCode}`), style: { cursor: 'pointer' } })}
                />
              </>
            )}
          </div>
        </div>

        {/* Right sidebar */}
        <div style={{ width: 290, flexShrink: 0, display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="card">
            <div className="card-header" style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '12px 16px' }}>
              <ArrowUpRight size={14} color="#f53f3f" />
              <span style={{ fontSize: 13, fontWeight: 600 }}>当日新晋</span>
              {newEntries.length > 0 && <Tag color="red" style={{ fontSize: 11, marginLeft: 4 }}>{newEntries.length}</Tag>}
            </div>
            <div className="card-body" style={{ padding: 0 }}>
              {newEntries.length === 0 ? (
                <div style={{ padding: 24, textAlign: 'center', color: '#86909c', fontSize: 13 }}>
                  {prevDate ? `相比 ${prevDate} 无新增` : '暂无对比数据'}
                </div>
              ) : (
                <div style={{ maxHeight: 340, overflow: 'auto' }}>
                  {newEntries.map(item => renderStockCard(item, 'new'))}
                </div>
              )}
            </div>
          </div>

          <div className="card">
            <div className="card-header" style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '12px 16px' }}>
              <ArrowDownRight size={14} color="#86909c" />
              <span style={{ fontSize: 13, fontWeight: 600 }}>当日剔除</span>
              {removedEntries.length > 0 && <Tag color="gray" style={{ fontSize: 11, marginLeft: 4 }}>{removedEntries.length}</Tag>}
            </div>
            <div className="card-body" style={{ padding: 0 }}>
              {removedEntries.length === 0 ? (
                <div style={{ padding: 24, textAlign: 'center', color: '#86909c', fontSize: 13 }}>
                  {prevDate ? `相比 ${prevDate} 无剔除` : '暂无对比数据'}
                </div>
              ) : (
                <div style={{ maxHeight: 340, overflow: 'auto' }}>
                  {removedEntries.map(item => renderStockCard(item, 'removed'))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
