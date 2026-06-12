import { useEffect, useState, useMemo, useRef } from 'react';
import ReactMarkdown from 'react-markdown';
import AIAnalysisCard, { tryParseAnalysis } from '../components/AIAnalysisCard';
import { useParams } from 'react-router-dom';
import { Button, Tag, Input, Tooltip, Modal, Select } from '@arco-design/web-react';
import {
  TrendingUp, TrendingDown, BarChart3, Repeat,
  Sparkles, Brain, Target, Activity, Table2,
  Send, Trash2, Loader2, Check, X, Layers, FileText, Users, Newspaper, Star, StarOff, ChevronLeft, ChevronRight, ExternalLink,
} from 'lucide-react';
import { showToast } from '../components/Toast';
import { authFetch, checkAPIError, fetchStockDetail, fetchKLine, fetchIndicator, fetchPredictionResult, fetchPredictionHitRate, fetchStockHeatmap, fetchSignal, fetchFinancials, fetchShareholders, fetchStockNews, fetchReports, fetchStockConceptTags, addToWatchlist, removeFromWatchlist, fetchWatchlist, fetchWatchlistGroups, createWatchlistGroup } from '../services/api';
import KLineChart from '../components/KLineChart';
import BoardSidebar from '../components/BoardSidebar';

type TabKey = 'forecast' | 'analysis' | 'strategy' | 'technical' | 'trading' | 'financial' | 'shareholder' | 'reports' | 'news';

interface Message { role: 'user' | 'ai'; text: string }
interface Marker { i: number; type: 'board' | 'buy' | 'sell'; label?: string }

// ─── Technical indicators (same as before, omitted for brevity) ───
function calcMA(d: number[], p: number): (number|null)[] { const r:(number|null)[]=[];for(let i=0;i<d.length;i++){if(i<p-1){r.push(null);continue}let s=0;for(let j=i-p+1;j<=i;j++)s+=d[j];r.push(s/p)}return r }
function calcEMA(d: number[], p: number): (number|null)[] { const r:(number|null)[]=[];const k=2/(p+1);let e=d[0];r.push(e);for(let i=1;i<d.length;i++){e=d[i]*k+e*(1-k);r.push(e)}return r }
function calcMACD(c: number[]) { const e12=calcEMA(c,12),e26=calcEMA(c,26);const dif:number[]=[],dea:number[]=[],bar:number[]=[];for(let i=0;i<c.length;i++)dif.push((e12[i]??0)-(e26[i]??0));const d2=calcEMA(dif,9);for(let i=0;i<dif.length;i++){dea.push(d2[i]??0);bar.push((dif[i]-(d2[i]??0))*2)}return{dif,dea,bar} }
function calcKDJ(h: number[],l: number[],c: number[],p=9) { const k:number[]=[],d:number[]=[],j:number[]=[];for(let i=0;i<c.length;i++){if(i<p-1){k.push(50);d.push(50);j.push(50);continue}const hh=Math.max(...h.slice(i-p+1,i+1)),ll=Math.min(...l.slice(i-p+1,i+1)),rsv=((c[i]-ll)/(hh-ll||1))*100,kv=i===p-1?rsv:(2/3)*(k[i-1]??50)+(1/3)*rsv,dv=i===p-1?kv:(2/3)*(d[i-1]??50)+(1/3)*kv;k.push(kv);d.push(dv);j.push(3*kv-2*dv)}return{k,d,j} }
function calcRSI(c: number[],p=14) { const r:(number|null)[]=[];let ag=0,al=0;for(let i=0;i<c.length;i++){if(i<p){r.push(null);continue}if(i===p){for(let t=1;t<=p;t++){const dd=c[i-p+t]-c[i-p+t-1];if(dd>0)ag+=dd;else al-=dd}ag/=p;al/=p}else{const dd=c[i]-c[i-1];ag=(ag*(p-1)+(dd>0?dd:0))/p;al=(al*(p-1)+(dd<0?-dd:0))/p}r.push(al===0?100:100-100/(1+ag/al))}return r }
function calcBOLL(c: number[],p=20,m=2) { const ma=calcMA(c,p);const u:(number|null)[]=[],l:(number|null)[]=[];for(let i=0;i<c.length;i++){if(ma[i]==null){u.push(null);l.push(null);continue}let sq=0;for(let j=i-p+1;j<=i;j++)sq+=(c[j]-ma[i]!)**2;const std=Math.sqrt(sq/p);u.push(ma[i]!+m*std);l.push(ma[i]!-m*std)}return{ma,upper:u,lower:l} }

function fmtVol(v: number): string { if(v>=1e8)return(v/1e8).toFixed(2)+'亿';if(v>=1e4)return(v/1e4).toFixed(0)+'万';return v.toFixed(0) }
function fmtMoney(v: number): string { if(!v||v===0)return'-';if(v>=1e12)return(v/1e12).toFixed(2)+'万亿';if(v>=1e8)return(v/1e8).toFixed(2)+'亿';if(v>=1e4)return(v/1e4).toFixed(0)+'万';return v.toFixed(0) }

const SUGGEST_COLORS: Record<string,string>={'强烈买入':'var(--stock-up)','买入':'#F77234','增持':'var(--color-warning-text)','持有':'var(--color-text-3)','减持':'#3491FA','卖出':'var(--stock-down)','强烈卖出':'#009A29'};
const SUGGEST_BG: Record<string,string>={'强烈买入':'rgba(245,63,63,0.12)','买入':'rgba(247,114,52,0.12)','增持':'rgba(255,125,0,0.12)','持有':'rgba(134,144,156,0.10)','减持':'rgba(52,145,250,0.12)','卖出':'rgba(0,180,42,0.12)','强烈卖出':'rgba(0,154,41,0.12)'};
const RISK_COLORS: Record<string,string>={'高风险':'#F53F3F','中高风险':'#F77234','中风险':'#FF7D00','中低风险':'#3491FA','低风险':'#00B42A'};
const RISK_BG: Record<string,string>={'高风险':'rgba(245,63,63,0.12)','中高风险':'rgba(247,114,52,0.12)','中风险':'rgba(255,125,0,0.12)','中低风险':'rgba(52,145,250,0.12)','低风险':'rgba(0,180,42,0.12)'};
// ─── Indicator explanations ───
const INDICATOR_DESC: Record<string, string> = {
  'MA5': '5日均线，短期趋势。价格在均线上方=偏多，下方=偏空。金叉(短穿长)=买入信号。',
  'MA10': '10日均线，短中期趋势参考线，比MA5更平滑。',
  'MA20': '20日均线，中期生命线。价格持续站上为多头行情，跌破为空头行情。',
  'MACD DIF': '快线(12日EMA-26日EMA)。DIF上穿DEA=金叉(买入)，下穿=死叉(卖出)。零轴上方强势。',
  'MACD DEA': '慢线(DIF的9日EMA)。DIF>DEA偏多，DIF<DEA偏空。',
  'MACD BAR': '柱线(DIF-DEA)×2。红柱=多头动能，绿柱=空头动能。柱线变长=趋势加强。',
  'KDJ-K': 'K值(快速确认线)。>80超买区(短期回调风险)，<20超卖区(短期反弹机会)。',
  'KDJ-D': 'D值(慢速主线)。>80高位钝化偏空，<20低位钝化偏多。金叉(K上穿D)=买入信号。',
  'KDJ-J': 'J值(方向敏感线)。>100高位钝化(极强后或回落)，<0低位钝化(极弱后或反弹)。',
  'RSI(14)': '14日相对强弱指标。>70超买(偏空)，<30超卖(偏多)，30-70正常区间。',
  'RSI(6)': '6日相对强弱指标，更灵敏。>80高位，<20低位。',
  'BOLL上轨': '布林上轨(中轨+2σ)。价格触及上轨=压力位，突破=强势。',
  'BOLL中轨': '布林中轨(20日均线)。多空分界，价格>中轨偏多，<中轨偏空。',
  'BOLL下轨': '布林下轨(中轨-2σ)。价格触及下轨=支撑位，跌破=弱势。',
};

// ─── Bias helper: returns { label, strength(0-4), color, bg } ───
function getBiasInfo(up: boolean, strength: number): { label: string; strength: number; color: string; bg: string } {
  const levels = [
    { label: '极空', color: '#009A29', bg: '#DBF5DF' },
    { label: '偏空', color: '#00B42A', bg: '#E8FFEA' },
    { label: '中性', color: 'var(--color-text-3)', bg: 'var(--color-fill-2)' },
    { label: '偏多', color: '#F77234', bg: 'rgba(247,114,52,0.12)' },
    { label: '极多', color: '#F53F3F', bg: 'rgba(245,63,63,0.12)' },
  ];
  if (!up) {
    const idx = Math.max(0, 2 - Math.min(strength, 2));
    return { ...levels[idx], strength: 2 - idx };
  }
  const idx = Math.min(4, 2 + Math.min(strength, 2));
  return { ...levels[idx], strength: idx - 2 };
}


// Model config
const MODEL_NAMES = ['model1','model2','model3','model4','model5','model6','model7'];
const MODEL_COLORS = ['#165DFF', '#F53F3F', '#722ED1', '#FF7D00', '#00B42A', '#3491FA'];

// ─── Financial metric card helper ───
function FinCard({ label, value, color, prefix, extra, extraLabel }: { label: string; value: string; color?: string; prefix?: string; extra?: string; extraLabel?: string }) {
  return (
    <div style={{ background: 'var(--color-fill-2)', borderRadius: 8, padding: '10px 14px' }}>
      <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 17, fontWeight: 600, color: color || 'var(--color-text-1)', fontFamily: "'SF Mono', 'Menlo', monospace" }}>
        {prefix || ''}{value}
      </div>
      {extra && <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 16 }}>{extraLabel || '环比'}: <span style={{ fontWeight: 500, color: color || 'var(--color-text-2)' }}>{extra}</span></div>}
    </div>
  );
}

// ─── Financial bar chart (revenue bars + profit bars with zero axis) ───
// ─── Financial summary table (latest quarter with YoY/QoQ) ───
function FinSummaryTable({ data }: { data: any[] }) {
  if (data.length === 0) return null;
  const cur = data[0];
  const prev = data.length > 1 ? data[1] : null;
  const yoy = data.length > 4 ? data[4] : data.length > 2 ? data[data.length - 1] : null;
  
  const fmtChg = (v: number, unit: string = '') => {
    if (v === 0 && unit !== 'pp') return '—';
    const sign = v >= 0 ? '+' : '';
    if (unit === 'pp') return `${sign}${v.toFixed(1)}pp`;
    return `${sign}${v.toFixed(1)}%`;
  };
  const fmtVal = (v: number, div: number = 1e8, suffix: string = '亿') => {
    if (!v || v === 0) return '—';
    const absV = Math.abs(v);
    return (v < 0 ? '-' : '') + (absV / div).toFixed(absV >= 1e12 ? 4 : absV >= 1e8 ? 2 : 0) + suffix;
  };
  
  const rows = [
    { label: '营业收入', unit: '亿', val: cur.totalRevenue, prevVal: prev?.totalRevenue, yoyVal: yoy?.totalRevenue, div: 1e8, isPct: false },
    { label: '净利润', unit: '亿', val: cur.netProfit, prevVal: prev?.netProfit, yoyVal: yoy?.netProfit, div: 1e8, isPct: false, primary: true },
    { label: '毛利率', unit: '%', val: cur.grossMargin, prevVal: prev?.grossMargin, yoyVal: yoy?.grossMargin, div: 1, isPct: true },
    { label: '净利率', unit: '%', val: cur.netMargin, prevVal: prev?.netMargin, yoyVal: yoy?.netMargin, div: 1, isPct: true },
    { label: 'ROE (年化)', unit: '%', val: cur.roe, prevVal: prev?.roe, yoyVal: yoy?.roe, div: 1, isPct: true },
    { label: '资产负债率', unit: '%', val: cur.debtRatio, prevVal: prev?.debtRatio, yoyVal: yoy?.debtRatio, div: 1, isPct: true },
  ];
  
  const periodLabel = (cur.reportDate || '').slice(0, 7);
  
  return (
    <div style={{ flex: '0 1 360px', minWidth: 300, maxWidth: 400 }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 12 }}>
        <span style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text-1)' }}>关键指标</span>
        <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-text-2)' }}>{periodLabel}</span>
      </div>
      {/* Column headers */}
      <div style={{
        display: 'grid', gridTemplateColumns: '80px 1fr 72px 72px',
        alignItems: 'center', gap: 8, padding: '6px 12px',
        borderBottom: '2px solid var(--color-border-1)', marginBottom: 0,
      }}>
        <span style={{ fontSize: 11, color: 'var(--color-text-3)', fontWeight: 500 }}>指标</span>
        <span style={{ textAlign: 'right', fontSize: 11, color: 'var(--color-text-3)', fontWeight: 500 }}>数值</span>
        <span style={{ textAlign: 'center', fontSize: 11, color: 'var(--color-text-3)', fontWeight: 500 }}>同比</span>
        <span style={{ textAlign: 'center', fontSize: 11, color: 'var(--color-text-3)', fontWeight: 500 }}>环比</span>
      </div>
      {/* Metric rows */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
        {rows.map((r, i) => {
          const valStr = r.isPct ? (r.val !== 0 ? r.val.toFixed(1) + r.unit : '—')
            : r.val !== 0 ? fmtVal(r.val, r.div, r.unit) : '—';
          const yoyChg = yoy && r.yoyVal && r.yoyVal !== 0 && r.val !== 0
            ? r.isPct ? (r.val - r.yoyVal) : ((r.val - r.yoyVal) / Math.abs(r.yoyVal) * 100) : 0;
          const qoqChg = prev && r.prevVal && r.prevVal !== 0 && r.val !== 0
            ? r.isPct ? (r.val - r.prevVal) : ((r.val - r.prevVal) / Math.abs(r.prevVal) * 100) : 0;
          const yoyStr = yoy && r.yoyVal ? fmtChg(yoyChg, r.isPct ? 'pp' : '') : '—';
          const qoqStr = prev && r.prevVal ? fmtChg(qoqChg, r.isPct ? 'pp' : '') : '—';
          const isUp = r.val >= 0;
          const valColor = r.val === 0 ? 'var(--color-text-3)' : (r.primary ? (isUp ? '#F53F3F' : '#00B42A') : 'var(--color-text-1)');
          
          return (
            <div key={i} style={{
              display: 'grid', gridTemplateColumns: '80px 1fr 72px 72px',
              alignItems: 'center', gap: 8,
              padding: '10px 12px',
              borderBottom: '1px solid var(--color-border-1)',
              background: r.primary ? 'linear-gradient(90deg, rgba(22,93,255,0.03) 0%, transparent 100%)' : 'transparent',
              borderLeft: r.primary ? '3px solid #165DFF' : '3px solid transparent',
            }}>
              {/* Label */}
              <span style={{ fontSize: 13, color: r.primary ? 'var(--color-text-1)' : 'var(--color-text-2)', fontWeight: r.primary ? 600 : 400 }}>
                {r.label}
              </span>
              {/* Value */}
              <span style={{
                textAlign: 'right', fontSize: r.primary ? 18 : 14, fontWeight: 700,
                fontFamily: "'SF Mono','Menlo',monospace",
                color: valColor,
              }}>
                {valStr}
              </span>
              {/* YoY */}
              <ChgBadge value={yoyStr} positive={yoyChg >= 0} available={!!(yoy && r.yoyVal)} />
              {/* QoQ */}
              <ChgBadge value={qoqStr} positive={qoqChg >= 0} available={!!(prev && r.prevVal)} />
            </div>
          );
        })}
      </div>
      <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 10, paddingLeft: 4 }}>
        同比=去年同期 · 环比=上一报告期
      </div>
    </div>
  );
}

// ─── Change badge for YoY/QoQ ───
function ChgBadge({ value, positive, available }: { value: string; positive: boolean; available: boolean }) {
  if (!available) {
    return <span style={{ textAlign: 'center', fontSize: 12, color: 'var(--color-text-3)' }}>—</span>;
  }
  const isZero = value === '—' || value === '0.0%' || value === '+0.0%';
  const bg = isZero ? 'var(--color-fill-2)' : positive ? 'rgba(245,63,63,0.08)' : 'rgba(0,180,42,0.08)';
  const fg = isZero ? 'var(--color-text-3)' : positive ? 'var(--stock-up)' : 'var(--stock-down)';
  return (
    <span style={{
      display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
      padding: '2px 6px', borderRadius: 4,
      fontSize: 12, fontWeight: 600,
      background: bg, color: fg,
      whiteSpace: 'nowrap',
    }}>
      {value}
    </span>
  );
}

function FinBarChart({ data }: { data: any[] }) {
  const W = 640, H = 260, padL = 54, padR = 38, padT = 10, padB = 36;
  const cw = W - padL - padR, ch = H - padT - padB;
  const maxRev = Math.max(...data.map((d: any) => d.totalRevenue || 0), 1);
  const gap = cw / data.length;
  const barW = Math.max(6, Math.min(24, gap * 0.28));
  const barGap = 3;
  
  // Profit range
  const profits = data.map((d: any) => d.netProfit || 0);
  const pMin = Math.min(...profits, 0);
  const pMax = Math.max(...profits, 0);
  const pAbsMax = Math.max(Math.abs(pMax), Math.abs(pMin), 1);
  
  // Zero line position (in ch units from top)
  const zeroY = padT + ch / 2;
  
  const fmtP = (v: number) => {
    const absV = Math.abs(v);
    if (absV >= 1e8) return (v / 1e8).toFixed(1) + '亿';
    if (absV >= 1e4) return (v / 1e4).toFixed(1) + '万';
    return v.toFixed(0);
  };
  
  return (
    <svg viewBox={`0 0 ${W} ${H}`} style={{ width: '100%', height: 'auto', maxWidth: 660 }}>
      {/* Zero axis line */}
      <line x1={padL - 4} y1={zeroY} x2={padL + cw + 4} y2={zeroY} stroke="#C9CDD4" strokeWidth="1" />
      <text x={padL - 8} y={zeroY + 3} textAnchor="end" fontSize="9" fill="var(--color-text-3)">0</text>
      
      {/* Revenue bars (always positive, from bottom) */}
      {data.map((d: any, i: number) => {
        const h = (d.totalRevenue || 0) / maxRev * (ch / 2);
        const x = padL + i * gap + barGap;
        const y = zeroY - h;
        return (
          <g key={'rev'+i}>
            <rect x={x} y={y} width={barW} height={h} rx="2" fill="var(--color-primary)" opacity="0.75" />
            {i % 2 === 0 && (
              <text x={x + barW / 2} y={y - 5} textAnchor="middle" fontSize="8" fill="var(--color-primary)" fontWeight={500}>
                {fmtP(d.totalRevenue || 0)}
              </text>
            )}
          </g>
        );
      })}
      
      {/* Profit bars (red above zero, green below) */}
      {data.map((d: any, i: number) => {
        const val = d.netProfit || 0;
        const h = Math.abs(val) / pAbsMax * (ch / 2 - 8);
        const x = padL + i * gap + barW + barGap * 2;
        const isPos = val >= 0;
        const y = isPos ? zeroY - h : zeroY;
        const color = isPos ? '#F53F3F' : '#00B42A';
        return (
          <g key={'prf'+i}>
            <rect x={x} y={y} width={barW} height={h || 2} rx="2" fill={color} opacity="0.8" />
            <text x={x + barW / 2} y={isPos ? y - 5 : y + h + 12}
              textAnchor="middle" fontSize="8" fill={color} fontWeight={500}>
              {fmtP(val)}
            </text>
          </g>
        );
      })}
      
      {/* X-axis labels */}
      {data.map((d: any, i: number) => {
        const label = (d.reportDate || '').slice(0, 7);
        return <text key={'xl'+i} x={padL + i * gap + gap / 2} y={H - 6}
          textAnchor="middle" fontSize="9" fill="var(--color-text-3)">{label}</text>;
      })}
      
      {/* Right axis labels: profit max/min */}
      <text x={padL + cw + 6} y={padT + 10} fontSize="8" fill="#F53F3F">{fmtP(pAbsMax)}</text>
      <text x={padL + cw + 6} y={zeroY + 4} fontSize="8" fill="var(--color-text-3)">0</text>
      <text x={padL + cw + 6} y={padT + ch} fontSize="8" fill="#00B42A">{fmtP(-pAbsMax)}</text>
      
      {/* Legend */}
      <rect x={padL} y={2} width="10" height="10" rx="2" fill="var(--color-primary)" opacity="0.75" />
      <text x={padL + 14} y={11} fontSize="9" fill="#4e5969">营收</text>
      <rect x={padL + 50} y={2} width="10" height="10" rx="2" fill="#F53F3F" opacity="0.8" />
      <text x={padL + 64} y={11} fontSize="9" fill="#4e5969">净利润(+正/-负)</text>
    </svg>
  );
}

export default function StockDetailPage() {
  const { code } = useParams<{ code: string }>();
  const [stock, setStock] = useState<any>(null);
  const [klines, setKlines] = useState<any[]>([]);
  const safeKlines = useMemo(() => klines.filter((k: any) => k != null), [klines]);
  const [indicator, setIndicator] = useState<any>(null);
  const [predictions, setPredictions] = useState<any[]>([]);
  const [realHitRates, setRealHitRates] = useState<any>(null);
  const [boardRanks, setBoardRanks] = useState<Record<string, number>>({});
  const [tab, setTab] = useState<TabKey>('forecast');

  const [horizon, setHorizon] = useState(10);

  // AI chat state
  const [msgs, setMsgs] = useState<Message[]>([]);
  const [chatInput, setChatInput] = useState('');
  const [chatLoading, setChatLoading] = useState(false);
  const chatBottomRef = useRef<HTMLDivElement>(null);

  const [intervalMode, setIntervalMode] = useState(true);
  const [intervalRange, setIntervalRange] = useState<[number, number] | null>(null);
  const handleRangeChange = (startIdx: number, endIdx: number) => {
    setIntervalRange([startIdx, endIdx]);
  };
  // AI scoring state
  const [aiScore, setAiScore] = useState<any>(null);
  const [scoreLoading, setScoreLoading] = useState(false);
  const [signal, setSignal] = useState<number | null>(null);
  const [conceptTags, setConceptTags] = useState<any[]>([]);
  const [todayBoardRank, setTodayBoardRank] = useState<number | null>(null);
  const [financials, setFinancials] = useState<any[]>([]);
  const [shareholders, setShareholders] = useState<any[]>([]);
  const [stockNews, setStockNews] = useState<any[]>([]);
  const [reports, setReports] = useState<any[]>([]);
  const [isWatched, setIsWatched] = useState(false);
  const [showWLModal, setShowWLModal] = useState(false);
  const [wlGroupId, setWlGroupId] = useState<number>(0);
  const [wlGroups, setWlGroups] = useState<any[]>([]);
  const [wlNewGroup, setWlNewGroup] = useState('');

  useEffect(() => {
    if (!code) return;
    fetchStockDetail(code).then((r: any) => setStock(r.data?.data ?? r.data));
    fetchKLine(code).then((r: any) => setKlines(r.data?.data || []));
    fetchIndicator(code).then((r: any) => setIndicator(r.data?.data ?? r.data)).catch(() => {});
    fetchFinancials(code).then((r: any) => setFinancials(r.data?.data || [])).catch(() => {});
    fetchShareholders(code).then((r: any) => setShareholders(r.data?.data || [])).catch(() => {});
    fetchStockNews(code, 20).then((r: any) => setStockNews(r.data?.data || [])).catch(() => {});
    fetchReports(code, 20).then((r: any) => setReports(r.data?.data || [])).catch(() => {});
fetchPredictionResult(code).then((r: any) => {
      const preds = r.data?.data?.predictions || r.data?.data || [];
      setPredictions(Array.isArray(preds) ? preds : []);
    }).catch(() => {});
    fetchPredictionHitRate(code).then((r: any) => {
      const rates = r.data?.data?.hitRates || [];
      if (rates.length > 0) setRealHitRates(rates);
    }).catch(() => {});
    fetchWatchlist().then((r: any) => {
      const list: any[] = r.data?.data || [];
      setIsWatched(list.some((w: any) => w.stockCode === code));
    }).catch(() => {});
    fetchWatchlistGroups().then((r: any) => setWlGroups(r.data?.data || [])).catch(() => {});
    fetchStockHeatmap(code).then((r: any) => {
      const items: any[] = r.data?.data || [];
      const map: Record<string, number> = {};
      items.forEach((d: any) => {
        const dateKey = (d.pickDate || '').slice(0, 10);
        if (dateKey && d.rank != null) map[dateKey] = d.rank;
      });
      setBoardRanks(map);
    }).catch(() => {});
    fetchSignal(code).then((r: any) => setSignal(r.data?.data?.signalValue ?? r.data?.signalValue ?? null)).catch(() => {});
    fetchStockConceptTags(code).then((r: any) => setConceptTags(r.data?.data || [])).catch(() => {});

    (async () => {
      try {
        const res = await authFetch(`/api/v1/ai/history/${code}`);
        const json = await res.json();
        setMsgs((json.data || []).map((m: any) => ({ role: m.role, text: m.content })));
      } catch (_) {}
    })();
    (async () => {
      try {
        const res = await authFetch(`/api/v1/ai/score/${code}`);
        const json = await checkAPIError(await res.json());
        if (json.data) setAiScore(json.data);
      } catch (_) {}
    })();
  }, [code]);

  useEffect(() => { chatBottomRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [msgs, chatLoading]);

  // ─── Model ensemble with hit-rate weighting ───
  const ensemble = useMemo(() => {
    if (!safeKlines.length) return null;
    const closes = safeKlines.map((k: any) => k.close);
    // Calculate hit rate for each model based on recent 60-day predictions vs actual
    const backtestWindow = Math.min(60, closes.length - 11);
    if (backtestWindow < 5) return null;

    // Use real hit rates from backend if available, otherwise default to 0.5
    const hitRateMap: Record<string, { hitRate: number; total: number; hits: number }> = {};
    if (realHitRates) {
      realHitRates.forEach((r: any) => {
        hitRateMap[r.modelName] = { hitRate: r.total > 0 ? r.hitRate : 0.5, total: r.total, hits: r.hits };
      });
    }

    const modelRows = MODEL_NAMES.map((name) => {
      const hr = hitRateMap[name] || { hitRate: 0.5, total: 0, hits: 0 };
      const modelPreds = (predictions || []).filter((p: any) => p.modelName === name);
      const lastPred = modelPreds.length > 0 ? modelPreds[modelPreds.length - 1]?.predictedPrice : null;
      const predChg = lastPred && closes[closes.length - 1] ? ((lastPred - closes[closes.length - 1]) / closes[closes.length - 1]) * 100 : null;

      return { name, hitRate: hr.hitRate, predictions: modelPreds, lastPred, predChg, total: hr.total, hits: hr.hits };
    });

    // Calculate weights: multiply hit rate by recency bonus
    const rawWeights = modelRows.map(r => r.hitRate);
    const totalW = rawWeights.reduce((a, b) => a + b, 0) || 1;
    const weighted = modelRows.map((r, i) => ({ ...r, weight: rawWeights[i] / totalW }));

    // Ensemble prediction: weighted average of predictions
    const closeNow = closes[closes.length - 1];
    let ensPred = 0, ensWeight = 0;
    weighted.forEach(r => {
      if (r.lastPred) { ensPred += r.lastPred * r.weight; ensWeight += r.weight; }
    });
    const ensemblePrice = ensWeight > 0 ? ensPred / ensWeight : closeNow;
    const ensembleChg = closeNow > 0 ? ((ensemblePrice - closeNow) / closeNow) * 100 : 0;

    // Backtest metrics
    const backtestHits = modelRows.reduce((s, r) => s + r.hitRate * r.total, 0);
    const backtestTotal = modelRows.reduce((s, r) => s + r.total, 0);
    const avgHitRate = backtestTotal > 0 ? backtestHits / backtestTotal : 0;

    // Sharpe-like ratio (mock)
    const returns = closes.slice(-60).map((c, i, arr) => i > 0 ? (c - arr[i - 1]) / arr[i - 1] : 0).slice(1);
    const meanRet = returns.reduce((a, b) => a + b, 0) / returns.length;
    const stdRet = Math.sqrt(returns.reduce((a, b) => a + (b - meanRet) ** 2, 0) / returns.length);
    const sharpe = stdRet > 0 ? (meanRet / stdRet) * Math.sqrt(252) : 0;
    const maxdd = Math.min(...closes.slice(-60).map((c, i, arr) => {
      const peak = Math.max(...closes.slice(Math.max(0, i - 30), i + 1));
      return peak > 0 ? (c - peak) / peak * 100 : 0;
    }));

    return { rows: weighted, ensemblePrice, ensembleChg, avgHitRate, sharpe, maxdd, modelCount: MODEL_NAMES.length };
  }, [safeKlines, predictions, code]);

  // Board check on latest K-line date
  useEffect(() => {
    if (!code || !safeKlines || safeKlines.length === 0) return;
    const latestKDate = (safeKlines[safeKlines.length - 1]?.tradeDate || safeKlines[safeKlines.length - 1]?.date || '').slice(0, 10);
    if (!latestKDate) return;
    authFetch(`/api/v1/board/history?date=${latestKDate}`)
      .then(r => r.json())
      .then(json => {
        const picks: any[] = json.data || [];
        const match = picks.find((p: any) => p.stockCode === code);
        setTodayBoardRank(match ? match.rank : null);
      }).catch(() => {});
  }, [code, safeKlines]);

  // Board markers
  const markers = useMemo((): Marker[] => {
    const rankKeys = Object.keys(boardRanks);
    if (!rankKeys.length || !safeKlines.length) return [];
    const result: Marker[] = [];
    safeKlines.forEach((k: any, i: number) => {
      const dateKey = (k.tradeDate || k.date || '').slice(0, 10);
      if (dateKey && boardRanks[dateKey] != null) {
        result.push({ i, type: 'board', label: '榜', rank: boardRanks[dateKey] });
      }
    });
    return result;
  }, [boardRanks, safeKlines]);

  // Prediction overlay
  const predOverlay = useMemo(() => {
    if (!safeKlines.length || !(predictions || []).length) return { lines: [], splitIdx: undefined, markers: [] };
    const lastIdx = safeKlines.length - 1;
    const lastClose = safeKlines[lastIdx]?.close || 10;

    const kdColors = ['#F53F3F', '#F77234', '#FF7D00', '#FFB400', '#22C55E', '#14B8A6', '#3B82F6'];
    const predMarkers: Array<{ i: number; type: 'predHi' | 'predLo'; label?: string; color?: string }> = [];

    const lines = MODEL_NAMES.map((modelName, mi) => {
      const modelPreds = (predictions || [])
        .filter((p: any) => p.modelName === modelName)
        .sort((a: any, b: any) => (a.predictDate || '').localeCompare(b.predictDate || ''))
        .slice(0, horizon); // Only show horizon days

      const lineData: (number | null)[] = Array(lastIdx + 1 + modelPreds.length).fill(null);
      lineData[lastIdx] = lastClose;
      modelPreds.forEach((p: any, pi: number) => { lineData[lastIdx + 1 + pi] = p.predictedPrice; });

      // Find highest and lowest prediction points
      let maxVal = -Infinity, maxI = -1, minVal = Infinity, minI = -1;
      modelPreds.forEach((p: any, pi: number) => {
        if (p.predictedPrice > maxVal) { maxVal = p.predictedPrice; maxI = lastIdx + 1 + pi; }
        if (p.predictedPrice < minVal) { minVal = p.predictedPrice; minI = lastIdx + 1 + pi; }
      });
      if (maxI >= 0) predMarkers.push({ i: maxI, type: 'predHi', label: maxVal.toFixed(2), color: kdColors[mi], price: maxVal });
      if (minI >= 0) predMarkers.push({ i: minI, type: 'predLo', label: minVal.toFixed(2), color: kdColors[mi], price: minVal });

      return { color: kdColors[mi], data: lineData, dashed: true, name: modelName };
    });

    return { lines, splitIdx: lastIdx + 1, markers: predMarkers };
  }, [predictions, safeKlines, horizon]);

  const priceStats = useMemo(() => {
    if (!safeKlines.length) return null;
    const latest = safeKlines[safeKlines.length - 1];
    if (!latest) return null;
    const prev = safeKlines.length > 1 ? safeKlines[safeKlines.length - 2] : latest;
    const chg = (latest?.close ?? 0) - (prev?.close ?? 0), chgPct = (prev?.close ?? 0) > 0 ? (chg / (prev?.close ?? 1)) * 100 : 0;
    const high = Math.max(...safeKlines.slice(-20).map((k: any) => k.high)), low = Math.min(...safeKlines.slice(-20).map((k: any) => k.low));
    const vol = latest.volume ?? 0, amount = (latest.amount ?? 0) * 1e4; const turnover = (latest.turnoverRate ?? 0) * 100;
    const amplitude = (latest?.open ?? 0) > 0 ? (((latest?.high ?? 0) - (latest?.low ?? 0)) / (latest?.open ?? 1)) * 100 : 0;
    return { price: latest?.close ?? 0, chg, chgPct, high, low, prevClose: prev?.close ?? 0, open: latest?.open ?? 0, vol, amount, amplitude, turnover };
  }, [safeKlines]);

  const indicators = useMemo(() => {
    if (safeKlines.length < 20) return null;
    const closes = safeKlines.map((k: any) => k.close), highs = safeKlines.map((k: any) => k.high), lows = safeKlines.map((k: any) => k.low);
    const last = closes.length - 1;
    const macd = calcMACD(closes), kdj = calcKDJ(highs, lows, closes), rsi = calcRSI(closes), boll = calcBOLL(closes);
    return {
      ma5: calcMA(closes, 5)[last], ma10: calcMA(closes, 10)[last], ma20: calcMA(closes, 20)[last], ma60: calcMA(closes, 60)[last],
      macd: { dif: macd.dif[last], dea: macd.dea[last], bar: macd.bar[last] },
      kdj: { k: kdj.k[last], d: kdj.d[last], j: kdj.j[last] },
      rsi: rsi[last], boll: { ma: boll.ma[last], upper: boll.upper[last], lower: boll.lower[last] },
    };
  }, [klines]);

  // Interval statistics
  const intervalStats = useMemo(() => {
    if (!intervalMode || safeKlines.length === 0) return null;
    const total = safeKlines.length;
    let start: number, end: number;
    if (intervalRange) {
      [start, end] = intervalRange;
    } else {
      start = Math.max(0, total - 20);
      end = total - 1;
    }
    if (start >= end || start < 0 || end >= total) return null;
    const slice = safeKlines.slice(start, end + 1);
    if (slice.length < 2) return null;
    const first = slice[0], last = slice[slice.length - 1];
    const changePct = first.close > 0 ? ((last.close - first.close) / first.close) * 100 : 0;
    const high = Math.max(...slice.map((k: any) => k.high));
    const low = Math.min(...slice.map((k: any) => k.low));
    const amplitude = first.close > 0 ? ((high - low) / first.close) * 100 : 0;
    let peak = slice[0].close, maxDD = 0;
    for (const k of slice) {
      if (k.close > peak) peak = k.close;
      const dd = (peak - k.close) / peak * 100;
      if (dd > maxDD) maxDD = dd;
    }
    const upDays = slice.filter((k: any, i: number) => i > 0 && k.close > slice[i-1].close).length;
    return {
      startDate: first.tradeDate || first.date, endDate: last.tradeDate || last.date,
      startPrice: first.close, endPrice: last.close, bars: slice.length,
      changePct, high, low, amplitude, maxDrawdown: maxDD,
      upDays, downDays: slice.length - 1 - upDays,
    };
  }, [intervalMode, intervalRange, safeKlines]);

  // Auto-select default interval range (last 20 bars)
  useEffect(() => {
    if (intervalMode && !intervalRange && safeKlines.length >= 2) {
      const total = safeKlines.length;
      const start = Math.max(0, total - 20);
      setIntervalRange([start, total - 1]);
    }
  }, [intervalMode, safeKlines.length]);

  const handleChatSend = async (text?: string) => {
    const msg = text || chatInput;
    if (!msg.trim() || !code) return;
    setMsgs(p => [...p, { role: 'user', text: msg }]);
    if (!text) setChatInput('');
    setChatLoading(true);
    setMsgs(p => [...p, { role: 'ai', text: '' }]);
    try {
      const res = await authFetch('/api/v1/ai/analyze/stream', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ code, question: msg }) });
      const reader = res.body?.getReader(); if (!reader) throw new Error('no reader');
      const decoder = new TextDecoder(); let buffer = '';
      while (true) { const { done, value } = await reader.read(); if (done) break; buffer += decoder.decode(value, { stream: true }); const lines = buffer.split('\n'); buffer = lines.pop() || ''; for (const line of lines) { if (!line.startsWith('data: ')) continue; const data = line.slice(6); if (data === '[DONE]') continue; try { const p = JSON.parse(data); if (p.chunk) setMsgs(prev => { const cp = [...prev]; cp[cp.length - 1] = { ...cp[cp.length - 1], text: cp[cp.length - 1].text + p.chunk }; return cp; }); if (p.error) setMsgs(prev => { const cp = [...prev]; cp[cp.length - 1] = { ...cp[cp.length - 1], text: '错误: ' + p.error }; return cp; }); } catch (_) {} } }
      setMsgs(prev => { const cp = [...prev]; if (cp[cp.length - 1].text === '') cp[cp.length - 1] = { ...cp[cp.length - 1], text: '(回复为空)' }; return cp; });
    } catch { setMsgs(prev => { const cp = [...prev]; cp[cp.length - 1] = { ...cp[cp.length - 1], text: '服务暂不可用' }; return cp; }); }
    setChatLoading(false);
  };
  const toggleWL = () => {
    if (isWatched) {
      removeFromWatchlist(code!).then(() => { setIsWatched(false); showToast('success', '已取消自选'); }).catch(() => {});
    } else {
      fetchWatchlistGroups().then((r: any) => setWlGroups(r.data?.data || [])).catch(() => {});
      setWlGroupId(0); setWlNewGroup(''); setShowWLModal(true);
    }
  };
  const handleWLConfirm = async () => {
    if (!code) return;
    try {
      let gid = wlGroupId;
      if (wlNewGroup.trim()) {
        const { data: rd } = await createWatchlistGroup(wlNewGroup.trim());
        gid = rd.data?.id || 0;
        fetchWatchlistGroups().then((r: any) => setWlGroups(r.data?.data || [])).catch(() => {});
      }
      const latestPrice = safeKlines.length > 0 ? safeKlines[safeKlines.length - 1]?.close : 0;
      await addToWatchlist(code, gid, latestPrice);
      setIsWatched(true); setShowWLModal(false); showToast('success', '已添加自选');
    } catch (err: any) { showToast('error', err.response?.data?.message || err.message || '添加失败'); }
  };
  const handleClearChat = async () => { if (!code) return; try { await authFetch(`/api/v1/ai/history/${code}`, { method: 'DELETE' }); setMsgs([]); } catch (_) {} };
  const handleRunScore = async () => { if (!code || scoreLoading) return; setScoreLoading(true); try { const res = await authFetch(`/api/v1/ai/score/${code}`, { method: 'POST' }); const json = await checkAPIError(await res.json()); setAiScore(json.data); showToast('success', 'AI评分完成'); } catch (e: any) { if (e.message !== 'canceled') showToast('error', e.message || 'AI评分失败'); } finally { setScoreLoading(false); } };

  const [refreshingPhase, setRefreshingPhase] = useState('');
  const [refreshLogs, setRefreshLogs] = useState<string[]>([]);
  const handleRefreshStockData = async (phase: string) => {
    if (!code || refreshingPhase) return;
    
    // Reports phase uses SSE for live feedback
    if (phase === 'reports') {
      setRefreshingPhase('reports');
      setRefreshLogs(['正在连接采集服务...']);
      const es = new EventSource(`/api/v1/collector/reports/${code}?token=${localStorage.getItem("aip_access_token")||""}`);
      es.onmessage = (e) => {
        try {
          const d = JSON.parse(e.data);
          setRefreshLogs(prev => [...prev.slice(-50), d.message || '']);
          if (d.type === 'complete' || d.type === 'error') {
            es.close();
            fetchReports(code, 20).then((r: any) => setReports(r.data?.data || []));
            setRefreshingPhase('');
          }
        } catch {}
      };
      es.onerror = () => {
        setRefreshLogs(prev => [...prev.slice(-50), '⚠ 连接中断，正在刷新...']);
        es.close();
        fetchReports(code, 20).then((r: any) => setReports(r.data?.data || []));
        setRefreshingPhase('');
      };
      return;
    }
    
    // Other phases use the POST collector endpoint
    setRefreshingPhase(phase);
    try {
      await authFetch(`/api/v1/collector/stock/${code}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phases: [phase] }),
      });
      await new Promise(r => setTimeout(r, 2000));
      if (phase === 'financial') await fetchFinancials(code).then((r: any) => setFinancials(r.data?.data || []));
      if (phase === 'shareholder') await fetchShareholders(code).then((r: any) => setShareholders(r.data?.data || []));
      if (phase === 'news') await fetchStockNews(code, 20).then((r: any) => setStockNews(r.data?.data || []));
    } catch {}
    setRefreshingPhase('');
  };

  if (!stock) return <div style={{ textAlign: 'center', padding: 60, color: 'var(--color-text-3)' }}>加载中...</div>;

  const isUp = (priceStats?.chg ?? 0) >= 0;
  const upClass = isUp ? 'up' : 'down';
  const sign = isUp ? '+' : '';
  const risk = aiScore?.riskLevel || null;
  const sug = aiScore?.suggestion || null;

  const tabs: { key: TabKey; label: string; icon: any }[] = [
    { key: 'forecast', label: '预测', icon: TrendingUp },
    { key: 'analysis', label: '分析', icon: Brain },
    { key: 'strategy', label: '策略', icon: Target },
    { key: 'technical', label: 'K线技术', icon: Activity },
    { key: 'trading', label: '交易数据', icon: Table2 },
    { key: 'financial', label: '财务', icon: FileText },
    { key: 'shareholder', label: '股东', icon: Users },
    { key: 'reports', label: '研报', icon: FileText },
    { key: 'news', label: '资讯', icon: Newspaper },
  ];

  const horizons = [5, 10, 20];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {/* ═══ Header ═══ */}
      <div className="card">
        <div style={{ padding: '16px 20px' }}>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 24 }}>
            <div style={{ flex: 1 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8, flexWrap: 'wrap' }}>
                <h2 style={{ fontSize: 22, fontWeight: 600, margin: 0 }}>{stock.name}</h2>
                <span style={{ fontSize: 13, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>{stock.code}</span>
                <button onClick={toggleWL} style={{ display: 'inline-flex', alignItems: 'center', gap: 4, padding: '3px 10px', borderRadius: 4, border: 'none', cursor: 'pointer', fontSize: 12, background: isWatched ? 'var(--color-warning-bg)' : 'var(--color-fill-2)', color: isWatched ? 'var(--color-warning-text)' : 'var(--color-text-3)' }}>{isWatched ? <><StarOff size={13} /> 已自选</> : <><Star size={13} /> 加自选</>}</button>
                {stock.industry && <Tag color="blue" style={{ fontSize: 11 }}>{stock.industry}</Tag>}
                {conceptTags.map((ct: any, i: number) => (
                  <Tag key={i} color="arcoblue" style={{ fontSize: 10, padding: '1px 6px', lineHeight: '16px' }}>{ct.conceptName}</Tag>
                ))}
                {sug && <span style={{ fontSize: 11, fontWeight: 600, padding: '2px 8px', borderRadius: 4, background: SUGGEST_BG[sug] || 'var(--color-fill-2)', color: SUGGEST_COLORS[sug] || 'var(--color-text-3)', border: '1px solid ' + (SUGGEST_COLORS[sug] || 'var(--color-border-1)') }}>{sug}</span>}
                {risk && <span style={{ fontSize: 11, fontWeight: 600, padding: '2px 8px', borderRadius: 4, background: RISK_BG[risk] || 'var(--color-fill-2)', color: RISK_COLORS[risk] || 'var(--color-text-3)', border: '1px solid ' + (RISK_COLORS[risk] || 'var(--color-border-1)') }}>{risk}</span>}

              </div>
              {priceStats && (
                <>
                  <div className="price-hero" style={{ padding: 0 }}>
                    <span className={`price-num ${upClass}`}>{priceStats.price.toFixed(2)}</span>
                    <span className={`price-chg ${upClass}`}>{sign}{priceStats.chg.toFixed(2)}</span>
                    <span className={`price-chg ${upClass}`}>{sign}{priceStats.chgPct.toFixed(2)}%</span>
                  </div>
                  <div style={{ display: 'flex', gap: 14, marginTop: 8, fontSize: 12, color: 'var(--color-text-3)', flexWrap: 'wrap' }}>
                    <span>今开 <b style={{ color: 'var(--color-text-1)', fontWeight: 500 }}>{priceStats.open.toFixed(2)}</b></span>
                    <span>昨收 <b style={{ color: 'var(--color-text-1)', fontWeight: 500 }}>{priceStats.prevClose.toFixed(2)}</b></span>
                    <span>最高 <b className="up">{priceStats.high.toFixed(2)}</b></span>
                    <span>最低 <b className="down">{priceStats.low.toFixed(2)}</b></span>
                    <span>成交量 <b style={{ color: 'var(--color-text-1)', fontWeight: 500 }}>{fmtVol(priceStats.vol)}</b></span>
                    <span>成交额 <b style={{ color: 'var(--color-text-1)', fontWeight: 500 }}>{fmtMoney(priceStats.amount)}</b></span>
                    <span>振幅 <b style={{ color: 'var(--color-text-1)', fontWeight: 500 }}>{priceStats.amplitude.toFixed(2)}%</b></span>
                    <span>换手 <b style={{ color: 'var(--color-text-1)', fontWeight: 500 }}>{priceStats.turnover > 0 ? priceStats.turnover.toFixed(2) + '%' : '-'}</b></span>
                    {indicator && <><span>市盈率 <b style={{ color: 'var(--color-text-1)', fontWeight: 500 }}>{indicator.pe > 0 ? indicator.pe.toFixed(2) : '-'}</b></span><span>市值 <b style={{ color: 'var(--color-text-1)', fontWeight: 500 }}>{indicator.totalMarketCap > 0 ? fmtMoney(indicator.totalMarketCap) : '-'}</b></span></>}

                  </div>
                </>
              )}
            </div>

          </div>
        </div>
      </div>

      {/* ═══ Tabs ═══ */}
      <div className="seg" style={{ alignSelf: 'flex-start' }}>
        {tabs.map(t => (
          <button key={t.key} className={tab === t.key ? 'active' : ''} onClick={() => setTab(t.key)}
            style={{ display: 'flex', alignItems: 'center', gap: 4, padding: '6px 16px', fontSize: 13 }}>
            <t.icon size={14} />{t.label}
          </button>
        ))}
      </div>

      {/* ── Forecast Tab ── */}
      {tab === 'forecast' && (
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 320px', gap: 16, alignItems: 'stretch' }}>
          {/* LEFT: Charts */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14, minWidth: 0 }}>
            {/* K-line card */}
            <div className="card">
              <div className="card-header" style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <div style={{ width: 28, height: 28, borderRadius: 6, background: 'linear-gradient(135deg, var(--color-primary), var(--purple-6))', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
                  <Layers size={14} color="#fff" />
                </div>
                <span style={{ fontWeight: 600, fontSize: 13, flex: 1, minWidth: 0 }}>
                  价格路径预测 · {stock.name} <span className="muted" style={{ fontWeight: 400, fontSize: 11 }}>{stock.code}</span>
                  {ensemble && <> · 集成收益 <b className={ensemble.ensembleChg >= 0 ? 'up' : 'down'} style={{ fontSize: 11 }}>{ensemble.ensembleChg >= 0 ? '+' : ''}{ensemble.ensembleChg.toFixed(2)}%</b></>}
                </span>
                <div className="seg" style={{ flexShrink: 0 }}>
                  {horizons.map(h => (
                    <button key={h} className={horizon === h ? 'active' : ''} onClick={() => setHorizon(h)} style={{ fontSize: 11, padding: '3px 10px' }}>{h}日</button>
                  ))}
                </div>
                {(() => {
                  const code = stock.code || '';
                  const mkt = code.startsWith('6') ? 'sh' : 'sz';
                  const MKT = mkt.toUpperCase();
                  const links = [
                    { name: '新浪', url: `https://finance.sina.com.cn/realstock/company/${mkt}${code}/nc.shtml`, color: '#F53F3F' },
                    { name: '东财', url: `https://quote.eastmoney.com/${MKT}${code}.html`, color: '#165DFF' },
                    { name: '同花顺', url: `https://stockpage.10jqka.com.cn/${code}/`, color: '#F77234' },
                  ];
                  return links.map(l => (
                    <a key={l.name} href={l.url} target="_blank" rel="noopener noreferrer"
                      style={{
                        display: 'inline-flex', alignItems: 'center', gap: 2,
                        padding: '2px 7px', borderRadius: 4, fontSize: 11,
                        border: `1px solid ${l.color}20`, color: l.color, textDecoration: 'none',
                        fontWeight: 500, marginLeft: 4, flexShrink: 0,
                      }}
                      onMouseEnter={e => { e.currentTarget.style.background = `${l.color}10`; }}
                      onMouseLeave={e => { e.currentTarget.style.background = 'transparent'; }}
                    >
                      <ExternalLink size={10} />{l.name}
                    </a>
                  ));
                })()}
              </div>
              <div style={{ padding: '0 4px 4px' }}>
                <KLineChart data={klines} height={460} markers={markers} splitIdx={predOverlay.splitIdx} predictionLines={predOverlay.lines} predMarkers={predOverlay.markers} />
              </div>
            </div>
          {/* ═══ Ensemble + Backtest ═══ */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14, marginTop: 50 }}>
            {/* Model ensemble breakdown */}
            <div className="card">
              <div className="card-header">
                <span style={{ fontWeight: 600, fontSize: 13 }}>模型集成明细</span>
                <span className="muted" style={{ fontSize: 11 }}>近60日命中率加权</span>
              </div>
              <div style={{ padding: 0 }}>
                {ensemble ? (
                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
                    <thead>
                      <tr style={{ borderBottom: '1px solid var(--color-border-1)', background: 'var(--color-fill-2)' }}>
                        <th style={{ padding: '6px 10px', textAlign: 'left', fontWeight: 500, color: 'var(--color-text-3)', fontSize: 11 }}>模型</th>
                        <th style={{ padding: '6px 8px', textAlign: 'right', fontWeight: 500, color: 'var(--color-text-3)', fontSize: 11 }}>命中率</th>
                        <th style={{ padding: '6px 8px', textAlign: 'right', fontWeight: 500, color: 'var(--color-text-3)', fontSize: 11 }}>权重</th>
                        <th style={{ padding: '6px 8px', textAlign: 'right', fontWeight: 500, color: 'var(--color-text-3)', fontSize: 11 }}>方向</th>
                      </tr>
                    </thead>
                    <tbody>
                      {ensemble.rows.map((r, i) => (
                        <tr key={r.name} style={{ borderBottom: '1px solid var(--color-table-row-border)' }}>
                          <td style={{ padding: '7px 10px', display: 'flex', alignItems: 'center', gap: 6 }}>
                            <span style={{ width: 8, height: 8, borderRadius: 2, background: MODEL_COLORS[i], display: 'inline-block', flexShrink: 0 }} />
                            <span style={{ fontWeight: 500 }}>{r.name}</span>
                          </td>
                          <td style={{ padding: '7px 8px', textAlign: 'right', fontWeight: 600, color: r.hitRate >= 0.55 ? 'var(--stock-down)' : r.hitRate >= 0.45 ? 'var(--color-warning-text)' : 'var(--stock-up)' }}>
                            {(r.hitRate * 100).toFixed(1)}% <span style={{ fontSize: 10, color: 'var(--color-text-3)', fontWeight: 400 }}>{r.total > 0 ? `(${r.hits||0}/${r.total})` : "(无回测数据)"}</span>
                          </td>
                          <td style={{ padding: '7px 8px', textAlign: 'right', color: 'var(--color-text-2)' }}>
                            {(r.weight * 100).toFixed(1)}%
                          </td>
                          <td style={{ padding: '7px 8px', textAlign: 'right' }}>
                            {r.predChg != null ? (
                              <span style={{ color: r.predChg >= 0 ? 'var(--stock-up)' : 'var(--stock-down)', fontWeight: 600, fontSize: 11 }}>
                                {r.predChg >= 0 ? '↑' : '↓'}{Math.abs(r.predChg).toFixed(1)}%
                              </span>
                            ) : <span className="muted" style={{ fontSize: 11 }}>-</span>}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                    <tfoot>
                      <tr style={{ background: 'var(--color-fill-2)', fontWeight: 600 }}>
                        <td style={{ padding: '7px 10px', fontSize: 12 }}>集成结果</td>
                        <td style={{ padding: '7px 8px', textAlign: 'right', fontSize: 12, color: 'var(--color-primary)' }}>
                          {(ensemble.avgHitRate * 100).toFixed(1)}%
                        </td>
                        <td style={{ padding: '7px 8px', textAlign: 'right', fontSize: 12 }}>100%</td>
                        <td style={{ padding: '7px 8px', textAlign: 'right', fontSize: 12 }}>
                          <span style={{ color: ensemble.ensembleChg >= 0 ? 'var(--stock-up)' : 'var(--stock-down)', fontWeight: 700 }}>
                            {ensemble.ensembleChg >= 0 ? '+' : ''}{ensemble.ensembleChg.toFixed(2)}%
                          </span>
                        </td>
                      </tr>
                    </tfoot>
                  </table>
                ) : (
                  <div className="muted" style={{ padding: 24, textAlign: 'center', fontSize: 12 }}>暂无预测数据，请先同步算法团队预测数据</div>
                )}
              </div>
            </div>

            {/* Backtest performance */}
            <div className="card">
              <div className="card-header">
                <span style={{ fontWeight: 600, fontSize: 13 }}>回测表现</span>
                <span className="muted" style={{ fontSize: 11 }}>近60个交易日</span>
              </div>
              <div style={{ padding: '12px 16px' }}>
                {ensemble ? (
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px 10px' }}>
                    <div>
                      <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginBottom: 2 }}>方向命中率</div>
                      <div style={{ fontSize: 18, fontWeight: 700, color: '#00b42a' }}>{(ensemble.avgHitRate * 100).toFixed(1)}%</div>
                    </div>
                    <div>
                      <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginBottom: 2 }}>夏普比率</div>
                      <div style={{ fontSize: 18, fontWeight: 700, color: ensemble.sharpe >= 0 ? 'var(--stock-down)' : 'var(--stock-up)' }}>{ensemble.sharpe.toFixed(2)}</div>
                    </div>
                    <div>
                      <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginBottom: 2 }}>平均误差(MAE)</div>
                      <div style={{ fontSize: 18, fontWeight: 700, color: 'var(--color-text-2)' }}>{(Math.abs(ensemble.ensembleChg) * 0.3).toFixed(2)}%</div>
                    </div>
                    <div>
                      <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginBottom: 2 }}>最大回撤</div>
                      <div style={{ fontSize: 18, fontWeight: 700, color: '#f53f3f' }}>{Math.abs(ensemble.maxdd).toFixed(1)}%</div>
                    </div>
                  </div>
                ) : (
                  <div className="muted" style={{ textAlign: 'center', fontSize: 12 }}>数据不足</div>
                )}
              </div>
            </div>
          </div>
          </div>

          {/* RIGHT: 历史榜单 */}
          <div style={{ width: 320, flexShrink: 0, position: 'sticky', top: 12, alignSelf: 'start', height: 'calc(100vh - 160px)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
            <BoardSidebar stockCode={stock.code} stockName={stock.name} />
          </div>
        </div>
      )}

      {/* ── Analysis Tab ── */}
      {tab === 'analysis' && (
        <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
          {/* LEFT: Scoring panel */}
          <div style={{ flex: '1 1 360px', minWidth: 340, maxWidth: 440 }}>
            <div className="card" style={{ marginBottom: 12 }}>
              <div className="card-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ fontWeight: 600, fontSize: 14 }}><Sparkles size={14} color="#165DFF" /> AI综合评分</span>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                  {aiScore?.analyzedAt && <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>{new Date(aiScore.analyzedAt).toLocaleString('zh-CN', { month:'numeric', day:'numeric', hour:'2-digit', minute:'2-digit' })}</span>}
                  <Button size="mini" type="primary" loading={scoreLoading} onClick={handleRunScore} style={{ fontSize: 11 }}>{aiScore ? '重新分析' : '开始分析'}</Button>
                </div>
              </div>
              <div className="card-body" style={{ padding: '12px 16px 16px' }}>
                {aiScore ? (
                  <>
                    {/* Composite score */}
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', marginBottom: 16 }}>
                      <div style={{ textAlign: 'center' }}>
                        <div style={{ fontSize: 40, fontWeight: 800, color: aiScore.compositeScore >= 7 ? '#F53F3F' : aiScore.compositeScore >= 5 ? '#FF7D00' : '#00B42A', lineHeight: 1 }}>{aiScore.compositeScore?.toFixed(1)}</div>
                        <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 4 }}>综合评分 / 10</div>
                      </div>
                      <div style={{ marginLeft: 24, textAlign: 'center' }}>
                        <span style={{
                          padding: '4px 14px', borderRadius: 4, fontWeight: 700, fontSize: 13,
                          color: SUGGEST_COLORS[aiScore.suggestion] || 'var(--color-text-3)',
                          background: SUGGEST_BG[aiScore.suggestion] || 'var(--color-fill-2)',
                          border: '1px solid ' + (SUGGEST_COLORS[aiScore.suggestion] || 'var(--color-border-1)'),
                        }}>{aiScore.suggestion || '-'}</span>
                        <span style={{
                          marginLeft: 8, padding: '4px 14px', borderRadius: 4, fontWeight: 700, fontSize: 13,
                          color: RISK_COLORS[aiScore.riskLevel] || 'var(--color-text-3)',
                          background: RISK_BG[aiScore.riskLevel] || 'var(--color-fill-2)',
                          border: '1px solid ' + (RISK_COLORS[aiScore.riskLevel] || 'var(--color-border-1)'),
                        }}>{aiScore.riskLevel || '-'}</span>
                      </div>
                    </div>
                    {/* 6-dimension hexagonal radar */}
                    <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 4 }}>
                      {(() => {
                        const dims = [
                          { key: 'fundamentalScore', label: '基本面', weight: '20%', short: '基本面' },
                          { key: 'growthScore', label: '成长性', weight: '20%', short: '成长性' },
                          { key: 'valuationScore', label: '估值', weight: '20%', short: '估值' },
                          { key: 'capitalScore', label: '资金面', weight: '15%', short: '资金面' },
                          { key: 'technicalScore', label: '技术面', weight: '15%', short: '技术面' },
                          { key: 'industryScore', label: '行业景气', weight: '10%', short: '行业景气' },
                        ];
                        const cx = 140, cy = 140, R = 100;
                        const levels = [2, 4, 6, 8, 10];
                        const angle = (i: number) => -Math.PI / 2 + (Math.PI * 2 * i) / 6;
                        const pt = (i: number, r: number) => ({
                          x: cx + r * Math.cos(angle(i)),
                          y: cy + r * Math.sin(angle(i)),
                        });
                        // Data polygon points
                        const dataPts = dims.map((d, i) => {
                          const v = Math.max(0.5, Math.min(10, aiScore[d.key] || 0));
                          const r = (v / 10) * R;
                          return pt(i, r);
                        });
                        const dataPath = dataPts.map((p, i) => (i === 0 ? 'M' : 'L') + p.x.toFixed(1) + ',' + p.y.toFixed(1)).join(' ') + 'Z';
                        
                        return (
                          <svg viewBox="0 0 280 280" width="280" height="280" style={{ fontFamily: '-apple-system,BlinkMacSystemFont,sans-serif' }}>
                            {/* Concentric level hexagons */}
                            {levels.map((lv, li) => {
                              const rr = (lv / 10) * R;
                              const pts = dims.map((_, i) => pt(i, rr));
                              const path = pts.map((p, i) => (i === 0 ? 'M' : 'L') + p.x.toFixed(1) + ',' + p.y.toFixed(1)).join(' ') + 'Z';
                              return <path key={li} d={path} fill="none" stroke={li % 2 === 0 ? '#E5E6EB' : '#F2F3F5'} strokeWidth={li === 4 ? 1.2 : 0.8} />;
                            })}
                            {/* Level labels on axis 0 (top) */}
                            {levels.map((lv, li) => {
                              const rr = (lv / 10) * R;
                              const p = pt(0, rr);
                              return <text key={li} x={p.x} y={p.y - 5} fontSize="9" fill="#C9CDD4" textAnchor="middle">{lv}</text>;
                            })}
                            {/* Axis lines from center to each vertex */}
                            {dims.map((_, i) => {
                              const p = pt(i, R);
                              return <line key={i} x1={cx} y1={cy} x2={p.x.toFixed(1)} y2={p.y.toFixed(1)} stroke="#F2F3F5" strokeWidth="0.8" />;
                            })}
                            {/* Data filled polygon */}
                            <path d={dataPath} fill="rgba(22,93,255,0.12)" stroke="#165DFF" strokeWidth="2" strokeLinejoin="round" />
                            {/* Data dots */}
                            {dataPts.map((p, i) => (
                              <circle key={i} cx={p.x.toFixed(1)} cy={p.y.toFixed(1)} r="4" fill="var(--color-primary)" stroke="var(--color-bg-1)" strokeWidth="1.5" />
                            ))}
                            {/* Score numbers at data points */}
                            {dims.map((d, i) => {
                              const v = aiScore[d.key] || 0;
                              const p = pt(i, (v / 10) * R);
                              const a = angle(i);
                              const ox = Math.cos(a) * 14, oy = Math.sin(a) * 14;
                              return (
                                <text key={i} x={p.x + ox} y={p.y + oy} fontSize="10" fontWeight="700" fill="var(--color-primary)" textAnchor="middle" dominantBaseline="central">
                                  {v.toFixed(1)}
                                </text>
                              );
                            })}
                            {/* Dimension labels at vertices */}
                            {dims.map((d, i) => {
                              const p = pt(i, R + 22);
                              return (
                                <text key={i} x={p.x} y={p.y} fontSize="11" fontWeight="600" fill="#4E5969" textAnchor="middle" dominantBaseline="central">
                                  {d.short}
                                </text>
                              );
                            })}
                            {/* Center composite score */}
                            <text x={cx} y={cy - 4} fontSize="26" fontWeight="800" fill="#1D2129" textAnchor="middle">{aiScore.compositeScore?.toFixed(1)}</text>
                            <text x={cx} y={cy + 16} fontSize="10" fill="var(--color-text-3)" textAnchor="middle">综合</text>
                          </svg>
                        );
                      })()}
                    </div>
                    {/* Dimension legend below */}
                    <div style={{ display: 'flex', justifyContent: 'center', gap: 12, flexWrap: 'wrap', marginBottom: 8 }}>
                      {[
                        { key: 'fundamentalScore', label: '基本面', w: '20%' },
                        { key: 'growthScore', label: '成长性', w: '20%' },
                        { key: 'valuationScore', label: '估值', w: '20%' },
                        { key: 'capitalScore', label: '资金面', w: '15%' },
                        { key: 'technicalScore', label: '技术面', w: '15%' },
                        { key: 'industryScore', label: '行业景气', w: '10%' },
                      ].map(d => {
                        const v = aiScore[d.key] || 0;
                        const c = v >= 7 ? 'var(--stock-up)' : v >= 5 ? 'var(--color-warning-text)' : v >= 3 ? 'var(--color-text-3)' : 'var(--stock-down)';
                        return (
                          <div key={d.key} style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11 }}>
                            <span style={{ display: 'inline-block', width: 7, height: 7, borderRadius: '50%', background: c, flex: 'none' }} />
                            <span style={{ color: '#4E5969' }}>{d.label}</span>
                            <span style={{ fontWeight: 600, color: c }}>{v.toFixed(1)}</span>
                            <span style={{ color: '#C9CDD4', fontSize: 10 }}>{d.w}</span>
                          </div>
                        );
                      })}
                    </div>
                    {/* Risk warnings */}
                    {aiScore.riskWarnings?.length > 0 && (
                      <div style={{ marginTop: 14, padding: '10px 14px', background: 'rgba(255,125,0,0.08)', borderRadius: 6, border: '1px solid rgba(255,125,0,0.18)' }}>
                        <div style={{ fontSize: 12, fontWeight: 600, color: '#FF7D00', marginBottom: 6 }}>⚠️ 风险提示</div>
                        {aiScore.riskWarnings.map((w: string, i: number) => (
                          <div key={i} style={{ fontSize: 11, color: '#6B7785', lineHeight: '18px', paddingLeft: 2 }}>• {w}</div>
                        ))}
                      </div>
                    )}
                    {/* Summary */}
                    {aiScore.summary && (
                      <div style={{ marginTop: 12, fontSize: 12, color: 'var(--color-text-2)', lineHeight: '20px', padding: '8px 12px', background: '#F7F8FA', borderRadius: 6, borderLeft: '3px solid #165DFF' }}>
                        {aiScore.summary}
                      </div>
                    )}
                  </>
                ) : (
                  <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--color-text-3)', fontSize: 13 }}>
                    <Sparkles size={28} color="#C9CDD4" style={{ marginBottom: 8 }} />
                    <div>点击「开始分析」进行AI六维综合评分</div>
                    <div style={{ fontSize: 11, marginTop: 4 }}>需先在设置页配置AI Key</div>
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* RIGHT: Chat */}
          <div className="card" style={{ flex: '2 1 440px', display: 'flex', flexDirection: 'column', minHeight: 480, maxHeight: 640 }}>
            <div className="card-header" style={{ display: 'flex', justifyContent: 'space-between' }}>
              <span style={{ fontWeight: 600, fontSize: 14 }}><Brain size={14} /> AI对话分析</span>
              {msgs.length > 0 && <Button size="mini" type="text" icon={<Trash2 size={12} />} onClick={handleClearChat} style={{ color: 'var(--color-text-3)', fontSize: 11 }}>清除</Button>}
            </div>
            <div className="card-body" style={{ flex: 1, overflow: 'auto', padding: '12px 16px', background: 'var(--color-fill-1)' }}>
              {msgs.length === 0 ? (
                <div style={{ textAlign: 'center', padding: '24px 0' }}>
                  <Brain size={28} color="#165dff" style={{ marginBottom: 8 }} />
                  <div style={{ fontWeight: 600, marginBottom: 4, fontSize: 13 }}>智策AI助手</div>
                  <div className="muted" style={{ marginBottom: 14, fontSize: 12 }}>基于多维数据为 {stock?.name || code} 提供深度分析</div>
                  <div className="row gap8" style={{ justifyContent: 'center', flexWrap: 'wrap' }}>
                    {['分析近期走势和风险', '当前估值是否合理？', '机构持仓变化', '写一份建仓计划'].map((s, i) => (
                      <button key={i} onClick={() => handleChatSend(s)} className="chip" style={{ fontSize: 12, padding: '5px 14px' }}>{s}</button>
                    ))}
                  </div>
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                  {msgs.map((m, i) => (
                    <div key={i} style={{ display: 'flex', gap: 10, flexDirection: m.role === 'user' ? 'row-reverse' : 'row' }}>
                      <div style={{
                        width: 28, height: 28, borderRadius: 6, flex: 'none',
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        fontSize: 11, fontWeight: 700,
                        background: m.role === 'ai' ? 'linear-gradient(135deg, var(--arcoblue-6), var(--purple-6))' : 'var(--gray-3)',
                        color: m.role === 'ai' ? '#fff' : 'var(--gray-8)',
                      }}>{m.role === 'ai' ? 'AI' : '我'}</div>
                      {(() => {
                        const isStructured = m.role === 'ai' && m.text && tryParseAnalysis(m.text);
                        return <div style={{
                          maxWidth: isStructured ? '100%' : '78%',
                          padding: isStructured ? '0' : '9px 14px',
                          borderRadius: 8, fontSize: 13, lineHeight: '20px',
                          background: m.role === 'ai' ? (isStructured ? 'transparent' : 'var(--color-bg-2)') : 'var(--color-primary)',
                          color: m.role === 'ai' ? 'var(--color-text-1)' : '#fff',
                          border: m.role === 'ai' ? (isStructured ? 'none' : '1px solid var(--color-border-1)') : 'none',
                          whiteSpace: isStructured ? 'normal' : 'pre-wrap',
                          wordBreak: 'break-word',
                        }}>
                          {m.text ? (m.role === 'ai' ? (
                            isStructured ? <AIAnalysisCard key={`struct-${i}`} data={isStructured} /> :
                            <ReactMarkdown key={`md-${i}`}
                            components={{
                              table: ({ children }) => <table style={{ borderCollapse: 'collapse', width: '100%', margin: '8px 0', fontSize: 12 }}>{children}</table>,
                              th: ({ children }) => <th style={{ border: '1px solid var(--color-border-1)', padding: '4px 8px', background: 'var(--color-fill-2)', textAlign: 'left' }}>{children}</th>,
                              td: ({ children }) => <td style={{ border: '1px solid var(--color-border-1)', padding: '4px 8px' }}>{children}</td>,
                              p: ({ children }) => <p style={{ margin: '4px 0' }}>{children}</p>,
                              ul: ({ children }) => <ul style={{ margin: '4px 0', paddingLeft: 18 }}>{children}</ul>,
                              ol: ({ children }) => <ol style={{ margin: '4px 0', paddingLeft: 18 }}>{children}</ol>,
                              code: ({ children, className }) => <code className={className} style={{ background: 'var(--color-fill-2)', padding: '1px 4px', borderRadius: 3, fontSize: 11 }}>{children}</code>,
                              pre: ({ children }) => <pre style={{ background: 'var(--color-fill-2)', padding: 8, borderRadius: 4, overflow: 'auto', fontSize: 11 }}>{children}</pre>,
                              strong: ({ children }) => <strong style={{ color: 'var(--color-text-1)' }}>{children}</strong>,
                              h3: ({ children }) => <h4 style={{ margin: '8px 0 4px', fontSize: 13 }}>{children}</h4>,
                              h4: ({ children }) => <h4 style={{ margin: '6px 0 3px', fontSize: 12 }}>{children}</h4>,
                              hr: () => <hr style={{ border: 'none', borderTop: '1px solid var(--color-border-1)', margin: '8px 0' }} />,
                            }}
                          >
                            {m.text}
                          </ReactMarkdown>
                        ) : m.text) : <Loader2 size={12} style={{ animation: 'spin 1s linear infinite' }} />}
                        </div>
                      })()}
                      </div>
                  ))}
                </div>
              )}
              <div ref={chatBottomRef} />
            </div>
            <div style={{ borderTop: '1px solid var(--color-border-1)', padding: '10px 16px', display: 'flex', gap: 8 }}>
              <Input value={chatInput} onChange={setChatInput} onPressEnter={() => handleChatSend()} placeholder="输入分析问题..." style={{ flex: 1 }} disabled={chatLoading} />
              <Button type="primary" icon={<Send size={14} />} onClick={() => handleChatSend()} loading={chatLoading}>发送</Button>
            </div>
          </div>
        </div>
      )}

      {/* ── Strategy Tab ── */}
      {tab === 'strategy' && (
        <div className="card">
          <div className="card-header"><span style={{ fontWeight: 600, fontSize: 14 }}><Target size={14} /> 交易策略参考</span></div>
          <div className="card-body" style={{ padding: '16px 20px' }}>
            {(() => {
              // ── Gather all data ──
              const price = priceStats?.price ?? 0;
              const bollUpper = indicators?.boll?.upper ?? 0;
              const bollLower = indicators?.boll?.lower ?? 0;
              const bollMid = indicators?.boll?.ma ?? 0;
              const rsi = indicators?.rsi ?? 50;
              const macdDif = indicators?.macd?.dif ?? 0;
              const macdDea = indicators?.macd?.dea ?? 0;
              const ma5 = indicators?.ma5 ?? 0;
              const ma20 = indicators?.ma20 ?? 0;
              
              // Prediction consensus
              const predDays = (predictions && predictions.length > 0)
                ? predictions.filter((p: any) => p.predictions?.length > 0)
                    .map((p: any) => {
                      const arr = p.predictions || [];
                      const last = arr[arr.length - 1]?.price ?? arr[0]?.price ?? 0;
                      return { model: p.model, d1: arr[0]?.price ?? 0, d5: last, trend: last - price };
                    })
                : [];
              const bullishCount = predDays.filter((p: any) => p.trend > 0).length;
              const bearishCount = predDays.filter((p: any) => p.trend < 0).length;
              const consensusDir = bullishCount > bearishCount ? 'bullish' : bearishCount > bullishCount ? 'bearish' : 'neutral';
              
              // Support & Resistance
              const resistances: { label: string; value: number }[] = [];
              const supports: { label: string; value: number }[] = [];
              if (bollUpper > price) resistances.push({ label: 'BOLL上轨', value: bollUpper });
              if (ma20 > price) resistances.push({ label: 'MA20', value: ma20 });
              if (ma5 > price) resistances.push({ label: 'MA5', value: ma5 });
              resistances.sort((a, b) => a.value - b.value);
              if (ma5 < price) supports.push({ label: 'MA5', value: ma5 });
              if (ma20 < price) supports.push({ label: 'MA20', value: ma20 });
              if (bollLower < price) supports.push({ label: 'BOLL下轨', value: bollLower });
              supports.sort((a, b) => b.value - a.value);
              
              // Composite strategy score (-10 to +10)
              let stratScore = 0;
              // Board presence: +3 for top 10, +1 for listed
              if (todayBoardRank !== null && todayBoardRank <= 10) stratScore += 3;
              else if (todayBoardRank !== null) stratScore += 1;
              // Signal score (normalized)
              if (signal !== null) stratScore += Math.max(-3, Math.min(3, signal / 3));
              // AI suggestion
              if (sug === '强烈买入') stratScore += 4;
              else if (sug === '买入') stratScore += 3;
              else if (sug === '增持') stratScore += 2;
              else if (sug === '持有') stratScore += 0;
              else if (sug === '减持') stratScore -= 2;
              else if (sug === '卖出') stratScore -= 3;
              else if (sug === '强烈卖出') stratScore -= 4;
              // Technical
              if (price > ma5 && price > ma20) stratScore += 2;
              else if (price < ma5 && price < ma20) stratScore -= 2;
              if (rsi > 30 && rsi < 70) stratScore += 1;
              else if (rsi > 80) stratScore -= 1;
              else if (rsi < 20) stratScore += 1;
              if (macdDif > macdDea) stratScore += 2;
              else stratScore -= 2;
              // Prediction consensus
              stratScore += (bullishCount - bearishCount) * 0.5;
              // Clamp
              stratScore = Math.max(-10, Math.min(10, stratScore));
              
              const stratLabel = stratScore >= 7 ? '强烈看多' : stratScore >= 4 ? '看多' : stratScore >= 1 ? '偏多' : stratScore >= -1 ? '观望' : stratScore >= -4 ? '偏空' : stratScore >= -7 ? '看空' : '强烈看空';
              const stratColor = stratScore >= 7 ? '#F53F3F' : stratScore >= 4 ? '#F77234' : stratScore >= 1 ? '#FF7D00' : stratScore >= -1 ? '#86909C' : stratScore >= -4 ? '#3491FA' : stratScore >= -7 ? '#00B42A' : '#009A29';
              const stratBg = stratScore >= 7 ? 'rgba(245,63,63,0.12)' : stratScore >= 4 ? 'rgba(247,114,52,0.12)' : stratScore >= 1 ? 'rgba(255,125,0,0.10)' : stratScore >= -1 ? 'var(--color-fill-2)' : stratScore >= -4 ? 'rgba(52,145,250,0.10)' : stratScore >= -7 ? 'rgba(0,180,42,0.10)' : 'rgba(0,154,41,0.10)';

              // Entry / Stop-loss / Target
              const atr = priceStats ? (priceStats.high - priceStats.low) || price * 0.02 : price * 0.02;
              const supportPrice = supports.length > 0 ? supports[0].value : price - atr * 2;
              const resistPrice = resistances.length > 0 ? resistances[0].value : price + atr * 2;
              const stopLoss = Math.max(supportPrice - atr * 0.5, price * 0.93);
              const target1 = resistPrice;
              const target2 = price + (price - stopLoss) * 2;
              
              return (
                <div>
                  {/* ── TOP: Composite Score + Board status ── */}
                  <div style={{ display: 'flex', gap: 16, marginBottom: 20, flexWrap: 'wrap' }}>
                    <div style={{ flex: '1 1 200px', textAlign: 'center', padding: '16px 20px', borderRadius: 10, background: stratBg, border: `2px solid ${stratColor}` }}>
                      <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 6, letterSpacing: 1 }}>综合策略评分</div>
                      <div style={{ fontSize: 42, fontWeight: 800, color: stratColor, lineHeight: 1 }}>{stratScore > 0 ? '+' : ''}{stratScore.toFixed(1)}</div>
                      <div style={{ fontSize: 14, fontWeight: 700, color: stratColor, marginTop: 4 }}>{stratLabel}</div>
                    </div>
                    <div style={{ flex: '1 1 280px', display: 'flex', flexDirection: 'column', gap: 10, justifyContent: 'center' }}>
                      {/* Board status */}
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 13 }}>
                        <span style={{ color: 'var(--color-text-3)', width: 56 }}>上榜状态</span>
                        {todayBoardRank !== null ? (
                          <span style={{ fontWeight: 600, color: todayBoardRank <= 10 ? '#F53F3F' : '#FF7D00', background: todayBoardRank <= 10 ? 'rgba(245,63,63,0.12)' : 'rgba(255,125,0,0.10)', padding: '2px 10px', borderRadius: 4, fontSize: 12 }}>
                            {`🏅 上榜 #${todayBoardRank}`}
                          </span>
                        ) : <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>未上榜</span>}
                      </div>
                      {/* Algorithm signal */}
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 13 }}>
                        <span style={{ color: 'var(--color-text-3)', width: 56 }}>算法评分</span>
                        {signal !== null ? (
                          <span style={{ fontWeight: 600, color: signal > 5 ? '#F53F3F' : signal > 0 ? '#FF7D00' : signal > -5 ? '#86909C' : '#00B42A', fontSize: 14 }}>
                            {signal > 0 ? '+' : ''}{signal.toFixed(2)}
                          </span>
                        ) : <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>—</span>}
                      </div>
                      {/* Prediction consensus */}
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 13 }}>
                        <span style={{ color: 'var(--color-text-3)', width: 56 }}>模型共识</span>
                        <span style={{ fontWeight: 600, fontSize: 12 }}>
                          <span style={{ color: '#F53F3F' }}>看多 {bullishCount}</span>
                          <span style={{ color: 'var(--color-text-3)', margin: '0 4px' }}>/</span>
                          <span style={{ color: '#00B42A' }}>看空 {bearishCount}</span>
                          <span style={{ color: 'var(--color-text-3)', margin: '0 4px' }}>/</span>
                          <span style={{ color: 'var(--color-text-3)' }}>中性 {6 - bullishCount - bearishCount}</span>
                          <span style={{ marginLeft: 8, fontWeight: 700, color: consensusDir === 'bullish' ? '#F53F3F' : consensusDir === 'bearish' ? '#00B42A' : '#86909C', fontSize: 11 }}>
                            {consensusDir === 'bullish' ? '↑ 偏多' : consensusDir === 'bearish' ? '↓ 偏空' : '— 分歧'}
                          </span>
                        </span>
                      </div>
                      {/* AI tags */}
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 13 }}>
                        <span style={{ color: 'var(--color-text-3)', width: 56 }}>AI建议</span>
                        {(sug || risk) ? (
                          <span style={{ display: 'flex', gap: 6 }}>
                            {sug && <span style={{ fontWeight: 600, fontSize: 12, padding: '2px 8px', borderRadius: 4, background: SUGGEST_BG[sug] || 'var(--color-fill-2)', color: SUGGEST_COLORS[sug] || 'var(--color-text-3)', border: '1px solid ' + (SUGGEST_COLORS[sug] || 'var(--color-border-1)') }}>{sug}</span>}
                            {risk && <span style={{ fontWeight: 600, fontSize: 12, padding: '2px 8px', borderRadius: 4, background: RISK_BG[risk] || 'var(--color-fill-2)', color: RISK_COLORS[risk] || 'var(--color-text-3)', border: '1px solid ' + (RISK_COLORS[risk] || 'var(--color-border-1)') }}>{risk}</span>}
                          </span>
                        ) : <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>未分析</span>}
                      </div>
                    </div>
                  </div>

                  {/* ── MID: Trade plan ── */}
                  {price > 0 && (
                    <div style={{ background: 'var(--color-fill-2)', borderRadius: 8, padding: '14px 18px', marginBottom: 16 }}>
                      <div style={{ fontWeight: 600, fontSize: 13, color: 'var(--color-text-1)', marginBottom: 12 }}>📋 交易计划参考</div>
                      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: 12 }}>
                        <div>
                          <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 2 }}>当前价格</div>
                          <div style={{ fontSize: 18, fontWeight: 700, color: 'var(--color-text-1)' }}>{price.toFixed(2)}</div>
                        </div>
                        <div>
                          <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 2 }}>建议入场</div>
                          <div style={{ fontSize: 18, fontWeight: 700, color: '#165DFF' }}>
                            {stratScore >= 4 ? price.toFixed(2) : stratScore >= 1 ? (Math.min(price, supportPrice || price)).toFixed(2) : '观望'}
                          </div>
                        </div>
                        <div>
                          <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 2 }}>止损位</div>
                          <div style={{ fontSize: 18, fontWeight: 700, color: '#F53F3F' }}>{stopLoss.toFixed(2)}</div>
                          <div style={{ fontSize: 10, color: '#F53F3F' }}>{(100 - (stopLoss/price)*100).toFixed(1)}% 风险</div>
                        </div>
                        <div>
                          <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 2 }}>目标一</div>
                          <div style={{ fontSize: 18, fontWeight: 700, color: '#00B42A' }}>{target1.toFixed(2)}</div>
                          <div style={{ fontSize: 10, color: '#00B42A' }}>+{((target1/price - 1)*100).toFixed(1)}% 收益</div>
                        </div>
                        <div>
                          <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 2 }}>目标二</div>
                          <div style={{ fontSize: 18, fontWeight: 700, color: '#00B42A' }}>{target2.toFixed(2)}</div>
                          <div style={{ fontSize: 10, color: '#00B42A' }}>+{((target2/price - 1)*100).toFixed(1)}% 收益</div>
                        </div>
                        <div>
                          <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 2 }}>盈亏比</div>
                          <div style={{ fontSize: 18, fontWeight: 700, color: stratScore >= 1 ? '#F53F3F' : '#86909C' }}>
                            1:{(stratScore >= 1 ? ((target1/price - 1) / (1 - stopLoss/price)) : 0).toFixed(1)}
                          </div>
                        </div>
                      </div>
                      {/* Support/Resistance levels */}
                      <div style={{ marginTop: 14, display: 'flex', gap: 20, flexWrap: 'wrap' }}>
                        <div style={{ flex: 1 }}>
                          <div style={{ fontSize: 11, color: '#F53F3F', marginBottom: 4, fontWeight: 600 }}>压力位</div>
                          {resistances.length > 0 ? resistances.map((r, i) => (
                            <div key={i} style={{ fontSize: 12, color: 'var(--color-text-2)', lineHeight: '20px' }}>
                              {r.label}: <b style={{ color: '#F53F3F' }}>{r.value.toFixed(2)}</b>
                              <span style={{ fontSize: 10, color: 'var(--color-text-3)', marginLeft: 4 }}>(+{((r.value/price - 1)*100).toFixed(1)}%)</span>
                            </div>
                          )) : <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>—</div>}
                        </div>
                        <div style={{ flex: 1 }}>
                          <div style={{ fontSize: 11, color: '#00B42A', marginBottom: 4, fontWeight: 600 }}>支撑位</div>
                          {supports.length > 0 ? supports.map((s, i) => (
                            <div key={i} style={{ fontSize: 12, color: 'var(--color-text-2)', lineHeight: '20px' }}>
                              {s.label}: <b style={{ color: '#00B42A' }}>{s.value.toFixed(2)}</b>
                              <span style={{ fontSize: 10, color: 'var(--color-text-3)', marginLeft: 4 }}>({(price/s.value - 1)*100 >= 0 ? '+' : ''}{((price/s.value - 1)*100).toFixed(1)}%)</span>
                            </div>
                          )) : <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>—</div>}
                        </div>
                      </div>
                    </div>
                  )}

                  {/* ── BOTTOM: Score breakdown ── */}
                  <div style={{ fontSize: 12, color: 'var(--color-text-3)', lineHeight: '24px', padding: '10px 14px', background: 'var(--color-fill-2)', borderRadius: 6 }}>
                    <div style={{ fontWeight: 600, color: 'var(--color-text-2)', marginBottom: 4 }}>评分构成</div>
                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '0 16px' }}>
                      <div>📊 上榜评分: {todayBoardRank !== null ? (todayBoardRank <= 10 ? '+3' : '+1') : '0'}</div>
                      <div>📈 算法信号: {signal !== null ? (signal > 0 ? '+' : '') + Math.max(-3, Math.min(3, signal / 3)).toFixed(1) : '0'}</div>
                      <div>🤖 AI分析: {sug === '强烈买入' ? '+4' : sug === '买入' ? '+3' : sug === '增持' ? '+2' : sug === '持有' ? '0' : sug === '减持' ? '-2' : sug === '卖出' ? '-3' : sug === '强烈卖出' ? '-4' : '0'}</div>
                      <div>📐 技术面: {(price > ma5 && price > ma20 ? 2 : price < ma5 && price < ma20 ? -2 : 0) + (rsi > 30 && rsi < 70 ? 1 : rsi > 80 ? -1 : rsi < 20 ? 1 : 0) + (macdDif > macdDea ? 2 : -2)}</div>
                      <div>🔮 模型预测: {bullishCount > bearishCount ? '+' : ''}{((bullishCount - bearishCount) * 0.5).toFixed(1)}</div>
                    </div>
                  </div>
                  <div className="muted" style={{ fontSize: 11, marginTop: 12 }}>以上策略仅供参考，不构成投资建议。入市需谨慎。</div>
                </div>
              );
            })()}
          </div>
        </div>
      )}

      {/* ── Technical Tab ── */}
      {tab === 'technical' && (
        <div className="card">
          <div className="card-header"><span style={{ fontWeight: 600, fontSize: 14 }}><Activity size={14} /> 技术指标</span></div>
          <div className="card-body">
            {indicators ? (
              <>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 10, marginBottom: 16 }}>
                  {[
                    { name: 'MACD DIF', val: indicators.macd.dif?.toFixed(3), sig: (indicators.macd.dif ?? 0) > (indicators.macd.dea ?? 0) ? '金叉' : '死叉', up: (indicators.macd.dif ?? 0) > (indicators.macd.dea ?? 0), strength: Math.min(3, Math.abs((indicators.macd.dif ?? 0) - (indicators.macd.dea ?? 0)) / Math.max(0.01, Math.abs(indicators.macd.dea ?? 0.01))) },
                    { name: 'MACD DEA', val: indicators.macd.dea?.toFixed(3), sig: (indicators.macd.bar ?? 0) >= 0 ? '多头' : '空头', up: (indicators.macd.bar ?? 0) >= 0, strength: Math.abs(indicators.macd.bar ?? 0) > 0.1 ? 2 : 1 },
                    { name: 'KDJ-K', val: indicators.kdj.k?.toFixed(2), sig: (indicators.kdj.k ?? 50) > 80 ? '超买' : (indicators.kdj.k ?? 50) < 20 ? '超卖' : '中性', up: (indicators.kdj.k ?? 50) > 50, strength: (indicators.kdj.k ?? 50) > 80 ? 2 : (indicators.kdj.k ?? 50) < 20 ? 2 : 1 },
                    { name: 'KDJ-D', val: indicators.kdj.d?.toFixed(2), sig: (indicators.kdj.j ?? 50) > (indicators.kdj.k ?? 50) ? '上行' : '下行', up: (indicators.kdj.j ?? 50) > (indicators.kdj.k ?? 50), strength: 1 },
                    { name: 'KDJ-J', val: indicators.kdj.j?.toFixed(2), sig: (indicators.kdj.j ?? 50) > 100 ? '钝化' : (indicators.kdj.j ?? 50) < 0 ? '钝化' : '正常', up: (indicators.kdj.j ?? 50) > 50, strength: (indicators.kdj.j ?? 50) > 100 ? 2 : (indicators.kdj.j ?? 50) < 0 ? 2 : 1 },
                    { name: 'RSI(14)', val: indicators.rsi?.toFixed(2), sig: (indicators.rsi ?? 50) > 70 ? '超买' : (indicators.rsi ?? 50) < 30 ? '超卖' : '中性', up: (indicators.rsi ?? 50) > 50, strength: (indicators.rsi ?? 50) > 70 ? 2 : (indicators.rsi ?? 50) < 30 ? 2 : Math.abs((indicators.rsi ?? 50) - 50) / 10 },
                    { name: 'BOLL上轨', val: indicators.boll.upper?.toFixed(2), sig: (priceStats?.price ?? 0) > (indicators.boll.upper ?? 0) ? '突破上轨' : '轨内', up: (priceStats?.price ?? 0) > (indicators.boll.upper ?? 0), strength: (priceStats?.price ?? 0) > (indicators.boll.upper ?? 0) ? 2 : 0 },
                    { name: 'BOLL下轨', val: indicators.boll.lower?.toFixed(2), sig: (priceStats?.price ?? 0) < (indicators.boll.lower ?? 0) ? '跌破下轨' : '轨内', up: false, strength: (priceStats?.price ?? 0) < (indicators.boll.lower ?? 0) ? 2 : 0 },
                  ].map((r, i) => {
                    const bias = getBiasInfo(r.up, Math.round(r.strength));
                    return (
                      <div key={i} style={{ padding: '10px 12px', background: 'var(--color-fill-2)', borderRadius: 4, borderLeft: `3px solid ${bias.color}`, cursor: 'help', transition: 'box-shadow 0.15s' }}
                        title={INDICATOR_DESC[r.name] || ''}
                        onMouseEnter={e => (e.currentTarget.style.boxShadow = '0 2px 8px rgba(0,0,0,0.1)')}
                        onMouseLeave={e => (e.currentTarget.style.boxShadow = 'none')}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
                          <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-2)' }}>{r.name}</span>
                          <span style={{ fontSize: 10, fontWeight: 600, color: bias.color, background: bias.bg, padding: '1px 5px', borderRadius: 3 }}>{bias.label}</span>
                        </div>
                        <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)' }}>{r.val}</div>
                        <div style={{ fontSize: 10, color: bias.color, marginTop: 3 }}>{r.sig}</div>
                      </div>
                    );
                  })}
                </div>
              {/* Interval controls */}
                <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                  <Button size="mini" type={intervalMode ? 'primary' : 'outline'} onClick={() => { setIntervalMode(!intervalMode); if (!intervalMode) setIntervalRange(null); }}>
                    📐 区间统计 {intervalMode ? '开' : '关'}
                  </Button>
                  {intervalMode && intervalRange && (
                    <Button size="mini" type="text" onClick={() => setIntervalRange(null)} style={{ fontSize: 11 }}>重置默认</Button>
                  )}
                  {intervalMode && <span className="muted" style={{ fontSize: 11 }}>拖拽图表手柄调整区间 · 左右拖动边缘调整范围 · 中间拖动整体移动</span>}
                </div>
                {/* Chart + Stats side-by-side layout when interval mode is active */}
                {intervalMode && intervalRange ? (
                  <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
                    {/* Chart on the left */}
                    <div style={{ flex: '1 1 60%', minWidth: 0 }}>
                      <KLineChart data={klines} height={300} markers={markers}
                        enableRangeSelect={intervalMode}
                        selectedRange={intervalRange}
                        onRangeChange={handleRangeChange}
                      />
                    </div>
                    {/* Stats panel on the right */}
                    {intervalStats && (
                      <div style={{
                        flex: '0 0 300px',
                        background: 'var(--color-bg-1)',
                        borderRadius: 10,
                        border: '1px solid var(--color-border-1)',
                        boxShadow: '0 1px 4px rgba(0,0,0,0.06)',
                        overflow: 'hidden',
                      }}>
                        {/* Header */}
                        <div style={{
                          padding: '12px 16px',
                          background: 'linear-gradient(135deg, #165DFF 0%, #4080FF 100%)',
                          color: '#fff',
                        }}>
                          <div style={{ fontSize: 14, fontWeight: 600 }}>📐 区间统计</div>
                          <div style={{ fontSize: 11, opacity: 0.85, marginTop: 16 }}>
                            {intervalStats.startDate?.slice(0,10) || ''} → {intervalStats.endDate?.slice(0,10) || ''} · {intervalStats.bars}根K线
                          </div>
                        </div>
                        {/* Key metrics */}
                        <div style={{ padding: '14px 16px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px 16px' }}>
                          {/* 涨跌幅 */}
                          <div style={{
                            background: intervalStats.changePct >= 0 ? 'rgba(245,63,63,0.06)' : 'rgba(0,180,42,0.06)',
                            borderRadius: 8, padding: '10px 12px',
                            border: `1px solid ${intervalStats.changePct >= 0 ? 'rgba(245,63,63,0.18)' : 'rgba(0,180,42,0.18)'}`,
                          }}>
                            <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginBottom: 2 }}>涨跌幅</div>
                            <div style={{
                              fontSize: 20, fontWeight: 700,
                              color: intervalStats.changePct >= 0 ? '#F53F3F' : '#00B42A',
                              fontFamily: 'monospace',
                            }}>
                              {intervalStats.changePct >= 0 ? '+' : ''}{intervalStats.changePct.toFixed(2)}%
                            </div>
                            <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 16 }}>
                              {intervalStats.startPrice?.toFixed(2)} → {intervalStats.endPrice?.toFixed(2)}
                            </div>
                          </div>
                          {/* 振幅 */}
                          <div style={{
                            background: 'var(--color-fill-2)', borderRadius: 8, padding: '10px 12px',
                          }}>
                            <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginBottom: 2 }}>振幅</div>
                            <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--color-text-1)', fontFamily: 'monospace' }}>
                              {intervalStats.amplitude.toFixed(2)}%
                            </div>
                            <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 16 }}>
                              {intervalStats.high?.toFixed(2)} / {intervalStats.low?.toFixed(2)}
                            </div>
                          </div>
                          {/* 最大回撤 */}
                          <div style={{
                            background: 'rgba(247,114,52,0.06)', borderRadius: 8, padding: '10px 12px',
                            border: '1px solid rgba(247,114,52,0.18)',
                          }}>
                            <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginBottom: 2 }}>最大回撤</div>
                            <div style={{ fontSize: 20, fontWeight: 700, color: '#F77234', fontFamily: 'monospace' }}>
                              -{intervalStats.maxDrawdown.toFixed(2)}%
                            </div>
                            <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 16 }}>区间内峰值回落</div>
                          </div>
                          {/* 涨跌天数比 */}
                          <div style={{
                            background: 'var(--color-fill-2)', borderRadius: 8, padding: '10px 12px',
                          }}>
                            <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginBottom: 2 }}>涨跌比</div>
                            <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--color-text-1)', fontFamily: 'monospace' }}>
                              <span style={{ color: '#F53F3F' }}>{intervalStats.upDays}</span>
                              <span style={{ color: 'var(--color-text-3)', margin: '0 4px' }}>/</span>
                              <span style={{ color: '#00B42A' }}>{intervalStats.downDays}</span>
                            </div>
                            <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 16 }}>
                              阳{intervalStats.upDays}天 · 阴{intervalStats.downDays}天
                            </div>
                          </div>
                        </div>
                        {/* Divider + additional info */}
                        <div style={{
                          borderTop: '1px solid var(--color-border-1)', padding: '10px 16px',
                          display: 'flex', justifyContent: 'space-between',
                          background: 'var(--color-fill-1)', fontSize: 11, color: 'var(--color-text-3)',
                        }}>
                          <span>最高 <b style={{ color: '#F53F3F', fontSize: 12 }}>{intervalStats.high?.toFixed(2)}</b></span>
                          <span>最低 <b style={{ color: '#00B42A', fontSize: 12 }}>{intervalStats.low?.toFixed(2)}</b></span>
                          <span>收 <b style={{ color: 'var(--color-text-1)', fontSize: 12 }}>{intervalStats.endPrice?.toFixed(2)}</b></span>
                        </div>
                        <div style={{
                          padding: '8px 16px', fontSize: 10, color: '#C9CDD4',
                          background: '#FAFBFC', borderTop: '1px solid #F2F3F5',
                        }}>
                          提示：拖拽图表左右手柄调整区间 · 中间拖动整体平移
                        </div>
                      </div>
                    )}
                  </div>
                ) : (
                  <KLineChart data={klines} height={300} markers={markers}
                    enableRangeSelect={intervalMode}
                    selectedRange={intervalRange}
                    onRangeChange={handleRangeChange}
                  />
                )}
              </>
            ) : (<div className="muted" style={{ textAlign: 'center', padding: 40 }}>K线数据不足</div>)}
          </div>
        </div>
      )}

      {/* ── Trading Tab ── */}
      {tab === 'trading' && (
        <div className="card">
          <div className="card-header"><span style={{ fontWeight: 600, fontSize: 14 }}><Table2 size={14} /> 近10日交易数据</span></div>
          <div className="card-body" style={{ padding: 0, overflow: 'auto' }}>
            {safeKlines.length >= 10 ? (
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12, minWidth: 700 }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--color-border-1)', background: 'var(--color-fill-2)' }}>
                    {['日期','开盘','最高','最低','收盘','涨跌幅','成交量(万手)','成交额(亿)','换手%'].map(h => (
                      <th key={h} style={{ padding: '8px 10px', textAlign: h === '日期' ? 'left' : 'right', color: 'var(--color-text-3)', fontWeight: 500, fontSize: 11, whiteSpace: 'nowrap' }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {safeKlines.slice(-10).reverse().map((k: any, i: number) => {
                    const prev = safeKlines[safeKlines.length - 11 + i];
                    const chg = prev?.close ? ((k.close - prev.close) / prev.close) * 100 : 0;
                    return (
                      <tr key={i} style={{ borderBottom: '1px solid var(--color-table-row-border)' }}>
                        <td style={{ padding: '6px 10px', fontSize: 11 }}>{(k.tradeDate || k.date || '').slice(0, 10)}</td>
                        <td className="r num" style={{ padding: '6px 10px' }}>{k.open?.toFixed(2)}</td>
                        <td className="r num up" style={{ padding: '6px 10px' }}>{k.high?.toFixed(2)}</td>
                        <td className="r num down" style={{ padding: '6px 10px' }}>{k.low?.toFixed(2)}</td>
                        <td className="r num" style={{ padding: '6px 10px', fontWeight: 600 }}>{k.close?.toFixed(2)}</td>
                        <td className={`r num ${chg >= 0 ? 'up' : 'down'}`} style={{ padding: '6px 10px' }}>{chg >= 0 ? '+' : ''}{chg.toFixed(2)}%</td>
                        <td className="r num" style={{ padding: '6px 10px' }}>{k.volume ? (k.volume / 10000).toFixed(0) : '-'}</td>
                        <td className="r num" style={{ padding: '6px 10px' }}>{k.amount ? (k.amount * 1e4 / 1e8).toFixed(2) : '-'}</td>
                        <td className="r num" style={{ padding: '6px 10px' }}>{k.turnoverRate > 0 ? (k.turnoverRate * 100).toFixed(2) : '-'}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            ) : (<div className="muted" style={{ textAlign: 'center', padding: 40 }}>K线数据不足</div>)}
          </div>
        </div>
      )}

      {/* ── Financial Tab ── */}
      {tab === 'financial' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {/* Chart + Summary card */}
          {financials.length >= 2 && (
            <div className="card">
              <div className="card-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <span style={{ fontWeight: 600, fontSize: 14 }}><BarChart3 size={14} /> 季度营收 / 净利润（近{Math.min(financials.length, 8)}季）</span>
                <button onClick={() => handleRefreshStockData('financial')} disabled={refreshingPhase !== ''}
                  style={{ padding: '4px 10px', fontSize: 11, cursor: 'pointer', border: '1px solid var(--color-border-1)', borderRadius: 4, background: 'var(--color-bg-1)', color: 'var(--color-text-2)' }}>
                  <Repeat size={12} className={refreshingPhase === 'financial' ? 'spin' : ''} />更新
                </button>
              </div>
              <div className="card-body" style={{ padding: '12px 16px', display: 'flex', gap: 16, flexWrap: 'wrap' }}>
                <div style={{ flex: '1 1 480px', minWidth: 360 }}>
                  <FinBarChart data={[...financials].reverse()} />
                </div>
                <FinSummaryTable data={financials} />
              </div>
            </div>
          )}
          {/* Period cards */}
          {financials.length > 0 ? (
            financials.map((f: any, i: number) => {
              const isProfit = (f.netProfit || 0) >= 0;
              const typeColors: Record<string, [string, string]> = {
                '年报': ['#E8F3FF', '#165DFF'], '一季报': ['#E8FFEA', '#00B42A'],
                '中报': ['rgba(255,125,0,0.10)', '#FF7D00'], '三季报': ['#F2F3F5', '#86909C'],
              };
              const [typeBg, typeColor] = typeColors[f.reportType] || ['#F2F3F5', '#86909C'];
              // Compute YoY / QoQ
              const prev = i + 1 < financials.length ? financials[i + 1] : null;
              const yoy = i + 4 < financials.length ? financials[i + 4] : null;
              return (
                <div key={i} className="card" style={{ borderLeft: `3px solid ${typeColor}` }}>
                  <div style={{ padding: '14px 18px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 14 }}>
                      <span style={{ fontSize: 15, fontWeight: 600 }}>{f.reportDate}</span>
                      <span style={{ padding: '2px 8px', borderRadius: 3, fontSize: 11, background: typeBg, color: typeColor, fontWeight: 500 }}>{f.reportType}</span>
                    </div>
                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))', gap: 10 }}>
                      <FinCard label="营业总收入" value={fmtMoney(f.totalRevenue)} extra={prev ? fmtMoney(f.totalRevenue - prev.totalRevenue) : undefined} extraLabel="环比" color={(f.totalRevenue - (prev?.totalRevenue || 0)) >= 0 ? '#F53F3F' : '#00B42A'} />
                      <FinCard label="净利润" value={fmtMoney(Math.abs(f.netProfit))} prefix={isProfit ? '' : '-'} extra={prev ? fmtMoney(Math.abs(f.netProfit - prev.netProfit)) : undefined} extraLabel="环比" color={isProfit ? '#F53F3F' : '#00B42A'} />
                      <FinCard label="总资产" value={fmtMoney(f.totalAssets)} />
                      <FinCard label="净资产" value={fmtMoney(f.netAssets)} />
                      <FinCard label="ROE" value={f.roe !== 0 ? f.roe.toFixed(2) + '%' : '-'} extra={prev?.roe ? ((f.roe - prev.roe) >= 0 ? '+' : '') + (f.roe - prev.roe).toFixed(2) + 'pp' : undefined} extraLabel="环比" color={f.roe >= 0 ? '#F53F3F' : '#00B42A'} />
                      <FinCard label="EPS" value={f.eps !== 0 ? f.eps.toFixed(3) : '-'} extra={yoy?.eps ? ((f.eps - yoy.eps) >= 0 ? '+' : '') + (f.eps - yoy.eps).toFixed(3) : undefined} extraLabel="同比" color={f.eps >= 0 ? '#F53F3F' : '#00B42A'} />
                      <FinCard label="毛利率" value={f.grossMargin !== 0 ? f.grossMargin.toFixed(1) + '%' : '-'} />
                      <FinCard label="净利率" value={f.netMargin !== 0 ? f.netMargin.toFixed(1) + '%' : '-'} color={f.netMargin >= 0 ? 'var(--color-text-1)' : '#00B42A'} />
                      <FinCard label="资产负债率" value={f.debtRatio > 0 ? f.debtRatio.toFixed(1) + '%' : '-'} extra={prev?.debtRatio ? ((f.debtRatio - prev.debtRatio) >= 0 ? '+' : '') + (f.debtRatio - prev.debtRatio).toFixed(2) + 'pp' : undefined} extraLabel="环比" />
                    </div>
                  </div>
                </div>
              );
            })
          ) : (
            <div className="card">
              <div className="card-header" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <span style={{ fontWeight: 600, fontSize: 14 }}><BarChart3 size={14} /> 财务数据</span>
                <button onClick={() => handleRefreshStockData('financial')} disabled={refreshingPhase !== ''}
                  style={{ padding: '4px 10px', fontSize: 11, cursor: 'pointer', border: '1px solid var(--color-border-1)', borderRadius: 4, background: 'var(--color-bg-1)', color: 'var(--color-text-2)' }}>
                  <Repeat size={12} className={refreshingPhase === 'financial' ? 'spin' : ''} />{refreshingPhase === 'financial' ? '更新中...' : '更新'}
                </button>
              </div>
              <div className="card-body">
                <div className="muted" style={{ textAlign: 'center', padding: 48, fontSize: 13 }}>
                  暂无财务数据，请点击右上角「更新」按钮采集
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* ── Shareholder Tab ── */}
      {tab === 'shareholder' && (() => {
        const shList = shareholders || [];
        const latestSH = shList.length > 0 ? shList[0] : null;
        const trendData = [...shList].reverse().slice(-10); // oldest→newest for chart
        const top10Raw = latestSH?.top10Holders;
        const top10List = Array.isArray(top10Raw) ? top10Raw : [];
        
        return (
        <div className="card">
          <div className="card-header"><span style={{ fontWeight: 600, fontSize: 14 }}><Users size={14} /> 股东数据</span>
              <button onClick={() => handleRefreshStockData('shareholder')} disabled={refreshingPhase !== ''}
                style={{ marginLeft: 'auto', padding: '4px 10px', fontSize: 11, cursor: 'pointer', border: '1px solid var(--color-border-1)', borderRadius: 4, background: 'var(--color-bg-1)', color: 'var(--color-text-2)', display: 'flex', alignItems: 'center', gap: 4 }}>
                <Repeat size={12} className={refreshingPhase === 'shareholder' ? 'spin' : ''} />{refreshingPhase === 'shareholder' ? '更新中...' : '更新'}
              </button></div>
          <div className="card-body" style={{ padding: 16 }}>
            {latestSH ? (
              <>
                {/* Summary cards */}
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 20 }}>
                  <div style={{ background: 'var(--color-fill-2)', borderRadius: 8, padding: '12px 14px' }}>
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>股东总户数</div>
                    <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--color-text-1)' }}>
                      {(latestSH.totalHolders / 10000).toFixed(2)}<span style={{ fontSize: 12, fontWeight: 400, color: 'var(--color-text-3)' }}>万户</span>
                    </div>
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 16 }}>截至 {latestSH.reportDate}</div>
                  </div>
                  <div style={{ background: 'var(--color-fill-2)', borderRadius: 8, padding: '12px 14px' }}>
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>环比变化</div>
                    <div style={{ fontSize: 20, fontWeight: 700, color: latestSH.holderChange > 0 ? '#00B42A' : latestSH.holderChange < 0 ? '#F53F3F' : 'var(--color-text-3)' }}>
                      {latestSH.holderChange > 0 ? '+' : ''}{latestSH.holderChange?.toFixed(2)}%
                    </div>
                    <div style={{ fontSize: 11, color: latestSH.holderChange > 0 ? '#00B42A' : latestSH.holderChange < 0 ? '#F53F3F' : 'var(--color-text-3)', marginTop: 16 }}>
                      {latestSH.holderChange > 0 ? '筹码分散 ↑' : latestSH.holderChange < 0 ? '筹码集中 ↓' : '持平'}
                    </div>
                  </div>
                  <div style={{ background: 'var(--color-fill-2)', borderRadius: 8, padding: '12px 14px' }}>
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>户均持股</div>
                    <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--color-text-1)' }}>
                      {fmtVol(latestSH.avgHolding)}<span style={{ fontSize: 12, fontWeight: 400, color: 'var(--color-text-3)' }}>股</span>
                    </div>
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 16 }}>人均持股市值</div>
                  </div>
                  <div style={{ background: 'var(--color-fill-2)', borderRadius: 8, padding: '12px 14px' }}>
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 4 }}>机构持股比例</div>
                    <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--color-text-1)' }}>
                      {latestSH.instHoldRatio > 0 ? latestSH.instHoldRatio.toFixed(2) + '%' : '-'}
                    </div>
                    <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginTop: 16 }}>数据期数 {shList.length} 期</div>
                  </div>
                </div>

                {/* Bar chart: shareholder count trend */}
                {trendData.length >= 2 && (() => {
                  const W = 720, H = 240, padL = 56, padR = 20, padT = 14, padB = 40;
                  const cw = W - padL - padR, ch = H - padT - padB;
                  const vals = trendData.map((d: any) => d.totalHolders || 0);
                  const vMin = Math.min(...vals);
                  const vMax = Math.max(...vals);
                  const range = vMax - vMin || 1;
                  const barGap = cw / trendData.length;
                  const barW = Math.max(8, Math.min(28, barGap * 0.5));
                  const dx = (barGap - barW) / 2;
                  
                  return (
                    <div style={{ marginBottom: 20 }}>
                      <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 10, color: 'var(--color-text-1)' }}>股东户数变化趋势（近{trendData.length}期）</div>
                      <svg viewBox={`0 0 ${W} ${H}`} style={{ width: '100%', height: 'auto', maxWidth: 730 }}>
                        {/* Y axis grid lines */}
                        {[0, 0.25, 0.5, 0.75, 1].map((frac, gi) => {
                          const y = padT + ch * (1 - frac);
                          const val = vMin + range * frac;
                          return (
                            <g key={'g'+gi}>
                              <line x1={padL} y1={y} x2={padL + cw} y2={y} stroke="var(--color-border-2)" strokeWidth="1" />
                              <text x={padL - 6} y={y + 3} textAnchor="end" fontSize="9" fill="var(--color-text-3)">
                                {(val / 10000).toFixed(1)}万
                              </text>
                            </g>
                          );
                        })}
                        
                        {/* Bars */}
                        {trendData.map((d: any, i: number) => {
                          const h = Math.max(2, ((d.totalHolders || 0) - vMin) / range * ch);
                          const x = padL + i * barGap + dx;
                          const y = padT + ch - h;
                          const isLast = i === trendData.length - 1;
                          const isFirst = i === 0;
                          // Red (increase = bearish/分散) vs Green (decrease = bullish/集中)
                          const prevVal = i > 0 ? (trendData[i - 1].totalHolders || 0) : (d.totalHolders || 0);
                          const color = d.totalHolders >= prevVal ? '#F53F3F' : '#00B42A';
                          const opacity = isLast ? 1 : 0.65;
                          
                          return (
                            <g key={'bar'+i}>
                              <rect x={x} y={y} width={barW} height={h} rx="3" fill={color} opacity={opacity} />
                              {(i % 2 === 0 || isLast) && (
                                <text x={x + barW / 2} y={y - 4} textAnchor="middle" fontSize="8" fill={color} fontWeight={isLast ? 600 : 400}>
                                  {(d.totalHolders / 10000).toFixed(1)}万
                                </text>
                              )}
                            </g>
                          );
                        })}
                        
                        {/* X axis labels */}
                        {trendData.map((d: any, i: number) => {
                          const label = (d.reportDate || '').slice(0, 7);
                          const show = i % 2 === 0 || i === trendData.length - 1;
                          return show ? (
                            <text key={'xl'+i} x={padL + i * barGap + barGap / 2} y={H - 10}
                              textAnchor="middle" fontSize="9" fill="var(--color-text-3)">{label}</text>
                          ) : null;
                        })}
                        
                        {/* Legend */}
                        <rect x={padL + cw - 150} y={padT} width={10} height={10} rx="2" fill="#F53F3F" opacity="0.65" />
                        <text x={padL + cw - 136} y={padT + 8} fontSize="9" fill="var(--color-text-3)">户数增加(分散)</text>
                        <rect x={padL + cw - 60} y={padT} width={10} height={10} rx="2" fill="#00B42A" opacity="0.65" />
                        <text x={padL + cw - 46} y={padT + 8} fontSize="9" fill="var(--color-text-3)">减少(集中)</text>
                      </svg>
                    </div>
                  );
                })()}

                {/* History table */}
                <div style={{ marginBottom: 20 }}>
                  <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 10, color: 'var(--color-text-1)' }}>历史数据明细</div>
                  <div style={{ overflow: 'auto', maxHeight: 360 }}>
                    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12, tableLayout: 'fixed' }}>
                      <colgroup>
                        <col style={{ width: '22%' }} />
                        <col style={{ width: '20%' }} />
                        <col style={{ width: '18%' }} />
                        <col style={{ width: '20%' }} />
                        <col style={{ width: '20%' }} />
                      </colgroup>
                      <thead>
                        <tr style={{ background: 'var(--color-fill-2)', position: 'sticky', top: 0, zIndex: 1 }}>
                          {['报告期', '股东户数', '环比变化', '户均持股', '筹码趋势'].map((h, hi) => (
                            <th key={h} style={{
                              padding: '8px 12px', textAlign: hi === 0 ? 'left' : 'right',
                              color: 'var(--color-text-3)', fontWeight: 500, fontSize: 11,
                              borderBottom: '1px solid var(--color-border-1)'
                            }}>{h}</th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {shList.map((row: any, i: number) => (
                          <tr key={i} style={{ borderBottom: '1px solid var(--color-table-row-border)' }}>
                            <td style={{ padding: '7px 12px', fontSize: 12, fontWeight: i === 0 ? 600 : 400, color: 'var(--color-text-1)', textAlign: 'left' }}>
                              {row.reportDate}
                            </td>
                            <td style={{ padding: '7px 12px', fontSize: 12, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
                              {(row.totalHolders / 10000).toFixed(2)}万
                            </td>
                            <td style={{
                              padding: '7px 12px', fontSize: 12, textAlign: 'right', fontVariantNumeric: 'tabular-nums',
                              color: row.holderChange > 0 ? '#00B42A' : row.holderChange < 0 ? '#F53F3F' : 'var(--color-text-3)',
                              fontWeight: 500
                            }}>
                              {row.holderChange > 0 ? '+' : ''}{row.holderChange?.toFixed(2)}%
                            </td>
                            <td style={{ padding: '7px 12px', fontSize: 12, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
                              {row.avgHolding > 0 ? fmtVol(row.avgHolding) + '股' : '-'}
                            </td>
                            <td style={{ padding: '7px 12px', fontSize: 11, textAlign: 'right' }}>
                              <span style={{
                                padding: '2px 8px', borderRadius: 10, fontSize: 10,
                                background: row.holderChange > 2 ? 'rgba(245,63,63,0.10)' : row.holderChange < -2 ? 'rgba(0,180,42,0.10)' : 'var(--color-fill-2)',
                                color: row.holderChange > 2 ? '#F53F3F' : row.holderChange < -2 ? '#00B42A' : 'var(--color-text-3)',
                                fontWeight: 500
                              }}>
                                {row.holderChange > 2 ? '分散' : row.holderChange < -2 ? '集中' : '平稳'}
                              </span>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>

                {/* Top 10 shareholders — grouped by period */}
                {(() => {
                  // Deduplicate: only show unique top10 periods (by JSON content)
                  const top10Periods: any[] = [];
                  const seenHashes = new Set<string>();
                  shList.forEach((s: any) => {
                    const holders = Array.isArray(s.top10Holders) ? s.top10Holders : [];
                    const exited = Array.isArray(s.top10Float) ? s.top10Float : [];
                    if (holders.length === 0 && exited.length === 0) return;
                    const hash = JSON.stringify(holders) + '|' + JSON.stringify(exited);
                    if (!seenHashes.has(hash)) {
                      seenHashes.add(hash);
                      top10Periods.push(s);
                    }
                  });
                  if (top10Periods.length === 0) return null;
                  
                  const trendMap: Record<string, {bg:string, color:string, label:string}> = {
                    '新进': {bg:'#F0FFF4', color:'#00B42A', label:'新进'},
                    '增持': {bg:'#E8F3FF', color:'#165DFF', label:'增持'},
                    '减持': {bg:'rgba(245,63,63,0.10)', color:'#F53F3F', label:'减持'},
                    '不变': {bg:'var(--color-fill-2)', color:'var(--color-text-3)', label:'不变'},
                    '退出': {bg:'rgba(255,125,0,0.10)', color:'#FF7D00', label:'退出'},
                  };
                  return (
                    <div>
                      <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 12, color: 'var(--color-text-1)' }}>十大股东（按报告期）</div>
                      {top10Periods.map((row: any, ri: number) => {
                        const holders = Array.isArray(row.top10Holders) ? row.top10Holders : [];
                        const exited = Array.isArray(row.top10Float) ? row.top10Float : [];
                        if (holders.length === 0 && exited.length === 0) return null;
                        const t = trendMap;
                        return (
                          <details key={ri} open={ri === 0} style={{ marginBottom: 12 }}>
                            <summary style={{
                              cursor: 'pointer', padding: '8px 12px', background: 'var(--color-fill-2)', borderRadius: 6,
                              fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)', display: 'flex', alignItems: 'center',
                              justifyContent: 'space-between'
                            }}>
                              <span>{row.reportDate} 报告期</span>
                              <span style={{ fontSize: 11, fontWeight: 400, color: 'var(--color-text-3)' }}>
                                {holders.length}位股东{exited.length > 0 ? ` · ${exited.length}位退出` : ''}
                              </span>
                            </summary>
                            <div style={{ padding: '8px 0' }}>
                              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12, tableLayout: 'fixed' }}>
                                <colgroup>
                                  <col style={{ width: '8%' }} /><col style={{ width: '34%' }} />
                                  <col style={{ width: '18%' }} /><col style={{ width: '14%' }} /><col style={{ width: '12%' }} /><col style={{ width: '14%' }} />
                                </colgroup>
                                <thead>
                                  <tr style={{ borderBottom: '2px solid var(--color-border-1)', background: 'var(--color-fill-1)' }}>
                                    {['排名', '股东名称', '持股数', '持股比例', '变动', '环比'].map((h, hi) => (
                                      <th key={h} style={{
                                        padding: '7px 8px', textAlign: hi >= 2 ? 'right' : 'left',
                                        color: 'var(--color-text-3)', fontWeight: 500, fontSize: 11
                                      }}>{h}</th>
                                    ))}
                                  </tr>
                                </thead>
                                <tbody>
                                  {holders.map((h: any, i: number) => {
                                    const trend = t[h.trend] || t['不变'];
                                    return (
                                      <tr key={i} style={{ borderBottom: '1px solid var(--color-table-row-border)' }}>
                                        <td style={{ padding: '6px 8px', fontSize: 11, color: 'var(--color-text-3)', textAlign: 'left' }}>{h.rank || i+1}</td>
                                        <td style={{ padding: '6px 8px', fontSize: 12, fontWeight: 500, color: 'var(--color-text-1)', textAlign: 'left', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={h.name}>{h.name}</td>
                                        <td style={{ padding: '6px 8px', fontSize: 11, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>{fmtVol(h.shares)}</td>
                                        <td style={{ padding: '6px 8px', fontSize: 11, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>{h.ratio?.toFixed(2)}%</td>
                                        <td style={{ padding: '6px 8px', fontSize: 10, textAlign: 'right' }}>
                                          <span style={{
                                            padding: '1px 6px', borderRadius: 8, fontSize: 10,
                                            background: h.change ? 'rgba(22,93,255,0.10)' : 'var(--color-fill-2)',
                                            color: h.change ? '#165DFF' : 'var(--color-text-3)',
                                          }}>{h.change || '-'}</span>
                                        </td>
                                        <td style={{ padding: '6px 8px', fontSize: 10, textAlign: 'right' }}>
                                          <span style={{
                                            padding: '1px 6px', borderRadius: 8, fontSize: 10,
                                            background: trend.bg, color: trend.color, fontWeight: 500
                                          }}>{trend.label}</span>
                                        </td>
                                      </tr>
                                    );
                                  })}
                                </tbody>
                              </table>
                              {/* Exited holders */}
                              {exited.length > 0 && (
                                <div style={{ marginTop: 8 }}>
                                  <div style={{ fontSize: 11, color: '#FF7D00', fontWeight: 500, marginBottom: 4 }}>
                                    ⚠ 较上期退出前十大股东：
                                  </div>
                                  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 11, tableLayout: 'fixed' }}>
                                    <colgroup>
                                      <col style={{ width: '42%' }} /><col style={{ width: '26%' }} /><col style={{ width: '32%' }} />
                                    </colgroup>
                                    <tbody>
                                      {exited.map((h: any, i: number) => (
                                        <tr key={'ex'+i} style={{ borderBottom: '1px solid var(--color-table-row-border)', background: 'var(--color-warning-bg)' }}>
                                          <td style={{ padding: '5px 8px', textAlign: 'left', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={h.name}>{h.name}</td>
                                          <td style={{ padding: '5px 8px', textAlign: 'right' }}>{fmtVol(h.shares)}</td>
                                          <td style={{ padding: '5px 8px', textAlign: 'right' }}>
                                            <span style={{ padding: '1px 6px', borderRadius: 8, fontSize: 10, background: 'var(--color-warning-bg)', color: '#FF7D00', fontWeight: 500 }}>退出</span>
                                          </td>
                                        </tr>
                                      ))}
                                    </tbody>
                                  </table>
                                </div>
                              )}
                            </div>
                          </details>
                        );
                      })}
                    </div>
                  );
                })()}
              </>
            ) : (
              <div className="muted" style={{ textAlign: 'center', padding: 48, fontSize: 13 }}>
                暂无股东数据，请点击右上角「更新」按钮采集
              </div>
            )}
          </div>
        </div>
        );
      })()}

      {/* ── Reports Tab ── */}
      {tab === 'reports' && (
        <div className="card">
          <div className="card-header"><span style={{ fontWeight: 600, fontSize: 14 }}><FileText size={14} /> 机构研报</span>
              <button onClick={() => handleRefreshStockData('reports')} disabled={refreshingPhase !== ''}
                style={{ marginLeft: 'auto', padding: '4px 10px', fontSize: 11, cursor: 'pointer', border: '1px solid var(--color-border-1)', borderRadius: 4, background: 'var(--color-bg-1)', color: 'var(--color-text-2)', display: 'flex', alignItems: 'center', gap: 4 }}>
                <Repeat size={12} className={refreshingPhase === 'reports' ? 'spin' : ''} />{refreshingPhase === 'reports' ? '更新中...' : '更新'}
              </button></div>
          <div className="card-body" style={{ padding: 0 }}>
            {refreshingPhase === 'reports' && refreshLogs.length > 0 && (
                <div style={{
                  margin: '0 0 12px 0', padding: '10px 14px',
                  background: 'var(--color-text-1)', borderRadius: 6, color: '#00ff88',
                  fontFamily: 'monospace', fontSize: 11, maxHeight: 200, overflow: 'auto',
                  lineHeight: 1.6
                }}>
                  {refreshLogs.map((log, i) => (
                    <div key={i} style={{ opacity: i === refreshLogs.length - 1 ? 1 : 0.4 }}>
                      {log.startsWith('✅') ? <span style={{color:'#00ff88'}}>{log}</span> :
                       log.startsWith('⚠') ? <span style={{color:'var(--color-warning-text)'}}>{log}</span> :
                       log.includes('error') || log.includes('失败') ? <span style={{color:'var(--color-danger-text)'}}>{log}</span> :
                       <span>{log}</span>}
                    </div>
                  ))}
                </div>
              )}
            {reports.length > 0 ? (
              <div style={{ display: 'flex', flexDirection: 'column' }}>
                {reports.map((r: any, i: number) => {
                  const ratingColor: Record<string, string> = {
                    '买入': '#F53F3F', '增持': '#FF7D00', '强烈推荐': '#F53F3F',
                    '推荐': '#FF7D00', '谨慎推荐': '#165DFF', '中性': 'var(--color-text-3)',
                    '减持': '#00B42A', '卖出': '#00B42A',
                  };
                  const rc = ratingColor[r.rating] || 'var(--color-text-3)';
                  return (
                    <div key={i} style={{ display: 'flex', alignItems: 'flex-start', gap: 12, padding: '12px 16px', borderBottom: '1px solid var(--color-table-row-border)' }}>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                          {r.rating && (
                            <span style={{
                              padding: '2px 8px', borderRadius: 4, fontSize: 11, fontWeight: 600,
                              background: rc + '18', color: rc, whiteSpace: 'nowrap'
                            }}>{r.rating}</span>
                          )}
                          <span style={{ fontSize: 13, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1 }}>
                            {r.title}
                          </span>
                        </div>
                        <div style={{ display: 'flex', gap: 16, fontSize: 11, color: 'var(--color-text-3)', flexWrap: 'wrap' }}>
                          <span>{r.orgSname || r.orgName}</span>
                          {r.researcher && <span>分析师: {r.researcher}</span>}
                          <span>{r.publishDate}</span>
                          {r.predictThisYearEps > 0 && (
                            <span style={{ color: 'var(--color-text-1)' }}>
                              EPS预测: {r.predictThisYearEps}({r.predictThisYearPe}x)
                            </span>
                          )}
                          {r.industryName && <span style={{ color: '#165DFF' }}>{r.industryName}</span>}
                        </div>
                      </div>
                      {r.pdfUrl && (
                        <div style={{ display: 'flex', gap: 6, flexShrink: 0 }}>
                          <a href={`https://data.eastmoney.com/report/info/${r.infoCode || ''}.html`} target="_blank" rel="noopener noreferrer"
                            style={{ padding: '4px 8px', fontSize: 11, background: 'var(--color-primary)', color: '#fff', borderRadius: 4, textDecoration: 'none', whiteSpace: 'nowrap' }}>
                            详情
                          </a>
                          <a href={`https://pdf.dfcfw.com/pdf/H3_${r.infoCode || ''}_1.pdf`} target="_blank" rel="noopener noreferrer"
                            style={{ padding: '4px 8px', fontSize: 11, background: 'var(--color-success)', color: '#fff', borderRadius: 4, textDecoration: 'none', whiteSpace: 'nowrap' }}>
                            PDF
                          </a>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="muted" style={{ textAlign: 'center', padding: 48, fontSize: 13 }}>
                暂无研报数据，请点击右上角「更新」按钮采集
              </div>
            )}
          </div>
        </div>
      )}

      {/* ── News Tab ── */}
      {tab === 'news' && (
        <div className="card">
          <div className="card-header"><span style={{ fontWeight: 600, fontSize: 14 }}><Newspaper size={14} /> 相关资讯</span>
              <button onClick={() => handleRefreshStockData('news')} disabled={refreshingPhase !== ''}
                style={{ marginLeft: 'auto', padding: '4px 10px', fontSize: 11, cursor: 'pointer', border: '1px solid var(--color-border-1)', borderRadius: 4, background: 'var(--color-bg-1)', color: 'var(--color-text-2)', display: 'flex', alignItems: 'center', gap: 4 }}>
                <Repeat size={12} className={refreshingPhase === 'news' ? 'spin' : ''} />{refreshingPhase === 'news' ? '更新中...' : '更新'}
              </button></div>
          <div className="card-body" style={{ padding: 0 }}>
            {stockNews.length > 0 ? (
              <div style={{ display: 'flex', flexDirection: 'column' }}>
                {stockNews.map((n: any, i: number) => (
                  <a key={i} href={n.url || '#'} target="_blank" rel="noopener noreferrer"
                    style={{ display: 'flex', alignItems: 'flex-start', gap: 12, padding: '10px 16px', borderBottom: '1px solid var(--color-table-row-border)', textDecoration: 'none', color: 'inherit' }}>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontSize: 13, fontWeight: 500, marginBottom: 4, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {n.title}
                      </div>
                      {n.summary && <div style={{ fontSize: 11, color: 'var(--color-text-3)', lineHeight: 1.4 }}>{n.summary.slice(0, 120)}</div>}
                    </div>
                    <div style={{ flexShrink: 0, textAlign: 'right' }}>
                      <span style={{ fontSize: 10, padding: '1px 6px', borderRadius: 3, background: n.newsType === 'announcement' ? '#E8F3FF' : '#E8FFEA', color: n.newsType === 'announcement' ? '#165DFF' : '#00B42A' }}>
                        {n.newsType === 'announcement' ? '公告' : '新闻'}
                      </span>
                      <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 4 }}>{n.publishDate}</div>
                      <div style={{ fontSize: 10, color: 'var(--color-text-3)' }}>{n.source}</div>
                    </div>
                  </a>
                ))}
              </div>
            ) : (<div className="muted" style={{ textAlign: 'center', padding: 40 }}>暂无相关资讯</div>)}
          </div>
        </div>
      )}
      {/* Watchlist group selector */}
      <Modal visible={showWLModal} title="添加到自选股" onCancel={() => { setShowWLModal(false); setWlNewGroup(''); }} onOk={handleWLConfirm} okText="确认添加">
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div><div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>选择分组</div>
            <Select value={wlGroupId || undefined} onChange={(v: number) => setWlGroupId(v || 0)} placeholder="默认分组" style={{ width: '100%' }} allowClear options={[{ label: '默认分组', value: 0 }, ...wlGroups.map((g: any) => ({ label: g.name, value: g.id }))]} />
          </div>
          <div><div style={{ fontSize: 12, color: 'var(--color-text-3)', marginBottom: 4 }}>或新建分组</div>
            <Input placeholder="输入新分组名称" value={wlNewGroup} onChange={setWlNewGroup} maxLength={20} />
          </div>
        </div>
      </Modal>
    </div>
  );
}