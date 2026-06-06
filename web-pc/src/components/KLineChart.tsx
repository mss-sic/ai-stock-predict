import { useState, useMemo, useCallback, useRef } from 'react';

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

interface Props {
  data: KLineItem[];
  height?: number;
  markers?: Marker[];
  predictionLines?: PredictionLine[];
  splitIdx?: number;
}

const UP = '#F53F3F';
const DOWN = '#00B42A';

export default function KLineChart({
  data,
  height = 420,
  markers = [],
  predictionLines = [],
  splitIdx,
}: Props) {
  const [hoverIdx, setHoverIdx] = useState<number | null>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const [tooltipPos, setTooltipPos] = useState<{ x: number; y: number } | null>(null);

  const isEmpty = !data || data.length === 0;

  // ═══ All hooks must be called unconditionally ═══
  const W = 960;
  const H = height;
  const padL = 54, padR = 60, padT = 14, padB = 30;
  const volH = 70;
  const priceH = H - padT - padB - volH - 8;
  const innerW = W - padL - padR;

  const maxPredExtra = predictionLines.reduce((max, l) => {
    const extra = l.data.length - data.length;
    return extra > max ? extra : max;
  }, 0);
  const totalN = (isEmpty ? 0 : data.length) + Math.max(0, maxPredExtra);
  const step = innerW / (totalN || 1);
  const bw = Math.max(2, Math.min(10, step * 0.72));

  const hi = isEmpty ? 0 : Math.max(...data.map(d => d.high));
  const lo = isEmpty ? 0 : Math.min(...data.map(d => d.low));
  let extHi = hi, extLo = lo;
  if (!isEmpty && splitIdx != null) {
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

  const volMax = isEmpty ? 1 : Math.max(...data.map(d => d.volume || 0)) || 1;
  const volBaseY = padT + priceH + 8 + volH;
  const vy = (v: number) => volBaseY - (v / volMax) * volH;
  const fx = (i: number) => padL + i * step + step / 2;

  const yTicks = 5;
  const grids = Array.from({ length: yTicks }, (_, i) => {
    const v = plotLo + (plotRange * i) / (yTicks - 1);
    return { y: py(v), label: v.toFixed(2) };
  });

  const xLabels = useMemo(() => {
    const total = isEmpty ? 0 : data.length;
    const labels: number[] = [];
    if (total <= 8) { for (let i = 0; i < total; i++) labels.push(i); }
    else {
      const s = Math.floor(total / 7);
      for (let i = 0; i < total; i += s) labels.push(i);
      if (labels[labels.length - 1] !== total - 1) labels.push(total - 1);
    }
    if (maxPredExtra > 0) labels.push(totalN - 1);
    return labels;
  }, [data.length, totalN, maxPredExtra, isEmpty]);

  const calcMA = (period: number) =>
    data.map((_, i) => {
      if (i < period - 1) return null;
      let s = 0; for (let k = i - period + 1; k <= i; k++) s += data[k].close;
      return s / period;
    });

  const ma5 = useMemo(() => calcMA(5), [data]);
  const ma10 = useMemo(() => calcMA(10), [data]);
  const ma20 = useMemo(() => calcMA(20), [data]);

  const maPath = (arr: (number | null)[], color: string) => {
    let d = '';
    arr.forEach((v, i) => {
      if (v == null) return;
      d += d === '' ? `M${fx(i).toFixed(1)},${py(v).toFixed(1)}` : ` L${fx(i).toFixed(1)},${py(v).toFixed(1)}`;
    });
    return <path d={d} stroke={color} strokeWidth="1.2" fill="none" />;
  };

  const dateLabel = (i: number) => {
    if (i < data.length) {
      const ds = (data[i]?.tradeDate || data[i]?.date || '');
      return ds.length >= 10 ? ds.slice(5, 10) : `T-${data.length - i}`;
    }
    return `+${i - data.length + 1}`;
  };

  const handleMouseMove = useCallback((e: React.MouseEvent<SVGSVGElement>) => {
    if (isEmpty) return;
    const svg = svgRef.current; if (!svg) return;
    const rect = svg.getBoundingClientRect();
    const scaleX = W / rect.width;
    const mx = (e.clientX - rect.left) * scaleX;
    const idx = Math.round((mx - padL - step / 2) / step);
    if (idx >= 0 && idx < data.length) {
      setHoverIdx(idx);
      setTooltipPos({ x: e.clientX - rect.left, y: e.clientY - rect.top });
    } else { setHoverIdx(null); setTooltipPos(null); }
  }, [data.length, step, isEmpty]);

  const handleMouseLeave = () => { setHoverIdx(null); setTooltipPos(null); };

  const hoverData = hoverIdx != null ? data[hoverIdx] : null;
  const hoverX = hoverIdx != null ? fx(hoverIdx) : 0;

  // Early return AFTER all hooks
  if (isEmpty) {
    return <div className="muted" style={{ textAlign: 'center', padding: 60 }}>暂无K线数据，请先触发数据采集</div>;
  }

  return (
    <div style={{ position: 'relative' }}>
      <svg ref={svgRef} viewBox={`0 0 ${W} ${H}`} width="100%" style={{ display: 'block', fontFamily: 'monospace', cursor: 'crosshair' }}
        onMouseMove={handleMouseMove} onMouseLeave={handleMouseLeave}>

        {/* Prediction bg */}
        {splitIdx != null && splitIdx < data.length && (
          <>
            <rect x={fx(splitIdx) - step / 2} y={padT} width={fx(totalN - 1) - fx(splitIdx) + step} height={priceH} fill="#F7F8FA" opacity="0.5" />
            <line x1={fx(splitIdx) - step / 2} x2={fx(splitIdx) - step / 2} y1={padT} y2={padT + priceH} stroke="#C9CDD4" strokeDasharray="4 3" strokeWidth="1" />
            <text x={fx(splitIdx)} y={padT + 11} fontSize="9" fill="#86909C">←历史 预测→</text>
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
        {data.map((d, i) => {
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
          return <polyline key={`pl${li}`} points={pts.join(' ')} stroke={line.color} strokeWidth="2"
            strokeDasharray={line.dashed ? '5 3' : 'none'} fill="none" opacity="0.85" />;
        })}

        {/* Candles */}
        {data.map((d, i) => {
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
          if (m.i < 0 || m.i >= data.length) return null;
          const x = fx(m.i);
          if (m.type === 'buy') {
            const yTop = py(data[m.i].high) - 16;
            return <g key={`mk${k}`}><polygon points={`${x},${yTop + 10} ${x - 6},${yTop} ${x + 6},${yTop}`} fill="#165DFF" /><text x={x} y={yTop - 4} fontSize="9" fill="#165DFF" textAnchor="middle" fontWeight="700">{m.label || 'B'}</text></g>;
          }
          const yBot = py(data[m.i].low) + 15;
          return <g key={`mk${k}`}><circle cx={x} cy={yBot} r="4" fill="#F53F3F" stroke="#fff" strokeWidth="1.2" /><text x={x} y={yBot + 12} fontSize="8" fill="#F53F3F" textAnchor="middle">{m.label || '上榜'}</text></g>;
        })}

        {/* Crosshair */}
        {hoverIdx != null && (
          <>
            <line x1={hoverX} x2={hoverX} y1={padT} y2={padT + priceH} stroke="#1D2129" strokeWidth="0.8" strokeDasharray="2 4" opacity="0.5" />
            {hoverData && <line x1={padL} x2={W - padR} y1={py(hoverData.close)} y2={py(hoverData.close)} stroke="#86909C" strokeWidth="0.6" strokeDasharray="2 3" opacity="0.35" />}
          </>
        )}

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
      {hoverData && hoverIdx != null && tooltipPos && (
        <div style={{
          position: 'absolute', left: tooltipPos.x + 16, top: Math.min(tooltipPos.y - 80, height - 160),
          background: 'rgba(29, 33, 41, 0.92)', color: '#fff', padding: '10px 14px',
          borderRadius: 6, fontSize: 12, lineHeight: '20px', pointerEvents: 'none', zIndex: 100,
          fontFamily: 'monospace', whiteSpace: 'nowrap', boxShadow: '0 2px 12px rgba(0,0,0,0.15)',
        }}>
          <div style={{ fontWeight: 600, marginBottom: 4, color: '#C9CDD4' }}>{(hoverData.tradeDate || hoverData.date || '').slice(0, 10)}</div>
          <div>开 <span style={{ color: '#fff', fontWeight: 500 }}>{hoverData.open?.toFixed(2)}</span></div>
          <div>高 <span style={{ color: UP, fontWeight: 500 }}>{hoverData.high?.toFixed(2)}</span></div>
          <div>低 <span style={{ color: DOWN, fontWeight: 500 }}>{hoverData.low?.toFixed(2)}</span></div>
          <div>收 <span style={{ color: hoverData.close >= hoverData.open ? UP : DOWN, fontWeight: 600, fontSize: 13 }}>{hoverData.close?.toFixed(2)}</span></div>
          <div style={{ marginTop: 4, color: '#C9CDD4' }}>量 {(hoverData.volume || 0) >= 1e8 ? ((hoverData.volume || 0) / 1e8).toFixed(2) + '亿' : ((hoverData.volume || 0) / 1e4).toFixed(0) + '万手'}</div>
          {hoverData.close !== undefined && hoverIdx > 0 && (
            <div style={{ color: hoverData.close >= data[hoverIdx - 1].close ? UP : DOWN, fontWeight: 600 }}>
              涨跌 {((hoverData.close - data[hoverIdx - 1].close) >= 0 ? '+' : '')}{(hoverData.close - data[hoverIdx - 1].close).toFixed(2)}
              {' '}({data[hoverIdx - 1].close ? (((hoverData.close - data[hoverIdx - 1].close) / data[hoverIdx - 1].close) * 100).toFixed(2) : '0.00'}%)
            </div>
          )}
        </div>
      )}
    </div>
  );
}
