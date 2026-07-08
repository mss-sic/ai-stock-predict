import { useState, useEffect } from 'react';
import { Table, Button, Card, Spin, Pagination, Drawer, Tag, Tooltip, Empty, Message, Popconfirm } from '@arco-design/web-react';
import {
  ShieldAlert, AlertTriangle, Info, TrendingUp, Activity,
  Eye, CheckCircle, Zap, Gauge, Search, RefreshCw, XCircle,
  BarChart3, Target, Layers,
} from 'lucide-react';
import {
  fetchRiskDashboard, fetchRiskAlertList, fetchRiskSnapshots,
  acknowledgeRiskAlert, triggerRiskScan,
} from '../services/api';

// ── Constants ──
const levelBadge: Record<string, { color: string; bg: string; label: string; icon: any }> = {
  high:   { color: '#f53f3f', bg: '#f53f3f14', label: '高风险', icon: AlertTriangle },
  medium: { color: '#ff7d00', bg: '#ff7d0014', label: '中风险', icon: ShieldAlert },
  low:    { color: '#00b42a', bg: '#00b42a14', label: '低风险', icon: Info },
};

const dimLabel: Record<string, string> = {
  market: '市场', stock: '个股', portfolio: '组合', liquidity: '流动性', event: '事件', behavior: '行为',
};

const dimColor: Record<string, string> = {
  market: '#165DFF', stock: '#722ED1', portfolio: '#14C9C9', liquidity: '#FF7D00', event: '#F53F3F', behavior: '#00B42A',
};

const mktGauge: Record<string, { color: string; bg: string; text: string; desc: string }> = {
  low:      { color: '#00b42a', bg: '#00b42a10', text: '低风险', desc: '市场运行平稳，正常操作' },
  medium:   { color: '#ff7d00', bg: '#ff7d0010', text: '中风险', desc: '关注市场变化，谨慎加仓' },
  high:     { color: '#f53f3f', bg: '#f53f3f10', text: '高风险', desc: '建议减仓观望，控制敞口' },
  critical: { color: '#cb2ecb', bg: '#cb2ecb10', text: '危险', desc: '触发熔断！暂停所有买入' },
};

// ── Rule explanations ──
interface RuleInfo {
  source: string;   // 数据来源
  method: string;   // 计算方式
  meaning: string;  // 风控意义
  threshold: string; // 阈值说明
}

const ruleExplain: Record<string, RuleInfo> = {
  // ── Market ──
  fear_greed_overheat: { source: 'market_sentiment 表（恐贪指数）', method: '检测恐贪指数连续 ≥3 日超过 80', meaning: '市场极度亢奋，历史上往往是阶段性顶部，应全线减仓防御', threshold: 'threshold=80, consecutive_days=3' },
  market_breadth_decay: { source: 'market_style_daily 表 + stocks_daily_k 表', method: '全市场站上 MA20 股票数 / 总股票数 < 30%', meaning: '少数股票支撑指数，市场宽度恶化，系统性下跌风险增大', threshold: 'threshold=0.30（30%）' },
  northbound_outflow_streak: { source: 'northbound_daily_view 视图', method: '北向资金连续 ≥5 日净流出', meaning: '北向资金被称为"聪明钱"，连续流出预示外资看空 A 股', threshold: 'threshold_days=5' },
  volatility_spike: { source: 'stocks_daily_k 表（高/低/前收）', method: '当日全市场振幅中位数 vs 历史 60 日 90 分位值', meaning: '振幅飙升 → 多空分歧剧烈，持仓风险急剧上升', threshold: 'percentile=0.90' },

  // ── Stock ──
  heavy_volume_drop: { source: 'stocks_daily_k（涨跌幅）+ stocks_daily_indicator（量比）', method: '最新交易日跌幅 ≤-5% 且量比 ≥2.0', meaning: '放量下跌 = 主力资金出逃经典信号，量大抛压强，后续继续跌', threshold: 'drop_pct=-5, volume_ratio=2.0' },
  shrinking_rebound: { source: 'stocks_daily_k 表（收盘价 + 成交量）', method: '连续 3 日价格上涨但成交量逐日递减', meaning: '缩量反弹 = 假反弹，上涨缺乏资金支撑，容易被空头反扑', threshold: 'rebound_days=3' },
  ma_bearish_alignment: { source: 'stocks_daily_k 表（收盘价计算均线）', method: '窗口函数计算 MA5/MA10/MA20/MA60，检测 MA5<MA10<MA20<MA60', meaning: '均线空头排列 = 趋势全面走弱，短中长期均线向下压制', threshold: 'buffer_pct=0.02' },
  rsi_overbought: { source: 'stocks_daily_k 表（收盘价计算 RSI）', method: 'RSI = 100-[100/(1+RS)], 14 日周期计算, 检测 >80', meaning: 'RSI>80 = 短期涨幅过大，买方力量衰竭，回调概率高', threshold: 'threshold=80, period=14' },
  macd_divergence: { source: 'stocks_daily_k 表（收盘价计算 MACD）', method: 'EMA12-EMA26 得 DIF，30 日内价格新高但 MACD 值未新高', meaning: '顶背离 = 强反转信号，价格创新高但动能已在衰竭，是多头陷阱', threshold: 'lookback=60' },
  bollinger_squeeze: { source: 'stocks_daily_k 表（收盘价计算布林带）', method: '带宽 = (上轨-下轨)/中轨，低于历史 20 分位值预警', meaning: '布林带收窄 = 波动率极低，通常是大行情前兆，方向选择在即', threshold: 'percentile=0.20, period=20' },
  turnover_abnormal: { source: 'stocks_daily_indicator 表（换手率）', method: '换手率 >20%（异常活跃）或 <0.1%（僵尸股）', meaning: '异常高换手可能对倒出货；异常低换手则流动性枯竭', threshold: 'high=20, low=0.1' },
  major_outflow_streak: { source: 'stock_capital_flow 表（主力资金流向）', method: '主力净流出（超大单+大单）连续 ≥5 日', meaning: '主力资金连续流出 = 机构在撤退，散户接盘往往亏损', threshold: 'days=5' },
  margin_collapse: { source: 'margin_trading 表（融资余额）', method: '近 5 日融资余额变化率 < -10%', meaning: '融资余额骤降 = 杠杆资金恐慌出逃，可能引发踩踏式下跌', threshold: 'days=5, drop_pct=-10' },
  block_discount: { source: 'block_trade 表（大宗交易）', method: '最新大宗交易成交价较收盘价折价 >8%', meaning: '大宗大幅折价 = 股东急于套现，接盘方要求补偿流动性风险', threshold: 'discount_pct=-8' },
  dragon_institution_sell: { source: 'dragon_tiger_detail 表（龙虎榜明细）', method: '机构席位卖出额 / 买入额 >2.0 倍', meaning: '龙虎榜机构净卖出 = 专业机构在撤退，后市大概率下跌', threshold: 'sell_buy_ratio=2.0' },
  st_delist_risk: { source: 'stocks_basic 表（股票名称）', method: '名称含"ST"或"退"即触发', meaning: 'ST 股票面临退市风险，涨跌幅限制 5% 且流动性差', threshold: '无（名称匹配触发）' },
  ai_score_crash: { source: 'ai_stock_scores 表（AI 综合评分）', method: '3 日内 AI 评分下降 >2 分', meaning: 'AI 评分骤降通常预示基本面或技术面恶化，是领先指标', threshold: 'days=3, drop=2.0' },
  sharp_decline: { source: 'stocks_daily_k 表（涨跌幅序列）', method: '近 5 日累计跌幅 > -8%', meaning: '短期大幅下跌 = 趋势已转空头，应止损或减仓', threshold: 'days=5, drop_pct=-8' },
  ma20_breakdown: { source: 'stocks_daily_k 表（收盘价 + MA20）', method: '今日收盘 < MA20 且昨日收盘 ≥ MA20（下穿均线）', meaning: '跌破 MA20 = 中期趋势可能转空，是经典减仓信号', threshold: 'buffer_pct=0.02' },
  pe_extreme: { source: 'stocks_daily_indicator（PE）+ stocks_basic（行业）', method: '个股 PE vs 同行业均值，PE>200 高风险, >100 中风险', meaning: '市盈率远超同行 = 估值泡沫，业绩不达预期将戴维斯双杀', threshold: 'pe_high=200, pe_warn=100' },
  profit_decline: { source: 'stock_financials 表（利润/营收增长率）', method: '最新财报净利润同比增长率 <-50% 高风险, <-30% 中风险', meaning: '业绩是股价基石，利润大幅下滑将导致估值下修和抛售', threshold: 'decline_pct=-50, warn_pct=-30' },

  // ── Portfolio ──
  industry_concentration: { source: 'holdings 表（持仓）+ stocks_basic（行业）', method: '按用户+策略分组，单一行业市值占比 >40%', meaning: '行业过度集中无法分散风险，政策利空时损失惨重', threshold: 'max_pct=0.40（40%）' },
  correlation_high: { source: 'stocks_daily_k 表（日收益率序列）', method: 'Pearson 相关系数，成对计算 >0.70', meaning: '持仓高度相关 = 同涨同跌，丧失分散化优势，风险敞口放大', threshold: 'threshold=0.70, min_history_days=60' },
  var_breach: { source: 'stocks_daily_k（持仓权重 + 收益率）', method: '历史模拟法，95% 置信度日 VaR > 总资产 5%', meaning: 'VaR 超限 = 正常市场下可能损失超承受能力，需降仓位', threshold: 'confidence=0.95, max_var_pct=0.05' },
  position_overlimit: { source: 'holdings 表 + strategy_fund_allocations 表', method: '总持仓市值 /（现金+持仓） > 80%', meaning: '仓位过重 = 无子弹应对波动，下跌无法补仓摊薄成本', threshold: 'max_total_pct=0.80（80%）' },

  // ── Liquidity ──
  volume_too_low: { source: 'stocks_daily_k 表（成交额 amount）', method: '近 5 日均成交额 <2000 万', meaning: '成交额过低 = 想卖时无对手盘，或卖出会砸低股价', threshold: 'avg_days=5, min_amount=2000万' },
  limit_down_locked: { source: 'stocks_daily_k + stocks_daily_indicator（换手率）', method: '收盘价≈跌停价且换手率 <0.5%', meaning: '跌停封板 = 无法卖出，次日大概率继续跌，流动性危机极端表现', threshold: '跌停价±0.5%, 换手率<0.5%' },
  turnover_decay: { source: 'stocks_daily_indicator 表（换手率）', method: '近 30 日均换手率 <0.5% 且趋势向下', meaning: '换手率持续走低 = 市场关注度下降，被边缘化，可能长期阴跌', threshold: 'days=30, min_turnover=0.005' },

  // ── Event ──
  major_reduction: { source: 'cninfo_announcements 表（巨潮公告标题）', method: '近 30 天标题含"减持/股份变动/权益变动/转让"', meaning: '大股东减持 = 最直接利空，掌握内幕的人在用脚投票', threshold: 'lookback_days=30' },
  litigation_violation: { source: 'cninfo_announcements 表（巨潮公告标题）', method: '近 30 天标题含"诉讼/违规/处罚/立案/调查"', meaning: '重大诉讼违规 = 可能导致巨额赔偿、业务停滞甚至退市', threshold: 'lookback_days=30' },
  dividend_ex_near: { source: 'dividend_history 表（除权除息日）', method: '未来 5 日内有除权除息', meaning: '除权除息日股价跳空低开（分红金额），短线持仓需关注', threshold: 'lookahead_days=5' },

  // ── Behavior ──
  overtrading: { source: 'live_trades 表（实盘成交记录）', method: '当日已成交笔数 >5 笔', meaning: '频繁交易 = 散户亏损首要原因，手续费侵蚀+情绪化决策', threshold: 'max_trades_per_day=5' },
  stop_loss_missed: { source: 'holdings（成本/现价）+ strategies（止损线）', method: '亏损超止损线+2% 容忍度但未卖出', meaning: '止损未执行 = 风控纪律失效，小亏变大亏，需立即检查', threshold: 'tolerance_pct=0.02' },
  live_backtest_divergence: { source: 'daily_portfolio_snapshots + backtest_results', method: '|实盘收益率 - 回测收益率| > 15%', meaning: '实盘与回测大幅偏离 = 市场环境已变，策略可能已失效', threshold: 'max_divergence_pct=0.15' },
};

// ── Component ──
export default function RiskDashboard() {
  const [dash, setDash] = useState<any>({});
  const [alerts, setAlerts] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);
  const [scanMsg, setScanMsg] = useState('');

  const [page, setPage] = useState(1);
  const [filterLevel, setFilterLevel] = useState('');
  const [filterDim, setFilterDim] = useState('');
  const pageSize = 15;

  const [detailAlert, setDetailAlert] = useState<any>(null);
  const [showBreakdown, setShowBreakdown] = useState(false);
  const breakdown = dash.marketRiskBreakdown || null;

  const loadAll = async () => {
    setLoading(true);
    try {
      const dashRes = await fetchRiskDashboard();
      setDash(dashRes.data?.data || {});
    } catch (e) {
      console.error('[RiskDashboard] load failed:', e);
    } finally { setLoading(false); }
  };

  const loadAlerts = async () => {
    try {
      const res: any = await fetchRiskAlertList({ page, pageSize, level: filterLevel || undefined, dimension: filterDim || undefined });
      setAlerts(res.data?.data?.list || []);
      setTotal(res.data?.data?.total || 0);
    } catch (e) { console.error('[RiskDashboard] alerts failed:', e); }
  };

  useEffect(() => { loadAll(); loadAlerts(); }, []);
  useEffect(() => { loadAlerts(); }, [page, filterLevel, filterDim]);

  const handleScan = async () => {
    setScanning(true);
    setScanMsg('');
    try {
      const res: any = await triggerRiskScan();
      const count = res.data?.data?.alertsGenerated || 0;
      setScanMsg(`扫描完成，生成 ${count} 条预警`);
      await loadAll();
      await loadAlerts();
    } catch (e) {
      setScanMsg('扫描失败，请重试');
      console.error('[RiskDashboard] scan failed:', e);
    } finally { setScanning(false); }
  };

  const handleAcknowledge = async (id: number) => {
    try {
      await acknowledgeRiskAlert(id);
      setAlerts(prev => prev.map(a => a.id === id ? { ...a, status: 'acknowledged' } : a));
      Message.success('已确认');
    } catch { Message.error('操作失败'); }
  };

  const mktLevel: string = dash.marketRiskLevel || 'low';
  const gauge = mktGauge[mktLevel] || mktGauge.low;
  const totalAlerts = (dash.highAlerts || 0) + (dash.mediumAlerts || 0) + (dash.lowAlerts || 0);

  const displayStockName = (record: any) => {
    if (record.stockName && record.stockName !== '__MARKET__' && !record.stockName.startsWith('__PORTFOLIO_')) {
      return record.stockName;
    }
    if (record.stockCode === '__MARKET__') return '全市场';
    if (record.stockCode?.startsWith('__PORTFOLIO_')) return '组合';
    return record.stockName || record.stockCode || '—';
  };

  return (
    <div style={{ padding: '0 0 24px 0' }}>
      {/* ═══════════ Hero Banner ═══════════ */}
      <div style={{
        background: `linear-gradient(135deg, ${gauge.color}12, var(--color-bg-2))`,
        border: `1px solid ${gauge.color}25`,
        borderRadius: 14, marginBottom: 20, overflow: 'hidden',
      }}>
        {/* Top row: level + scan button */}
        <div style={{
          padding: '20px 28px', display: 'flex', alignItems: 'center',
          justifyContent: 'space-between', flexWrap: 'wrap', gap: 16,
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <div style={{
              width: 52, height: 52, borderRadius: 14,
              background: `linear-gradient(135deg, ${gauge.color}, ${gauge.color}cc)`,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              boxShadow: `0 6px 20px ${gauge.color}35`,
            }}>
              <Gauge size={28} color="#fff" />
            </div>
            <div>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 10, marginBottom: 4 }}>
                <span style={{ fontSize: 28, fontWeight: 800, color: gauge.color, lineHeight: 1 }}>
                  {gauge.text}
                </span>
                <span style={{
                  fontSize: 14, fontWeight: 600, color: gauge.color,
                  fontFamily: "'SF Mono', monospace",
                  background: `${gauge.color}15`, padding: '2px 10px', borderRadius: 8,
                }}>
                  {dash.marketRiskScore || 0} 分
                </span>
              </div>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', letterSpacing: 0.5 }}>
                当前市场风险等级 · {gauge.desc}
              </div>
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            {scanMsg && (
              <span style={{ fontSize: 12, color: 'var(--color-text-2)', background: 'var(--color-fill-1)', padding: '4px 12px', borderRadius: 8 }}>
                {scanMsg}
              </span>
            )}
            <Button size="small" type="text"
              onClick={() => setShowBreakdown(!showBreakdown)}
              style={{ fontSize: 12, color: 'var(--color-text-3)' }}>
              {showBreakdown ? '收起分析' : '查看分析'}
            </Button>
            <Button
              type="primary"
              icon={scanning ? <RefreshCw size={15} className="spin-icon" /> : <Search size={15} />}
              loading={scanning}
              onClick={handleScan}
              style={{ borderRadius: 8, fontWeight: 500 }}
            >
              {scanning ? '扫描中...' : '立即扫描'}
            </Button>
          </div>
        </div>

        {/* Expandable breakdown panel */}
        {showBreakdown && breakdown && (
          <div style={{
            borderTop: `1px solid ${gauge.color}15`,
            padding: '20px 28px',
            display: 'grid', gridTemplateColumns: '1fr 1fr',
            gap: 20,
          }}>
            {/* Left: Score factors */}
            <div>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 12, color: 'var(--color-text-2)' }}>
                评分因子
              </div>
              {breakdown.factors?.map((f: any, i: number) => {
                const pct = f.max > 0 ? Math.round((f.score / f.max) * 100) : 0;
                const barColor = pct > 70 ? '#f53f3f' : pct > 40 ? '#ff7d00' : '#00b42a';
                return (
                  <div key={i} style={{ marginBottom: 10 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                      <span style={{ fontSize: 12, color: 'var(--color-text-2)' }}>{f.name}</span>
                      <span style={{ fontSize: 11, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>
                        {f.score.toFixed(1)} / {f.max}
                      </span>
                    </div>
                    <div style={{
                      height: 4, borderRadius: 2, background: 'var(--color-fill-2)',
                      overflow: 'hidden',
                    }}>
                      <div style={{
                        height: '100%', width: `${pct}%`, borderRadius: 2,
                        background: barColor, transition: 'width .4s ease',
                      }} />
                    </div>
                    <div style={{ fontSize: 10, color: 'var(--color-text-3)', marginTop: 2 }}>
                      {f.detail}
                    </div>
                  </div>
                );
              })}
            </div>

            {/* Right: Alerts + Advice */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              {/* Active market alerts */}
              {breakdown.activeAlerts && breakdown.activeAlerts.length > 0 && (
                <div>
                  <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8, color: 'var(--color-text-2)' }}>
                    活跃市场预警 ({breakdown.activeAlerts.length})
                  </div>
                  {breakdown.activeAlerts.map((a: any, i: number) => {
                    const cfg = levelBadge[a.level];
                    return (
                      <div key={i} style={{
                        padding: '8px 12px', borderRadius: 6, marginBottom: 6,
                        background: `${cfg?.color || '#86909c'}0a`,
                        border: `1px solid ${cfg?.color || '#86909c'}18`,
                        display: 'flex', alignItems: 'center', gap: 8,
                      }}>
                        <span style={{
                          fontSize: 10, fontWeight: 600, color: cfg?.color,
                          background: cfg?.bg, padding: '1px 6px', borderRadius: 6,
                          flexShrink: 0,
                        }}>
                          {cfg?.label}
                        </span>
                        <span style={{ fontSize: 12, color: 'var(--color-text-1)', fontWeight: 500 }}>{a.type}</span>
                        <span style={{ fontSize: 11, color: 'var(--color-text-3)', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {a.description}
                        </span>
                      </div>
                    );
                  })}
                </div>
              )}

              {/* Advice */}
              {breakdown.advice && (
                <div style={{
                  padding: 12, borderRadius: 8,
                  background: `${gauge.color}0d`, border: `1px solid ${gauge.color}20`,
                }}>
                  <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 6, color: gauge.color }}>
                    💡 操作建议
                  </div>
                  <div style={{ fontSize: 13, color: 'var(--color-text-1)', lineHeight: 1.7 }}>
                    {breakdown.advice}
                  </div>
                </div>
              )}

              {(!breakdown.activeAlerts || breakdown.activeAlerts.length === 0) && !breakdown.advice && (
                <div style={{ fontSize: 12, color: 'var(--color-text-3)', textAlign: 'center', padding: 20 }}>
                  暂无详细分析数据，点击「立即扫描」获取最新风险画像
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {/* ═══════════ Stat Cards ═══════════ */}
      <div style={{
        display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 14, marginBottom: 20,
      }}>
        {[
          { label: '高风险', count: dash.highAlerts || 0, color: '#f53f3f', bg: '#f53f3f0d', icon: AlertTriangle },
          { label: '中风险', count: dash.mediumAlerts || 0, color: '#ff7d00', bg: '#ff7d000d', icon: ShieldAlert },
          { label: '低风险', count: dash.lowAlerts || 0, color: '#00b42a', bg: '#00b42a0d', icon: Info },
          { label: '综合评分', count: dash.marketRiskScore || 0, color: gauge.color, bg: gauge.bg, icon: Target, isScore: true },
        ].map((card, i) => (
          <div key={i} style={{
            background: card.bg,
            border: `1px solid ${card.color}20`,
            borderRadius: 12, padding: '16px 20px',
            display: 'flex', alignItems: 'center', gap: 14,
            transition: 'box-shadow .2s',
          }}
            onMouseEnter={e => (e.currentTarget.style.boxShadow = `0 4px 16px ${card.color}15`)}
            onMouseLeave={e => (e.currentTarget.style.boxShadow = 'none')}
          >
            <div style={{
              width: 40, height: 40, borderRadius: 10,
              background: `${card.color}18`, display: 'flex',
              alignItems: 'center', justifyContent: 'center',
            }}>
              <card.icon size={20} color={card.color} />
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 2 }}>{card.label}</div>
              {(card as any).isScore ? (
                <>
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 6, marginBottom: 4 }}>
                    <span style={{
                      fontSize: 22, fontWeight: 700, color: card.color,
                      fontFamily: "'SF Mono', 'Inter', monospace",
                    }}>
                      {card.count}
                    </span>
                    <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>分</span>
                    <span style={{
                      fontSize: 11, fontWeight: 600, color: gauge.color,
                      background: `${gauge.color}15`, padding: '1px 6px', borderRadius: 6,
                    }}>
                      {gauge.text}
                    </span>
                  </div>
                  {/* Mini gauge bar */}
                  <div style={{
                    height: 4, borderRadius: 2, background: 'var(--color-fill-2)',
                    overflow: 'hidden',
                  }}>
                    <div style={{
                      height: '100%', width: `${Math.min(card.count, 100)}%`, borderRadius: 2,
                      background: `linear-gradient(90deg, #00b42a, #ff7d00, #f53f3f, #cb2ecb)`,
                      transition: 'width .4s ease',
                    }} />
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 3 }}>
                    <span style={{ fontSize: 9, color: 'var(--color-text-3)' }}>0 安全</span>
                    <span style={{ fontSize: 9, color: 'var(--color-text-3)' }}>100 危险</span>
                  </div>
                </>
              ) : (
                <div style={{
                  fontSize: 22, fontWeight: 700, color: card.color,
                  fontFamily: "'SF Mono', 'Inter', monospace",
                }}>
                  {card.count}
                </div>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Rule manual link */}
      <div style={{
        display: 'flex', justifyContent: 'flex-end', marginBottom: 14,
      }}>
        <span style={{
          fontSize: 12, color: 'var(--color-text-3)',
          display: 'flex', alignItems: 'center', gap: 5,
          cursor: 'pointer', padding: '4px 0',
        }}
          onClick={() => window.dispatchEvent(new CustomEvent('app:open-help', { detail: { section: 'rules' } }))}
          onMouseEnter={e => (e.currentTarget.style.color = '#165DFF')}
          onMouseLeave={e => (e.currentTarget.style.color = 'var(--color-text-3)')}
        >
          📖 查看全部 {total} 条风险规则及判断标准 →
        </span>
      </div>

      {/* ═══════════ Alert Table Card ═══════════ */}
      <Card
        style={{ borderRadius: 12 }}
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Activity size={17} color="var(--color-text-2)" />
            <span style={{ fontSize: 15, fontWeight: 600 }}>预警列表</span>
            <Tag style={{ marginLeft: 6, borderRadius: 10, fontSize: 11 }}>
              共 {total} 条
            </Tag>
          </div>
        }
        extra={
          <div style={{ display: 'flex', gap: 8 }}>
            {['', 'high', 'medium', 'low'].map(lv => (
              <Tag
                key={lv}
                checkable
                checked={filterLevel === lv}
                onClick={() => { setFilterLevel(filterLevel === lv ? '' : lv); setPage(1); }}
                style={{
                  borderRadius: 8, cursor: 'pointer', fontSize: 11,
                  ...(lv === '' ? {} : { color: levelBadge[lv]?.color, background: levelBadge[lv]?.bg, border: 'none' }),
                }}
              >
                {lv === '' ? '全部' : levelBadge[lv]?.label}
              </Tag>
            ))}
          </div>
        }
      >
        {loading ? (
          <div style={{ textAlign: 'center', padding: 60 }}>
            <Spin size={32} />
            <div style={{ marginTop: 12, fontSize: 13, color: 'var(--color-text-3)' }}>加载中...</div>
          </div>
        ) : total === 0 ? (
          <Empty
            description={
              <span style={{ color: 'var(--color-text-3)' }}>
                暂无风险预警{filterLevel ? `（${levelBadge[filterLevel]?.label || filterLevel}）` : ''}
              </span>
            }
            style={{ padding: '50px 0' }}
          />
        ) : (
          <div style={{ overflow: 'auto' }}>
            <Table
              data={alerts}
              rowKey="id"
              size="small"
              scroll={{ x: 780 }}
              rowClassName={(record: any, _idx: number) =>
                record.level === 'high' ? 'row-high-risk' : ''
              }
              columns={[
                {
                  title: '股票', dataIndex: 'stockName', width: 110,
                  render: (_: string, record: any) => (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                      <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>
                        {displayStockName(record)}
                      </span>
                      <span style={{ fontSize: 10, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>
                        {record.stockCode === '__MARKET__' || record.stockCode?.startsWith('__PORTFOLIO_') ? '' : record.stockCode}
                      </span>
                    </div>
                  ),
                },
                {
                  title: '维度', dataIndex: 'dimension', width: 68,
                  render: (v: string) => (
                    <Tag style={{
                      background: `${dimColor[v] || '#86909c'}14`,
                      color: dimColor[v] || '#86909c',
                      border: 'none', borderRadius: 8, fontSize: 10,
                      padding: '1px 8px',
                    }}>
                      {dimLabel[v] || v}
                    </Tag>
                  ),
                },
                {
                  title: '等级', dataIndex: 'level', width: 70,
                  render: (v: string) => {
                    const cfg = levelBadge[v];
                    return (
                      <span style={{
                        display: 'flex', alignItems: 'center', gap: 4,
                        color: cfg?.color, fontSize: 12, fontWeight: 600,
                      }}>
                        <cfg.icon size={12} />{cfg?.label}
                      </span>
                    );
                  },
                },
                {
                  title: '类型', dataIndex: 'type', width: 110,
                  render: (v: string, record: any) => (
                    <Tooltip content={
                        ruleExplain[record.ruleKey]
                          ? `${ruleExplain[record.ruleKey].source}
${ruleExplain[record.ruleKey].meaning}`
                          : v
                      } position="top">
                      <span style={{
                        fontSize: 13, cursor: 'help',
                        borderBottom: '1px dashed var(--color-text-3)',
                      }}>
                        {v}
                      </span>
                    </Tooltip>
                  ),
                },
                {
                  title: '说明', dataIndex: 'description', ellipsis: true,
                  render: (v: string) => (
                    <span style={{ fontSize: 12, color: 'var(--color-text-2)', lineHeight: 1.5 }}>{v}</span>
                  ),
                },
                {
                  title: '时间', dataIndex: 'hitDate', width: 90,
                  render: (v: string) => (
                    <span style={{ fontSize: 11, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>
                      {v ? new Date(v).toLocaleDateString('zh-CN') : '-'}
                    </span>
                  ),
                },
                {
                  title: '操作', width: 72, fixed: 'right' as const,
                  render: (_: any, record: any) => (
                    <div style={{ display: 'flex', gap: 2 }}>
                      <Tooltip content="查看详情">
                        <Button size="mini" type="text" icon={<Eye size={14} />}
                          onClick={() => setDetailAlert(record)} />
                      </Tooltip>
                      {record.status === 'active' && (
                        <Popconfirm
                          title="确认已了解此风险？"
                          onOk={() => handleAcknowledge(record.id)}
                        >
                          <Tooltip content="确认预警">
                            <Button size="mini" type="text"
                              style={{ color: '#00b42a' }}
                              icon={<CheckCircle size={14} />} />
                          </Tooltip>
                        </Popconfirm>
                      )}
                    </div>
                  ),
                },
              ]}
              pagination={false}
              border={false}
              stripe
            />
            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 14 }}>
              <Pagination current={page} pageSize={pageSize} total={total} size="small"
                onChange={(p: number) => setPage(p)} showTotal />
            </div>
          </div>
        )}
      </Card>

      {/* ═══════════ Detail Drawer ═══════════ */}
      <Drawer
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Zap size={17} color="#f53f3f" />
            <span style={{ fontWeight: 600 }}>预警详情</span>
          </div>
        }
        visible={!!detailAlert}
        onCancel={() => setDetailAlert(null)}
        footer={null}
        width={460}
      >
        {detailAlert && (
          <div>
            {/* Header */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 20, flexWrap: 'wrap' }}>
              {(() => {
                const cfg = levelBadge[detailAlert.level];
                return (
                  <span style={{
                    display: 'flex', alignItems: 'center', gap: 5, fontSize: 12, fontWeight: 600,
                    color: cfg?.color, background: cfg?.bg, padding: '4px 12px', borderRadius: 10,
                  }}>
                    <cfg.icon size={13} />{cfg?.label}
                  </span>
                );
              })()}
              <span style={{ fontSize: 15, fontWeight: 600 }}>{detailAlert.type}</span>
              <span style={{
                fontSize: 11, color: 'var(--color-text-3)',
                background: 'var(--color-fill-1)', padding: '2px 8px', borderRadius: 8,
              }}>
                {dimLabel[detailAlert.dimension] || detailAlert.dimension}
              </span>
            </div>

            {/* Stock Info */}
            <div style={{
              display: 'flex', alignItems: 'center', gap: 10, marginBottom: 20,
              padding: '12px 16px', borderRadius: 8,
              background: 'var(--color-fill-1)', border: '1px solid var(--color-border-1)',
            }}>
              <span style={{ fontSize: 14, fontWeight: 600 }}>
                {displayStockName(detailAlert)}
              </span>
              {detailAlert.stockCode !== '__MARKET__' && !detailAlert.stockCode?.startsWith('__PORTFOLIO_') && (
                <span style={{ fontSize: 11, color: 'var(--color-text-3)', fontFamily: 'monospace' }}>
                  {detailAlert.stockCode}
                </span>
              )}
              <span style={{ fontSize: 11, color: 'var(--color-text-3)', marginLeft: 'auto' }}>
                风险评分: <strong style={{ color: 'var(--color-text-1)' }}>{detailAlert.severityScore || 0}</strong>
              </span>
            </div>

            {/* Description */}
            <div style={{
              padding: 16, borderRadius: 8, marginBottom: 16,
              background: 'var(--color-fill-1)', border: '1px solid var(--color-border-1)',
            }}>
              <div style={{ fontSize: 14, color: 'var(--color-text-1)', lineHeight: 1.7, fontWeight: 500 }}>
                {detailAlert.description}
              </div>
              <div style={{ fontSize: 12, color: 'var(--color-text-3)', marginTop: 8 }}>
                {detailAlert.hitDate ? new Date(detailAlert.hitDate).toLocaleString('zh-CN') : '-'}
              </div>
            </div>

            {/* Rule Explanation */}
            {detailAlert.ruleKey && ruleExplain[detailAlert.ruleKey] && (() => {
              const info = ruleExplain[detailAlert.ruleKey];
              return (
                <div style={{
                  padding: 16, borderRadius: 8, marginBottom: 16,
                  background: '#165DFF06', border: '1px solid #165DFF18',
                }}>
                  <div style={{ fontSize: 13, fontWeight: 600, color: '#165DFF', marginBottom: 12 }}>
                    📖 指标详解
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                    <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
                      <span style={{ fontSize: 11, color: '#86909c', minWidth: 56, fontWeight: 500, flexShrink: 0 }}>数据来源</span>
                      <span style={{ fontSize: 12, color: 'var(--color-text-1)', lineHeight: 1.6, fontFamily: "'SF Mono', monospace" }}>{info.source}</span>
                    </div>
                    <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
                      <span style={{ fontSize: 11, color: '#86909c', minWidth: 56, fontWeight: 500, flexShrink: 0 }}>计算方式</span>
                      <span style={{ fontSize: 12, color: 'var(--color-text-1)', lineHeight: 1.6 }}>{info.method}</span>
                    </div>
                    <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
                      <span style={{ fontSize: 11, color: '#86909c', minWidth: 56, fontWeight: 500, flexShrink: 0 }}>风控意义</span>
                      <span style={{ fontSize: 12, color: '#f53f3f', lineHeight: 1.6, fontWeight: 500 }}>{info.meaning}</span>
                    </div>
                    <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
                      <span style={{ fontSize: 11, color: '#86909c', minWidth: 56, fontWeight: 500, flexShrink: 0 }}>阈值配置</span>
                      <span style={{ fontSize: 11, color: 'var(--color-text-3)', lineHeight: 1.5, fontFamily: "'SF Mono', monospace", background: 'var(--color-fill-1)', padding: '2px 6px', borderRadius: 4 }}>{info.threshold}</span>
                    </div>
                  </div>
                </div>
              );
            })()}

            {/* Evidence Data */}
            {detailAlert.evidence && Object.keys(detailAlert.evidence).length > 0 && (
              <div>
                <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 10, color: 'var(--color-text-2)' }}>
                  证据数据
                </div>
                <div style={{
                  background: '#1d2129', borderRadius: 8, padding: 14,
                  fontFamily: "'SF Mono', monospace", fontSize: 11, lineHeight: 1.7,
                  color: '#e5e6eb', overflowX: 'auto',
                }}>
                  {Object.entries(detailAlert.evidence).map(([k, v]) => (
                    <div key={k} style={{ display: 'flex', gap: 12 }}>
                      <span style={{ color: '#86909c', minWidth: 80 }}>{k}</span>
                      <span style={{ color: '#f2f3f5' }}>
                        {typeof v === 'number' ? v.toLocaleString() : String(v)}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </Drawer>

      {/* Spin animation style */}
      <style>{`
        @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
        .spin-icon { animation: spin 1s linear infinite; }
        .row-high-risk td { background: #f53f3f06 !important; }
      `}</style>
    </div>
  );
}
