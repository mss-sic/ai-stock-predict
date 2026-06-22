import React, { useState, useMemo, useCallback, useRef, useEffect } from 'react';
import { useTheme } from '../services/ThemeContext';
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight, ZoomIn, ZoomOut } from 'lucide-react';

interface KLineItem {
  tradeDate?: string; date?: string;
  open: number; close: number; high: number; low: number;
  volume?: number;
  turnoverRate?: number;
}

interface Marker {
  i: number;
  type: 'board' | 'buy' | 'sell' | 't';
  label?: string;
  rank?: number;
  price?: number;
  buyPrice?: number;
  sellPrice?: number;
  quantity?: number;
  buyQty?: number;
  sellQty?: number;
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
  buyPrice?: number;
  sellPrice?: number;
  quantity?: number;
  buyQty?: number;
  sellQty?: number;
}

interface Props {
  data: KLineItem[];
  height?: number;
  markers?: Marker[];
  predictionLines?: PredictionLine[];
  splitIdx?: number;
  predMarkers?: PredMarker[];
  enableRangeSelect?: boolean;
  onMarkerClick?: (index: number) => void;
  selectedRange?: [number, number] | null;
  onRangeChange?: (startIdx: number, endIdx: number) => void;
  costLine?: number | null;
}

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
  costLine = null,
  onMarkerClick,
}: Props) {
  const { isDark } = useTheme();
  const UP = isDark ? '#f85149' : '#F53F3F';
  const DOWN = isDark ? '#3fb950' : '#00B42A';
  const [hoverIdx, setHoverIdx] = useState<number | null>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const [tooltipPos, setTooltipPos] = useState<{ x: number; y: number } | null>(null);
  const [dragStart, setDragStart] = useState<number | null>(null);
  const [dragEnd, setDragEnd] = useState<number | null>(null);
  const [dragMode, setDragMode] = useState<'new' | 'move' | 'resizeStart' | 'resizeEnd' | null>(null);
  const [dragOffset, setDragOffset] = useState(0);
  const [dragRangeWidth, setDragRangeWidth] = useState(0);
  const isDragging = dragStart !== null;

  // ─── Chart pan & zoom ───
  const [candlesPerScreen, setCandlesPerScreen] = useState(60);
  const [panOffset, setPanOffset] = useState(0); // px offset from right edge

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

  // Prediction extension
  const maxPredExtra = predictionLines.reduce((max, l) => {
    const extra = l.data.length - safeData.length;
    return extra > max ? extra : max;
  }, 0);
  const totalN = safeData.length + Math.max(0, maxPredExtra);
  const predExtra = Math.max(0, maxPredExtra);

  // Visible window (includes prediction slots when present)
  const visCount = Math.min(candlesPerScreen + predExtra, totalN);

  // Theme colors
  const chartBg = isDark ? '#121215' : '#fff';
  const gridColor = isDark ? '#27272a' : '#F2F3F5';
  const textColor = isDark ? '#8a8d91' : 'var(--color-text-3)';
  const axisColor = isDark ? '#b0b3b8' : '#4E5969';
  const crosshairColor = isDark ? '#b0b3b8' : '#C9CDD4';
  const predBg = isDark ? '#1c1c20' : '#F7F8FA';
  // ═══ All hooks must be called unconditionally ═══
  const W = 960;
  const H = height;
  const padL = 54, padR = 60, padT = 14, padB = 30;
  const volH = 50;
  const macdH = 100;
  const priceH = H - padT - padB - volH - macdH - 14;
  const innerW = W - padL - padR;

  const step = innerW / (visCount || 1);
  const startIdx = Math.max(0, Math.min(totalN - visCount, totalN - visCount + Math.round(panOffset / Math.max(step, 0.01))));
  const bw = Math.max(2, Math.min(22, step * 0.78));
  const useLineMode = (visCount - predExtra) > 60;

  const visibleSlice = safeData.slice(startIdx, Math.min(startIdx + visCount, safeData.length));
  const hi = visibleSlice.length === 0 ? 0 : Math.max(...visibleSlice.map(d => d.high));
  const lo = visibleSlice.length === 0 ? 0 : Math.min(...visibleSlice.map(d => d.low));
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
  const padding = range * 0.03;
  const plotLo = extLo - padding;
  const plotRange = extHi - extLo + padding * 2;
  const py = (v: number) => padT + priceH - ((v - plotLo) / plotRange) * priceH;

  const volMax = visibleSlice.length === 0 ? 1 : Math.max(...visibleSlice.map(d => d.volume || 0)) || 1;
  const volBaseY = padT + priceH + 8 + volH;
  const vy = (v: number) => volBaseY - (v / volMax) * volH;

  // ═══ MACD calculation ═══
  const calcEMA = (arr: number[], period: number) => {
    if (arr.length === 0) return [];
    const k = 2 / (period + 1);
    const result = [arr[0]];
    for (let i = 1; i < arr.length; i++) result.push(arr[i] * k + result[i - 1] * (1 - k));
    return result;
  };
  const closes = safeData.map(d => d.close);
  const ema12Arr = calcEMA(closes, 12);
  const ema26Arr = calcEMA(closes, 26);
  const difArr = ema12Arr.map((v, i) => v - (ema26Arr[i] ?? 0));
  const deaArr = calcEMA(difArr, 9);
  const macdArr = difArr.map((v, i) => (v - (deaArr[i] ?? 0)) * 2);

  const macdVisible = macdArr.slice(startIdx, Math.min(startIdx + visCount, macdArr.length));
  const macdAbsMax = Math.max(
    Math.abs(Math.max(...macdVisible.filter(v => !isNaN(v)), 0)),
    Math.abs(Math.min(...macdVisible.filter(v => !isNaN(v)), 0)),
    1
  );
  const macdBaseY = volBaseY + 6 + macdH;
  const my = (v: number) => macdBaseY - macdH / 2 - (v / macdAbsMax) * (macdH / 2);
  const macdZeroY = macdBaseY - macdH / 2;

  // Cross detection
  type CrossSignal = { i: number; type: 'golden' | 'death' };
  const crosses: CrossSignal[] = [];
  for (let i = 1; i < difArr.length; i++) {
    if (difArr[i - 1] <= (deaArr[i - 1] ?? 0) && difArr[i] > (deaArr[i] ?? 0))
      crosses.push({ i, type: 'golden' });
    else if (difArr[i - 1] >= (deaArr[i - 1] ?? 0) && difArr[i] < (deaArr[i] ?? 0))
      crosses.push({ i, type: 'death' });
  }
  const fx = (i: number) => padL + (i - startIdx) * step + step / 2;
  const visStartIdx = startIdx;
  const visEndIdx = startIdx + visCount;

  const yTicks = 5;
  const grids = Array.from({ length: yTicks }, (_, i) => {
    const v = plotLo + (plotRange * i) / (yTicks - 1);
    return { y: py(v), label: v.toFixed(2) };
  });

  const xLabels = useMemo(() => {
    const end = Math.min(startIdx + visCount, safeData.length);
    const count = end - startIdx;
    const labels: number[] = [];
    if (count <= 0) return labels;
    // Show ~6 evenly-spaced labels within visible window
    const step_labels = Math.max(1, Math.floor(count / 6));
    for (let i = startIdx; i < end; i += step_labels) {
      labels.push(i);
    }
    // Ensure last visible index is labeled
    if (labels.length > 0 && labels[labels.length - 1] !== end - 1) {
      labels.push(end - 1);
    }
    // Also keep the first label if not already there
    if (labels.length > 0 && labels[0] !== startIdx) {
      labels.unshift(startIdx);
    }
    if (maxPredExtra > 0) labels.push(totalN - 1);
    return labels;
  }, [safeData.length, startIdx, visCount, totalN, maxPredExtra]);

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
      return ds.length >= 10 ? ds.slice(0, 10).replace(/-/g, '') : `T-${safeData.length - i}`;
    }
    return `+${i - safeData.length + 1}`;
  };

  const getIdxFromEvent = useCallback((e: React.MouseEvent<SVGSVGElement>) => {
    const svg = svgRef.current; if (!svg) return -1;
    const rect = svg.getBoundingClientRect();
    const scaleX = W / rect.width;
    const mx = (e.clientX - rect.left) * scaleX;
    return startIdx + Math.round((mx - padL - step / 2) / step);
  }, [W, padL, step, startIdx]);

  // ═══ Wheel zoom ═══
  // Shared zoom helper for buttons (keep center stable)
  const handleZoom = useCallback((delta: number) => {
    const newCandles = Math.max(15, Math.min(safeData.length, candlesPerScreen + delta));
    if (newCandles === candlesPerScreen) return;
    const centerIdx = startIdx + visCount / 2;
    const newStartIdx = Math.max(0, Math.min(safeData.length - newCandles, Math.round(centerIdx - newCandles / 2)));
    const newStep = innerW / newCandles;
    const newPanOffset = (newStartIdx - (safeData.length - newCandles)) * newStep;
    setCandlesPerScreen(newCandles);
    setPanOffset(newPanOffset);
  }, [candlesPerScreen, startIdx, visCount, safeData.length, innerW]);


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
        style={{ width: '100%', height, background: chartBg, borderRadius: 10, cursor: chartCursor }}
        onMouseMove={handleMouseMove}
        onMouseLeave={handleMouseLeave}
        onMouseDown={handleMouseDown}
        onMouseUp={handleMouseUp}
      >
        {/* Prediction bg */}
        {splitIdx != null && splitIdx <= safeData.length && (
          <>
            <rect x={fx(splitIdx) - step / 2} y={padT} width={fx(totalN - 1) - fx(splitIdx) + step} height={priceH} fill={predBg} opacity="0.5" />
            <rect x={fx(splitIdx) - step / 2} y={padT} width={2} height={priceH} fill={crosshairColor} opacity="0.3" />
            <line x1={fx(splitIdx) - step / 2} x2={fx(splitIdx) - step / 2} y1={padT} y2={padT + priceH} stroke={crosshairColor} strokeDasharray="4 3" strokeWidth="1" />
            <text x={fx(splitIdx) - step / 4} y={padT + 11} fontSize="9" fill={textColor}>←历史 预测→</text>
          </>
        )}

        {/* Grid */}
        {grids.map((g, i) => (
          <g key={i}>
            <line x1={padL} x2={W - padR} y1={g.y} y2={g.y} stroke={gridColor} strokeWidth="0.8" />
            <text x={W - padR + 6} y={g.y + 3} fontSize="10" fill={textColor}>{g.label}</text>
          </g>
        ))}

      {/* Volume label */}
        <text x={padL} y={padT + priceH + 12} fontSize="10" fill={textColor} fontWeight={500}>▎成交量</text>

        {/* Volume bars */}
        {safeData.map((d, i) => {
          const isUp = d.close >= d.open;
          const c = isUp ? UP : DOWN;
          const x = fx(i);
          const vh = Math.max(1, volBaseY - vy(d.volume || 0));
          return <rect key={`v${i}`} x={x - bw / 2} y={volBaseY - vh} width={bw} height={Math.max(1, vh)} fill={c} opacity={isUp ? 0.3 : 0.25} />;
        })}

        {/* ═══ MACD ═══ */}
        {/* MACD label */}
        <text x={padL} y={volBaseY + 14} fontSize="10" fill={textColor} fontWeight={500}>▎MACD</text>
        {/* Legend */}
        <text x={padL + 48} y={volBaseY + 14} fontSize="9" fill="#F77234">DIF</text>
        <text x={padL + 78} y={volBaseY + 14} fontSize="9" fill="#3491FA">DEA</text>

        {/* Zero line */}
        <line x1={padL} x2={W - padR} y1={macdZeroY} y2={macdZeroY} stroke={gridColor} strokeWidth="0.8" />

        {/* MACD histogram */}
        {safeData.map((d, i) => {
          if (i >= macdArr.length) return null;
          const v = macdArr[i];
          if (isNaN(v)) return null;
          const x = fx(i);
          const h = Math.max(1, Math.abs(v) / macdAbsMax * (macdH / 2));
          const isUp = v >= 0;
          return <rect key={`macd-${i}`} x={x - bw / 2} y={isUp ? macdZeroY - h : macdZeroY} width={bw} height={h}
            fill={isUp ? UP : DOWN} opacity={isUp ? 0.4 : 0.3} rx="1" />;
        })}

        {/* DIF line */}
        <polyline
          points={difArr.map((v, i) => v != null && !isNaN(v) ? `${fx(i).toFixed(1)},${my(v).toFixed(1)}` : '').filter(Boolean).join(' ')}
          stroke="#F77234" strokeWidth="1.2" fill="none" />

        {/* DEA line */}
        <polyline
          points={deaArr.map((v, i) => v != null && !isNaN(v) ? `${fx(i).toFixed(1)},${my(v).toFixed(1)}` : '').filter(Boolean).join(' ')}
          stroke="#3491FA" strokeWidth="1.2" fill="none" />

        {/* Cross signals */}
        {crosses.map((c, k) => {
          const x = fx(c.i);
          const y = my(difArr[c.i]);
          const isGolden = c.type === 'golden';
          return (
            <g key={`cross-${k}`}>
              <circle cx={x} cy={y} r={4}
                fill={isGolden ? '#F53F3F' : '#00B42A'} stroke="#fff" strokeWidth="1.2"
                opacity="0.85" />
              <text x={x} y={y - 8} fontSize="8"
                fill={isGolden ? '#F53F3F' : '#00B42A'} textAnchor="middle" fontWeight={700}>
                {isGolden ? '金叉' : '死叉'}
              </text>
            </g>
          );
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

      {/* Candles (short-term) or Line (long-term) */}
        {!useLineMode && safeData.map((d, i) => {
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

        {/* Line mode: close price line for long-term view */}
        {useLineMode && (
          <>
            <polyline
              points={safeData.map((d, i) => `${fx(i).toFixed(1)},${py(d.close).toFixed(1)}`).join(' ')}
              stroke={isDark ? '#e06060' : '#F53F3F'} strokeWidth="1.5" fill="none" opacity="0.8"
            />
            {safeData.length > 1 && (() => {
              const pts = safeData.map((d, i) => `${fx(i).toFixed(1)},${py(d.close).toFixed(1)}`);
              const firstX = fx(0), lastX = fx(safeData.length - 1);
              const base = py(plotLo);
              const fillPts = pts.join(' ') + ` ${lastX.toFixed(1)},${base.toFixed(1)} ${firstX.toFixed(1)},${base.toFixed(1)}`;
              return <polygon points={fillPts} fill={UP} opacity="0.06" />;
            })()}
          </>
        )}

        {/* MAs */}
        {maPath(ma5, '#F77234')}
        {maPath(ma10, '#722ED1')}
        {maPath(ma20, '#3491FA')}

        {/* Cost line — horizontal dashed line at holding cost */}
        {costLine != null && costLine > 0 && (() => {
          const y = py(costLine);
          return (
            <g>
              <line x1={padL} x2={W - padR} y1={y} y2={y}
                stroke="#FF7D00" strokeWidth="1.5" strokeDasharray="6 4" opacity="0.7" />
              <rect x={W - padR - 52} y={y - 9} width="52" height="18" rx="4"
                fill="#FF7D00" opacity="0.85" />
              <text x={W - padR - 4} y={y + 4} fontSize="10" fill="#fff"
                textAnchor="end" fontWeight="600">成本 ¥{costLine.toFixed(2)}</text>
            </g>
          );
        })()}

        {/* Markers — buy below candle, sell above candle */}
        {markers.map((m, k) => {
          if (m.i < 0 || m.i >= safeData.length) return null;
          const x = fx(m.i);
          const candleHigh = py(safeData[m.i].high);
          const candleLow = py(safeData[m.i].low);
          
          if (m.type === 'buy') {
            // Buy: red arrow below candle pointing up, price label below arrow
            const my = candleLow + 14;
            return (
              <g key={`mk${k}`}>
                <line x1={x} y1={candleLow} x2={x} y2={my + 10} stroke="#F53F3F" strokeWidth="1.5" opacity="0.8" />
                <circle cx={x} cy={my} r="7" fill="#F53F3F" stroke="#fff" strokeWidth="1.5" style={{cursor:'pointer'}} onClick={() => onMarkerClick?.(m.i)} />
                <text x={x} y={my + 3.5} fontSize="9" fill="#fff" textAnchor="middle" fontWeight="700">B</text>
                <text x={x} y={my + 22} fontSize="10" fill="#F53F3F" textAnchor="middle" fontWeight="600">{m.label || ''}</text>
              </g>
            );
          }
          if (m.type === 'sell') {
            // Sell: green arrow above candle pointing down, price label above arrow
            const my = candleHigh - 14;
            return (
              <g key={`mk${k}`}>
                <line x1={x} y1={candleHigh} x2={x} y2={my - 10} stroke="#00B42A" strokeWidth="1.5" opacity="0.8" />
                <circle cx={x} cy={my} r="7" fill="#00B42A" stroke="#fff" strokeWidth="1.5" style={{cursor:'pointer'}} onClick={() => onMarkerClick?.(m.i)} />
                <text x={x} y={my + 3.5} fontSize="9" fill="#fff" textAnchor="middle" fontWeight="700">S</text>
                <text x={x} y={my - 10} fontSize="10" fill="#00B42A" textAnchor="middle" fontWeight="600">{m.label || ''}</text>
              </g>
            );
          }
          if (m.type === 't') {
            // T-trade: orange diamond below candle
            const my = candleLow + 14;
            const DIAMOND_COLOR = '#FF7D00';
            return (
              <g key={'mk'+k}>
                <line x1={x} y1={candleLow} x2={x} y2={my + 10} stroke={DIAMOND_COLOR} strokeWidth="1.5" opacity="0.8" />
                <rect x={x - 6} y={my - 6} width={12} height={12} rx={2} fill={DIAMOND_COLOR} stroke="#fff" strokeWidth="1.5" style={{cursor:'pointer'}} onClick={() => onMarkerClick?.(m.i)} transform={'rotate(45 ' + x + ' ' + my + ')'} />
                <text x={x} y={my + 3.5} fontSize="9" fill="#fff" textAnchor="middle" fontWeight="700">T</text>
                <text x={x} y={my + 22} fontSize="10" fill={DIAMOND_COLOR} textAnchor="middle" fontWeight="600">{m.label || ''}</text>
              </g>
            );
          }
          // Board markers
          const BOARD_PURPLE = '#9333ea';
          const bmy = candleHigh - 8;
          return (
            <g key={`mk${k}`}>
              <circle cx={x} cy={bmy - 12} r="8" fill={BOARD_PURPLE} stroke="#fff" strokeWidth="1.5" opacity="0.9" style={{cursor:'pointer'}} onClick={() => onMarkerClick?.(m.i)} />
              <text x={x} y={bmy - 8} fontSize="9" fill="#fff" textAnchor="middle" fontWeight="700">榜</text>
              {m.rank && <text x={x} y={bmy + 4} fontSize="8" fill={BOARD_PURPLE} textAnchor="middle">#{m.rank}</text>}
            </g>
          );
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
              <polygon points={tri} fill={m.color || (isDark ? '#fbbf24' : '#FFB400')} stroke="#fff" strokeWidth="1" />
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
        {xLabels.map((idx, k) => <text key={k} x={fx(idx)} y={H - 6} fontSize="10" fill={textColor} textAnchor="middle">{dateLabel(idx)}</text>)}

        {/* Legend */}
        <g transform={`translate(${padL}, ${padT - 1})`} fontSize="10">
          <text x="0" y="0" fill={isDark ? "#d29922" : "#F77234"}>— MA5</text>
          <text x="50" y="0" fill="#722ED1">— MA10</text>
          <text x="108" y="0" fill="#3491FA">— MA20</text>
          {predictionLines.length > 0 && <text x="170" y="0" fill={predictionLines[0].color}>--- 预测</text>}
        </g>
      {/* Scrollbar */}
        {totalN > visCount && (
          <g transform={`translate(${padL}, ${H - 8})`}>
            <rect x={0} y={0} width={innerW} height={4} rx={2} fill={gridColor} opacity="0.5" />
            <rect
              x={Math.max(0, (startIdx / Math.max(totalN - 1, 1)) * innerW)}
              y={0}
              width={Math.max(20, (visCount / totalN) * innerW)}
              height={4} rx={2}
              fill={isDark ? '#4a4a6a' : '#c9cdd4'}
              style={{ cursor: 'pointer' }}
            />
          </g>
        )}

        {/* Pan/Zoom hint */}
        {safeData.length > 0 && (
          <text x={W - padR} y={H - 2} fontSize="9" fill={textColor} textAnchor="end" opacity="0.6">
            底部按钮操作 · {startIdx + 1}-{Math.min(startIdx + visCount, totalN)}/{totalN}
          </text>
        )}
      </svg>

      {/* Zoom/Pan button bar */}
      <style>{`
        .kl-btn { background: none; border: 1px solid transparent; cursor: pointer; font-size: 13px; color: inherit; padding: 5px 10px; border-radius: 6px; display: inline-flex; align-items: center; justify-content: center; transition: all 0.15s; font-weight: 500; line-height: 20px; user-select: none; }
        .kl-btn:hover { background: ${isDark ? "rgba(22,93,255,0.15)" : "#e8f0fe"}; border-color: ${isDark ? "#165DFF" : "#165DFF40"}; }
        .kl-btn:active { transform: scale(0.96); }
        .kl-btn.active { background: ${isDark ? "rgba(22,93,255,0.2)" : "rgba(22,93,255,0.1)"}; border-color: #165DFF; color: #165DFF; font-weight: 700; }
        .kl-btn:disabled { opacity: .35; cursor: not-allowed; }
        .kl-btn:disabled:hover { background: none; border-color: transparent; transform: none; }
      `}</style>
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
        padding: '8px 10px', borderTop: `1px solid ${gridColor}`,
        background: chartBg, color: textColor, flexWrap: 'wrap', userSelect: 'none'
      }}>
        <button onClick={() => setPanOffset(p => p - step * 30)} disabled={startIdx <= 0}
          className="kl-btn" title="左移30根"><ChevronsLeft size={15} /></button>
        <button onClick={() => setPanOffset(p => p - step * 5)} disabled={startIdx <= 0}
          className="kl-btn" title="左移5根"><ChevronLeft size={15} /></button>

        <button onClick={() => handleZoom(-10)}
          className="kl-btn" title="放大（减少K线数）"><ZoomIn size={15} /></button>

        <span style={{ fontSize: 12, color: axisColor, minWidth: 90, textAlign: 'center', fontWeight: 500, fontVariantNumeric: 'tabular-nums' }}>
          {startIdx + 1}-{Math.min(startIdx + visCount, totalN)} / {totalN}
        </span>

        <button onClick={() => handleZoom(10)}
          className="kl-btn" title="缩小（增加K线数）"><ZoomOut size={15} /></button>

        <button onClick={() => setPanOffset(p => p + step * 5)} disabled={startIdx + visCount >= totalN}
          className="kl-btn" title="右移5根"><ChevronRight size={15} /></button>
        <button onClick={() => setPanOffset(p => p + step * 30)} disabled={startIdx + visCount >= totalN}
          className="kl-btn" title="右移30根"><ChevronsRight size={15} /></button>

        <span style={{ width: 1, height: 18, background: gridColor, margin: '0 6px' }} />

        {[
          { label: '1月', days: 22 },
          { label: '3月', days: 66 },
          { label: '半年', days: 132 },
          { label: '1年', days: 264 },
          { label: '全部', days: safeData.length },
        ].map(p => {
          const isActive = (visCount - predExtra) === p.days || (p.label === '全部' && (visCount - predExtra) >= safeData.length);
          return (
            <button key={p.label} onClick={() => { setCandlesPerScreen(p.days); setPanOffset(0); }}
              className={`kl-btn${isActive ? ' active' : ''}`}
            >{p.label}</button>
          );
        })}
      </div>

      {/* Tooltip */}
      {((hoverData && hoverIdx != null) || isHoverPredict) && tooltipPos && (
        <div style={{
          position: 'absolute', left: tooltipPos.x + 16, top: Math.min(tooltipPos.y - 80, height - 160),
          background: isDark ? 'rgba(0,0,0,0.94)' : 'rgba(255,255,255,0.96)',
          color: isDark ? 'var(--color-border-1)' : 'var(--color-text-1)',
          padding: '10px 14px', borderRadius: 8, fontSize: 12, lineHeight: '20px',
          pointerEvents: 'none', zIndex: 100, fontFamily: 'monospace', whiteSpace: 'nowrap',
          boxShadow: isDark ? '0 2px 12px rgba(0,0,0,0.3)' : '0 2px 12px rgba(0,0,0,0.12)',
          border: isDark ? '1px solid rgba(255,255,255,0.08)' : '1px solid rgba(0,0,0,0.06)',
        }}>
          <div style={{ fontWeight: 600, marginBottom: 4, color: isDark ? '#a0a4a8' : 'var(--color-text-2)' }}>{dateLabel(hoverIdx!)}</div>
          {hoverData ? (
            <>
              <div>开 <span style={{ color: isDark ? 'var(--color-border-1)' : 'var(--color-text-1)', fontWeight: 500 }}>{hoverData?.open?.toFixed(2) ?? '-'}</span></div>
              <div>高 <span style={{ color: UP, fontWeight: 500 }}>{hoverData?.high?.toFixed(2) ?? '-'}</span></div>
              <div>低 <span style={{ color: DOWN, fontWeight: 500 }}>{hoverData?.low?.toFixed(2) ?? '-'}</span></div>
              <div>收 <span style={{ color: (hoverData?.close ?? 0) >= (hoverData?.open ?? 0) ? UP : DOWN, fontWeight: 600, fontSize: 13 }}>{hoverData?.close?.toFixed(2) ?? '-'}</span></div>
              <div style={{ marginTop: 4, color: isDark ? '#a0a4a8' : 'var(--color-text-3)' }}>量 {(hoverData?.volume || 0) >= 1e8 ? ((hoverData?.volume || 0) / 1e6).toFixed(1) + '万手' : ((hoverData?.volume || 0) / 1e6).toFixed(2) + '万手'}</div>
              <div style={{ color: isDark ? '#a0a4a8' : 'var(--color-text-3)' }}>换手 {(hoverData?.turnoverRate || 0) > 0 ? ((hoverData?.turnoverRate || 0) * 100).toFixed(2) + '%' : '-'}</div>
            </>
          ) : (
            <>
              <div style={{ fontWeight: 600, marginBottom: 4, color: 'var(--color-warning-text)' }}>📈 预测价格</div>
              {hoverPreds.map((p, i) => (
                <div key={i}>
                  <span style={{ display: 'inline-block', width: 6, height: 6, borderRadius: 3, background: p.color, marginRight: 6 }} />
                  {p.name} <span style={{ color: isDark ? '#fff' : 'var(--color-text-1)', fontWeight: 600 }}>{p.price!.toFixed(2)}</span>
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
          {hoverIdx != null && (() => { const bm = markers.find(m => m.type === 'board' && m.i === hoverIdx); return bm?.rank != null ? (<div style={{ color: 'var(--purple-6)', fontWeight: 600, marginTop: 2 }}>🏆 榜单第 {bm.rank} 名</div>) : null; })()}
          {hoverIdx != null && (() => {
            const buyM = markers.find(m => m.type === 'buy' && m.i === hoverIdx && m.price != null);
            const sellM = markers.find(m => m.type === 'sell' && m.i === hoverIdx && m.price != null);
            const t = buyM || sellM;
            if (t) {
              const qtyStr = t.quantity != null ? ' × ' + t.quantity + '股' : '';
              return (
                <div style={{ color: buyM ? UP : DOWN, fontWeight: 600, marginTop: 2 }}>
                  {buyM ? '买入' : '卖出'} ¥{t.price!.toFixed(2)}{qtyStr}
                </div>
              );
            }
            return null;
          })()}
          {hoverIdx != null && (() => {
            const tM = markers.find(m => m.type === 't' && m.i === hoverIdx);
            if (!tM || tM.buyPrice == null || tM.sellPrice == null) return null;
            const buyQty = tM.buyQty || 0;
            const sellQty = tM.sellQty || 0;
            const buyAmt = tM.buyPrice * buyQty;
            const sellAmt = tM.sellPrice * sellQty;
            const tProfit = sellAmt - buyAmt;
            const tProfitPct = buyAmt > 0 ? (tProfit / buyAmt) * 100 : 0;
            return (
              <div style={{ marginTop: 2 }}>
                <div style={{ color: '#FF7D00', fontWeight: 600 }}>做T</div>
                <div style={{ color: UP }}>买入 ¥{tM.buyPrice.toFixed(2)} × {buyQty}股</div>
                <div style={{ color: DOWN }}>卖出 ¥{tM.sellPrice.toFixed(2)} × {sellQty}股</div>
                <div style={{ color: tProfit >= 0 ? UP : DOWN, fontWeight: 600 }}>
                  收益 {tProfit >= 0 ? '+' : ''}{tProfit.toFixed(2)} ({tProfitPct >= 0 ? '+' : ''}{tProfitPct.toFixed(2)}%)
                </div>
              </div>
            );
          })()}
        </div>
      )}
    </div>
  );
}
