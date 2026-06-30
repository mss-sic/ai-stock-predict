import React, { useState } from 'react';
import { DollarSign, Repeat } from 'lucide-react';

interface StockFundFlowTabProps {
  code: string;
  fundFlow: any[];
  safeKlines: any[];
  refreshingPhase: string;
  refreshLogs: string[];
  handleRefreshStockData: (phase: string) => void;
}

const StockFundFlowTab: React.FC<StockFundFlowTabProps> = ({ code, fundFlow, safeKlines, refreshingPhase, refreshLogs, handleRefreshStockData }) => {
        const [activePeriod, setActivePeriod] = useState(20);
        const [hoveredSlice, setHoveredSlice] = useState<number | null>(null);

        const ff = [...fundFlow].reverse();
        const todayData = ff.length > 0 ? ff[ff.length - 1] : null;

        const fmtW = (v: number) => {
          const abs = Math.abs(v);
          const sign = v > 0 ? '+' : v < 0 ? '-' : '';
          if (abs >= 10000) return sign + (abs / 10000).toFixed(2) + '亿';
          if (abs < 1 && abs > 0) return sign + (abs * 10000).toFixed(0) + '元';
          return sign + abs.toFixed(0) + '万';
        };

        const periodNet = (days: number) => ff.slice(-days).reduce((s: number, d: any) => s + (d.mainNet || 0), 0);
        const barData = ff.slice(-activePeriod);
        const barMax = Math.max(...barData.map((d: any) => Math.abs(d.mainNet || 0)), 1);
        const barTotal = periodNet(activePeriod);

        const klineMap: Record<string, number> = {};
        safeKlines.forEach((k: any) => {
          const dt = (k.tradeDate || '').slice(0, 10);
          if (dt && k.close > 0) klineMap[dt] = k.close;
        });

        // ── Pie chart: direction-aware (inflow right, outflow left) ──
        const rawFlows = todayData ? [
          { label: '超大单', val: todayData.superNet || 0, inColor: '#F53F3F', outColor: '#F59999' },
          { label: '大单',   val: todayData.largeNet || 0, inColor: '#FF7D00', outColor: '#FFB366' },
          { label: '中单',   val: todayData.midNet || 0,   inColor: '#165DFF', outColor: '#8CB3FF' },
          { label: '小单',   val: todayData.smallNet || 0, inColor: '#00B42A', outColor: '#6AD98A' },
        ] : [];
        // Build directed segments: group by direction for clean half-pie
        const inflowSegs: any[] = [];
        const outflowSegs: any[] = [];
        rawFlows.forEach(d => {
          if (d.val > 0.01) inflowSegs.push({ ...d, abs: d.val });
          else if (d.val < -0.01) outflowSegs.push({ ...d, abs: -d.val });
        });
        const allSegs = [...inflowSegs, ...outflowSegs];
        const pieTotal = allSegs.reduce((s: number, d: any) => s + d.abs, 0) || 1;
        const inflowTotal = inflowSegs.reduce((s: number, d: any) => s + d.val, 0);
        const outflowTotal = outflowSegs.reduce((s: number, d: any) => s + d.val, 0);

        return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {ff.length > 0 && (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12 }}>
              {[
                { label: '近5日主力净流入', v: fmtW(periodNet(5)), c: periodNet(5) >= 0 ? '#F53F3F' : '#00B42A' },
                { label: '近20日主力净流入', v: fmtW(periodNet(20)), c: periodNet(20) >= 0 ? '#F53F3F' : '#00B42A' },
                { label: '近60日主力净流入', v: fmtW(periodNet(60)), c: periodNet(60) >= 0 ? '#F53F3F' : '#00B42A' },
                { label: '数据覆盖', v: ff.length + '个交易日', c: 'var(--color-text-1)' },
              ].map((c, i) => (
                <div key={i} className="card" style={{ padding: '14px 16px' }}>
                  <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>{c.label}</div>
                  <div style={{ fontSize: 20, fontWeight: 700, color: c.c, fontVariantNumeric: 'tabular-nums' }}>{c.v}</div>
                </div>
              ))}
            </div>
          )}

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
            <div className="card">
              <div className="card-header"><span style={{ fontWeight: 600, fontSize: 13 }}>今日资金流向分布</span></div>
              <div className="card-body" style={{ padding: '12px 16px', display: 'flex', alignItems: 'center', gap: 20 }}>
                {allSegs.length > 0 ? (
                  <>
                    <svg width="150" height="150" viewBox="0 0 150 150">
                      {(() => {
                        const cx = 75, cy = 75, r = 60, ir = 30;
                        // Lay out inflow on right half, outflow on left half
                        const inSum = inflowSegs.reduce((s: number, d: any) => s + d.abs, 0) || 0;
                        const outSum = outflowSegs.reduce((s: number, d: any) => s + d.abs, 0) || 0;
                        const totalSum = inSum + outSum || 1;
                        // Inflow arcs: start from top, clockwise on right
                        let angle = -Math.PI / 2;
                        const arcs: any[] = [];
                        // outflow first (left half, top-to-bottom)
                        outflowSegs.forEach((d: any, i: number) => {
                          const frac = d.abs / totalSum;
                          const slice = frac * Math.PI * 2;
                          arcs.push({ ...d, start: angle, slice, idx: i, dir: 'out' });
                          angle += slice;
                        });
                        const outEndAngle = angle;
                        // inflow next (right half)
                        inflowSegs.forEach((d: any, i: number) => {
                          const frac = d.abs / totalSum;
                          const slice = frac * Math.PI * 2;
                          arcs.push({ ...d, start: angle, slice, idx: i, dir: 'in' });
                          angle += slice;
                        });
                        return arcs.map((d: any) => {
                          const s = hoveredSlice === arcs.indexOf(d) ? 1.06 : 1;
                          const x1o = cx + r * s * Math.cos(d.start);
                          const y1o = cy + r * s * Math.sin(d.start);
                          const x2o = cx + r * s * Math.cos(d.start + d.slice);
                          const y2o = cy + r * s * Math.sin(d.start + d.slice);
                          const large = d.slice > Math.PI ? 1 : 0;
                          const pathD = 'M ' + (cx + ir * Math.cos(d.start + d.slice / 2)) + ' ' + (cy + ir * Math.sin(d.start + d.slice / 2)) +
                            ' L ' + x1o + ' ' + y1o +
                            ' A ' + (r * s) + ' ' + (r * s) + ' 0 ' + large + ' 1 ' + x2o + ' ' + y2o + ' Z';
                          const color = d.dir === 'in' ? d.inColor : d.outColor;
                          return <path key={arcs.indexOf(d)} d={pathD} fill={color} stroke="var(--color-bg-1)" strokeWidth="1"
                            opacity={hoveredSlice !== null && hoveredSlice !== arcs.indexOf(d) ? 0.55 : 1}
                            style={{ cursor: 'pointer', transition: 'opacity 0.15s' }}
                            onMouseEnter={() => setHoveredSlice(arcs.indexOf(d))}
                            onMouseLeave={() => setHoveredSlice(null)} />;
                        });
                      })()}
                      <circle cx="75" cy="75" r="28" fill="var(--color-bg-2)" stroke="var(--color-border-1)" strokeWidth="0.5" />
                      <text x="75" y="68" textAnchor="middle" fontSize="13" fontWeight="700"
                        fill={todayData?.mainNet >= 0 ? '#F53F3F' : '#00B42A'}>{fmtW(todayData?.mainNet || 0)}</text>
                      <text x="75" y="84" textAnchor="middle" fontSize="9" fill="var(--color-text-3)">主力净额</text>
                    </svg>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 5, flex: 1 }}>
                      {inflowSegs.length > 0 && (
                        <div style={{ fontSize: 10, fontWeight: 600, color: '#F53F3F', marginBottom: 1 }}>
                          ▲ 流入 {fmtW(inflowTotal)}
                        </div>
                      )}
                      {inflowSegs.map((d: any, i: number) => {
                        const idx = allSegs.indexOf(d);
                        return (
                          <div key={'in'+i} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 11,
                            background: hoveredSlice === idx ? 'var(--color-fill-1)' : 'transparent', padding: '1px 6px', borderRadius: 4,
                            paddingLeft: 14 }}>
                            <div style={{ width: 8, height: 8, borderRadius: 2, background: d.inColor }} />
                            <span style={{ color: 'var(--color-text-2)', minWidth: 36 }}>{d.label}</span>
                            <span style={{ fontWeight: 600, color: '#F53F3F', flex: 1, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>{fmtW(d.val)}</span>
                            <span style={{ fontSize: 9, color: 'var(--color-text-3)', minWidth: 32, textAlign: 'right' }}>{(d.abs/pieTotal*100).toFixed(1)}%</span>
                          </div>
                        );
                      })}
                      {outflowSegs.length > 0 && (
                        <div style={{ fontSize: 10, fontWeight: 600, color: '#00B42A', marginBottom: 1, marginTop: 4 }}>
                          ▼ 流出 {fmtW(outflowTotal)}
                        </div>
                      )}
                      {outflowSegs.map((d: any, i: number) => {
                        const idx = allSegs.indexOf(d);
                        return (
                          <div key={'out'+i} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 11,
                            background: hoveredSlice === idx ? 'var(--color-fill-1)' : 'transparent', padding: '1px 6px', borderRadius: 4,
                            paddingLeft: 14 }}>
                            <div style={{ width: 8, height: 8, borderRadius: 2, background: d.outColor }} />
                            <span style={{ color: 'var(--color-text-2)', minWidth: 36 }}>{d.label}</span>
                            <span style={{ fontWeight: 600, color: '#00B42A', flex: 1, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>{fmtW(d.val)}</span>
                            <span style={{ fontSize: 9, color: 'var(--color-text-3)', minWidth: 32, textAlign: 'right' }}>{(d.abs/pieTotal*100).toFixed(1)}%</span>
                          </div>
                        );
                      })}
                    </div>
                  </>
                ) : (
                  <div style={{ textAlign: 'center', padding: 30, color: 'var(--color-text-3)', fontSize: 12, flex: 1 }}>
                    暂无今日数据
                  </div>
                )}
              </div>
            </div>

            <div className="card">
              <div className="card-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ fontWeight: 600, fontSize: 13 }}>主力动向</span>
                  <span style={{ fontSize: 11, color: barTotal >= 0 ? '#F53F3F' : '#00B42A' }}>
                    累计 {fmtW(barTotal)}
                  </span>
                </div>
                <div style={{ display: 'flex', gap: 4 }}>
                  {[5, 10, 20, 60].map(d => (
                    <button key={d} onClick={() => setActivePeriod(d)} style={{
                      padding: '2px 8px', fontSize: 10, borderRadius: 4, cursor: 'pointer',
                      border: '1px solid var(--color-border-2)',
                      background: activePeriod === d ? 'var(--color-primary)' : 'transparent',
                      color: activePeriod === d ? '#fff' : 'var(--color-text-2)',
                    }}>近{d}日</button>
                  ))}
                </div>
              </div>
              <div className="card-body" style={{ padding: '8px 12px' }}>
                {barData.length > 0 ? (() => {
                  const BW = Math.max(barData.length * 22, 260);
                  const H = 200, padL = 48, padR = 8, padT = 8, padB = 24;
                  const cw = BW - padL - padR, ch = H - padT - padB;
                  const zeroY = padT + ch * 0.55;
                  const ticks = [barMax, barMax * 0.5, 0, -barMax * 0.5, -barMax];
                  return (
                  <svg width="100%" height={H} viewBox={'0 0 ' + BW + ' ' + H} preserveAspectRatio="xMidYMid meet">
                    <defs>
                      <linearGradient id="ffUp" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor="#F53F3F" stopOpacity="0.9" />
                        <stop offset="100%" stopColor="#F53F3F" stopOpacity="0.3" />
                      </linearGradient>
                      <linearGradient id="ffDown" x1="0" y1="1" x2="0" y2="0">
                        <stop offset="0%" stopColor="#00B42A" stopOpacity="0.9" />
                        <stop offset="100%" stopColor="#00B42A" stopOpacity="0.3" />
                      </linearGradient>
                    </defs>
                    {/* Grid lines */}
                    {ticks.map((v: number, i: number) => {
                      const y = zeroY - (v / barMax) * ch * 0.5;
                      return <line key={'g'+i} x1={padL} y1={y} x2={BW - padR} y2={y}
                        stroke={v === 0 ? 'var(--color-text-3)' : 'var(--color-border-1)'}
                        strokeWidth={v === 0 ? 1 : 0.5}
                        strokeDasharray={v === 0 ? '6,4' : '3,3'} />;
                    })}
                    {/* Bars */}
                    {barData.map((d: any, i: number) => {
                      const barW = Math.max(4, Math.min(16, cw / barData.length * 0.55));
                      const gap = cw / barData.length;
                      const h = Math.max(1, (Math.abs(d.mainNet || 0) / barMax) * ch * 0.5);
                      const x = padL + i * gap + (gap - barW) / 2;
                      const fill = (d.mainNet || 0) >= 0 ? 'url(#ffUp)' : 'url(#ffDown)';
                      return <rect key={i} x={x} y={(d.mainNet || 0) >= 0 ? zeroY - h : zeroY}
                        width={barW} height={h} fill={fill} rx="2" />;
                    })}
                    {/* Y-axis labels */}
                    {ticks.map((v: number, i: number) => {
                      const y = zeroY - (v / barMax) * ch * 0.5;
                      return <text key={'yl'+i} x={padL - 6} y={y + 3} fontSize="8" fill="var(--color-text-3)" textAnchor="end">{fmtW(v)}</text>;
                    })}
                    {/* X-axis labels */}
                    {barData.filter((_: any, i: number) => i % Math.max(1, Math.ceil(barData.length / 7)) === 0)
                      .map((d: any, i: number) => {
                        const idx = barData.indexOf(d);
                        const x = padL + (idx / Math.max(barData.length - 1, 1)) * cw;
                        return <text key={i} x={x} y={H - 5} textAnchor="middle" fontSize="8" fill="var(--color-text-3)">
                          {(d.tradeDate || '').slice(5, 10)}
                        </text>;
                    })}
                  </svg>);
                })() : (
                  <div style={{ textAlign: 'center', padding: 30, color: 'var(--color-text-3)', fontSize: 12 }}>暂无数据</div>
                )}
              </div>
            </div>
          </div>

          <div className="card">
            <div className="card-header">
              <span style={{ fontWeight: 600, fontSize: 13, display: 'flex', alignItems: 'center', gap: 6 }}>
                <DollarSign size={14} /> 资金流快照
              </span>
              <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>
                数据截止 {todayData?.tradeDate?.slice(0, 10) || '--'}
              </span>
            </div>
            <div className="card-body" style={{ padding: '12px 16px' }}>
              {todayData ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                  {/* Main force net */}
                  <div style={{ textAlign: 'center', padding: '6px 0' }}>
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 2 }}>主力净流入</div>
                    <div style={{ fontSize: 26, fontWeight: 700, fontVariantNumeric: 'tabular-nums',
                      color: todayData.mainNet >= 0 ? '#F53F3F' : '#00B42A' }}>
                      {fmtW(todayData.mainNet || 0)}
                    </div>
                    <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 2 }}>
                      截止 {todayData?.tradeDate?.slice(0, 10) || '--'}
                    </div>
                  </div>
                  {/* Divider */}
                  <div style={{ height: 1, background: 'var(--color-border-1)' }} />
                  {/* Inflow rows */}
                  {inflowSegs.length > 0 && (
                    <div>
                      <div style={{ fontSize: 10, fontWeight: 600, color: '#F53F3F', marginBottom: 4 }}>▲ 资金流入</div>
                      {inflowSegs.map((d: any, i: number) => (
                        <div key={'s'+i} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '3px 0', fontSize: 12 }}>
                          <div style={{ width: 8, height: 8, borderRadius: 2, background: d.inColor }} />
                          <span style={{ color: 'var(--color-text-2)', flex: 1 }}>{d.label}</span>
                          <span style={{ fontWeight: 600, color: '#F53F3F', fontVariantNumeric: 'tabular-nums' }}>{fmtW(d.val)}</span>
                          <span style={{ fontSize: 10, color: 'var(--color-text-3)', minWidth: 32, textAlign: 'right' }}>{(d.abs/pieTotal*100).toFixed(0)}%</span>
                        </div>
                      ))}
                    </div>
                  )}
                  {/* Outflow rows */}
                  {outflowSegs.length > 0 && (
                    <div>
                      <div style={{ fontSize: 10, fontWeight: 600, color: '#00B42A', marginBottom: 4, marginTop: inflowSegs.length > 0 ? 2 : 0 }}>▼ 资金流出</div>
                      {outflowSegs.map((d: any, i: number) => (
                        <div key={'so'+i} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '3px 0', fontSize: 12 }}>
                          <div style={{ width: 8, height: 8, borderRadius: 2, background: d.outColor }} />
                          <span style={{ color: 'var(--color-text-2)', flex: 1 }}>{d.label}</span>
                          <span style={{ fontWeight: 600, color: '#00B42A', fontVariantNumeric: 'tabular-nums' }}>{fmtW(d.val)}</span>
                          <span style={{ fontSize: 10, color: 'var(--color-text-3)', minWidth: 32, textAlign: 'right' }}>{(d.abs/pieTotal*100).toFixed(0)}%</span>
                        </div>
                      ))}
                    </div>
                  )}
                  {/* Main force vs retail comparison */}
                  {(() => {
                    const mfNet = (todayData?.superNet || 0) + (todayData?.largeNet || 0);
                    const retailNet = (todayData?.midNet || 0) + (todayData?.smallNet || 0);
                    const absTotal = Math.abs(mfNet) + Math.abs(retailNet) || 1;
                    const mfPct = Math.abs(mfNet) / absTotal * 100;
                    return (
                      <div style={{ marginTop: 4 }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 10, color: 'var(--color-text-3)', marginBottom: 4 }}>
                          <span>主力 vs 散户净额占比</span>
                        </div>
                        <div style={{ display: 'flex', gap: 4, alignItems: 'center' }}>
                          <span style={{ fontSize: 10, fontWeight: 600, color: mfNet >= 0 ? '#F53F3F' : '#00B42A', minWidth: 36 }}>主力 {mfPct.toFixed(0)}%</span>
                          <div style={{ flex: 1, height: 6, borderRadius: 3, background: 'var(--color-fill-2)', overflow: 'hidden', display: 'flex' }}>
                            <div style={{ width: mfPct + '%', background: 'linear-gradient(90deg, #F53F3F, #FF7D00)', borderRadius: mfPct > 95 ? 3 : '3px 0 0 3px', transition: 'width 0.3s' }} />
                          </div>
                          <span style={{ fontSize: 10, fontWeight: 600, color: retailNet >= 0 ? '#F53F3F' : '#00B42A', minWidth: 48, textAlign: 'right' }}>散户 {(100-mfPct).toFixed(0)}%</span>
                        </div>
                      </div>
                    );
                  })()}
                </div>
              ) : (
                <div style={{ textAlign: 'center', padding: 30, color: 'var(--color-text-3)', fontSize: 12 }}>
                  暂无今日数据
                </div>
              )}
            </div>
          </div>

          <div className="card">
            <div className="card-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ fontWeight: 600, fontSize: 13 }}>资金流趋势 vs 股价</span>
              <div style={{ display: 'flex', gap: 4 }}>
                {[5, 20, 60].map(d => (
                  <button key={d} onClick={() => setActivePeriod(d)} style={{
                    padding: '2px 8px', fontSize: 10, borderRadius: 4, cursor: 'pointer',
                    border: '1px solid var(--color-border-2)',
                    background: activePeriod === d ? 'var(--color-primary)' : 'transparent',
                    color: activePeriod === d ? '#fff' : 'var(--color-text-2)',
                  }}>{d}日</button>
                ))}
              </div>
            </div>
            <div className="card-body" style={{ padding: '12px 16px' }}>
              {ff.length > 0 ? (() => {
                const periodSlice = ff.slice(-activePeriod);
                const merged = periodSlice.map((d: any) => ({
                  date: (d.tradeDate || '').slice(0, 10),
                  mainNet: d.mainNet || 0,
                  close: klineMap[(d.tradeDate || '').slice(0, 10)] || 0,
                })).filter((d: any) => d.close > 0);

                if (merged.length < 2) return (
                  <div style={{ textAlign: 'center', padding: 30, color: 'var(--color-text-3)', fontSize: 12 }}>
                    需要更多K线数据，请先采集该股K线
                  </div>
                );

                const totalNet = merged.reduce((s: number, d: any) => s + d.mainNet, 0);
                const inflowRatio = merged.filter((d: any) => d.mainNet > 0).length / merged.length * 100;
                const firstPrice = merged[0].close;
                const lastPrice = merged[merged.length - 1].close;
                const priceChg = ((lastPrice - firstPrice) / firstPrice * 100);

                const fMax = Math.max(...merged.map((d: any) => Math.abs(d.mainNet)), 1);
                const pMin = Math.min(...merged.map((d: any) => d.close));
                const pMax = Math.max(...merged.map((d: any) => d.close));
                const pRange = Math.max(pMax - pMin, 0.01);
                const W = Math.max(merged.length * 35, 300), H = 240, padL = 55, padR = 55, padT = 12, padB = 22;
                const cw = W - padL - padR, ch = H - padT - padB;

                // Price axis ticks (left y-axis, top half of chart)
                const priceTicks = [pMin, pMin + pRange * 0.5, pMax];
                // Fund flow axis ticks (right y-axis, bottom half centered at zero)
                const flowTicks = [-fMax, -fMax * 0.5, 0, fMax * 0.5, fMax];

                return (
                  <div>
                    <div style={{ display: 'flex', gap: 20, marginBottom: 10, fontSize: 12 }}>
                      <span>主力净流入 <b style={{ color: totalNet >= 0 ? '#F53F3F' : '#00B42A' }}>{fmtW(totalNet)}</b></span>
                      <span>日均 <b>{fmtW(totalNet / merged.length)}</b></span>
                      <span>流入占比 <b>{inflowRatio.toFixed(0)}%</b></span>
                      <span style={{ marginLeft: 'auto' }}>股价 <b style={{ color: priceChg >= 0 ? '#F53F3F' : '#00B42A' }}>
                        {priceChg >= 0 ? '+' : ''}{priceChg.toFixed(2)}%
                      </b></span>
                    </div>
                    <svg width="100%" height={H} viewBox={'0 0 ' + W + ' ' + H} preserveAspectRatio="xMidYMid meet">
                      {/* Grid lines for price scale */}
                      {priceTicks.map((v, i) => {
                        const y = padT + (1 - (v - pMin) / pRange) * ch * 0.48;
                        return <line key={'gp'+i} x1={padL} y1={y} x2={W - padR} y2={y} stroke="var(--color-border-1)" strokeWidth="0.5" strokeDasharray="3,3" />;
                      })}
                      {/* Zero line for fund flow */}
                      <line x1={padL} y1={padT + ch * 0.52} x2={W - padR} y2={padT + ch * 0.52} stroke="var(--color-text-3)" strokeWidth="0.8" strokeDasharray="4,3" />
                      {/* Grid lines for fund flow scale */}
                      {flowTicks.filter(v => v !== 0).map((v, i) => {
                        const y = padT + ch * 0.52 - (v / fMax) * ch * 0.45;
                        return <line key={'gf'+i} x1={padL} y1={y} x2={W - padR} y2={y} stroke="var(--color-border-1)" strokeWidth="0.3" />;
                      })}
                      {/* Fund flow bars */}
                      {merged.map((d: any, i: number) => {
                        const barW = Math.max(4, Math.min(14, cw / merged.length * 0.5));
                        const gap = cw / merged.length;
                        const h = (Math.abs(d.mainNet) / fMax) * ch * 0.45;
                        const x = padL + i * gap + (gap - barW) / 2;
                        const zeroY = padT + ch * 0.52;
                        const color = d.mainNet >= 0 ? '#F53F3F88' : '#00B42A88';
                        return <rect key={i} x={x} y={d.mainNet >= 0 ? zeroY - h : zeroY}
                          width={barW} height={Math.max(h, 1)} fill={color} rx="2" />;
                      })}
                      {/* Price line */}
                      {merged.length > 1 && (
                        <polyline points={merged.map((d: any, i: number) =>
                          (padL + (i / Math.max(merged.length - 1, 1)) * cw) + ',' + (padT + (1 - (d.close - pMin) / pRange) * ch * 0.48)).join(' ')}
                          fill="none" stroke="#165DFF" strokeWidth="2.5" />)}
                      {merged.map((d: any, i: number) => (
                        <circle key={i} cx={padL + (i / Math.max(merged.length - 1, 1)) * cw}
                          cy={padT + (1 - (d.close - pMin) / pRange) * ch * 0.48} r="3"
                          fill="#165DFF" stroke="var(--color-bg-1)" strokeWidth="1" />))}
                      {/* Left Y-axis: price labels */}
                      <text x={padL - 6} y={padT - 4} fontSize="9" fill="#165DFF" textAnchor="end" fontWeight="600">股价</text>
                      {priceTicks.map((v, i) => {
                        const y = padT + (1 - (v - pMin) / pRange) * ch * 0.48;
                        return <text key={'py'+i} x={padL - 6} y={y + 3} fontSize="8" fill="#165DFF" textAnchor="end">{v.toFixed(2)}</text>;
                      })}
                      {/* Right Y-axis: fund flow labels */}
                      <text x={W - padR + 6} y={padT - 4} fontSize="9" fill="#F53F3F" textAnchor="start" fontWeight="600">资金</text>
                      {flowTicks.map((v, i) => {
                        const y = padT + ch * 0.52 - (v / fMax) * ch * 0.45;
                        return <text key={'fy'+i} x={W - padR + 6} y={y + 3} fontSize="8" fill="#F53F3F" textAnchor="start">{fmtW(v)}</text>;
                      })}
                      {/* X-axis date labels */}
                      {merged.filter((_: any, i: number) => i % Math.max(1, Math.floor(merged.length / 6)) === 0)
                        .map((d: any, i: number) => (
                        <text key={i} x={padL + (merged.indexOf(d) / Math.max(merged.length - 1, 1)) * cw} y={H - 4}
                          textAnchor="middle" fontSize="8" fill="var(--color-text-3)">{d.date.slice(5)}</text>
                      ))}
                    </svg>
                  </div>
                );
              })() : (
                <div style={{ textAlign: 'center', padding: 30, color: 'var(--color-text-3)', fontSize: 12 }}>
                  暂无资金流数据，请先采集
                </div>
              )}
            </div>
          </div>

          <div className="card">
            <div className="card-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span style={{ fontWeight: 600, fontSize: 14, display: 'flex', alignItems: 'center', gap: 6 }}>
                <DollarSign size={14} /> 资金流明细
              </span>
              <button onClick={() => handleRefreshStockData('fund_flow')} disabled={refreshingPhase !== ''}
                style={{ padding: '4px 10px', fontSize: 11, cursor: 'pointer', border: '1px solid var(--color-border-1)', borderRadius: 4, background: 'var(--color-bg-1)', color: 'var(--color-text-2)', display: 'flex', alignItems: 'center', gap: 4 }}>
                <Repeat size={12} className={refreshingPhase === 'fund_flow' ? 'spin' : ''} />{refreshingPhase === 'fund_flow' ? '更新中...' : '更新'}
              </button>
            </div>
            {refreshingPhase === 'fund_flow' && refreshLogs.length > 0 && (
              <div style={{ padding: '6px 16px', background: 'var(--color-fill-1)', borderBottom: '1px solid var(--color-border-1)', maxHeight: 120, overflow: 'auto' }}>
                {refreshLogs.map((log, i) => (
                  <div key={i} style={{ fontSize: 11, color: 'var(--color-text-2)', fontFamily: 'monospace', opacity: i === refreshLogs.length - 1 ? 1 : 0.4 }}>{log}</div>
                ))}
              </div>
            )}
            <div className="card-body" style={{ padding: 0 }}>
              {fundFlow.length > 0 ? (
                <div style={{ maxHeight: 400, overflow: 'auto' }}>
                  <table style={{ width: '100%', tableLayout: 'fixed', borderCollapse: 'collapse', fontSize: 12 }}>
                    <colgroup>
                      <col style={{ width: '14%' }} /><col style={{ width: '17%' }} />
                      <col style={{ width: '17%' }} /><col style={{ width: '17%' }} />
                      <col style={{ width: '17%' }} /><col style={{ width: '17%' }} />
                    </colgroup>
                    <thead>
                      <tr style={{ background: 'var(--color-fill-1)', borderBottom: '2px solid var(--color-border-2)', position: 'sticky', top: 0 }}>
                        {['日期', '主力净流入', '超大单', '大单', '中单', '小单'].map(h => (
                          <th key={h} style={{ padding: '8px 8px', textAlign: h === '日期' ? 'left' : 'right', fontSize: 11, color: 'var(--color-text-3)', fontWeight: 500 }}>{h}</th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {fundFlow.map((d: any, i: number) => (
                        <tr key={i} style={{ borderBottom: '1px solid var(--color-border-1)' }}>
                          <td style={{ padding: '5px 8px', fontSize: 11 }}>{d.tradeDate?.slice(0, 10)}</td>
                          <td style={{ padding: '5px 8px', textAlign: 'right', fontWeight: 600, color: (d.mainNet || 0) >= 0 ? '#F53F3F' : '#00B42A', fontVariantNumeric: 'tabular-nums' }}>{fmtW(d.mainNet || 0)}</td>
                          <td style={{ padding: '5px 8px', textAlign: 'right', color: (d.superNet || 0) >= 0 ? '#F53F3F' : '#00B42A', fontVariantNumeric: 'tabular-nums' }}>{fmtW(d.superNet || 0)}</td>
                          <td style={{ padding: '5px 8px', textAlign: 'right', color: (d.largeNet || 0) >= 0 ? '#F53F3F' : '#00B42A', fontVariantNumeric: 'tabular-nums' }}>{fmtW(d.largeNet || 0)}</td>
                          <td style={{ padding: '5px 8px', textAlign: 'right', color: (d.midNet || 0) >= 0 ? '#F53F3F' : '#00B42A', fontVariantNumeric: 'tabular-nums' }}>{fmtW(d.midNet || 0)}</td>
                          <td style={{ padding: '5px 8px', textAlign: 'right', color: (d.smallNet || 0) >= 0 ? '#F53F3F' : '#00B42A', fontVariantNumeric: 'tabular-nums' }}>{fmtW(d.smallNet || 0)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div style={{ textAlign: 'center', padding: 48, fontSize: 13, color: 'var(--color-text-3)' }}>
                  暂无资金流数据，请点击右上角「更新」按钮采集
                </div>
              )}
            </div>
          </div>
        </div>
        );

};

export default StockFundFlowTab;
