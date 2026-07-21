import { useEffect, useState, useMemo } from 'react';
import ReactMarkdown from 'react-markdown';
import { useNavigate } from 'react-router-dom';
import { DatePicker, Spin, Tag, Tooltip, Select } from '@arco-design/web-react';
import { Activity, TrendingUp, DollarSign, Shield, Zap, Info, ChevronRight, BarChart3, GanttChart, Target, RefreshCw, Building2 } from 'lucide-react';
import { fetchMarketStyleCurve, fetchMarketDailyReview, fetchMarketAIInterpretation } from '../services/api';

const STYLE_COLORS: Record<string, string> = {
  broad_rally: '#F7BA1E', trend_up: '#00B42A', structural: '#FF7D00',
  choppy: '#E6A23C', weak_range: '#D97A1E', decline: '#F53F3F', crash: '#1A1A1A',
};
const STYLE_LABELS: Record<string, string> = {
  broad_rally: '普涨行情', trend_up: '趋势上行', structural: '结构行情',
  choppy: '震荡博弈', weak_range: '弱势震荡', decline: '持续下跌', crash: '恐慌崩盘',
};
const STYLE_ACTIONS: Record<string, string> = {
  broad_rally: '重仓做多', trend_up: '正常做多', structural: '精选个股',
  choppy: '轻仓波段', weak_range: '观望为主', decline: '减仓/空仓', crash: '清仓避险',
};
const STYLE_ROWS = Object.entries(STYLE_LABELS).map(([key, label]) => ({ key, label }));

/** Maps legacy style codes to current ones */

const REGIME_COLORS: Record<string, string> = {
  expansion: '#00B42A', neutral: '#86909C', contraction: '#F53F3F',
};
const REGIME_LABELS: Record<string, string> = {
  expansion: '上涨格局', neutral: '震荡格局', contraction: '下跌格局',
};
const REGIME_ADVICE: Record<string, string> = {
  expansion: '顺势做多，仓位70-90%', neutral: '高抛低吸，仓位40-60%', contraction: '防守观望，仓位10-30%',
};
const normalizeStyle = (s: string) => {
  const m: Record<string, string> = {
    bull_rally: 'trend_up', mild_bull: 'trend_up',
    recovery: 'choppy', bear: 'decline', divergence: 'decline',
    transitional: 'choppy', rotation: 'choppy',
    risk_off: 'decline', bottoming: 'weak_range',
  };
  return m[s] || s;
};

/** Returns today's date as YYYY-MM-DD. */
const getToday = () => {
  const d = new Date();
  return d.toISOString().slice(0, 10);
};

type ViewMode = 'grid' | 'bar';

export default function MarketStylePage() {
  const [loading, setLoading] = useState(true);
  const [curveData, setCurveData] = useState<any[]>([]);
  const [review, setReview] = useState<any>(null);
  const [reviewLoading, setReviewLoading] = useState(false);
  const [aiLoading, setAiLoading] = useState(false);
  const [aiInterpretation, setAiInterpretation] = useState<string | null>(null);
  const [selectedDate, setSelectedDate] = useState<string>('');
  const [viewMode, setViewMode] = useState<ViewMode>('grid');
  const navigate = useNavigate();

  const loadCurve = async () => {
    try {
      const to = getToday();
      const from = new Date(Date.now() - 180 * 86400000).toISOString().slice(0, 10);
      const res: any = await fetchMarketStyleCurve({ from, to });
      const data = res.data?.data || [];
      setCurveData(data);
      if (data.length > 0) {
        const latest = data[data.length - 1].tradeDate;
        setSelectedDate(latest);
        loadReview(latest);
      }
    } catch (e) { console.error(e); }
    setLoading(false);
  };

  const loadReview = async (date: string) => {
    setReviewLoading(true);
    try {
      const res: any = await fetchMarketDailyReview({ date });
      setReview(res.data?.data || null);
    } catch (e) { console.error(e); setReview(null); }
    setReviewLoading(false);
  };

  const triggerAIInterpretation = async () => {
    if (!selectedDate || aiLoading) return;
    setAiLoading(true);
    setAiInterpretation(null);
    try {
      const res: any = await fetchMarketAIInterpretation({ date: selectedDate });
      const text = res.data?.data?.text || '';
      setAiInterpretation(text);
    } catch (e) {
      console.error('[AI] interpretation failed:', e);
    }
    setAiLoading(false);
  };

  useEffect(() => { loadCurve(); }, []);

  // Available dates set for DatePicker validation
  const dateSet = useMemo(() => new Set(curveData.map(d => d.tradeDate)), [curveData]);

  // Segments: consecutive same-style runs (computed from curveData, authoritative for duration)
  const segments = useMemo(() => {
    const segs: { style: string; startDate: string; endDate: string; duration: number }[] = [];
    if (curveData.length === 0) return segs;
    let start = curveData[0];
    for (let i = 1; i <= curveData.length; i++) {
      if (i === curveData.length || curveData[i].style !== start.style) {
        segs.push({ style: start.style, startDate: start.tradeDate, endDate: curveData[i - 1].tradeDate, duration: i - curveData.indexOf(start) });
        if (i < curveData.length) start = curveData[i];
      }
    }
    return segs;
  }, [curveData]);

  // Current style metadata (from segments, authoritative)
  const currentMeta = useMemo(() => {
    if (segments.length === 0) return null;
    const cur = segments[segments.length - 1];
    return { style: cur.style, duration: cur.duration, startDate: cur.startDate };
  }, [segments]);

  // Meaningful switches (from side lasted 3+ days)
  const meaningfulSwitches = useMemo(() => {
    const sw: { date: string; from: string; to: string }[] = [];
    for (let i = 1; i < segments.length; i++) {
      if (segments[i - 1].duration >= 3) {
        sw.push({ date: segments[i].startDate, from: segments[i - 1].style, to: segments[i].style });
      }
    }
    return sw;
  }, [segments]);

  // Month grouping for switches
  const switchesByMonth = useMemo(() => {
    const map: Record<string, number> = {};
    for (const sw of meaningfulSwitches) {
      const m = sw.date.slice(0, 7);
      map[m] = (map[m] || 0) + 1;
    }
    return map;
  }, [meaningfulSwitches]);

  const formatAmount = (v: number) => {
    if (!v || v === 0) return '—';
    if (v >= 1e12) return (v / 1e12).toFixed(2) + '万亿';
    return (v / 1e8).toFixed(0) + '亿';
  };

  if (loading) return <div style={{ padding: 60, textAlign: 'center' }}><Spin size={30} /></div>;

  return (
    <div style={{ padding: '20px 24px', maxWidth: 1200, margin: '0 auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20 }}>
        <div style={{ width: 36, height: 36, borderRadius: 10, background: 'linear-gradient(135deg, #FF7D00, #F7BA1E)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Activity size={20} color="#fff" />
        </div>
        <div>
          <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700 }}>市场风格</h2>
          <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>市场风格识别 · 结构性分析 · 最新: {curveData.length > 0 ? curveData[curveData.length - 1].tradeDate : '—'}</span>
        </div>
      </div>

      {/* Layer 1: Market Regime Banner */}
      {curveData.length > 0 && (() => {
        const latest = curveData[curveData.length - 1];
        const regime = latest.marketRegime || 'neutral';
        const leadIndustry = latest.leadIndustry || '';
        const leadConcept = latest.leadConcept || '';
        const gdFlow = latest.growthDefenseFlow || 0;
        const regimeColor = REGIME_COLORS[regime] || '#86909C';
        const flowColor = gdFlow > 0.3 ? '#00B42A' : gdFlow < -0.3 ? '#F53F3F' : '#86909C';
        const flowLabel = gdFlow > 0.5 ? '进攻' : gdFlow > 0.1 ? '偏进攻' : gdFlow < -0.5 ? '防御' : gdFlow < -0.1 ? '偏防御' : '均衡';
        return (
          <div style={{ display: 'flex', gap: 10, marginBottom: 16 }}>
            {/* Regime Card */}
            <div style={{ flex: 1, background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '12px 16px', display: 'flex', alignItems: 'center', gap: 12 }}>
              <div style={{ width: 8, height: 8, borderRadius: '50%', background: regimeColor }} />
              <div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>大盘格局</div>
                <div style={{ fontSize: 15, fontWeight: 700, color: regimeColor }}>{REGIME_LABELS[regime] || regime}</div>
              </div>
              <div style={{ fontSize: 11, color: 'var(--color-text-4)', marginLeft: 8 }}>{REGIME_ADVICE[regime] || ''}</div>
            </div>
            {/* Leadership Card - split industry & concept */}
            <div style={{ flex: 1, background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '12px 16px', display: 'flex', flexDirection: 'column', gap: 6 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <Building2 size={14} style={{ color: leadIndustry ? '#165DFF' : 'var(--color-text-4)' }} />
                <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>领涨行业</span>
                <span style={{ fontSize: 13, fontWeight: 600, color: leadIndustry ? '#165DFF' : 'var(--color-text-3)' }}>
                  {leadIndustry || '—'}
                </span>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <Target size={14} style={{ color: leadConcept ? '#FF7D00' : 'var(--color-text-4)' }} />
                <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>热门概念</span>
                <span style={{ fontSize: 13, fontWeight: 600, color: leadConcept ? '#FF7D00' : 'var(--color-text-3)' }}>
                  {leadConcept || '—'}
                </span>
              </div>
            </div>
            {/* Flow Card */}
            <div style={{ flex: 1, background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '12px 16px', display: 'flex', alignItems: 'center', gap: 12 }}>
              <RefreshCw size={18} style={{ color: flowColor }} />
              <div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>资金偏好</div>
                <div style={{ fontSize: 14, fontWeight: 600, color: flowColor, fontFamily: "'SF Mono',monospace" }}>
                  {gdFlow > 0 ? '+' : ''}{gdFlow.toFixed(2)}% <span style={{ fontSize: 11, fontWeight: 400 }}>{flowLabel}</span>
                </div>
              </div>
            </div>
          </div>
        );
      })()}

      {/* Style Timeline */}
      <div style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '16px 18px', marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
          <TrendingUp size={16} style={{ color: '#FF7D00' }} />
          <span style={{ fontSize: 14, fontWeight: 600 }}>风格时间轴</span>
          <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
            {curveData[0]?.tradeDate} → {curveData[curveData.length - 1]?.tradeDate} · {curveData.length}天
          </span>
          <span style={{ fontSize: 11, color: 'var(--color-text-4)' }}>|</span>
          <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
            {meaningfulSwitches.length} 次有效切换
          </span>
          <div style={{ flex: 1 }} />
          {/* View toggle */}
          <div style={{ display: 'flex', background: 'var(--color-fill-1)', borderRadius: 6, padding: 2 }}>
            {([
              { key: 'grid' as ViewMode, icon: <BarChart3 size={13} />, label: '网格' },
              { key: 'bar' as ViewMode, icon: <GanttChart size={13} />, label: '时间轴' },
            ]).map(v => (
              <div key={v.key} onClick={() => setViewMode(v.key)} style={{
                display: 'flex', alignItems: 'center', gap: 3, padding: '3px 8px', borderRadius: 4, cursor: 'pointer',
                fontSize: 11, color: viewMode === v.key ? '#fff' : 'var(--color-text-2)',
                background: viewMode === v.key ? 'var(--color-primary)' : 'transparent',
              }}>
                {v.icon} {v.label}
              </div>
            ))}
          </div>
        </div>

        {viewMode === 'grid' ? (
          /* Grid view: one row per style */
          <div style={{ overflowX: 'auto' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 2, minWidth: Math.max(curveData.length * 8, 600) }}>
              {STYLE_ROWS.map(row => (
                <div key={row.key} style={{ display: 'flex', alignItems: 'center', height: 22 }}>
                  <span style={{ width: 68, fontSize: 10, color: 'var(--color-text-3)', flexShrink: 0, textAlign: 'right', paddingRight: 6 }}>
                    {row.label}
                  </span>
                  <div style={{ display: 'flex', flex: 1, height: 16 }}>
                    {segments.map((seg, i) => (
                      <Tooltip key={i} content={`${row.label}: ${seg.startDate} → ${seg.endDate} (${seg.duration}天)`}>
                        <div onClick={() => { setSelectedDate(seg.startDate); loadReview(seg.startDate); }}
                          style={{
                            width: `${(seg.duration / curveData.length) * 100}%`, height: '100%',
                            background: seg.style === row.key ? STYLE_COLORS[row.key] || '#86909C' : 'transparent',
                            borderRadius: seg.style === row.key ? 3 : 0,
                            cursor: seg.style === row.key ? 'pointer' : 'default',
                            opacity: seg.style === row.key ? 0.85 : 0,
                          }}
                        />
                      </Tooltip>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        ) : (
          /* Bar view: horizontal timeline with one bar */
          <div style={{ overflowX: 'auto' }}>
            <div style={{ display: 'flex', height: 36, borderRadius: 6, overflow: 'hidden', minWidth: Math.max(curveData.length * 8, 600) }}>
              {segments.map((seg, i) => (
                <Tooltip key={i} content={`${STYLE_LABELS[normalizeStyle(seg.style)] || seg.style}\n${seg.startDate} → ${seg.endDate}\n持续 ${seg.duration} 天`}>
                  <div onClick={() => { setSelectedDate(seg.startDate); loadReview(seg.startDate); }}
                    style={{
                      width: `${(seg.duration / curveData.length) * 100}%`, height: '100%',
                      background: STYLE_COLORS[normalizeStyle(seg.style)] || '#86909C', cursor: 'pointer',
                      display: 'flex', alignItems: 'center', justifyContent: 'center',
                    }}>
                    {seg.duration >= 5 && (
                      <span style={{ fontSize: 10, color: '#fff', fontWeight: 600, textShadow: '0 1px 2px rgba(0,0,0,0.3)' }}>
                        {STYLE_LABELS[seg.style]} {seg.duration}d
                      </span>
                    )}
                  </div>
                </Tooltip>
              ))}
            </div>
            {/* Legend */}
            <div style={{ display: 'flex', gap: 10, marginTop: 8, flexWrap: 'wrap' }}>
              {segments.filter(s => s.duration >= 5).map((s, i) => (
                <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 10, color: 'var(--color-text-3)' }}>
                  <div style={{ width: 10, height: 10, borderRadius: 2, background: STYLE_COLORS[normalizeStyle(s.style)], flexShrink: 0 }} />
                  {s.startDate.slice(5)}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Switch summary by month */}
        <div style={{ marginTop: 8, fontSize: 11, color: 'var(--color-text-3)', display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
          {Object.entries(switchesByMonth).map(([m, c]) => (
            <span key={m} style={{ whiteSpace: 'nowrap' }}>
              <span style={{ color: 'var(--color-text-4)' }}>{m.slice(5)}月</span> {c}次
            </span>
          ))}
        </div>
      </div>

      {/* Daily Review */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
        {/* Date selector + style header */}
        <div style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '14px 18px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
            <DatePicker value={selectedDate} onChange={(v: string) => { if (v && dateSet.has(v)) { setSelectedDate(v); loadReview(v); } }}
              disabledDate={(current: any) => {
                const ds = current?.format?.('YYYY-MM-DD') || '';
                if (ds > getToday()) return true;
                return !dateSet.has(ds);
              }}
              style={{ width: 160 }} size="small" />
            {reviewLoading ? <Spin size={16} /> : review ? (
              <>
                {review.style && (
                  <Tag color={['broad_rally','trend_up'].includes(normalizeStyle(review.style)) ? 'green'
                    : ['decline','crash'].includes(normalizeStyle(review.style)) ? 'red'
                    : ['weak_range'].includes(normalizeStyle(review.style)) ? 'orangered'
                    : 'orange'} size="medium">
                    {review.styleName || review.style}
                  </Tag>
                )}
                <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>
                  置信度 {review.styleConfidence || 0}% · 已持续 {currentMeta?.duration || review.styleDuration || 0} 天
                </span>
                {review.transitionSignal && review.transitionSignal !== 'none' && (
                  <Tag size="small" color={review.transitionSignal === 'warming' ? 'green' : review.transitionSignal === 'cooling' ? 'orangered' : 'red'}>
                    {{warming:'🔥 回暖',cooling:'❄ 冷却',reversal:'⚠ 切换'}[review.transitionSignal] || review.transitionSignal}
                  </Tag>
                )}
              </>
            ) : (
              <span style={{ fontSize: 12, color: 'var(--color-text-4)' }}>该日无风格数据</span>
            )}
          </div>
        </div>

        {review && (
          <>
            {/* Key indicators */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 10 }}>
              <IndicatorCard icon={<TrendingUp size={16} />} color="#00B42A"
                label="涨跌比" value={`${review.upCount || 0}:${review.downCount || 0}`}
                sub={`${((review.upRatio || 0) * 100).toFixed(0)}% 上涨`} />
              <IndicatorCard icon={<Zap size={16} />} color="#F53F3F"
                label="涨跌停" value={`涨停${review.limitUpCount || 0} · 跌停${review.limitDownCount || 0}`}
                sub={(review.limitUpCount || 0) > (review.limitDownCount || 0) ? '多方占优' : '空方占优'} />
              <IndicatorCard icon={<DollarSign size={16} />} color="#165DFF"
                label="成交额" value={formatAmount(review.totalAmount)}
                sub={review.prevAmount ? `环比 ${(((review.totalAmount - review.prevAmount) / review.prevAmount) * 100).toFixed(0)}%` : '—'} />
              <IndicatorCard icon={<Shield size={16} />} color="#FF7D00"
                label="站上MA20" value={`${review.ma20Above || 0}家`}
                sub={`${review.totalStocks ? ((review.ma20Above || 0) / review.totalStocks * 100).toFixed(0) : 0}% · 新低${review.n60Low || 0}家`} />
            </div>

            {/* Capital flow + Structure */}
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
              <div style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '14px 18px' }}>
                <DollarSign size={14} style={{ marginRight: 6, color: '#165DFF' }} />
                <span style={{ fontSize: 13, fontWeight: 600 }}>资金面</span>
                <div style={{ marginTop: 8 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                    <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>北向资金</span>
                    <span style={{ fontSize: 13, fontWeight: 600, fontFamily: "'SF Mono',monospace",
                      color: (review.northboundNet || 0) >= 0 ? 'var(--stock-up)' : 'var(--stock-down)' }}>
                      {(review.northboundNet || 0) > 0 ? '+' : ''}{(review.northboundNet || 0).toFixed(1)}亿
                    </span>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>新高/新低</span>
                    <span style={{ fontSize: 12, fontFamily: "'SF Mono',monospace" }}>
                      <span style={{ color: '#F53F3F' }}>{review.n52High || 0}家新高</span>
                      {' · '}
                      <span style={{ color: '#00B42A' }}>{review.n60Low || 0}家新低</span>
                    </span>
                  </div>
                </div>
              </div>
              <div style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '14px 18px' }}>
                <Activity size={14} style={{ marginRight: 6, color: '#FF7D00' }} />
                <span style={{ fontSize: 13, fontWeight: 600 }}>结构与体制</span>
                <div style={{ marginTop: 8 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                    <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>大盘格局</span>
                    <span style={{ fontSize: 12, fontWeight: 600, color: REGIME_COLORS[review.marketRegime] || '#86909C' }}>
                      {REGIME_LABELS[review.marketRegime] || '—'}
                    </span>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                    <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>板块扩散度</span>
                    <span style={{ fontSize: 13, fontFamily: "'SF Mono',monospace" }}>
                      {((review.sectorDiffusion || 0) * 100).toFixed(1)}%
                    </span>
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>资金偏好</span>
                    <span style={{ fontSize: 13, fontFamily: "'SF Mono',monospace",
                      color: (review.growthDefenseFlow || 0) > 0 ? '#00B42A' : (review.growthDefenseFlow || 0) < 0 ? '#F53F3F' : 'var(--color-text-1)' }}>
                      {(review.growthDefenseFlow || 0) > 0 ? '+' : ''}{(review.growthDefenseFlow || 0).toFixed(2)}%
                    </span>
                  </div>
                </div>
              </div>
            </div>

            {/* Micro Structure Indicators */}
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 10 }}>
              <div style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '12px 14px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
                  <Zap size={13} style={{ color: '#FF7D00' }} />
                  <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>炸板率</span>
                </div>
                <span style={{ fontSize: 16, fontWeight: 700, fontFamily: "'SF Mono',monospace",
                  color: (review.breakRate || 0) > 0.3 ? '#F53F3F' : 'var(--color-text-1)' }}>
                  {((review.breakRate || 0) * 100).toFixed(1)}%
                </span>
                <div style={{ fontSize: 10, color: 'var(--color-text-4)', marginTop: 2 }}>
                  {(review.breakRate || 0) > 0.3 ? '封板意愿弱' : '封板正常'}
                </div>
              </div>
              <div style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '12px 14px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
                  <Target size={13} style={{ color: '#165DFF' }} />
                  <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>资金集中度</span>
                </div>
                <span style={{ fontSize: 16, fontWeight: 700, fontFamily: "'SF Mono',monospace" }}>
                  {((review.concentration || 0) * 100).toFixed(1)}%
                </span>
                <div style={{ fontSize: 10, color: 'var(--color-text-4)', marginTop: 2 }}>
                  Top100成交占比
                </div>
              </div>
              <div style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '12px 14px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
                  <RefreshCw size={13} style={{ color: '#F7BA1E' }} />
                  <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>轮动速度</span>
                </div>
                <span style={{ fontSize: 16, fontWeight: 700, fontFamily: "'SF Mono',monospace",
                  color: (review.rotationSpeed || 0) > 0.6 ? '#F53F3F' : 'var(--color-text-1)' }}>
                  {((review.rotationSpeed || 0) * 100).toFixed(0)}%
                </span>
                <div style={{ fontSize: 10, color: 'var(--color-text-4)', marginTop: 2 }}>
                  Top5行业更换率
                </div>
              </div>
            </div>

            {/* AI Analysis Summary */}
            <div style={{ background: 'linear-gradient(135deg, #165DFF10, #722ED110)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '14px 18px' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <Zap size={14} style={{ color: '#722ED1' }} />
                  <span style={{ fontSize: 13, fontWeight: 600, color: '#722ED1' }}>AI 市场解读</span>
                  {selectedDate && (
                    <span style={{ fontSize: 10, color: 'var(--color-text-4)', marginLeft: 4 }}>{selectedDate}</span>
                  )}
                </div>
                <button
                  onClick={triggerAIInterpretation}
                  disabled={aiLoading || !selectedDate}
                  style={{
                    display: 'inline-flex', alignItems: 'center', gap: 4,
                    padding: '4px 10px', borderRadius: 6, border: '1px solid #722ED140',
                    background: aiLoading ? 'var(--color-fill-2)' : '#722ED110',
                    color: aiLoading ? 'var(--color-text-3)' : '#722ED1',
                    fontSize: 11, fontWeight: 500, cursor: aiLoading ? 'not-allowed' : 'pointer',
                  }}
                >
                  <RefreshCw size={12} style={{ animation: aiLoading ? 'spin 1s linear infinite' : 'none' }} />
                  {aiLoading ? '生成中...' : '刷新 AI 分析'}
                </button>
              </div>
              {(aiInterpretation || review.analysisSummary) ? (
                <div style={{ fontSize: 13, lineHeight: 1.8, color: 'var(--color-text-2)' }}>
                  <ReactMarkdown
                    components={{
                      h2: ({ children }: any) => <div style={{ fontSize: 14, fontWeight: 700, color: '#722ED1', marginTop: 12, marginBottom: 6, paddingBottom: 4, borderBottom: '1px solid var(--color-border-1)' }}>{children}</div>,
                      h3: ({ children }: any) => <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)', marginTop: 8, marginBottom: 4 }}>{children}</div>,
                      p: ({ children }: any) => <p style={{ margin: '4px 0', fontSize: 13 }}>{children}</p>,
                      ul: ({ children }: any) => <ul style={{ margin: '4px 0', paddingLeft: 18 }}>{children}</ul>,
                      ol: ({ children }: any) => <ol style={{ margin: '4px 0', paddingLeft: 18 }}>{children}</ol>,
                      li: ({ children }: any) => <li style={{ margin: '2px 0', fontSize: 13 }}>{children}</li>,
                      strong: ({ children }: any) => <strong style={{ color: 'var(--color-text-1)', fontWeight: 700 }}>{children}</strong>,
                      code: ({ children }: any) => <code style={{ background: 'var(--color-fill-2)', padding: '1px 5px', borderRadius: 3, fontSize: 12 }}>{children}</code>,
                    }}
                  >
                    {aiInterpretation || review.analysisSummary || ''}
                  </ReactMarkdown>
                </div>
              ) : aiLoading ? (
                <div style={{ fontSize: 12, color: 'var(--color-text-3)', padding: '12px 0', textAlign: 'center' }}>
                  <Spin size={14} style={{ marginRight: 8 }} />
                  AI 正在分析市场数据，请稍候...
                </div>
              ) : (
                <div style={{ fontSize: 12, color: 'var(--color-text-4)', padding: '8px 0', textAlign: 'center' }}>
                  暂无 AI 分析，点击「刷新 AI 分析」按钮生成
                </div>
              )}
            </div>

            {/* Top Concepts */}
            {review.topConcepts && Array.isArray(review.topConcepts) && review.topConcepts.length > 0 && (
              <div style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '14px 18px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
                  <TrendingUp size={16} style={{ color: '#FF7D00' }} />
                  <span style={{ fontSize: 14, fontWeight: 600 }}>强势概念</span>
                  <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>
                    {review.topConcepts.filter((c: any) => c.consecutiveDays >= 5).length} 个持续抱团
                  </span>
                </div>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                  {review.topConcepts.slice(0, 15).map((c: any, i: number) => (
                    <Tooltip key={i} content={`涨幅 ${c.chgPct}% · 上涨比 ${(c.upRatio * 100).toFixed(0)}% · 放量 ${c.volRatio}x`}>
                      <div style={{
                        padding: '6px 10px', borderRadius: 6, fontSize: 11,
                        background: c.consecutiveDays >= 5 ? '#FF7D0018' : c.consecutiveDays >= 3 ? '#F7BA1E15' : 'var(--color-fill-1)',
                        border: `1px solid ${c.consecutiveDays >= 5 ? '#FF7D0040' : c.consecutiveDays >= 3 ? '#F7BA1E30' : 'var(--color-border-1)'}`,
                        cursor: 'default',
                      }}>
                        <span style={{ fontWeight: 600, cursor: 'pointer' }} onClick={(e) => { e.stopPropagation(); if (c.code) navigate('/concept/' + c.code); }} title={c.code ? '点击查看详情' : ''}>{c.name}</span>
                        <span style={{ marginLeft: 6, fontFamily: "'SF Mono',monospace", color: c.chgPct > 0 ? 'var(--stock-up)' : 'var(--stock-down)' }}>
                          {c.chgPct > 0 ? '+' : ''}{c.chgPct}%
                        </span>
                        {c.consecutiveDays >= 5 && <span style={{ marginLeft: 4, fontSize: 10, color: '#FF7D00' }}>🔥{c.consecutiveDays}d</span>}
                        {c.consecutiveDays >= 3 && c.consecutiveDays < 5 && <span style={{ marginLeft: 4, fontSize: 10, color: '#F7BA1E' }}>⚡{c.consecutiveDays}d</span>}
                      </div>
                    </Tooltip>
                  ))}
                </div>
              </div>
            )}

            {/* Top Sectors */}
            {review.topSectors && Array.isArray(review.topSectors) && review.topSectors.length > 0 && (
              <div style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '14px 18px' }}>
                <Info size={16} style={{ marginRight: 6, color: '#165DFF' }} />
                <span style={{ fontSize: 14, fontWeight: 600, marginLeft: 4 }}>行业板块</span>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 8 }}>
                  {review.topSectors.slice(0, 10).map((s: any, i: number) => (
                    <div key={i} style={{
                      padding: '4px 10px', borderRadius: 5, fontSize: 11,
                      background: s.chgPct > 0 ? '#00B42A15' : '#F53F3F15',
                      border: `1px solid ${s.chgPct > 0 ? '#00B42A30' : '#F53F3F30'}`,
                      fontFamily: "'SF Mono',monospace",
                    }}>
                      {s.name} {s.chgPct > 0 ? '+' : ''}{s.chgPct}%
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Operation Advice */}
            {review.operationAdvice && review.operationAdvice.length > 0 && (
              <div style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '14px 18px' }}>
                <ChevronRight size={16} style={{ color: '#165DFF', marginRight: 6 }} />
                <span style={{ fontSize: 14, fontWeight: 600 }}>操作建议</span>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4, marginTop: 8 }}>
                  {review.operationAdvice.map((a: string, i: number) => (
                    <div key={i} style={{ fontSize: 12, color: 'var(--color-text-2)', paddingLeft: 8, borderLeft: '2px solid var(--color-primary)' }}>
                      {a}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

function IndicatorCard({ icon, color, label, value, sub }: {
  icon: React.ReactNode; color: string; label: string; value: string; sub: string;
}) {
  return (
    <div style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)', padding: '12px 14px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
        <span style={{ color }}>{icon}</span>
        <span style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{label}</span>
      </div>
      <div style={{ fontSize: 16, fontWeight: 700, fontFamily: "'SF Mono',monospace" }}>{value}</div>
      <div style={{ fontSize: 10, color: 'var(--color-text-4)', marginTop: 2 }}>{sub}</div>
    </div>
  );
}
