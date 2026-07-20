import { useEffect, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { fetchEnrichedHeatmap } from '../services/api';
import { Flame } from 'lucide-react';

/* types for enriched data */
interface EnrichedCell {
  pickDate: string; stockCode: string; stockName: string;
  rank: number; score: number; open: number; close: number; chgPct: number; todayChgPct: number;
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
  return { bg: 'var(--color-fill-2)', fg: 'var(--color-text-3)' };
}

function scoreBg(score: number): { bg: string; fg: string } {
  if (score <= 0) return { bg: 'var(--color-fill-2)', fg: 'var(--color-text-3)' };
  const t = Math.min(1, (score - 60) / 39);
  const a = 0.22 + t * 0.74;
  return { bg: `rgba(22, 93, 255, ${a.toFixed(3)})`, fg: t > 0.55 ? '#fff' : 'rgb(15,65,200)' };
}

export default function HeatmapPage() {
  const [raw, setRaw] = useState<EnrichedCell[]>([]);
    const [view, setView] = useState<'calendar' | 'matrix'>('calendar');
  const [colorBy, setColorBy] = useState<'chg' | 'score'>('chg');
  const [chgMode, setChgMode] = useState<'pickDay' | 'today'>('pickDay');
  const [sortKey, setSortKey] = useState<'appearances' | 'streak' | 'score'>('appearances');
  const [hover, setHover] = useState<{ cell: EnrichedCell; row: StockRow; colIdx: number } | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    fetchEnrichedHeatmap().then((res: any) => setRaw(res.data?.data || []));
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
    const getChg = (m: EnrichedCell) => chgMode === 'pickDay' ? m.chgPct : (m.todayChgPct || 0);
    const cols = dates.map((d, di) => {
      const members = sortedStocks
        .filter(s => s.cells[di])
        .map(s => s.cells[di]!);
      members.sort((a, b) => getChg(b) - getChg(a));
      const upN = members.filter(m => getChg(m) > 0).length;
      return { date: d, di, members, upN };
    });

    const colW = 158, rowH = 26;

    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {/* Controls */}
        <div style={{
          padding: '12px 18px', display: 'flex', alignItems: 'center', gap: 14,
          background: 'var(--color-bg-1)', borderRadius: 6, border: '1px solid var(--color-border-1)',
        }}>
          <span style={{ fontSize: 16, fontWeight: 600, color: 'var(--color-text-1)' }}>20 交易日上榜热力图</span>
          <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>按日成列 · 单元格颜色 = {chgMode === 'pickDay' ? '上榜日涨跌幅' : '今日涨跌幅'}</span>
          <div style={{ display: 'flex', borderRadius: 4, overflow: 'hidden', border: '1px solid var(--color-border-1)' }}>
            <button onClick={() => setChgMode('pickDay')} style={{
              padding: '4px 10px', fontSize: 11, border: 'none', cursor: 'pointer',
              background: chgMode === 'pickDay' ? '#e8f3ff' : 'var(--color-bg-1)',
              color: chgMode === 'pickDay' ? '#165dff' : 'var(--color-text-2)', fontWeight: chgMode === 'pickDay' ? 500 : 400,
            }}>上榜日</button>
            <button onClick={() => setChgMode('today')} style={{
              padding: '4px 10px', fontSize: 11, border: 'none', cursor: 'pointer',
              background: chgMode === 'today' ? '#e8f3ff' : 'var(--color-bg-1)',
              color: chgMode === 'today' ? '#165dff' : 'var(--color-text-2)', fontWeight: chgMode === 'today' ? 500 : 400,
            }}>今日</button>
          </div>
          <div style={{ flex: 1 }} />
          <div style={{ display: 'flex', borderRadius: 4, overflow: 'hidden', border: '1px solid var(--color-border-1)' }}>
            <button onClick={() => setView('calendar')} style={{
              padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
              background: view === 'calendar' ? '#e8f3ff' : 'var(--color-bg-1)',
              color: view === 'calendar' ? '#165dff' : 'var(--color-text-2)', fontWeight: view === 'calendar' ? 500 : 400,
            }}>榜单日历</button>
            <button onClick={() => setView('matrix')} style={{
              padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
              background: view === 'matrix' ? '#e8f3ff' : 'var(--color-bg-1)',
              color: view === 'matrix' ? '#165dff' : 'var(--color-text-2)', fontWeight: view === 'matrix' ? 500 : 400,
              borderLeft: '1px solid var(--color-border-1)',
            }}>矩阵热力</button>
          </div>
        </div>

        {/* Calendar grid */}
        <div style={{
          background: 'var(--color-bg-1)', borderRadius: 6, border: '1px solid var(--color-border-1)',
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
                      borderBottom: isToday ? '2px solid #165dff' : '1px solid var(--color-border-1)',
                    }}>
                      <div style={{
                        fontSize: 13, fontWeight: 600,
                        color: isToday ? '#165dff' : 'var(--color-text-1)',
                        fontFamily: 'var(--font-family-mono, monospace)',
                      }}>{c.date}</div>
                      <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 2 }}>
                        <span style={{ color: '#f53f3f', fontWeight: 500 }}>涨{c.upN}</span>
                        <span style={{ margin: '0 4px', color: 'var(--color-text-3)' }}>/</span>
                        <span style={{ color: '#00b42a', fontWeight: 500 }}>跌{c.members.length - c.upN}</span>
                      </div>
                    </div>

                    {/* cells */}
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                      {c.members.map((m, mi) => {
                        const chgVal = getChg(m);
                        const colors = chgCell(chgVal);
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
                              {chgVal >= 0 ? '+' : ''}{chgVal.toFixed(1)}%
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
          background: 'var(--color-bg-1)', borderRadius: 6, border: '1px solid var(--color-border-1)',
          fontSize: 12, color: 'var(--color-text-2)',
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
            <span style={{ width: 14, height: 14, borderRadius: 3, background: 'var(--color-fill-2)' }} />
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
          <span style={{ color: 'var(--color-text-3)' }}>红色越深涨幅越大 · 绿色越深跌幅越大</span>
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
        background: 'var(--color-bg-1)', borderRadius: 6, border: '1px solid var(--color-border-1)',
      }}>
        <span style={{ fontSize: 16, fontWeight: 600, color: 'var(--color-text-1)' }}>20 交易日上榜热力图</span>
        <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>
          {colorBy === 'chg' ? chgMode === 'pickDay' ? '单元格颜色 = 上榜日涨跌幅' : '单元格颜色 = 今日涨跌幅' : '单元格颜色 = 当日算法评分'}
        </span>
        <div style={{ flex: 1 }} />
        <div style={{ display: 'flex', borderRadius: 4, overflow: 'hidden', border: '1px solid var(--color-border-1)' }}>
          <button onClick={() => setView('calendar')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: view === 'calendar' ? '#e8f3ff' : 'var(--color-bg-1)',
            color: view === 'calendar' ? '#165dff' : 'var(--color-text-2)',
          }}>榜单日历</button>
          <button onClick={() => setView('matrix')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: view === 'matrix' ? '#e8f3ff' : 'var(--color-bg-1)',
            color: view === 'matrix' ? '#165dff' : 'var(--color-text-2)', fontWeight: view === 'matrix' ? 500 : 400,
            borderLeft: '1px solid var(--color-border-1)',
          }}>矩阵热力</button>
        </div>
        <div style={{ display: 'flex', borderRadius: 4, overflow: 'hidden', border: '1px solid var(--color-border-1)' }}>
          <button onClick={() => setColorBy('chg')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: colorBy === 'chg' ? '#e8f3ff' : 'var(--color-bg-1)',
            color: colorBy === 'chg' ? '#165dff' : 'var(--color-text-2)', fontWeight: colorBy === 'chg' ? 500 : 400,
          }}>按涨跌幅</button>
          <button onClick={() => setColorBy('score')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: colorBy === 'score' ? '#e8f3ff' : 'var(--color-bg-1)',
            color: colorBy === 'score' ? '#165dff' : 'var(--color-text-2)', fontWeight: colorBy === 'score' ? 500 : 400,
            borderLeft: '1px solid var(--color-border-1)',
          }}>按评分</button>
        </div>
        <div style={{ display: 'flex', borderRadius: 4, overflow: 'hidden', border: '1px solid var(--color-border-1)' }}>
          <button onClick={() => setSortKey('appearances')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: sortKey === 'appearances' ? '#e8f3ff' : 'var(--color-bg-1)',
            color: sortKey === 'appearances' ? '#165dff' : 'var(--color-text-2)', fontWeight: sortKey === 'appearances' ? 500 : 400,
          }}>排:上榜</button>
          <button onClick={() => setSortKey('streak')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: sortKey === 'streak' ? '#e8f3ff' : 'var(--color-bg-1)',
            color: sortKey === 'streak' ? '#165dff' : 'var(--color-text-2)', fontWeight: sortKey === 'streak' ? 500 : 400,
            borderLeft: '1px solid var(--color-border-1)',
          }}>排:连榜</button>
          <button onClick={() => setSortKey('score')} style={{
            padding: '5px 14px', fontSize: 12, border: 'none', cursor: 'pointer',
            background: sortKey === 'score' ? '#e8f3ff' : 'var(--color-bg-1)',
            color: sortKey === 'score' ? '#165dff' : 'var(--color-text-2)', fontWeight: sortKey === 'score' ? 500 : 400,
            borderLeft: '1px solid var(--color-border-1)',
          }}>排:评分</button>
        </div>
      </div>

      {/* Main content: grid + sidebar */}
      <div style={{ display: 'flex', gap: 12 }}>
        {/* LEFT: matrix grid */}
        <div style={{
          flex: 1, background: 'var(--color-bg-1)', borderRadius: 6,
          border: '1px solid var(--color-border-1)', overflow: 'hidden',
        }}>
          {/* header row: stock name + date columns */}
          <div style={{
            display: 'flex', borderBottom: '2px solid var(--color-border-1)',
            background: 'var(--color-fill-2)', position: 'sticky', top: 0, zIndex: 2,
          }}>
            {/* stock name header */}
            <div style={{
              width: 120, flex: 'none', padding: '8px 12px',
              fontSize: 12, fontWeight: 600, color: 'var(--color-text-2)',
              borderRight: '1px solid var(--color-border-1)', display: 'flex', alignItems: 'center', gap: 4,
            }}>
              <span>{sortedStocks.length} 股</span>
              <span style={{ fontSize: 10, color: 'var(--color-text-3)', fontWeight: 400 }}>
                (上榜{maxAppear}/{dates.length})
              </span>
            </div>
            {dates.map((d, di) => {
              const isToday = di === 0;
              return (
                <div key={d} style={{
                  width: 40, flex: 'none', padding: '8px 0', textAlign: 'center',
                  fontSize: 11, fontWeight: isToday ? 600 : 400,
                  color: isToday ? '#165dff' : 'var(--color-text-3)',
                  borderLeft: '1px solid var(--color-border-1)',
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
                display: 'flex', borderBottom: '1px solid var(--color-border-1)',
                background: ri % 2 === 0 ? 'var(--color-bg-1)' : 'var(--color-fill-1)',
              }}
                onMouseEnter={() => {}}
              >
                {/* stock name */}
                <div style={{
                  width: 120, flex: 'none', padding: '6px 12px',
                  fontSize: 12, color: 'var(--color-text-1)',
                  borderRight: '1px solid var(--color-border-1)',
                  display: 'flex', alignItems: 'center', gap: 6,
                  cursor: 'pointer',
                }} onClick={() => navigate(`/stock/${s.code}`)}>
                  <span style={{ fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{s.name}</span>
                  <span style={{ fontSize: 10, color: 'var(--color-text-3)', fontFamily: 'var(--font-family-mono, monospace)', whiteSpace: 'nowrap' }}>{s.code}</span>
                </div>

                {/* cells */}
                {s.cells.map((cell, di) => {
                  if (!cell) {
                    return (
                      <div key={di} style={{
                        width: 40, flex: 'none', padding: '6px 0',
                        textAlign: 'center', borderLeft: '1px solid var(--color-border-1)',
                      }}>
                        <div style={{
                          width: 30, height: 22, margin: '0 auto',
                          borderRadius: 3, background: 'var(--color-fill-2)',
                          display: 'flex', alignItems: 'center', justifyContent: 'center',
                        }}>
                          <span style={{ fontSize: 9, color: 'var(--color-text-3)' }}>—</span>
                        </div>
                      </div>
                    );
                  }

                  const chgVal2 = chgMode === 'pickDay' ? cell.chgPct : (cell.todayChgPct || 0);
                  const colors = colorBy === 'chg' ? chgCell(chgVal2) : scoreBg(cell.score || (100 - (cell.rank - 1) * 2));
                  const val = colorBy === 'chg'
                    ? `${chgVal2 >= 0 ? '+' : ''}${chgVal2.toFixed(1)}%`
                    : `${(cell.score || (100 - (cell.rank - 1) * 2)).toFixed(0)}`;

                  return (
                    <div key={di} style={{
                      width: 40, flex: 'none', padding: '6px 0', textAlign: 'center',
                      borderLeft: '1px solid var(--color-border-1)',
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
          <div style={{ background: 'var(--color-bg-1)', borderRadius: 6, border: '1px solid var(--color-border-1)' }}>
            <div style={{
              padding: '10px 14px', borderBottom: '1px solid var(--color-border-1)',
              fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)',
            }}>
              {hover ? hover.cell.stockName : '悬停查看明细'}
            </div>
            <div style={{ padding: '12px 14px' }}>
              {hover ? (
                <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr', rowGap: 8, columnGap: 12, fontSize: 13 }}>
                  <span style={{ color: 'var(--color-text-3)' }}>日期</span>
                  <span style={{ fontFamily: 'var(--font-family-mono, monospace)', color: 'var(--color-text-1)' }}>{dates[hover.colIdx]}</span>
                  <span style={{ color: 'var(--color-text-3)' }}>状态</span>
                  <span style={{ color: '#f53f3f', fontWeight: 500 }}>在榜 #{hover.cell.rank}</span>
                  <span style={{ color: 'var(--color-text-3)' }}>涨跌</span>
                  <span style={{
                    fontFamily: 'var(--font-family-mono, monospace)',
                    color: hover.cell.chgPct >= 0 ? '#f53f3f' : '#00b42a', fontWeight: 500,
                  }}>
                    {hover.cell.chgPct >= 0 ? '+' : ''}{hover.cell.chgPct.toFixed(2)}%
                  </span>
                  {hover.cell.todayChgPct != null && hover.cell.todayChgPct !== 0 && (
                    <span style={{
                      fontFamily: 'var(--font-family-mono, monospace)',
                      color: hover.cell.todayChgPct >= 0 ? '#f53f3f' : '#00b42a', fontWeight: 500, fontSize: 12,
                    }}>
                      今日 {hover.cell.todayChgPct >= 0 ? '+' : ''}{hover.cell.todayChgPct.toFixed(2)}%
                    </span>
                  )}
                  <span style={{ color: 'var(--color-text-3)' }}>评分</span>
                  <span style={{ fontFamily: 'var(--font-family-mono, monospace)', color: 'var(--color-text-1)' }}>
                    {(hover.cell.score || (100 - (hover.cell.rank - 1) * 2)).toFixed(0)}
                  </span>
                  <span style={{ color: 'var(--color-text-3)' }}>累计上榜</span>
                  <span style={{ fontWeight: 500, color: 'var(--color-text-1)' }}>{hover.row.appearances} / {dates.length} 日</span>
                  <span style={{ color: 'var(--color-text-3)' }}>当前连榜</span>
                  <span style={{ fontWeight: 600, color: hover.row.latestStreak >= 5 ? '#f53f3f' : 'var(--color-text-1)' }}>
                    {hover.row.latestStreak} 日
                  </span>
                  <span style={{ color: 'var(--color-text-3)' }}>历史最长</span>
                  <span style={{ fontWeight: 500, color: 'var(--color-text-1)' }}>{hover.row.maxStreak} 日</span>
                </div>
              ) : (
                <p style={{ fontSize: 12, color: 'var(--color-text-3)', margin: 0, lineHeight: '20px' }}>
                  将鼠标移到任意单元格，查看该股在某交易日的上榜状态、涨跌幅与算法评分。
                </p>
              )}
            </div>
          </div>

          {/* distribution */}
          <div style={{ background: 'var(--color-bg-1)', borderRadius: 6, border: '1px solid var(--color-border-1)' }}>
            <div style={{
              padding: '10px 14px', borderBottom: '1px solid var(--color-border-1)',
              display: 'flex', alignItems: 'center', gap: 8,
            }}>
              <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>上榜次数分布</span>
              <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{dates.length}个交易日</span>
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
                      <span style={{ color: 'var(--color-text-2)' }}>
                        <b style={{ fontFamily: 'var(--font-family-mono, monospace)' }}>{k}</b> 次
                      </span>
                      <div style={{ height: 10, background: 'var(--color-fill-2)', borderRadius: 2, overflow: 'hidden' }}>
                        <div style={{
                          width: `${(v / maxVal) * 100}%`, height: '100%',
                          background: `rgba(22, 93, 255, ${0.3 + (k / 20) * 0.6})`, borderRadius: 2,
                        }} />
                      </div>
                      <span style={{ textAlign: 'right', color: 'var(--color-text-3)', fontSize: 11, fontFamily: 'var(--font-family-mono, monospace)' }}>
                        {v}
                      </span>
                    </div>
                  );
                })}
            </div>
          </div>

          {/* streak leaders */}
          <div style={{ background: 'var(--color-bg-1)', borderRadius: 6, border: '1px solid var(--color-border-1)' }}>
            <div style={{
              padding: '10px 14px', borderBottom: '1px solid var(--color-border-1)',
              fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)',
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
                    borderBottom: i === streakLeaders.length - 1 ? 'none' : '1px solid var(--color-border-1)',
                    fontSize: 13, transition: 'background 100ms',
                  }}
                  onMouseEnter={e => { (e.currentTarget as HTMLElement).style.background = 'var(--color-fill-2)'; }}
                  onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = ''; }}
                >
                  <span style={{ width: 16, fontSize: 11, color: 'var(--color-text-3)' }}>{i + 1}</span>
                  <span style={{ flex: 1, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{s.name}</span>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 3 }}>
                    <Flame size={12} color={s.latestStreak >= 5 ? '#f53f3f' : '#ff7d00'} />
                    <span style={{
                      fontWeight: 600, fontSize: 12, fontFamily: 'var(--font-family-mono, monospace)',
                      color: s.latestStreak >= 5 ? '#f53f3f' : '#ff7d00',
                    }}>{s.latestStreak}</span>
                    <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>连榜</span>
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
        background: 'var(--color-bg-1)', borderRadius: 6, border: '1px solid var(--color-border-1)',
        fontSize: 12, color: 'var(--color-text-2)',
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
            <span style={{ width: 1, height: 18, background: 'var(--color-border-1)' }} />
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ width: 14, height: 14, borderRadius: 3, background: 'var(--color-fill-2)', border: '1px solid var(--color-border-1)' }} />未上榜
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
            <span style={{ width: 1, height: 18, background: 'var(--color-border-1)' }} />
            <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <span style={{ width: 14, height: 14, borderRadius: 3, background: 'var(--color-fill-2)', border: '1px solid var(--color-border-1)' }} />未上榜
            </span>
          </>
        )}
        <span style={{ flex: 1 }} />
        <span style={{ color: 'var(--color-text-3)' }}>
          {colorBy === 'chg' ? '红色越深涨幅越大 · 绿色越深跌幅越大' : '蓝色越深评分越高'}
        </span>
      </div>
    </div>
  );
}
