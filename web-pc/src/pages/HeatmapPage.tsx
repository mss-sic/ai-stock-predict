import { useEffect, useState, useMemo } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { fetchEnrichedHeatmap, fetchConceptHeatmap } from '../services/api';
import { Flame } from 'lucide-react';

/* types for enriched data */
interface EnrichedCell {
  pickDate: string; stockCode: string; stockName: string;
  rank: number; score: number; open: number; close: number; chgPct: number;
}

interface StockRow {
  code: string; name: string;
  appearances: number; latestStreak: number; maxStreak: number;
  cells: (EnrichedCell | null)[];
}

/* ── color helpers (matching reference) ── */
function chgCell(chg: number): { bg: string; fg: string } {
  const t = Math.max(0, Math.min(1, Math.abs(chg) / 9));
  if (chg > 0.05) {
    const a = 0.16 + t * 0.78;
    return { bg: `rgba(245, 63, 63, ${a.toFixed(3)})`, fg: a > 0.5 ? '#fff' : 'rgb(203,39,45)' };
  }
  if (chg < -0.05) {
    const a = 0.16 + t * 0.78;
    return { bg: `rgba(0, 180, 42, ${a.toFixed(3)})`, fg: a > 0.5 ? '#fff' : 'rgb(0,128,38)' };
  }
  return { bg: '#f2f3f5', fg: '#86909c' };
}

function scoreBg(score: number): { bg: string; fg: string } {
  if (score <= 0) return { bg: '#f2f3f5', fg: '#86909c' };
  const t = Math.min(1, (score - 60) / 39);
  const a = 0.22 + t * 0.74;
  return { bg: `rgba(22, 93, 255, ${a.toFixed(3)})`, fg: t > 0.55 ? '#fff' : 'rgb(15,65,200)' };
}

export default function HeatmapPage() {
  const [raw, setRaw] = useState<EnrichedCell[]>([]);
  const [conceptData, setConceptData] = useState<any[]>([]);
    const location = useLocation();
  const [view, setView] = useState<'calendar' | 'matrix' | 'concept'>(
    location.pathname === '/board/concepts' ? 'concept' : 'calendar'
  );
  const [colorBy, setColorBy] = useState<'chg' | 'score'>('chg');
  const [sortKey, setSortKey] = useState<'appearances' | 'streak' | 'score'>('appearances');
  const [hover, setHover] = useState<{ cell: EnrichedCell; row: StockRow; colIdx: number } | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    fetchEnrichedHeatmap().then((res: any) => setRaw(res.data?.data || []));
    fetchConceptHeatmap().then((res: any) => setConceptData(res.data?.data || []));
  }, []);

  /* ── derived data ── */
  const { dates, sortedStocks } = useMemo(() => {
    const dateSet = new Set<string>();
    const codeMap = new Map<string, { name: string; cells: EnrichedCell[] }>();

    for (const r of raw) {
      const d = r.pickDate?.slice(0, 10);
      if (!d) continue;
      dateSet.add(d);
      if (!codeMap.has(r.stockCode)) codeMap.set(r.stockCode, { name: r.stockName || r.stockCode, cells: [] });
      codeMap.get(r.stockCode)!.cells.push(r);
    }

    const datesArr = [...dateSet].sort().reverse(); // newest first
    const rows: StockRow[] = [];

    for (const [code, info] of codeMap) {
      const dateCells = datesArr.map(d => info.cells.find(c => c.pickDate?.slice(0, 10) === d) || null);
      const appearances = dateCells.filter(Boolean).length;
      let latestStreak = 0, maxStreak = 0, cur = 0;
      for (let i = datesArr.length - 1; i >= 0; i--) {
        if (dateCells[i]) { cur++; maxStreak = Math.max(maxStreak, cur); }
        else { cur = 0; }
      }
      latestStreak = cur;
      rows.push({ code, name: info.name, appearances, latestStreak, maxStreak, cells: dateCells });
    }
    rows.sort((a, b) => {
      if (sortKey === 'streak') return b.latestStreak - a.latestStreak || b.appearances - a.appearances;
      if (sortKey === 'score') {
        const avgA = a.cells.filter(Boolean).reduce((s, c) => s + (c!.score || 0), 0) / Math.max(1, a.appearances);
        const avgB = b.cells.filter(Boolean).reduce((s, c) => s + (c!.score || 0), 0) / Math.max(1, b.appearances);
        return avgB - avgA || b.appearances - a.appearances;
      }
      return b.appearances - a.appearances || b.latestStreak - a.latestStreak;
    });
    return { dates: datesArr, sortedStocks: rows };
  }, [raw, sortKey]);

  /* ── distribution ── */
  const dist = useMemo(() => {
    const d: Record<number, number> = {};
    sortedStocks.forEach(s => { d[s.appearances] = (d[s.appearances] || 0) + 1; });
    return d;
  }, [sortedStocks]);

  const streakLeaders = useMemo(() => {
    return [...sortedStocks].sort((a, b) => b.latestStreak - a.latestStreak).slice(0, 8);
  }, [sortedStocks]);

  /* ═══════════════════════════════════════════
     Calendar View — date columns, chg% colored
     ═══════════════════════════════════════════ */
  if (view === 'calendar') {
    const cols = dates.map((d, di) => {
      const members = sortedStocks
        .filter(s => s.cells[di])
        .map(s => s.cells[di]!);
      members.sort((a, b) => b.chgPct - a.chgPct);
      const upN = members.filter(m => m.chgPct > 0).length;
      return { date: d, di, members, upN };
    });

    const colW = 158, rowH = 26;

    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {/* Controls */}
        <div style={{
          padding: '12px 18px', display: 'flex', alignItems: 'center', gap: 14,
          background: '#fff', borderRadius: 6, border: '1px solid #e5e6eb',
        }}>
          <span style={{ fontSize: 16, fontWeight: 600, color: '#1d2129' }}>20 交易日上榜热力图</span>
          <span style={{ fontSize: 12, color: '#86909c' }}>按日成列 · 单元格颜色 = 当日涨跌幅</span>
          <div style={{ flex: 1 }} />
          <div style={{ display: 'flex', borderRadius: 4, overflow: 'hidden', border: '1px solid #e5e6eb' }}>
            <button onClick={() => setView('calendar')} style={{
              padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
              background: view === 'calendar' ? '#e8f3ff' : '#fff',
              color: view === 'calendar' ? '#165dff' : '#4e5969', fontWeight: view === 'calendar' ? 500 : 400,
            }}>榜单日历</button>
            <button onClick={() => setView('matrix')} style={{
              padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
              background: view === 'matrix' ? '#e8f3ff' : '#fff',
              color: view === 'matrix' ? '#165dff' : '#4e5969', fontWeight: view === 'matrix' ? 500 : 400,
              borderLeft: '1px solid #e5e6eb',
            }}>矩阵热力</button>
            <button onClick={() => setView('concept')} style={{
              padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
              background: view === 'concept' ? '#e8f3ff' : '#fff',
              color: view === 'concept' ? '#165dff' : '#4e5969', fontWeight: view === 'concept' ? 500 : 400,
              borderLeft: '1px solid #e5e6eb',
            }}>概念板块</button>
          </div>
        </div>

        {/* Calendar grid */}
        <div style={{
          background: '#fff', borderRadius: 6, border: '1px solid #e5e6eb',
          overflow: 'hidden',
        }}>
          <div style={{ overflow: 'auto', padding: '12px 14px 16px' }}>
            <div style={{ display: 'flex', gap: 6, minWidth: cols.length * (colW + 6) }}>
              {cols.map((c, ci) => {
                const isToday = ci === 0;
                return (
                  <div key={c.date} style={{ width: colW, flex: 'none' }}>
                    {/* column header */}
                    <div style={{
                      textAlign: 'center', paddingBottom: 6, marginBottom: 6,
                      borderBottom: isToday ? '2px solid #165dff' : '1px solid #e5e6eb',
                    }}>
                      <div style={{
                        fontSize: 13, fontWeight: 600,
                        color: isToday ? '#165dff' : '#1d2129',
                        fontFamily: 'var(--font-family-mono, monospace)',
                      }}>{c.date}</div>
                      <div style={{ fontSize: 11, color: '#86909c', marginTop: 2 }}>
                        <span style={{ color: '#f53f3f', fontWeight: 500 }}>涨{c.upN}</span>
                        <span style={{ margin: '0 4px', color: '#c9cdd4' }}>/</span>
                        <span style={{ color: '#00b42a', fontWeight: 500 }}>跌{c.members.length - c.upN}</span>
                      </div>
                    </div>

                    {/* cells */}
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                      {c.members.map((m, mi) => {
                        const colors = chgCell(m.chgPct);
                        return (
                          <div
                            key={m.stockCode}
                            onClick={() => navigate(`/stock/${m.stockCode}`)}
                            style={{
                              height: rowH, display: 'flex', alignItems: 'center',
                              padding: '0 8px', borderRadius: 3,
                              background: colors.bg, color: colors.fg,
                              cursor: 'pointer', fontSize: 12, gap: 6,
                              transition: 'transform 100ms',
                            }}
                            onMouseEnter={e => { (e.currentTarget as HTMLElement).style.transform = 'scale(1.02)'; }}
                            onMouseLeave={e => { (e.currentTarget as HTMLElement).style.transform = ''; }}
                          >
                            <span style={{ fontWeight: 600, lineHeight: 1.1, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{m.stockName}</span>
                            <span style={{ fontSize: 11, opacity: 0.95, fontFamily: 'var(--font-family-mono, monospace)', whiteSpace: 'nowrap' }}>
                              {m.chgPct >= 0 ? '+' : ''}{m.chgPct.toFixed(1)}%
                            </span>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </div>

        {/* Legend */}
        <div style={{
          padding: '10px 18px', display: 'flex', alignItems: 'center', gap: 16,
          background: '#fff', borderRadius: 6, border: '1px solid #e5e6eb',
          fontSize: 12, color: '#4e5969',
        }}>
          <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(245,63,63,0.94)' }} />
            涨≥9%
          </span>
          <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(245,63,63,0.50)' }} />
            涨~5%
          </span>
          <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(245,63,63,0.20)' }} />
            涨~1%
          </span>
          <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <span style={{ width: 14, height: 14, borderRadius: 3, background: '#f2f3f5' }} />
            平
          </span>
          <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(0,180,42,0.20)' }} />
            跌~1%
          </span>
          <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(0,180,42,0.50)' }} />
            跌~5%
          </span>
          <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(0,180,42,0.94)' }} />
            跌≥9%
          </span>
          <span style={{ flex: 1 }} />
          <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(22,93,255,0.40)' }} />
            评分70+
          </span>
          <span style={{ color: '#86909c' }}>红色越深涨幅越大 · 绿色越深跌幅越大</span>
        </div>
      </div>
    );
  }

  /* ═══════════════════════════════════════════
     Matrix View — stock × date grid
     ═══════════════════════════════════════════ */
  const maxAppear = Math.max(...sortedStocks.map(s => s.appearances));

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {/* Controls */}
      <div style={{
        padding: '12px 18px', display: 'flex', alignItems: 'center', gap: 14, flexWrap: 'wrap',
        background: '#fff', borderRadius: 6, border: '1px solid #e5e6eb',
      }}>
        <span style={{ fontSize: 16, fontWeight: 600, color: '#1d2129' }}>20 交易日上榜热力图</span>
        <span style={{ fontSize: 12, color: '#86909c' }}>
          {colorBy === 'chg' ? '单元格颜色 = 当日涨跌幅' : '单元格颜色 = 当日算法评分'}
        </span>
        <div style={{ flex: 1 }} />
        <div style={{ display: 'flex', borderRadius: 4, overflow: 'hidden', border: '1px solid #e5e6eb' }}>
          <button onClick={() => setView('calendar')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: view === 'calendar' ? '#e8f3ff' : '#fff',
            color: view === 'calendar' ? '#165dff' : '#4e5969',
          }}>榜单日历</button>
          <button onClick={() => setView('matrix')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: view === 'matrix' ? '#e8f3ff' : '#fff',
            color: view === 'matrix' ? '#165dff' : '#4e5969', fontWeight: view === 'matrix' ? 500 : 400,
            borderLeft: '1px solid #e5e6eb',
          }}>矩阵热力</button>
        </div>
        <div style={{ display: 'flex', borderRadius: 4, overflow: 'hidden', border: '1px solid #e5e6eb' }}>
          <button onClick={() => setColorBy('chg')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: colorBy === 'chg' ? '#e8f3ff' : '#fff',
            color: colorBy === 'chg' ? '#165dff' : '#4e5969', fontWeight: colorBy === 'chg' ? 500 : 400,
          }}>按涨跌幅</button>
          <button onClick={() => setColorBy('score')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: colorBy === 'score' ? '#e8f3ff' : '#fff',
            color: colorBy === 'score' ? '#165dff' : '#4e5969', fontWeight: colorBy === 'score' ? 500 : 400,
            borderLeft: '1px solid #e5e6eb',
          }}>按评分</button>
        </div>
        <div style={{ display: 'flex', borderRadius: 4, overflow: 'hidden', border: '1px solid #e5e6eb' }}>
          <button onClick={() => setSortKey('appearances')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: sortKey === 'appearances' ? '#e8f3ff' : '#fff',
            color: sortKey === 'appearances' ? '#165dff' : '#4e5969', fontWeight: sortKey === 'appearances' ? 500 : 400,
          }}>排:上榜</button>
          <button onClick={() => setSortKey('streak')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: sortKey === 'streak' ? '#e8f3ff' : '#fff',
            color: sortKey === 'streak' ? '#165dff' : '#4e5969', fontWeight: sortKey === 'streak' ? 500 : 400,
            borderLeft: '1px solid #e5e6eb',
          }}>排:连榜</button>
          <button onClick={() => setSortKey('score')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: sortKey === 'score' ? '#e8f3ff' : '#fff',
            color: sortKey === 'score' ? '#165dff' : '#4e5969', fontWeight: sortKey === 'score' ? 500 : 400,
            borderLeft: '1px solid #e5e6eb',
          }}>排:评分</button>
        </div>
      </div>

      {/* Main content: grid + sidebar */}
      <div style={{ display: 'flex', gap: 12 }}>
        {/* LEFT: matrix grid */}
        <div style={{
          flex: 1, background: '#fff', borderRadius: 6,
          border: '1px solid #e5e6eb', overflow: 'hidden',
        }}>
          {/* header row: stock name + date columns */}
          <div style={{
            display: 'flex', borderBottom: '2px solid #e5e6eb',
            background: '#f7f8fa', position: 'sticky', top: 0, zIndex: 2,
          }}>
            {/* stock name header */}
            <div style={{
              width: 120, flex: 'none', padding: '8px 12px',
              fontSize: 12, fontWeight: 600, color: '#4e5969',
              borderRight: '1px solid #e5e6eb', display: 'flex', alignItems: 'center', gap: 4,
            }}>
              <span>{sortedStocks.length} 股</span>
              <span style={{ fontSize: 10, color: '#86909c', fontWeight: 400 }}>
                (上榜{maxAppear}/{dates.length})
              </span>
            </div>
            {dates.map((d, di) => {
              const isToday = di === 0;
              return (
                <div key={d} style={{
                  width: 40, flex: 'none', padding: '8px 0', textAlign: 'center',
                  fontSize: 11, fontWeight: isToday ? 600 : 400,
                  color: isToday ? '#165dff' : '#86909c',
                  borderLeft: '1px solid #f2f3f5',
                  fontFamily: 'var(--font-family-mono, monospace)',
                }}>
                  {d.slice(5)}
                </div>
              );
            })}
          </div>

          {/* grid body */}
          <div style={{ overflow: 'auto', maxHeight: 'calc(100vh - 280px)' }}>
            {sortedStocks.map((s, ri) => (
              <div key={s.code} style={{
                display: 'flex', borderBottom: '1px solid #f2f3f5',
                background: ri % 2 === 0 ? '#fff' : '#fafbfc',
              }}
                onMouseEnter={() => {}}
              >
                {/* stock name */}
                <div style={{
                  width: 120, flex: 'none', padding: '6px 12px',
                  fontSize: 12, color: '#1d2129',
                  borderRight: '1px solid #f2f3f5',
                  display: 'flex', alignItems: 'center', gap: 6,
                  cursor: 'pointer',
                }} onClick={() => navigate(`/stock/${s.code}`)}>
                  <span style={{ fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{s.name}</span>
                  <span style={{ fontSize: 10, color: '#86909c', fontFamily: 'var(--font-family-mono, monospace)', whiteSpace: 'nowrap' }}>{s.code}</span>
                </div>

                {/* cells */}
                {s.cells.map((cell, di) => {
                  if (!cell) {
                    return (
                      <div key={di} style={{
                        width: 40, flex: 'none', padding: '6px 0',
                        textAlign: 'center', borderLeft: '1px solid #f2f3f5',
                      }}>
                        <div style={{
                          width: 30, height: 22, margin: '0 auto',
                          borderRadius: 3, background: '#f7f8fa',
                          display: 'flex', alignItems: 'center', justifyContent: 'center',
                        }}>
                          <span style={{ fontSize: 9, color: '#c9cdd4' }}>—</span>
                        </div>
                      </div>
                    );
                  }

                  const colors = colorBy === 'chg' ? chgCell(cell.chgPct) : scoreBg(cell.score || (100 - (cell.rank - 1) * 2));
                  const val = colorBy === 'chg'
                    ? `${cell.chgPct >= 0 ? '+' : ''}${cell.chgPct.toFixed(1)}%`
                    : `${(cell.score || (100 - (cell.rank - 1) * 2)).toFixed(0)}`;

                  return (
                    <div key={di} style={{
                      width: 40, flex: 'none', padding: '6px 0', textAlign: 'center',
                      borderLeft: '1px solid #f2f3f5',
                    }}>
                      <div
                        onMouseEnter={() => setHover({ cell, row: s, colIdx: di })}
                        onMouseLeave={() => setHover(null)}
                        onClick={() => navigate(`/stock/${cell.stockCode}`)}
                        style={{
                          width: 30, height: 22, margin: '0 auto',
                          borderRadius: 3, background: colors.bg, color: colors.fg,
                          display: 'flex', alignItems: 'center', justifyContent: 'center',
                          cursor: 'pointer', fontSize: 9, fontWeight: 600,
                          fontFamily: 'var(--font-family-mono, monospace)',
                          transition: 'transform 100ms',
                        }}
                      >
                        {val}
                      </div>
                    </div>
                  );
                })}
              </div>
            ))}
          </div>
        </div>

        {/* RIGHT sidebar */}
        <div style={{ width: 220, flex: 'none', display: 'flex', flexDirection: 'column', gap: 12 }}>
          {/* hover detail card */}
          <div style={{ background: '#fff', borderRadius: 6, border: '1px solid #e5e6eb' }}>
            <div style={{
              padding: '10px 14px', borderBottom: '1px solid #e5e6eb',
              fontSize: 13, fontWeight: 600, color: '#1d2129',
            }}>
              {hover ? hover.cell.stockName : '悬停查看明细'}
            </div>
            <div style={{ padding: '12px 14px' }}>
              {hover ? (
                <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr', rowGap: 8, columnGap: 12, fontSize: 13 }}>
                  <span style={{ color: '#86909c' }}>日期</span>
                  <span style={{ fontFamily: 'var(--font-family-mono, monospace)', color: '#1d2129' }}>{dates[hover.colIdx]}</span>
                  <span style={{ color: '#86909c' }}>状态</span>
                  <span style={{ color: '#f53f3f', fontWeight: 500 }}>在榜 #{hover.cell.rank}</span>
                  <span style={{ color: '#86909c' }}>涨跌</span>
                  <span style={{
                    fontFamily: 'var(--font-family-mono, monospace)',
                    color: hover.cell.chgPct >= 0 ? '#f53f3f' : '#00b42a', fontWeight: 500,
                  }}>
                    {hover.cell.chgPct >= 0 ? '+' : ''}{hover.cell.chgPct.toFixed(2)}%
                  </span>
                  <span style={{ color: '#86909c' }}>评分</span>
                  <span style={{ fontFamily: 'var(--font-family-mono, monospace)', color: '#1d2129' }}>
                    {(hover.cell.score || (100 - (hover.cell.rank - 1) * 2)).toFixed(0)}
                  </span>
                  <span style={{ color: '#86909c' }}>累计上榜</span>
                  <span style={{ fontWeight: 500, color: '#1d2129' }}>{hover.row.appearances} / {dates.length} 日</span>
                  <span style={{ color: '#86909c' }}>当前连榜</span>
                  <span style={{ fontWeight: 600, color: hover.row.latestStreak >= 5 ? '#f53f3f' : '#1d2129' }}>
                    {hover.row.latestStreak} 日
                  </span>
                  <span style={{ color: '#86909c' }}>历史最长</span>
                  <span style={{ fontWeight: 500, color: '#1d2129' }}>{hover.row.maxStreak} 日</span>
                </div>
              ) : (
                <p style={{ fontSize: 12, color: '#86909c', margin: 0, lineHeight: '20px' }}>
                  将鼠标移到任意单元格，查看该股在某交易日的上榜状态、涨跌幅与算法评分。
                </p>
              )}
            </div>
          </div>

          {/* distribution */}
          <div style={{ background: '#fff', borderRadius: 6, border: '1px solid #e5e6eb' }}>
            <div style={{
              padding: '10px 14px', borderBottom: '1px solid #e5e6eb',
              display: 'flex', alignItems: 'center', gap: 8,
            }}>
              <span style={{ fontSize: 13, fontWeight: 600, color: '#1d2129' }}>上榜次数分布</span>
              <span style={{ fontSize: 11, color: '#86909c' }}>{dates.length}个交易日</span>
            </div>
            <div style={{ padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 7 }}>
              {Object.entries(dist)
                .map(([k, v]) => [Number(k), v] as [number, number])
                .sort((a, b) => b[0] - a[0])
                .map(([k, v]) => {
                  const maxVal = Math.max(...Object.values(dist));
                  return (
                    <div key={k} style={{
                      display: 'grid', gridTemplateColumns: '40px 1fr 24px',
                      alignItems: 'center', gap: 8, fontSize: 12,
                    }}>
                      <span style={{ color: '#4e5969' }}>
                        <b style={{ fontFamily: 'var(--font-family-mono, monospace)' }}>{k}</b> 次
                      </span>
                      <div style={{ height: 10, background: '#f2f3f5', borderRadius: 2, overflow: 'hidden' }}>
                        <div style={{
                          width: `${(v / maxVal) * 100}%`, height: '100%',
                          background: `rgba(22, 93, 255, ${0.3 + (k / 20) * 0.6})`, borderRadius: 2,
                        }} />
                      </div>
                      <span style={{ textAlign: 'right', color: '#86909c', fontSize: 11, fontFamily: 'var(--font-family-mono, monospace)' }}>
                        {v}
                      </span>
                    </div>
                  );
                })}
            </div>
          </div>

          {/* streak leaders */}
          <div style={{ background: '#fff', borderRadius: 6, border: '1px solid #e5e6eb' }}>
            <div style={{
              padding: '10px 14px', borderBottom: '1px solid #e5e6eb',
              fontSize: 13, fontWeight: 600, color: '#1d2129',
            }}>
              连榜王 Top 8
            </div>
            <div>
              {streakLeaders.map((s, i) => (
                <div key={s.code}
                  onClick={() => navigate(`/stock/${s.code}`)}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 10,
                    padding: '9px 14px', cursor: 'pointer',
                    borderBottom: i === streakLeaders.length - 1 ? 'none' : '1px solid #f2f3f5',
                    fontSize: 13, transition: 'background 100ms',
                  }}
                  onMouseEnter={e => { (e.currentTarget as HTMLElement).style.background = '#f7f8fa'; }}
                  onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = ''; }}
                >
                  <span style={{ width: 16, fontSize: 11, color: '#86909c' }}>{i + 1}</span>
                  <span style={{ flex: 1, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{s.name}</span>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 3 }}>
                    <Flame size={12} color={s.latestStreak >= 5 ? '#f53f3f' : '#ff7d00'} />
                    <span style={{
                      fontWeight: 600, fontSize: 12, fontFamily: 'var(--font-family-mono, monospace)',
                      color: s.latestStreak >= 5 ? '#f53f3f' : '#ff7d00',
                    }}>{s.latestStreak}</span>
                    <span style={{ fontSize: 11, color: '#86909c' }}>连榜</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Legend */}
      <div style={{
        padding: '10px 18px', display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap',
        background: '#fff', borderRadius: 6, border: '1px solid #e5e6eb',
        fontSize: 12, color: '#4e5969',
      }}>
        {colorBy === 'chg' ? (
          <>
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(245,63,63,0.94)' }} />涨≥9%
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(245,63,63,0.50)' }} />涨~5%
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(0,180,42,0.50)' }} />跌~5%
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(0,180,42,0.94)' }} />跌≥9%
            </span>
            <span style={{ width: 1, height: 18, background: '#e5e6eb' }} />
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ width: 14, height: 14, borderRadius: 3, background: '#f7f8fa', border: '1px solid #e5e6eb' }} />未上榜
            </span>
          </>
        ) : (
          <>
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(22,93,255,0.96)' }} />高评分
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(22,93,255,0.60)' }} />中评分
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ width: 14, height: 14, borderRadius: 3, background: 'rgba(22,93,255,0.25)' }} />低评分
            </span>
            <span style={{ width: 1, height: 18, background: '#e5e6eb' }} />
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ width: 14, height: 14, borderRadius: 3, background: '#f7f8fa', border: '1px solid #e5e6eb' }} />未上榜
            </span>
          </>
        )}
        <span style={{ flex: 1 }} />
        <span style={{ color: '#86909c' }}>
          {colorBy === 'chg' ? '红色越深涨幅越大 · 绿色越深跌幅越大' : '蓝色越深评分越高'}
        </span>
      </div>
    </div>
  );
}
