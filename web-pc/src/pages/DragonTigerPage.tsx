import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Tag, DatePicker } from '@arco-design/web-react';
import { Swords, TrendingUp, TrendingDown, ChevronRight, ChevronDown, Calendar, RefreshCw, DollarSign, ArrowUp, ArrowDown, Target } from 'lucide-react';
import { fetchDailyDragonTigerEnriched, fetchDragonTigerSeats } from '../services/api';

interface DTEnriched {
  id: number; code: string; name: string; tradeDate: string;
  reason: string; closePrice: number; changePct: number;
  netBuyAmt: number; buyAmt: number; sellAmt: number; turnoverPct: number;
  isToday: boolean; isYesterday: boolean; cnt5d: number; cnt20d: number; consecutiveDays: number;
  isAlgorithmPick: boolean; algorithmRank: number; algorithmScore: number;
}
interface DTSeat {
  id: number; code: string; tradeDate: string; seatName: string; seatCode: string;
  side: string; buyAmt: number; sellAmt: number; netAmt: number; isInstitution: boolean;
}

function fmtW(v: number): string {
  if (Math.abs(v) >= 10000) return (v / 10000).toFixed(1) + '亿';
  return (v / 10000).toFixed(0) + '万';
}

type SortKey = 'netBuyAmt' | 'changePct' | 'cnt5d' | 'cnt20d' | 'consecutiveDays' | 'buyAmt' | 'sellAmt';

export default function DragonTigerPage() {
  const navigate = useNavigate();
  const [date, setDate] = useState('');
  const [list, setList] = useState<DTEnriched[]>([]);
  const [loading, setLoading] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [seats, setSeats] = useState<Record<string, DTSeat[]>>({});
  const [seatsLoading, setSeatsLoading] = useState<Record<string, boolean>>({});
  const [sortKey, setSortKey] = useState<SortKey>('netBuyAmt');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc');

  const load = async (d: string) => {
    setLoading(true);
    try {
      const r: any = await fetchDailyDragonTigerEnriched(d);
      const data = r.data?.data || [];
      setList(data);
      // If auto-detected (no date passed), update date picker to match loaded data
      if (!d && data.length > 0 && data[0].tradeDate) {
        setDate(data[0].tradeDate);
      }
    } catch { setList([]); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(date); }, [date]);

  const toggleSeats = async (code: string, tradeDate: string) => {
    if (expanded === code) { setExpanded(null); return; }
    setExpanded(code);
    if (seats[code]) return;
    setSeatsLoading(p => ({ ...p, [code]: true }));
    try {
      const r: any = await fetchDragonTigerSeats(code, tradeDate);
      setSeats(p => ({ ...p, [code]: r.data?.data || [] }));
    } catch { setSeats(p => ({ ...p, [code]: [] })); }
    finally { setSeatsLoading(p => ({ ...p, [code]: false })); }
  };

  const handleSort = (key: SortKey) => {
    if (sortKey === key) { setSortDir(d => d === 'desc' ? 'asc' : 'desc'); }
    else { setSortKey(key); setSortDir('desc'); }
  };

  const sortedList = [...list].sort((a, b) => {
    const va = (a as any)[sortKey] ?? 0;
    const vb = (b as any)[sortKey] ?? 0;
    return sortDir === 'desc' ? vb - va : va - vb;
  });

  const totalNet = list.reduce((s, d) => s + d.netBuyAmt, 0);

  const SortIcon = ({ k }: { k: SortKey }) => (
    <span style={{ display: 'inline-flex', alignItems: 'center', cursor: 'pointer' }} onClick={() => handleSort(k)}>
      {sortKey === k ? (sortDir === 'desc' ? <ArrowDown size={10} /> : <ArrowUp size={10} />) : <ChevronDown size={10} style={{ opacity: 0.3 }} />}
    </span>
  );

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div className="page-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <h2><Swords size={20} style={{ marginRight: 8 }} />龙虎榜追踪</h2>
          <span className="muted">机构动向 · 游资席位 · 每日追踪</span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <DatePicker value={date} onChange={(v: string) => setDate(v)} style={{ width: 140 }} />
          <Button size="small" icon={<RefreshCw size={12} />} loading={loading} onClick={() => load(date)}>刷新</Button>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 12 }}>
        {[
          { label: '上榜家数', value: list.length, icon: <Swords size={18} />, color: '#e8654c' },
          { label: '总净买额', value: fmtW(totalNet), icon: <DollarSign size={18} />, color: totalNet >= 0 ? '#00b42a' : '#f53f3f' },
          { label: '今日精选上榜', value: list.filter(d => d.isToday).length, icon: <Target size={18} />, color: '#722ED1' },
          { label: '连续上榜≥3天', value: list.filter(d => d.consecutiveDays >= 3).length, icon: <TrendingUp size={18} />, color: '#e8654c' },
          { label: '算法精选', value: list.filter(d => d.isAlgorithmPick).length, icon: <Target size={18} />, color: '#722ED1' },
        ].map((c, i) => (
          <div key={i} className="card" style={{ padding: '14px 16px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
              <div style={{ color: c.color }}>{c.icon}</div>
              <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>{c.label}</span>
            </div>
            <div style={{ fontSize: 20, fontWeight: 700, color: c.color }}>{c.value}</div>
          </div>
        ))}
      </div>

      <div className="card">
        <div className="card-header" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Calendar size={14} color="var(--color-text-2)" />
          <span style={{ fontSize: 14, fontWeight: 600 }}>{date} 龙虎榜</span>
          {!loading && <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>共 {list.length} 只</span>}
        </div>
        <div className="card-body" style={{ padding: 0, overflow: 'auto' }}>
          {list.length === 0 && !loading ? (
            <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>
              当日无龙虎榜数据（非交易日或盘后未更新）
            </div>
          ) : (
            <table style={{ width: '100%', minWidth: 900, tableLayout: 'fixed', borderCollapse: 'collapse', fontSize: 12 }}>
              <thead>
                <tr style={{ background: 'var(--color-fill-1)', borderBottom: '2px solid var(--color-border-2)' }}>
                  <th style={{ padding: '8px 10px', textAlign: 'left', width: 30 }}></th>
                  <th style={{ padding: '8px 10px', textAlign: 'left', width: 120 }}>股票</th>
                  <th style={{ padding: '8px 10px', textAlign: 'right', width: 60 }}>涨跌幅</th>
                  <th style={{ padding: '8px 10px', textAlign: 'right', width: 80, cursor: 'pointer' }} onClick={() => handleSort('netBuyAmt')}>
                    净买额 <SortIcon k="netBuyAmt" />
                  </th>
                  <th style={{ padding: '8px 10px', textAlign: 'right', width: 70 }}>买入</th>
                  <th style={{ padding: '8px 10px', textAlign: 'right', width: 70 }}>卖出</th>
                  <th style={{ padding: '8px 10px', textAlign: 'center', width: 90 }}>精选标签</th>
                  <th style={{ padding: '8px 10px', textAlign: 'center', width: 60, cursor: 'pointer' }} onClick={() => handleSort('cnt5d')}>
                    精选5日 <SortIcon k="cnt5d" />
                  </th>
                  <th style={{ padding: '8px 10px', textAlign: 'center', width: 70, cursor: 'pointer' }} onClick={() => handleSort('cnt20d')}>
                    精选20日 <SortIcon k="cnt20d" />
                  </th>
                  <th style={{ padding: '8px 10px', textAlign: 'center', width: 70, cursor: 'pointer' }} onClick={() => handleSort('consecutiveDays')}>
                    连续精选 <SortIcon k="consecutiveDays" />
                  </th>
                  <th style={{ padding: '8px 10px', textAlign: 'left', width: 140 }}>上榜原因</th>
                </tr>
              </thead>
              <tbody>
                {sortedList.map((d) => (
                  <>
                    <tr key={d.id} onClick={() => toggleSeats(d.code, d.tradeDate)}
                      style={{ borderBottom: '1px solid var(--color-border-1)', cursor: 'pointer', background: expanded === d.code ? 'var(--color-fill-1)' : 'transparent' }}
                      onMouseEnter={e => { if (expanded !== d.code) (e.currentTarget as HTMLElement).style.background = 'var(--color-fill-1)'; }}
                      onMouseLeave={e => { if (expanded !== d.code) (e.currentTarget as HTMLElement).style.background = 'transparent'; }}>
                      <td style={{ padding: '6px 10px', textAlign: 'center' }}>
                        {(expanded === d.code ? <ChevronDown size={12} /> : <ChevronRight size={12} />)}
                      </td>
                      <td style={{ padding: '6px 10px' }}>
                        <span style={{ fontWeight: 600, cursor: 'pointer', color: 'var(--color-primary)' }}
                          onClick={e => { e.stopPropagation(); navigate('/stock/' + d.code); }}>
                          {d.name}
                        </span>
                        <span style={{ fontSize: 10, color: 'var(--color-text-3)', marginLeft: 4 }}>{d.code}</span>
                      </td>
                      <td style={{ padding: '6px 10px', textAlign: 'right', fontWeight: 600,
                        color: d.changePct > 0 ? '#F53F3F' : d.changePct < 0 ? '#00B42A' : 'var(--color-text-2)' }}>
                        {d.changePct > 0 ? '+' : ''}{d.changePct}%
                      </td>
                      <td style={{ padding: '6px 10px', textAlign: 'right', fontWeight: 600, fontVariantNumeric: 'tabular-nums',
                        color: d.netBuyAmt > 0 ? '#F53F3F' : d.netBuyAmt < 0 ? '#00B42A' : 'var(--color-text-2)' }}>
                        {fmtW(d.netBuyAmt)}
                      </td>
                      <td style={{ padding: '6px 10px', textAlign: 'right', color: '#F53F3F' }}>{fmtW(d.buyAmt)}</td>
                      <td style={{ padding: '6px 10px', textAlign: 'right', color: '#00B42A' }}>{fmtW(d.sellAmt)}</td>
                      <td style={{ padding: '6px 10px', textAlign: 'center' }}>
                        <div style={{ display: 'flex', gap: 4, justifyContent: 'center', flexWrap: 'wrap' }}>
                          {d.isAlgorithmPick && (
                            <Tag color="#722ED1" style={{ fontSize: 10, lineHeight: '16px', background: '#722ED115', border: '1px solid #722ED130' }}>
                              精选 #{d.algorithmRank}
                            </Tag>
                          )}
                          {d.isToday && <Tag color="red" style={{ fontSize: 10, lineHeight: '16px' }}>今日</Tag>}
                          {d.isYesterday && <Tag color="orange" style={{ fontSize: 10, lineHeight: '16px' }}>昨日</Tag>}
                        </div>
                      </td>
                      <td style={{ padding: '6px 10px', textAlign: 'center', fontWeight: 600,
                        color: d.cnt5d >= 3 ? '#F53F3F' : d.cnt5d >= 1 ? '#e8654c' : 'var(--color-text-3)' }}>
                        {d.cnt5d}次
                      </td>
                      <td style={{ padding: '6px 10px', textAlign: 'center', fontWeight: 600,
                        color: d.cnt20d >= 5 ? '#F53F3F' : d.cnt20d >= 2 ? '#e8654c' : 'var(--color-text-3)' }}>
                        {d.cnt20d}次
                      </td>
                      <td style={{ padding: '6px 10px', textAlign: 'center', fontWeight: 600,
                        color: d.consecutiveDays >= 3 ? '#F53F3F' : d.consecutiveDays >= 1 ? '#e8654c' : 'var(--color-text-3)' }}>
                        {d.consecutiveDays}天
                      </td>
                      <td style={{ padding: '6px 10px', maxWidth: 140, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 11, color: 'var(--color-text-2)' }}
                        title={d.reason}>
                        {d.reason?.length > 18 ? d.reason.slice(0, 18) + '...' : d.reason}
                      </td>
                    </tr>
                    {expanded === d.code && (
                      <tr key={d.id + '-seats'}>
                        <td colSpan={12} style={{ padding: '0 12px 12px 50px', background: 'var(--color-fill-1)' }}>
                          <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 6, color: 'var(--color-text-2)' }}>席位明细</div>
                          {seatsLoading[d.code] ? <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>加载中...</span> :
                            seats[d.code]?.length === 0 ? <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>暂无席位数据</span> :
                            <table style={{ width: '100%', tableLayout: 'fixed', borderCollapse: 'collapse', fontSize: 11 }}>
                              <thead><tr style={{ borderBottom: '1px solid var(--color-border-1)' }}>
                                <th style={{ padding: '4px 8px', textAlign: 'center', width: 48 }}>方向</th>
                                <th style={{ padding: '4px 8px', textAlign: 'left' }}>席位名称</th>
                                <th style={{ padding: '4px 8px', textAlign: 'right' }}>买入</th>
                                <th style={{ padding: '4px 8px', textAlign: 'right' }}>卖出</th>
                                <th style={{ padding: '4px 8px', textAlign: 'right' }}>净额</th>
                                <th style={{ padding: '4px 8px', textAlign: 'center' }}>类型</th>
                              </tr></thead>
                              <tbody>
                                {(seats[d.code] || []).map((s: DTSeat) => (
                                  <tr key={s.id} style={{ borderBottom: '1px solid var(--color-border-1)' }}>
                                    <td style={{ padding: '3px 8px', textAlign: 'center' }}>
                                      <Tag color={s.side === 'buy' ? 'red' : 'green'} style={{ fontSize: 10 }}>
                                        {s.side === 'buy' ? '买入' : '卖出'}
                                      </Tag>
                                    </td>
                                    <td style={{ padding: '3px 8px' }}>{s.seatName}</td>
                                    <td style={{ padding: '3px 8px', textAlign: 'right', color: '#F53F3F' }}>{fmtW(s.buyAmt)}</td>
                                    <td style={{ padding: '3px 8px', textAlign: 'right', color: '#00B42A' }}>{fmtW(s.sellAmt)}</td>
                                    <td style={{ padding: '3px 8px', textAlign: 'right', fontWeight: 600, color: s.netAmt > 0 ? '#F53F3F' : '#00B42A' }}>{fmtW(s.netAmt)}</td>
                                    <td style={{ padding: '3px 8px', textAlign: 'center' }}>
                                      <Tag color={s.isInstitution ? 'red' : 'blue'} style={{ fontSize: 10 }}>
                                        {s.isInstitution ? '机构' : '游资'}
                                      </Tag>
                                    </td>
                                  </tr>
                                ))}
                              </tbody>
                            </table>
                          }
                        </td>
                      </tr>
                    )}
                  </>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
}
