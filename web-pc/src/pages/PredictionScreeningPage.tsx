import { useState, useEffect, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Select, Pagination, Input, Message } from '@arco-design/web-react';
import {
  TrendingUp, Target, Layers, Shield, Zap, Filter,
  ArrowUpDown, ChevronUp, ChevronDown, LayoutGrid, List,
  PieChart, BarChart3, Search, Info, Star, AlertTriangle,
} from 'lucide-react';
import api from '../services/api';

interface ScreeningRow {
  code: string;
  name: string;
  industry: string;
  boardType: string;
  latestPrice: number;
  directionConsensus: number;
  expectedReturn: number;
  returnStddev: number;
  divergence: number;
  momentum: number;
  riskRatio: number;
  signalValue: number;
  signalSource: string;
}

interface Summary {
  totalStocks: number;
  avgReturn: number;
  strongConsensus: number;
  predictionDate: string;
  bullRatio: number;
  avgMomentum: number;
}

interface IndustryStat {
  industry: string;
  count: number;
  avgReturn: number;
  avgConsensus: number;
  avgMomentum: number;
  avgRiskRatio: number;
  bullCount: number;
  topStock: string;
  topStockName: string;
  topReturn: number;
}

interface BoardStat {
  boardType: string;
  count: number;
  avgReturn: number;
  avgConsensus: number;
}

const BOARD_LABELS: Record<string, string> = {
  sh: '沪市主板', sz: '深市主板', cy: '创业板', kc: '科创板', '其他': '其他',
};

const INDUSTRY_COLORS: Record<string, string> = {
  '银行': '#165DFF', '房地产': '#F53F3F', '电子': '#00B42A', '医药生物': '#722ED1',
  '食品饮料': '#F76900', '计算机': '#14C9C9', '非银金融': '#165DFF', '汽车': '#F53F3F',
  '机械设备': '#0FC6C2', '电力设备': '#F7BA1E', '基础化工': '#9FDB1D', '国防军工': '#D91A1A',
};

const CHART_COLORS = ['#F53F3F', '#F76900', '#F7BA1E', '#9FDB1D', '#00B42A', '#14C9C9', '#165DFF', '#722ED1'];

function getIndustryColor(ind: string): string {
  return INDUSTRY_COLORS[ind] || '#86909C';
}

const fmtPct = (v: number) => `${v > 0 ? '+' : ''}${v.toFixed(2)}%`;

export default function PredictionScreeningPage() {
  const navigate = useNavigate();
  const [data, setData] = useState<ScreeningRow[]>([]);
  const [summary, setSummary] = useState<Summary>({
    totalStocks: 0, avgReturn: 0, strongConsensus: 0, predictionDate: '', bullRatio: 0, avgMomentum: 0,
  });
  const [industries, setIndustries] = useState<string[]>([]);
  const [industryAnalysis, setIndustryAnalysis] = useState<IndustryStat[]>([]);
  const [boardAnalysis, setBoardAnalysis] = useState<BoardStat[]>([]);
  const [consensusDist, setConsensusDist] = useState<number[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);

  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [sortBy, setSortBy] = useState('expected_return');
  const [sortOrder, setSortOrder] = useState('desc');
  const [filterIndustry, setFilterIndustry] = useState('');
  const [filterBoard, setFilterBoard] = useState('');
  const [minConsensus, setMinConsensus] = useState(0);
  const [keyword, setKeyword] = useState('');

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const { data: res } = await api.get('/prediction/screening', {
        params: {
          page, pageSize, sort: sortBy, order: sortOrder,
          industry: filterIndustry || undefined,
          board: filterBoard || undefined,
          minConsensus: minConsensus || undefined,
          keyword: keyword || undefined,
        },
      });
      const d = res.data;
      setData(d.list || []);
      setTotal(d.total || 0);
      setSummary(d.summary || {});
      setIndustries(d.industries || []);
      setIndustryAnalysis(d.industryAnalysis || []);
      setBoardAnalysis(d.boardAnalysis || []);
      setConsensusDist(d.consensusDistribution || []);
    } catch { /* handled silently */ } finally { setLoading(false); }
  }, [page, pageSize, sortBy, sortOrder, filterIndustry, filterBoard, minConsensus, keyword]);

  useEffect(() => { loadData(); }, [loadData]);

  const toggleSort = (key: string) => {
    if (sortBy === key) { setSortOrder(o => o === 'desc' ? 'asc' : 'desc'); }
    else { setSortBy(key); setSortOrder('desc'); }
    setPage(1);
  };

  const SortIcon = ({ field }: { field: string }) => {
    if (sortBy !== field) return <ArrowUpDown size={10} style={{ opacity: 0.25 }} />;
    return sortOrder === 'desc'
      ? <ChevronDown size={12} style={{ color: 'var(--color-primary)' }} />
      : <ChevronUp size={12} style={{ color: 'var(--color-primary)' }} />;
  };

  const consensusColor = (n: number) => {
    if (n >= 6) return '#00B42A'; if (n >= 4) return '#165DFF';
    if (n >= 2) return '#F76900'; return '#F53F3F';
  };

  // Industry bar width
  const maxIndRet = useMemo(() => {
    return Math.max(...industryAnalysis.map(i => Math.abs(i.avgReturn)), 0.01);
  }, [industryAnalysis]);

  // Consensus distribution max
  const maxDist = useMemo(() => Math.max(...consensusDist, 1), [consensusDist]);

  // Prediction interval
  const predInterval = useMemo(() => {
    if (!summary.predictionDate) return '—';
    const d = new Date(summary.predictionDate);
    d.setDate(d.getDate() + 1);
    const e = new Date(d);
    e.setDate(e.getDate() + 19);
    return d.toISOString().slice(0, 10) + ' → ' + e.toISOString().slice(0, 10);
  }, [summary.predictionDate]);

  const cardStyle: React.CSSProperties = {
    background: 'var(--color-bg-2)', borderRadius: 10,
    border: '1px solid var(--color-border-2)', padding: '16px 20px',
  };

  return (
    <div style={{ padding: '0 0 40px' }}>
      {/* ── Page Header ── */}
      <div style={{ marginBottom: 20 }}>
        <h2 style={{ margin: 0, fontSize: 18, fontWeight: 700, color: 'var(--color-text-1)', display: 'flex', alignItems: 'center', gap: 8 }}>
          <Target size={20} style={{ color: '#165DFF' }} />
          预测精选
        </h2>
        <p style={{ margin: '4px 0 0', fontSize: 12, color: 'var(--color-text-3)' }}>
          基于7种算法KD分布的全市场预测分析 · 预测日期：{summary.predictionDate || '—'} · 预测区间：{predInterval}
        </p>
      </div>

      {/* ══════════════════════════════════════════════════
          LAYER 1: 宏观预测概览
          ══════════════════════════════════════════════════ */}
      <div style={{ marginBottom: 20 }}>
        <h3 style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)', margin: '0 0 12px', display: 'flex', alignItems: 'center', gap: 6 }}>
          <PieChart size={16} style={{ color: '#165DFF' }} /> 宏观预测概览
        </h3>

        {/* Summary Cards */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 12, marginBottom: 16 }}>
          {/* 覆盖标的 */}
          <div style={cardStyle}>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 6 }}>覆盖标的</div>
            <div style={{ fontSize: 24, fontWeight: 700, color: 'var(--color-text-1)', fontFamily: "'SF Mono', monospace" }}>
              {summary.totalStocks.toLocaleString()}
            </div>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>只个股</div>
          </div>
          {/* 平均预期收益 */}
          <div style={cardStyle}>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 6 }}>平均预期收益</div>
            <div style={{ fontSize: 24, fontWeight: 700, color: summary.avgReturn >= 0 ? '#F53F3F' : '#00B42A', fontFamily: "'SF Mono', monospace" }}>
              {fmtPct(summary.avgReturn)}
            </div>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>20日预测均值</div>
          </div>
          {/* 看涨比例 */}
          <div style={cardStyle}>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 6 }}>看涨比例</div>
            <div style={{ fontSize: 24, fontWeight: 700, color: summary.bullRatio >= 50 ? '#F53F3F' : '#00B42A', fontFamily: "'SF Mono', monospace" }}>
              {summary.bullRatio}%
            </div>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>预期收益 &gt; 0 的个股</div>
          </div>
          {/* 强共识 */}
          <div style={cardStyle}>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 6 }}>强共识标的</div>
            <div style={{ fontSize: 24, fontWeight: 700, color: '#165DFF', fontFamily: "'SF Mono', monospace" }}>
              {summary.strongConsensus.toLocaleString()}
            </div>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>≥5/7 算法一致看多</div>
          </div>
          {/* 平均动量 */}
          <div style={cardStyle}>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)', marginBottom: 6 }}>平均预测动量</div>
            <div style={{ fontSize: 24, fontWeight: 700, color: summary.avgMomentum >= 0 ? '#F53F3F' : '#00B42A', fontFamily: "'SF Mono', monospace" }}>
              {fmtPct(summary.avgMomentum)}
            </div>
            <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>末5日-初5日均值差</div>
          </div>
        </div>

        {/* Consensus Distribution Chart */}
        <div style={{ ...cardStyle, padding: '20px' }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-1)', marginBottom: 14 }}>
            方向共识分布 · 7种算法一致性统计
          </div>
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 8, height: 140, paddingTop: 8 }}>
            {consensusDist.map((count, i) => {
              const pct = maxDist > 0 ? (count / maxDist) * 100 : 0;
              return (
                <div key={i} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4 }}
                  onClick={() => { setMinConsensus(i); setPage(1); }}
                  title={`${i}/7 共识: ${count} 只 — 点击筛选`}
                >
                  <span style={{ fontSize: 11, fontWeight: 600, color: 'var(--color-text-2)', fontFamily: "'SF Mono', monospace" }}>{count}</span>
                  <div style={{
                    width: '100%', height: `${Math.max(pct, 2)}%`, minHeight: 3,
                    background: minConsensus === i ? consensusColor(i) : `${consensusColor(i)}60`,
                    borderRadius: '4px 4px 0 0', cursor: 'pointer',
                    transition: 'background 0.2s',
                  }} />
                  <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>{i}/7</span>
                </div>
              );
            })}
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 8, padding: '0 4px' }}>
            <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>
              🟢 ≥6强共识 {(consensusDist[6] || 0) + (consensusDist[7] || 0)} 只
            </span>
            <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>
              🔴 ≤1弱共识 {(consensusDist[0] || 0) + (consensusDist[1] || 0)} 只
            </span>
          </div>
        </div>
      </div>

      {/* ══════════════════════════════════════════════════
          LAYER 2: 行业 / 板块分析
          ══════════════════════════════════════════════════ */}
      <div style={{ marginBottom: 20 }}>
        <h3 style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)', margin: '0 0 12px', display: 'flex', alignItems: 'center', gap: 6 }}>
          <BarChart3 size={16} style={{ color: '#F76900' }} /> 行业预测排名
          <span style={{ fontSize: 11, fontWeight: 400, color: 'var(--color-text-3)' }}>— 点击行业筛选个股</span>
          {filterIndustry && (
            <span onClick={() => { setFilterIndustry(''); setPage(1); }}
              style={{ fontSize: 11, color: '#165DFF', cursor: 'pointer', marginLeft: 4, background: '#165DFF10', padding: '2px 8px', borderRadius: 10 }}>
              ✕ {filterIndustry}
            </span>
          )}
        </h3>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
          {/* Industry Ranking */}
          <div style={{ ...cardStyle, padding: '16px 20px 12px' }}>
            <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-1)', marginBottom: 12 }}>行业预期收益排名</div>
            {industryAnalysis.slice(0, 10).map((ind, idx) => {
              const barW = maxIndRet > 0 ? Math.abs(ind.avgReturn) / maxIndRet * 100 : 0;
              return (
                <div key={ind.industry}
                  onClick={() => { setFilterIndustry(ind.industry === filterIndustry ? '' : ind.industry); setPage(1); }}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6, cursor: 'pointer',
                    padding: '4px 6px', borderRadius: 6, background: filterIndustry === ind.industry ? 'var(--color-fill-2)' : 'transparent',
                    transition: 'background 0.15s',
                  }}>
                  <span style={{ width: 20, fontSize: 11, fontWeight: 700, color: idx < 3 ? '#F53F3F' : 'var(--color-text-3)', textAlign: 'right', fontFamily: "'SF Mono', monospace" }}>
                    {idx + 1}
                  </span>
                  <span style={{ width: 68, fontSize: 12, fontWeight: 500, color: 'var(--color-text-1)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {ind.industry}
                  </span>
                  <div style={{ flex: 1, height: 20, background: 'var(--color-fill-1)', borderRadius: 4, overflow: 'hidden', position: 'relative' }}>
                    <div style={{
                      height: '100%', width: `${barW}%`,
                      background: ind.avgReturn >= 0 ? `linear-gradient(90deg, #F53F3F20, #F53F3F)` : `linear-gradient(90deg, #00B42A20, #00B42A)`,
                      borderRadius: 4, transition: 'width 0.3s',
                    }} />
                  </div>
                  <span style={{ width: 68, fontSize: 11, fontWeight: 700, fontFamily: "'SF Mono', monospace", color: ind.avgReturn >= 0 ? '#F53F3F' : '#00B42A', textAlign: 'right' }}>
                    {fmtPct(ind.avgReturn)}
                  </span>
                  <span style={{ width: 38, fontSize: 10, color: 'var(--color-text-3)', textAlign: 'right' }}>
                    {ind.count}只
                  </span>
                </div>
              );
            })}
            {industryAnalysis.length > 10 && (
              <div style={{ fontSize: 11, color: 'var(--color-text-3)', textAlign: 'center', marginTop: 6 }}>
                共 {industryAnalysis.length} 个行业，显示前10
              </div>
            )}
          </div>

          {/* Board & Detail */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {/* Board Stats */}
            <div style={{ ...cardStyle, padding: '16px 20px' }}>
              <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--color-text-1)', marginBottom: 12 }}>板块对比</div>
              {boardAnalysis.map(ba => (
                <div key={ba.boardType}
                  onClick={() => { setFilterBoard(filterBoard === ba.boardType ? '' : ba.boardType); setPage(1); }}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6, cursor: 'pointer',
                    padding: '4px 6px', borderRadius: 6, background: filterBoard === ba.boardType ? 'var(--color-fill-2)' : 'transparent',
                  }}>
                  <span style={{ width: 72, fontSize: 12, fontWeight: 500, color: 'var(--color-text-1)' }}>
                    {BOARD_LABELS[ba.boardType] || ba.boardType}
                  </span>
                  <div style={{ flex: 1, height: 6, background: 'var(--color-fill-1)', borderRadius: 3, overflow: 'hidden' }}>
                    <div style={{
                      height: '100%', width: `${Math.min(Math.abs(ba.avgReturn) / Math.max(...boardAnalysis.map(b => Math.abs(b.avgReturn)), 0.01) * 100, 100)}%`,
                      background: ba.avgReturn >= 0 ? '#F53F3F' : '#00B42A', borderRadius: 3,
                    }} />
                  </div>
                  <span style={{ width: 60, fontSize: 11, fontWeight: 600, fontFamily: "'SF Mono', monospace", color: ba.avgReturn >= 0 ? '#F53F3F' : '#00B42A', textAlign: 'right' }}>
                    {fmtPct(ba.avgReturn)}
                  </span>
                  <span style={{ width: 36, fontSize: 10, color: 'var(--color-text-3)', textAlign: 'right' }}>{ba.count}</span>
                </div>
              ))}
            </div>

            {/* Selected Industry Detail */}
            {filterIndustry && (() => {
              const ind = industryAnalysis.find(i => i.industry === filterIndustry);
              if (!ind) return null;
              return (
                <div style={{ ...cardStyle, padding: '16px 20px', borderColor: '#165DFF40' }}>
                  <div style={{ fontSize: 12, fontWeight: 600, color: '#165DFF', marginBottom: 12, display: 'flex', alignItems: 'center', gap: 4 }}>
                    <Info size={14} /> {ind.industry} · 详细数据
                  </div>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px 12px' }}>
                    <div>
                      <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>覆盖标的</span>
                      <div style={{ fontSize: 16, fontWeight: 700, fontFamily: "'SF Mono', monospace" }}>{ind.count}<span style={{ fontSize: 11, fontWeight: 400 }}> 只</span></div>
                    </div>
                    <div>
                      <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>平均预期收益</span>
                      <div style={{ fontSize: 16, fontWeight: 700, fontFamily: "'SF Mono', monospace", color: ind.avgReturn >= 0 ? '#F53F3F' : '#00B42A' }}>{fmtPct(ind.avgReturn)}</div>
                    </div>
                    <div>
                      <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>看涨占比</span>
                      <div style={{ fontSize: 16, fontWeight: 700, fontFamily: "'SF Mono', monospace" }}>{ind.count > 0 ? Math.round(ind.bullCount / ind.count * 100) : 0}%</div>
                    </div>
                    <div>
                      <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>平均共识</span>
                      <div style={{ fontSize: 16, fontWeight: 700, fontFamily: "'SF Mono', monospace", color: consensusColor(Math.round(ind.avgConsensus)) }}>{ind.avgConsensus.toFixed(1)}/7</div>
                    </div>
                    <div style={{ gridColumn: '1 / -1' }}>
                      <span style={{ fontSize: 10, color: 'var(--color-text-3)' }}>龙头标的</span>
                      <div style={{ fontSize: 13, fontWeight: 600, cursor: 'pointer', color: '#165DFF' }}
                        onClick={() => navigate(`/stock/${ind.topStock}`)}>
                        {ind.topStock} {ind.topStockName}
                        <span style={{ fontSize: 11, fontWeight: 700, fontFamily: "'SF Mono', monospace", color: '#F53F3F', marginLeft: 8 }}>{fmtPct(ind.topReturn)}</span>
                      </div>
                    </div>
                  </div>
                </div>
              );
            })()}
          </div>
        </div>
      </div>

      {/* ══════════════════════════════════════════════════
          LAYER 3: 个股精选
          ══════════════════════════════════════════════════ */}
      <div>
        <h3 style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-text-1)', margin: '0 0 12px', display: 'flex', alignItems: 'center', gap: 6 }}>
          <Star size={16} style={{ color: '#F7BA1E' }} /> 个股精选
          <span style={{ fontSize: 11, fontWeight: 400, color: 'var(--color-text-3)' }}>
            {filterIndustry && `行业: ${filterIndustry}`}
            {filterBoard && ` · 板块: ${BOARD_LABELS[filterBoard] || filterBoard}`}
            {minConsensus > 0 && ` · 共识 ≥ ${minConsensus}/7`}
            {(!filterIndustry && !filterBoard && minConsensus === 0) && '全市场'}
            {' · '}共 {total} 只
          </span>
        </h3>

        {/* Filters Bar */}
        <div style={{ display: 'flex', gap: 10, marginBottom: 14, flexWrap: 'wrap', alignItems: 'center' }}>
          <Input
            prefix={<Search size={14} />}
            placeholder="搜索代码/名称"
            value={keyword}
            onChange={v => { setKeyword(v); setPage(1); }}
            style={{ width: 180 }}
            size="small"
            allowClear
          />
          <Select
            placeholder="行业筛选"
            value={filterIndustry || undefined}
            onChange={v => { setFilterIndustry(v || ''); setPage(1); }}
            style={{ width: 140 }}
            size="small"
            allowClear
            options={industries.map(i => ({ value: i, label: i }))}
            showSearch
          />
          <Select
            placeholder="板块筛选"
            value={filterBoard || undefined}
            onChange={v => { setFilterBoard(v || ''); setPage(1); }}
            style={{ width: 120 }}
            size="small"
            allowClear
            options={[
              { value: 'sh', label: '沪市主板' }, { value: 'sz', label: '深市主板' },
              { value: 'cy', label: '创业板' }, { value: 'kc', label: '科创板' },
            ]}
          />
          <Select
            placeholder="最小共识"
            value={minConsensus || undefined}
            onChange={v => { setMinConsensus(v || 0); setPage(1); }}
            style={{ width: 120 }}
            size="small"
            allowClear
            options={[
              { value: 5, label: '≥5/7 强共识' }, { value: 6, label: '≥6/7 超强' },
              { value: 7, label: '7/7 全票' },
            ]}
          />
          <Select
            placeholder="排序"
            value={sortBy}
            onChange={v => { setSortBy(v); setPage(1); }}
            style={{ width: 130 }}
            size="small"
            options={[
              { value: 'expected_return', label: '预期收益' }, { value: 'consensus', label: '方向共识' },
              { value: 'momentum', label: '预测动量' }, { value: 'risk_ratio', label: '收益风险比' },
              { value: 'signal', label: '信号值' },
            ]}
          />
          <span onClick={() => { setSortOrder(o => o === 'desc' ? 'asc' : 'desc'); setPage(1); }}
            style={{ cursor: 'pointer', color: 'var(--color-text-2)', display: 'flex', alignItems: 'center', gap: 4, fontSize: 12 }}>
            {sortOrder === 'desc' ? <><ChevronDown size={14} /> 降序</> : <><ChevronUp size={14} /> 升序</>}
          </span>
        </div>

        {/* Stock Table */}
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
            <thead>
              <tr style={{ background: 'var(--color-fill-1)', borderBottom: '2px solid var(--color-border-2)' }}>
                {[
                  { key: 'code', label: '代码', w: 85, align: 'left' as const },
                  { key: 'name', label: '名称', w: 100, align: 'left' as const },
                  { key: 'industry', label: '行业', w: 72, align: 'left' as const },
                  { key: 'boardType', label: '板块', w: 72, align: 'left' as const },
                  { key: 'expected_return', label: '预期收益', w: 100, sort: true },
                  { key: 'consensus', label: '共识', w: 130, sort: true },
                  { key: 'momentum', label: '动量', w: 80, sort: true },
                  { key: 'risk_ratio', label: '收益/风险', w: 90, sort: true },
                ].map(col => (
                  <th key={col.key} onClick={col.sort ? () => toggleSort(col.key) : undefined}
                    style={{
                      padding: '10px 12px', textAlign: col.align || 'right', fontSize: 11, fontWeight: 600,
                      color: 'var(--color-text-3)', cursor: col.sort ? 'pointer' : 'default',
                      width: col.w, whiteSpace: 'nowrap', userSelect: 'none',
                    }}>
                    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 3 }}>
                      {col.label}{col.sort && <SortIcon field={col.key} />}
                    </span>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.map((row, i) => (
                <tr key={row.code} onClick={() => navigate(`/stock/${row.code}`)}
                  style={{
                    borderBottom: '1px solid var(--color-border-1)', cursor: 'pointer',
                    background: i % 2 === 0 ? 'transparent' : 'var(--color-fill-1)',
                  }}
                  onMouseEnter={e => { e.currentTarget.style.background = 'var(--color-fill-2)'; }}
                  onMouseLeave={e => { e.currentTarget.style.background = i % 2 === 0 ? 'transparent' : 'var(--color-fill-1)'; }}>
                  <td style={{ padding: '9px 12px', color: 'var(--color-text-2)', fontSize: 11, fontFamily: "'SF Mono', monospace" }}>{row.code}</td>
                  <td style={{ padding: '9px 12px', fontWeight: 600, color: 'var(--color-text-1)' }}>{row.name || row.code}</td>
                  <td style={{ padding: '9px 12px', fontSize: 10, color: 'var(--color-text-3)' }}>
                    <span style={{ padding: '2px 6px', borderRadius: 4, background: getIndustryColor(row.industry) + '15', color: getIndustryColor(row.industry) }}>
                      {row.industry || '—'}
                    </span>
                  </td>
                  <td style={{ padding: '9px 12px', fontSize: 10, color: 'var(--color-text-3)' }}>
                    {row.boardType ? (BOARD_LABELS[row.boardType] || row.boardType) : '—'}
                  </td>
                  <td style={{ padding: '9px 12px', textAlign: 'right' }}>
                    <span style={{ fontSize: 12, fontWeight: 700, fontFamily: "'SF Mono', monospace", color: row.expectedReturn >= 0 ? '#F53F3F' : '#00B42A', background: (row.expectedReturn >= 0 ? '#F53F3F' : '#00B42A') + '10', padding: '3px 8px', borderRadius: 5 }}>
                      {fmtPct(row.expectedReturn)}
                    </span>
                  </td>
                  <td style={{ padding: '9px 12px', textAlign: 'right' }}>
                    <div style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                      <span style={{ fontSize: 11, fontWeight: 700, fontFamily: "'SF Mono', monospace", color: consensusColor(row.directionConsensus), minWidth: 22, textAlign: 'right' }}>
                        {row.directionConsensus}/7
                      </span>
                      <div style={{ display: 'flex', gap: 1.5 }}>
                        {[0, 1, 2, 3, 4, 5, 6].map(j => (
                          <div key={j} style={{
                            width: 7, height: 7, borderRadius: 3.5,
                            background: j < row.directionConsensus ? consensusColor(row.directionConsensus) : 'var(--color-fill-2)',
                          }} />
                        ))}
                      </div>
                    </div>
                  </td>
                  <td style={{ padding: '9px 12px', textAlign: 'right', fontSize: 11, fontFamily: "'SF Mono', monospace", color: row.momentum >= 0 ? '#F53F3F' : '#00B42A' }}>
                    {fmtPct(row.momentum)}
                  </td>
                  <td style={{ padding: '9px 12px', textAlign: 'right', fontSize: 11, fontFamily: "'SF Mono', monospace", color: 'var(--color-text-2)' }}>
                    {row.riskRatio.toFixed(2)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {data.length === 0 && !loading && (
          <div style={{ textAlign: 'center', padding: 40, color: 'var(--color-text-3)' }}>
            {filterIndustry || filterBoard || minConsensus > 0 ? '当前筛选条件下无匹配标的，请调整条件' : '暂无预测数据，请先导入预测数据'}
          </div>
        )}

        {/* Pagination */}
        {total > pageSize && (
          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'flex-end' }}>
            <Pagination total={total} current={page} pageSize={pageSize}
              onChange={(p: number) => setPage(p)}
              onPageSizeChange={(s: number) => { setPageSize(s); setPage(1); }}
              showTotal sizeCanChange />
          </div>
        )}
      </div>
    </div>
  );
}
