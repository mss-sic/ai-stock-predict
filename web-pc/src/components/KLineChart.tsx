import React, { useState, useMemo, useCallback, useRef } from 'react';

interface KLineItem {
  tradeDate?: string; date?: string;
  open: number; close: number; high: number; low: number;
  volume?: number;
}

interface Marker {
  i: number;
  type: 'board' | 'buy' | 'sell';
  label?: string;
}

interface PredictionLine {
  color: string;
  data: (number | null)[];
  dashed?: boolean;
  name?: string;
}

interface PredMarker {
  i: number;
  type: 'predHi' | 'predLo';
  label?: string;
  color?: string;
  price?: number;
}

interface Props {
  data: KLineItem[];
  height?: number;
  markers?: Marker[];
  predictionLines?: PredictionLine[];
  splitIdx?: number;
  predMarkers?: PredMarker[];
  enableRangeSelect?: boolean;
  selectedRange?: [number, number] | null;
  onRangeChange?: (startIdx: number, endIdx: number) => void;
}

const UP = '#F53F3F';
const DOWN = '#00B42A';

export default function KLineChart({
  data,
  height = 420,
  markers = [],
  predictionLines = [],
  splitIdx,
  predMarkers = [],
  enableRangeSelect = false,
  selectedRange = null,
  onRangeChange,
}: Props) {
  const [hoverIdx, setHoverIdx] = useState<number | null>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const [tooltipPos, setTooltipPos] = useState<{ x: number; y: number } | null>(null);
  const [dragStart, setDragStart] = useState<number | null>(null);
  const [dragEnd, setDragEnd] = useState<number | null>(null);
  const [dragMode, setDragMode] = useState<'new' | 'move' | 'resizeStart' | 'resizeEnd' | null>(null);
  const [dragOffset, setDragOffset] = useState(0);
  const [dragRangeWidth, setDragRangeWidth] = useState(0);
  const isDragging = dragStart !== null;

  // Effective selected range (considering drag in progress)
  const effectiveRange = useMemo((): [number, number] | null => {
    if (selectedRange) return selectedRange;
    if (isDragging && dragStart !== null && dragEnd !== null) {
      return [Math.min(dragStart, dragEnd), Math.max(dragStart, dragEnd)];
    }
    return null;
  }, [selectedRange, isDragging, dragStart, dragEnd]);

  const isEmpty = !data || data.length === 0;
  const safeData: KLineItem[] = isEmpty ? [] : data.filter((d: any) => d != null);

  // ═══ All hooks must be called unconditionally ═══
  const W = 960;
  const H = height;
  const padL = 54, padR = 60, padT = 14, padB = 30;
  const volH = 70;
  const priceH = H - padT - padB - volH - 8;
  const innerW = W - padL - padR;

  const maxPredExtra = predictionLines.reduce((max, l) => {
    const extra = l.data.length - safeData.length;
    return extra > max ? extra : max;
  }, 0);
  const totalN = safeData.length + Math.max(0, maxPredExtra);
  const step = innerW / (totalN || 1);
  const bw = Math.max(2, Math.min(10, step * 0.72));

  const hi = safeData.length === 0 ? 0 : Math.max(...safeData.map(d => d.high));
  const lo = safeData.length === 0 ? 0 : Math.min(...safeData.map(d => d.low));
  let extHi = hi, extLo = lo;
  if (safeData.length > 0 && splitIdx != null) {
    for (const line of predictionLines) {
      for (let i = splitIdx; i < line.data.length; i++) {
        const v = line.data[i];
        if (v != null) { if (v > extHi) extHi = v; if (v < extLo) extLo = v; }
      }
    }
  }
  const range = extHi - extLo || 1;
  const padding = range * 0.05;
  const plotLo = extLo - padding;
  const plotRange = extHi - extLo + padding * 2;
  const py = (v: number) => padT + priceH - ((v - plotLo) / plotRange) * priceH;

  const volMax = safeData.length === 0 ? 1 : Math.max(...safeData.map(d => d.volume || 0)) || 1;
  const volBaseY = padT + priceH + 8 + volH;
  const vy = (v: number) => volBaseY - (v / volMax) * volH;
  const fx = (i: number) => padL + i * step + step / 2;

  const yTicks = 5;
  const grids = Array.from({ length: yTicks }, (_, i) => {
    const v = plotLo + (plotRange * i) / (yTicks - 1);
    return { y: py(v), label: v.toFixed(2) };
  });

  const xLabels = useMemo(() => {
    const total = safeData.length;
    const labels: number[] = [];
    if (total <= 8) { for (let i = 0; i < total; i++) labels.push(i); }
    else {
      const s = Math.floor(total / 7);
      for (let i = 0; i < total; i += s) labels.push(i);
      if (labels[labels.length - 1] !== total - 1) labels.push(total - 1);
    }
    if (maxPredExtra > 0) labels.push(totalN - 1);
    return labels;
  }, [safeData.length, totalN, maxPredExtra]);

  const calcMA = (period: number) =>
    safeData.map((_, i) => {
      if (i < period - 1) return null;
      let s = 0; for (let k = i - period + 1; k <= i; k++) s += safeData[k].close;
      return s / period;
    });

  const ma5 = useMemo(() => calcMA(5), [safeData]);
  const ma10 = useMemo(() => calcMA(10), [safeData]);
  const ma20 = useMemo(() => calcMA(20), [safeData]);

  const maPath = (arr: (number | null)[], color: string) => {
    let d = '';
    arr.forEach((v, i) => {
      if (v == null) return;
      d += d === '' ? `M${fx(i).toFixed(1)},${py(v).toFixed(1)}` : ` L${fx(i).toFixed(1)},${py(v).toFixed(1)}`;
    });
    return <path d={d} stroke={color} strokeWidth="1.2" fill="none" />;
  };

  const dateLabel = (i: number) => {
    if (i < safeData.length) {
      const ds = (safeData[i]?.tradeDate || safeData[i]?.date || '');
      return ds.length >= 10 ? ds.slice(5, 10) : `T-${safeData.length - i}`;
    }
    return `+${i - safeData.length + 1}`;
  };

  const getIdxFromEvent = useCallback((e: React.MouseEvent<SVGSVGElement>) => {
    const svg = svgRef.current; if (!svg) return -1;
    const rect = svg.getBoundingClientRect();
    const scaleX = W / rect.width;
    const mx = (e.clientX - rect.left) * scaleX;
    return Math.round((mx - padL - step / 2) / step);
  }, [step]);

  const handleMouseMove = useCallback((e: React.MouseEvent<SVGSVGElement>) => {
    if (isEmpty) return;
    const idx = getIdxFromEvent(e);
    const maxIdx = totalN - 1;
    if (idx >= 0 && idx <= maxIdx) {
      setHoverIdx(idx);
      const svg = svgRef.current; if (!svg) return;
      const rect = svg.getBoundingClientRect();
      setTooltipPos({ x: e.clientX - rect.left, y: e.clientY - rect.top });
    } else { setHoverIdx(null); setTooltipPos(null); }

    if (!isDragging) return;
    const clampedIdx = Math.max(0, Math.min(safeData.length - 1, idx));
    if (clampedIdx < 0) return;

    if (dragMode === 'move') {
      const newStart = clampedIdx - dragOffset;
      const newEnd = newStart + dragRangeWidth;
      if (newStart >= 0 && newEnd < safeData.length) {
        setDragStart(newStart);
        setDragEnd(newEnd);
      }
    } else if (dragMode === 'resizeStart') {
      if (clampedIdx < dragStart!) setDragEnd(clampedIdx);
      else if (clampedIdx > dragStart!) {
        setDragMode('resizeEnd');
        const curEnd = dragEnd!;
        setDragStart(curEnd);
        setDragEnd(clampedIdx);
      }
    } else if (dragMode === 'resizeEnd') {
      if (clampedIdx > dragStart!) setDragEnd(clampedIdx);
      else if (clampedIdx < dragStart!) {
        setDragMode('resizeStart');
        const curEnd = dragStart!;
        setDragStart(clampedIdx);
        setDragEnd(curEnd);
      }
    } else {
      // 'new' mode
      setDragEnd(clampedIdx);
    }
  }, [isEmpty, step, totalN, isDragging, dragMode, dragOffset, dragRangeWidth, getIdxFromEvent, safeData.length, dragStart, dragEnd]);

  const handleMouseLeave = () => { setHoverIdx(null); setTooltipPos(null); };

  const handleMouseDown = useCallback((e: React.MouseEvent<SVGSVGElement>) => {
    if (!enableRangeSelect || e.button !== 0) return;
    const idx = getIdxFromEvent(e);
    if (idx < 0 || idx >= safeData.length) return;

    // Check if clicking on existing selected range for move/resize
    const curRange = selectedRange;
    if (curRange) {
      const [rs, re] = curRange;
      const edgeThreshold = Math.max(2, Math.floor((re - rs) * 0.12));
      if (idx >= rs - edgeThreshold && idx <= re + edgeThreshold) {
        if (idx <= rs + edgeThreshold) {
          setDragMode('resizeStart');
          setDragStart(re);
          setDragEnd(idx);
          return;
        }
        if (idx >= re - edgeThreshold) {
          setDragMode('resizeEnd');
          setDragStart(rs);
          setDragEnd(idx);
          return;
        }
        if (re - rs <= 3) {
          setDragMode('move');
          setDragOffset(idx - rs);
          setDragRangeWidth(re - rs);
          setDragStart(rs);
          setDragEnd(re);
          return;
        }
        setDragMode('move');
        setDragOffset(idx - rs);
        setDragRangeWidth(re - rs);
        setDragStart(rs);
        setDragEnd(re);
        return;
      }
    }
    // New range selection
    setDragMode('new');
    setDragStart(idx);
    setDragEnd(idx);
  }, [enableRangeSelect, getIdxFromEvent, safeData.length, selectedRange]);

  const handleMouseUp = useCallback(() => {
    if (isDragging && dragStart !== null && dragEnd !== null && onRangeChange) {
      const s = Math.min(dragStart, dragEnd);
      const e = Math.max(dragStart, dragEnd);
      if (s !== e) onRangeChange(s, e);
    }
    setDragStart(null);
    setDragEnd(null);
    setDragMode(null);
  }, [isDragging, dragStart, dragEnd, onRangeChange]);

  // Cursor style based on mode
  const chartCursor = useMemo(() => {
    if (!enableRangeSelect) return 'default';
    if (dragMode === 'move') return 'grabbing';
    if (dragMode === 'resizeStart' || dragMode === 'resizeEnd') return 'ew-resize';
    return selectedRange ? 'grab' : 'crosshair';
  }, [enableRangeSelect, dragMode, selectedRange]);

  React.useEffect(() => {
    if (isDragging) {
      const handleGlobalMouseUp = () => {
        if (dragStart !== null && dragEnd !== null && dragStart !== dragEnd && onRangeChange) {
          const s = Math.min(dragStart, dragEnd);
          const e = Math.max(dragStart, dragEnd);
          if (s !== e) onRangeChange(s, e);
        }
        setDragStart(null);
        setDragEnd(null);
        setDragMode(null);
      };
      window.addEventListener('mouseup', handleGlobalMouseUp);
      return () => window.removeEventListener('mouseup', handleGlobalMouseUp);
    }
  }, [isDragging]);

  const hoverData = hoverIdx != null && hoverIdx < safeData.length ? safeData[hoverIdx] : null;
  const isHoverPredict = hoverIdx != null && hoverIdx >= safeData.length;
  const hoverPreds = useMemo(() => {
    if (!isHoverPredict || hoverIdx == null) return [];
    return predictionLines.map(line => ({
      name: line.name || '',
      color: line.color,
      price: line.data[hoverIdx]
    })).filter(p => p.price != null);
  }, [isHoverPredict, hoverIdx, predictionLines]);
  const hoverX = hoverIdx != null ? fx(hoverIdx) : 0;

  if (isEmpty) {
    return <div className="muted" style={{ textAlign: 'center', padding: 60 }}>暂无K线数据，请先触发数据采集</div>;
  }

  return (
    <div style={{ position: 'relative', width: '100%', height }}>
      <svg
        ref={svgRef}
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="xMidYMid meet"
        style={{ width: '100%', height, background: '#1A1D23', borderRadius: 10, cursor: chartCursor }}
        onMouseMove={handleMouseMove}
        onMouseLeave={handleMouseLeave}
        onMouseDown={handleMouseDown}
        onMouseUp={handleMouseUp}
      >
        {/* Prediction bg */}
        {splitIdx != null && splitIdx <= safeData.length && (
          <>
            <rect x={fx(splitIdx) - step / 2} y={padT} width={fx(totalN - 1) - fx(splitIdx) + step} height={priceH} fill="#F7F8FA" opacity="0.5" />
            <rect x={fx(splitIdx) - step / 2} y={padT} width={2} height={priceH} fill="#C9CDD4" opacity="0.3" />
            <line x1={fx(splitIdx) - step / 2} x2={fx(splitIdx) - step / 2} y1={padT} y2={padT + priceH} stroke="#C9CDD4" strokeDasharray="4 3" strokeWidth="1" />
            <text x={fx(splitIdx) - step / 4} y={padT + 11} fontSize="9" fill="#86909C">←历史 预测→</text>
          </>
        )}

        {/* Grid */}
        {grids.map((g, i) => (
          <g key={i}>
            <line x1={padL} x2={W - padR} y1={g.y} y2={g.y} stroke="#F2F3F5" strokeWidth="0.8" />
            <text x={W - padR + 6} y={g.y + 3} fontSize="10" fill="#86909C">{g.label}</text>
          </g>
        ))}

        {/* Volume bars */}
        {safeData.map((d, i) => {
          const isUp = d.close >= d.open;
          const c = isUp ? UP : DOWN;
          const x = fx(i);
          const vh = Math.max(1, volBaseY - vy(d.volume || 0));
          return <rect key={`v${i}`} x={x - bw / 2} y={volBaseY - vh} width={bw} height={Math.max(1, vh)} fill={c} opacity={isUp ? 0.3 : 0.25} />;
        })}

        {/* Prediction lines */}
        {predictionLines.map((line, li) => {
          const pts: string[] = [];
          for (let i = 0; i < line.data.length; i++) {
            const v = line.data[i]; if (v == null) continue;
            pts.push(`${fx(i).toFixed(1)},${py(v).toFixed(1)}`);
          }
          if (pts.length < 2) return null;
          // Build fill polygon
          const fillPts: string[] = [];
          let firstX = 0, lastX = 0;
          const baseY = py(plotLo);
          for (let i = 0; i < line.data.length; i++) {
            const v = line.data[i]; if (v == null) continue;
            const px = fx(i);
            if (fillPts.length === 0) firstX = px;
            fillPts.push(`${px.toFixed(1)},${py(v).toFixed(1)}`);
            lastX = px;
          }
          const allFillPts = fillPts.length >= 2
            ? [...fillPts, `${lastX.toFixed(1)},${baseY.toFixed(1)}`, `${firstX.toFixed(1)},${baseY.toFixed(1)}`]
            : null;
          return (
            <g key={`pl${li}`}>
              {allFillPts && <polygon points={allFillPts.join(' ')} fill={line.color} opacity="0.06" />}
              <polyline points={pts.join(' ')} stroke={line.color} strokeWidth="2.5"
                strokeDasharray={line.dashed ? '6 3' : 'none'} fill="none" opacity="0.9" />
            </g>
          );
        })}

        {/* Candles */}
        {safeData.map((d, i) => {
          const x = fx(i), isUp = d.close >= d.open, c = isUp ? UP : DOWN;
          const yOpen = py(d.open), yClose = py(d.close), yHi = py(d.high), yLo = py(d.low);
          const top = Math.min(yOpen, yClose), bh = Math.max(1, Math.abs(yClose - yOpen));
          return (
            <g key={i} opacity={hoverIdx === i ? 0.6 : 1}>
              <line x1={x} x2={x} y1={yHi} y2={yLo} stroke={c} strokeWidth="1" />
              <rect x={x - bw / 2} y={top} width={Math.max(1, bw)} height={bh} fill={c} stroke={c} />
            </g>
          );
        })}

        {/* MAs */}
        {maPath(ma5, '#F77234')}
        {maPath(ma10, '#722ED1')}
        {maPath(ma20, '#3491FA')}

        {/* Markers */}
        {markers.map((m, k) => {
          if (m.i < 0 || m.i >= safeData.length) return null;
          const x = fx(m.i);
          if (m.type === 'buy') {
            const yTop = py(safeData[m.i].high) - 16;
            return <g key={`mk${k}`}><polygon points={`${x},${yTop + 10} ${x - 6},${yTop} ${x + 6},${yTop}`} fill="#165DFF" /><text x={x} y={yTop - 4} fontSize="9" fill="#165DFF" textAnchor="middle" fontWeight="700">{m.label || 'B'}</text></g>;
          }
          const yBot = py(safeData[m.i].low) + 15;
          return <g key={`mk${k}`}><circle cx={x} cy={yBot} r="4" fill="#F53F3F" stroke="#fff" strokeWidth="1.2" /><text x={x} y={yBot + 12} fontSize="8" fill="#F53F3F" textAnchor="middle">{m.label || '上榜'}</text></g>;
        })}

        {/* Prediction hi/lo markers */}
        {predMarkers.map((m, k) => {
          if (m.i < 0) return null;
          const x = fx(m.i);
          const price = m.price ?? (safeData.length > 0 ? safeData[safeData.length-1]?.close ?? 0 : 0);
          const v = m.type === 'predHi' ? py(price) - 14 : py(price) + 14;
          const tri = m.type === 'predHi'
            ? `${x},${v + 8} ${x - 6},${v} ${x + 6},${v}`
            : `${x},${v - 8} ${x - 6},${v} ${x + 6},${v}`;
          return (
            <g key={`pm${k}`}>
              <polygon points={tri} fill={m.color || '#FFB400'} stroke="#fff" strokeWidth="1" />
              <text x={x} y={m.type === 'predHi' ? v - 3 : v + 16} fontSize="9" fill={m.color || '#86909C'} textAnchor="middle" fontWeight="700">{m.label}</text>
            </g>
          );
        })}

        {/* Crosshair */}
        {hoverIdx != null && (
          <>
            <line x1={hoverX} x2={hoverX} y1={padT} y2={padT + priceH} stroke="#1D2129" strokeWidth="0.8" strokeDasharray="2 4" opacity="0.5" />
            {hoverData && <line x1={padL} x2={W - padR} y1={py(hoverData.close)} y2={py(hoverData.close)} stroke="#86909C" strokeWidth="0.6" strokeDasharray="2 3" opacity="0.35" />}
            {isHoverPredict && hoverIdx != null && predictionLines.map((l, li) => {
              const p = l.data[hoverIdx];
              if (p == null) return null;
              return <line key={`chl${li}`} x1={padL} x2={W - padR} y1={py(p)} y2={py(p)} stroke={l.color} strokeWidth="0.8" strokeDasharray="3 3" opacity="0.5" />;
            })}
          </>
        )}

        {/* Range selection highlight */}
        {effectiveRange && (() => {
          const [rs, re] = effectiveRange;
          if (rs === re) return null;
          const x1 = fx(rs) - step / 2, x2 = fx(re) + step / 2;
          const handleW = 8, handleH = 36;
          const isDraggingLeft = dragMode === 'resizeStart';
          const isDraggingRight = dragMode === 'resizeEnd';
          return (
            <g>
              {/* Selection background with gradient */}
              <defs>
                <linearGradient id="selGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="rgba(22, 93, 255, 0.14)" />
                  <stop offset="100%" stopColor="rgba(22, 93, 255, 0.04)" />
                </linearGradient>
                <filter id="glow">
                  <feGaussianBlur stdDeviation="2" result="blur" />
                  <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
                </filter>
              </defs>
              <rect x={x1} y={padT} width={x2 - x1} height={priceH} fill="url(#selGrad)" />
              {/* Top & bottom border */}
              <line x1={x1} y1={padT} x2={x2} y2={padT} stroke="rgba(22, 93, 255, 0.45)" strokeWidth="1.5" />
              <line x1={x1} y1={padT + priceH} x2={x2} y2={padT + priceH} stroke="rgba(22, 93, 255, 0.45)" strokeWidth="1.5" />
              
              {/* Left handle */}
              <g>
                <line x1={x1} y1={padT} x2={x1} y2={padT + priceH} stroke="#165DFF" strokeWidth="1.5" strokeDasharray="4 3" opacity="0.7" />
                <rect x={x1 - handleW/2} y={padT + priceH/2 - handleH/2} width={handleW} height={handleH} rx={4}
                  fill={isDraggingLeft ? '#0E42D2' : '#165DFF'} opacity={isDraggingLeft ? 0.95 : 0.82}
                  filter={isDraggingLeft ? 'url(#glow)' : undefined} />
                {[0, -5, 5].map(off => (
                  <circle key={`lg-${off}`} cx={x1} cy={padT + priceH/2 + off} r="1.2" fill="#fff" opacity="0.9" />
                ))}
                <polygon points={`${x1-5},${padT + priceH/2 - 7} ${x1-1},${padT + priceH/2} ${x1-5},${padT + priceH/2 + 7}`} fill="#165DFF" opacity="0.7" />
                <polygon points={`${x1+5},${padT + priceH/2 - 7} ${x1+1},${padT + priceH/2} ${x1+5},${padT + priceH/2 + 7}`} fill="#165DFF" opacity="0.7" />
              </g>
              
              {/* Right handle */}
              <g>
                <line x1={x2} y1={padT} x2={x2} y2={padT + priceH} stroke="#165DFF" strokeWidth="1.5" strokeDasharray="4 3" opacity="0.7" />
                <rect x={x2 - handleW/2} y={padT + priceH/2 - handleH/2} width={handleW} height={handleH} rx={4}
                  fill={isDraggingRight ? '#0E42D2' : '#165DFF'} opacity={isDraggingRight ? 0.95 : 0.82}
                  filter={isDraggingRight ? 'url(#glow)' : undefined} />
                {[0, -5, 5].map(off => (
                  <circle key={`rg-${off}`} cx={x2} cy={padT + priceH/2 + off} r="1.2" fill="#fff" opacity="0.9" />
                ))}
                <polygon points={`${x2-5},${padT + priceH/2 - 7} ${x2-1},${padT + priceH/2} ${x2-5},${padT + priceH/2 + 7}`} fill="#165DFF" opacity="0.7" />
                <polygon points={`${x2+5},${padT + priceH/2 - 7} ${x2+1},${padT + priceH/2} ${x2+5},${padT + priceH/2 + 7}`} fill="#165DFF" opacity="0.7" />
              </g>
              
              {/* Middle move indicator */}
              {re - rs > 4 && (
                <g opacity="0.45">
                  <line x1={(x1+x2)/2 - 8} y1={padT + priceH/2} x2={(x1+x2)/2 + 8} y2={padT + priceH/2} stroke="#165DFF" strokeWidth="1.5" />
                  <polygon points={`${(x1+x2)/2 - 14},${padT + priceH/2} ${(x1+x2)/2 - 4},${padT + priceH/2 - 4} ${(x1+x2)/2 - 4},${padT + priceH/2 + 4}`} fill="#165DFF" />
                  <polygon points={`${(x1+x2)/2 + 14},${padT + priceH/2} ${(x1+x2)/2 + 4},${padT + priceH/2 - 4} ${(x1+x2)/2 + 4},${padT + priceH/2 + 4}`} fill="#165DFF" />
                </g>
              )}
              
              {/* K-line count badge */}
              <rect x={(x1+x2)/2 - 40} y={padT + 2} width={80} height={16} rx={4} fill="rgba(22, 93, 255, 0.15)" />
              <text x={(x1+x2)/2} y={padT + 13} fontSize="9" fill="#165DFF" textAnchor="middle" fontWeight={600}>
                {re - rs + 1} 根K线
              </text>
            </g>
          );
        })()}

        {/* X labels */}
        {xLabels.map((idx, k) => <text key={k} x={fx(idx)} y={H - 6} fontSize="10" fill="#86909C" textAnchor="middle">{dateLabel(idx)}</text>)}

        {/* Legend */}
        <g transform={`translate(${padL}, ${padT - 1})`} fontSize="10">
          <text x="0" y="0" fill="#F77234">— MA5</text>
          <text x="50" y="0" fill="#722ED1">— MA10</text>
          <text x="108" y="0" fill="#3491FA">— MA20</text>
          {predictionLines.length > 0 && <text x="170" y="0" fill={predictionLines[0].color}>--- 预测</text>}
        </g>
      </svg>

      {/* Tooltip */}
      {((hoverData && hoverIdx != null) || isHoverPredict) && tooltipPos && (
        <div style={{
          position: 'absolute', left: tooltipPos.x + 16, top: Math.min(tooltipPos.y - 80, height - 160),
          background: 'rgba(29, 33, 41, 0.92)', color: '#fff', padding: '10px 14px',
          borderRadius: 6, fontSize: 12, lineHeight: '20px', pointerEvents: 'none', zIndex: 100,
          fontFamily: 'monospace', whiteSpace: 'nowrap', boxShadow: '0 2px 12px rgba(0,0,0,0.15)',
        }}>
          <div style={{ fontWeight: 600, marginBottom: 4, color: '#C9CDD4' }}>{dateLabel(hoverIdx!)}</div>
          {hoverData ? (
            <>
              <div>开 <span style={{ color: '#fff', fontWeight: 500 }}>{hoverData?.open?.toFixed(2) ?? '-'}</span></div>
              <div>高 <span style={{ color: UP, fontWeight: 500 }}>{hoverData?.high?.toFixed(2) ?? '-'}</span></div>
              <div>低 <span style={{ color: DOWN, fontWeight: 500 }}>{hoverData?.low?.toFixed(2) ?? '-'}</span></div>
              <div>收 <span style={{ color: (hoverData?.close ?? 0) >= (hoverData?.open ?? 0) ? UP : DOWN, fontWeight: 600, fontSize: 13 }}>{hoverData?.close?.toFixed(2) ?? '-'}</span></div>
              <div style={{ marginTop: 4, color: '#C9CDD4' }}>量 {(hoverData?.volume || 0) >= 1e8 ? ((hoverData?.volume || 0) / 1e8).toFixed(2) + '亿' : ((hoverData?.volume || 0) / 1e4).toFixed(0) + '万手'}</div>
            </>
          ) : (
            <>
              <div style={{ fontWeight: 600, marginBottom: 4, color: '#FFB400' }}>📈 预测价格</div>
              {hoverPreds.map((p, i) => (
                <div key={i}>
                  <span style={{ display: 'inline-block', width: 6, height: 6, borderRadius: 3, background: p.color, marginRight: 6 }} />
                  {p.name} <span style={{ color: '#fff', fontWeight: 600 }}>{p.price!.toFixed(2)}</span>
                </div>
              ))}
            </>
          )}
          {hoverData?.close !== undefined && hoverIdx != null && hoverIdx > 0 && hoverIdx < safeData.length && (
            <div style={{ color: (hoverData.close >= (safeData[hoverIdx - 1]?.close ?? 0)) ? UP : DOWN, fontWeight: 600 }}>
              涨跌 {((hoverData.close - (safeData[hoverIdx - 1]?.close ?? 0)) >= 0 ? '+' : '')}{(hoverData.close - (safeData[hoverIdx - 1]?.close ?? 0)).toFixed(2)}
              {' '}({(safeData[hoverIdx - 1]?.close ?? 0) > 0 ? (((hoverData.close - (safeData[hoverIdx - 1]?.close ?? 0)) / (safeData[hoverIdx - 1]?.close ?? 1)) * 100).toFixed(2) : '0.00'}%)
            </div>
          )}
        </div>
      )}
    </div>
  );
}
