import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Tag, Table, DatePicker } from '@arco-design/web-react';
import { Swords, TrendingUp, TrendingDown, ChevronRight, Calendar, RefreshCw, DollarSign } from 'lucide-react';
import { fetchDailyDragonTiger, fetchDragonTigerSeats } from '../services/api';

interface DTStock {
  id: number; code: string; name: string; tradeDate: string;
  reason: string; closePrice: number; changePct: number;
  netBuyAmt: number; buyAmt: number; sellAmt: number; turnoverPct: number;
}
interface DTSeat {
  id: number; code: string; tradeDate: string; seatName: string; seatCode: string;
  side: string; buyAmt: number; sellAmt: number; netAmt: number; isInstitution: boolean;
}

function fmtW(v: number): string {
  if (Math.abs(v) >= 10000) return (v / 10000).toFixed(1) + '亿';
  return (v / 10000).toFixed(0) + '万';
}

export default function DragonTigerPage() {
  const navigate = useNavigate();
  const [date, setDate] = useState('2026-06-29');
  const [list, setList] = useState<DTStock[]>([]);
  const [loading, setLoading] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [seats, setSeats] = useState<Record<string, DTSeat[]>>({});
  const [seatsLoading, setSeatsLoading] = useState<Record<string, boolean>>({});

  const load = async (d: string) => {
    setLoading(true);
    try {
      const r: any = await fetchDailyDragonTiger(d);
      setList(r.data?.data || []);
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

  const totalNet = list.reduce((s, d) => s + d.netBuyAmt, 0);
  const instCount = list.length;

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

      {/* 统计卡片 */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12 }}>
        {[
          { label: '上榜家数', value: instCount, icon: <Swords size={18} />, color: '#e8654c' },
          { label: '总净买额', value: fmtW(totalNet), icon: <DollarSign size={18} />, color: totalNet >= 0 ? '#00b42a' : '#f53f3f' },
          { label: '机构净买', value: fmtW(list.filter(d => d.netBuyAmt > 0).reduce((s, d) => s + d.netBuyAmt, 0)), icon: <TrendingUp size={18} />, color: '#165dff' },
          { label: '机构净卖', value: fmtW(Math.abs(list.filter(d => d.netBuyAmt < 0).reduce((s, d) => s + Math.abs(d.netBuyAmt), 0))), icon: <TrendingDown size={18} />, color: '#f53f3f' },
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

      {/* 龙虎榜列表 */}
      <div className="card">
        <div className="card-header" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Calendar size={14} color="var(--color-text-2)" />
          <span style={{ fontSize: 14, fontWeight: 600 }}>{date} 龙虎榜</span>
          {!loading && <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>共 {list.length} 只</span>}
        </div>
        <div className="card-body" style={{ padding: 0 }}>
          {list.length === 0 && !loading ? (
            <div style={{ padding: 40, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>
              {loading ? '加载中...' : '当日无龙虎榜数据（非交易日或盘后未更新）'}
            </div>
          ) : (
            <table style={{ width: '100%', tableLayout: 'fixed', borderCollapse: 'collapse', fontSize: 13 }}>
              <thead>
                <tr style={{ background: 'var(--color-fill-1)', borderBottom: '2px solid var(--color-border-2)' }}>
                  <th style={{ padding: '10px 12px', textAlign: 'left', width: 40 }}></th>
                  <th style={{ padding: '10px 12px', textAlign: 'left' }}>股票</th>
                  <th style={{ padding: '10px 12px', textAlign: 'right' }}>收盘价</th>
                  <th style={{ padding: '10px 12px', textAlign: 'right' }}>涨跌幅</th>
                  <th style={{ padding: '10px 12px', textAlign: 'right' }}>净买额</th>
                  <th style={{ padding: '10px 12px', textAlign: 'right' }}>买入</th>
                  <th style={{ padding: '10px 12px', textAlign: 'right' }}>卖出</th>
                  <th style={{ padding: '10px 12px', textAlign: 'left', maxWidth: 200 }}>上榜原因</th>
                </tr>
              </thead>
              <tbody>
                {list.map((d, i) => (
                  <>
                    <tr key={d.id} onClick={() => toggleSeats(d.code, d.tradeDate)}
                      style={{ borderBottom: '1px solid var(--color-border-1)', cursor: 'pointer', background: expanded === d.code ? 'var(--color-fill-1)' : 'transparent' }}
                      onMouseEnter={e => { if (expanded !== d.code) (e.currentTarget as HTMLElement).style.background = 'var(--color-fill-1)'; }}
                      onMouseLeave={e => { if (expanded !== d.code) (e.currentTarget as HTMLElement).style.background = 'transparent'; }}>
                      <td style={{ padding: '8px 12px', textAlign: 'center' }}>
                        <ChevronRight size={12} style={{ transform: expanded === d.code ? 'rotate(90deg)' : 'none', transition: '0.15s' }} />
                      </td>
                      <td style={{ padding: '8px 12px' }}>
                        <span style={{ fontWeight: 600, cursor: 'pointer', color: 'var(--color-primary)' }}
                          onClick={e => { e.stopPropagation(); navigate(`/stock/${d.code}`); }}>
                          {d.name}
                        </span>
                        <span style={{ fontSize: 11, color: 'var(--color-text-3)', marginLeft: 6 }}>{d.code}</span>
                      </td>
                      <td style={{ padding: '8px 12px', textAlign: 'right', fontFamily: "'SF Mono', monospace" }}>{d.closePrice?.toFixed(2)}</td>
                      <td style={{ padding: '8px 12px', textAlign: 'right', color: d.changePct > 0 ? 'var(--stock-up)' : d.changePct < 0 ? 'var(--stock-down)' : 'var(--color-text-2)', fontWeight: 600 }}>
                        {d.changePct > 0 ? '+' : ''}{d.changePct}%
                      </td>
                      <td style={{ padding: '8px 12px', textAlign: 'right', fontWeight: 600, color: d.netBuyAmt > 0 ? 'var(--stock-up)' : d.netBuyAmt < 0 ? 'var(--stock-down)' : 'var(--color-text-2)' }}>
                        {fmtW(d.netBuyAmt)}
                      </td>
                      <td style={{ padding: '8px 12px', textAlign: 'right', color: 'var(--color-text-2)' }}>{fmtW(d.buyAmt)}</td>
                      <td style={{ padding: '8px 12px', textAlign: 'right', color: 'var(--color-text-2)' }}>{fmtW(d.sellAmt)}</td>
                      <td style={{ padding: '8px 12px', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={d.reason}>
                        {d.reason?.length > 20 ? d.reason.slice(0, 20) + '...' : d.reason}
                      </td>
                    </tr>
                    {expanded === d.code && (
                      <tr key={`${d.id}-seats`}>
                        <td colSpan={8} style={{ padding: '0 12px 12px 60px', background: 'var(--color-fill-1)' }}>
                          <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 6, color: 'var(--color-text-2)' }}>席位明细</div>
                          {seatsLoading[d.code] ? <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>加载中...</span> :
                            seats[d.code]?.length === 0 ? <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>暂无席位数据</span> :
                            <table style={{ width: '100%', tableLayout: 'fixed', borderCollapse: 'collapse', fontSize: 12 }}>
                              <thead><tr style={{ borderBottom: '1px solid var(--color-border-1)' }}>
                                <th style={{ padding: '4px 8px', textAlign: 'left' }}>席位名称</th>
                                <th style={{ padding: '4px 8px', textAlign: 'right' }}>买入</th>
                                <th style={{ padding: '4px 8px', textAlign: 'right' }}>卖出</th>
                                <th style={{ padding: '4px 8px', textAlign: 'right' }}>净额</th>
                                <th style={{ padding: '4px 8px', textAlign: 'center' }}>类型</th>
                              </tr></thead>
                              <tbody>
                                {(seats[d.code] || []).map((s: DTSeat) => (
                                  <tr key={s.id} style={{ borderBottom: '1px solid var(--color-border-1)' }}>
                                    <td style={{ padding: '3px 8px' }}>{s.seatName}</td>
                                    <td style={{ padding: '3px 8px', textAlign: 'right', color: 'var(--stock-up)' }}>{fmtW(s.buyAmt)}</td>
                                    <td style={{ padding: '3px 8px', textAlign: 'right', color: 'var(--stock-down)' }}>{fmtW(s.sellAmt)}</td>
                                    <td style={{ padding: '3px 8px', textAlign: 'right', fontWeight: 600, color: s.netAmt > 0 ? 'var(--stock-up)' : 'var(--stock-down)' }}>{fmtW(s.netAmt)}</td>
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
