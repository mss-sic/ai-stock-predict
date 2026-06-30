import { useEffect, useState, useCallback, useMemo } from 'react';
import { Table, Card, Tag, Spin, Button, Select, Tooltip } from '@arco-design/web-react';
import { fetchIndustries, fetchIndustryStocks } from '../services/api';
import { TrendingUp, TrendingDown, Layers, Building2, ArrowUp, ArrowDown } from 'lucide-react';

interface IndustrySummary {
  industry: string;
  stockCount: number;
  peMedian: number;
  peP25: number;
  peP75: number;
  pbMedian: number;
  psMedian: number;
  avgMarketCap: number;
  avgWeekReturn: number;
  avgMonthReturn: number;
}

interface IndustryStock {
  code: string;
  name: string;
  pe: number;
  pb: number;
  ps: number;
  marketCap: number;
  peRank: number;
  weekReturn: number;
  close: number;
  changePct: number;
}

function fmtVal(v: number, digits = 2): string {
  if (!v || v === 0) return '-';
  return v.toFixed(digits);
}

function fmtCap(v: number): string {
  if (!v || v === 0) return '-';
  if (v >= 1e12) return (v / 1e12).toFixed(2) + '万亿';
  if (v >= 1e8) return (v / 1e8).toFixed(0) + '亿';
  return (v / 1e4).toFixed(0) + '万';
}

function pctColor(v: number): string {
  if (v > 0) return '#22c55e';
  if (v < 0) return '#ef4444';
  return 'var(--color-text-3)';
}

function pctTag(v: number) {
  const c = v > 0 ? 'green' : v < 0 ? 'red' : undefined;
  const prefix = v > 0 ? '+' : '';
  return <Tag color={c}>{prefix}{fmtVal(v)}%</Tag>;
}

export default function IndustryComparePage() {
  const [industries, setIndustries] = useState<IndustrySummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedIndustry, setExpandedIndustry] = useState<string | null>(null);
  const [stocks, setStocks] = useState<IndustryStock[]>([]);
  const [stocksLoading, setStocksLoading] = useState(false);
  const [stockSort, setStockSort] = useState<string>('pe');

  useEffect(() => {
    setLoading(true);
    fetchIndustries()
      .then((r: any) => {
        const data = r.data?.data || r.data || [];
        setIndustries(Array.isArray(data) ? data : []);
      })
      .catch((err) => console.error('[IndustryCompare] fetch industries failed:', err))
      .finally(() => setLoading(false));
  }, []);

  const loadStocks = useCallback(async (industry: string, sort: string) => {
    setStocksLoading(true);
    try {
      const r: any = await fetchIndustryStocks(industry, undefined, sort);
      setStocks(r.data?.data || r.data || []);
    } catch (err) {
      console.error('[IndustryCompare] fetch stocks failed:', err);
    } finally {
      setStocksLoading(false);
    }
  }, []);

  const handleExpand = useCallback((industry: string) => {
    if (expandedIndustry === industry) {
      setExpandedIndustry(null);
      setStocks([]);
    } else {
      setExpandedIndustry(industry);
      setStockSort('pe');
      loadStocks(industry, 'pe');
    }
  }, [expandedIndustry, loadStocks]);

  const handleStockSort = useCallback((val: string) => {
    setStockSort(val);
    if (expandedIndustry) loadStocks(expandedIndustry, val);
  }, [expandedIndustry, loadStocks]);

  const avgPE = useMemo(() => {
    const valid = industries.filter(i => i.peMedian > 0);
    if (!valid.length) return 0;
    return valid.reduce((s, i) => s + i.peMedian, 0) / valid.length;
  }, [industries]);

  const columns = [
    {
      title: '行业',
      dataIndex: 'industry',
      width: 120,
      render: (v: string, row: IndustrySummary) => (
        <div
          style={{ display: 'flex', alignItems: 'center', gap: 6, cursor: 'pointer', color: 'var(--color-primary)', fontWeight: 600 }}
          onClick={() => handleExpand(row.industry)}
        >
          <Building2 size={14} />
          {v}
          <span style={{ fontSize: 11, color: 'var(--color-text-3)', fontWeight: 400 }}>
            {row.stockCount}只
          </span>
        </div>
      ),
    },
    {
      title: 'PE 中位',
      dataIndex: 'peMedian',
      width: 90,
      sorter: (a: IndustrySummary, b: IndustrySummary) => a.peMedian - b.peMedian,
      render: (v: number) => (
        <Tooltip content={`P25: ${fmtVal(v * 0.7)} / P75: ${fmtVal(v * 1.5)}`}>
          <span style={{ fontFamily: "'SF Mono', monospace", fontSize: 13, fontWeight: 500, color: avgPE > 0 && v > avgPE * 1.5 ? '#ef4444' : avgPE > 0 && v < avgPE * 0.5 ? '#22c55e' : 'var(--color-text-1)' }}>
            {fmtVal(v)}
          </span>
        </Tooltip>
      ),
    },
    {
      title: 'PB 中位',
      dataIndex: 'pbMedian',
      width: 85,
      sorter: (a: IndustrySummary, b: IndustrySummary) => a.pbMedian - b.pbMedian,
      render: (v: number) => <span style={{ fontFamily: "'SF Mono', monospace", fontSize: 13 }}>{fmtVal(v)}</span>,
    },
    {
      title: 'PS 中位',
      dataIndex: 'psMedian',
      width: 85,
      sorter: (a: IndustrySummary, b: IndustrySummary) => a.psMedian - b.psMedian,
      render: (v: number) => <span style={{ fontFamily: "'SF Mono', monospace", fontSize: 13 }}>{fmtVal(v)}</span>,
    },
    {
      title: '平均市值',
      dataIndex: 'avgMarketCap',
      width: 100,
      sorter: (a: IndustrySummary, b: IndustrySummary) => a.avgMarketCap - b.avgMarketCap,
      render: (v: number) => <span style={{ fontFamily: "'SF Mono', monospace", fontSize: 12 }}>{fmtCap(v)}</span>,
    },
    {
      title: '周涨跌',
      dataIndex: 'avgWeekReturn',
      width: 85,
      sorter: (a: IndustrySummary, b: IndustrySummary) => a.avgWeekReturn - b.avgWeekReturn,
      defaultSortOrder: 'descend' as const,
      render: (v: number) => pctTag(v),
    },
    {
      title: '月涨跌',
      dataIndex: 'avgMonthReturn',
      width: 85,
      sorter: (a: IndustrySummary, b: IndustrySummary) => a.avgMonthReturn - b.avgMonthReturn,
      render: (v: number) => pctTag(v),
    },
  ];

  const stockColumns = [
    {
      title: '排名',
      dataIndex: 'peRank',
      width: 55,
      render: (v: number) => <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>#{v}</span>,
    },
    {
      title: '代码',
      dataIndex: 'code',
      width: 75,
      render: (v: string) => (
        <a href={`/stock/${v}`} style={{ color: 'var(--color-primary)', fontSize: 13, fontFamily: "'SF Mono', monospace" }}>{v}</a>
      ),
    },
    {
      title: '名称',
      dataIndex: 'name',
      width: 90,
      render: (v: string) => <span style={{ fontSize: 13 }}>{v}</span>,
    },
    {
      title: 'PE',
      dataIndex: 'pe',
      width: 70,
      render: (v: number) => <span style={{ fontFamily: "'SF Mono', monospace", fontSize: 13 }}>{fmtVal(v)}</span>,
    },
    {
      title: 'PB',
      dataIndex: 'pb',
      width: 70,
      render: (v: number) => <span style={{ fontFamily: "'SF Mono', monospace", fontSize: 13 }}>{fmtVal(v)}</span>,
    },
    {
      title: '市值',
      dataIndex: 'marketCap',
      width: 90,
      render: (v: number) => <span style={{ fontFamily: "'SF Mono', monospace", fontSize: 12 }}>{fmtCap(v)}</span>,
    },
    {
      title: '最新价',
      dataIndex: 'close',
      width: 75,
      render: (v: number) => <span style={{ fontFamily: "'SF Mono', monospace", fontSize: 13 }}>{fmtVal(v)}</span>,
    },
    {
      title: '日涨跌',
      dataIndex: 'changePct',
      width: 75,
      render: (v: number) => pctTag(v),
    },
    {
      title: '周涨跌',
      dataIndex: 'weekReturn',
      width: 75,
      render: (v: number) => pctTag(v),
    },
  ];

  const statsCards = useMemo(() => {
    if (!industries.length) return [];
    const topWeek = [...industries].sort((a, b) => b.avgWeekReturn - a.avgWeekReturn)[0];
    const topMonth = [...industries].sort((a, b) => b.avgMonthReturn - a.avgMonthReturn)[0];
    const totalStocks = industries.reduce((s, i) => s + i.stockCount, 0);
    return [
      { icon: TrendingUp, label: '周领涨行业', value: topWeek?.industry || '-', sub: topWeek ? `${topWeek.avgWeekReturn.toFixed(2)}%` : '' },
      { icon: ArrowUp, label: '月领涨行业', value: topMonth?.industry || '-', sub: topMonth ? `${topMonth.avgMonthReturn.toFixed(2)}%` : '' },
      { icon: Layers, label: '覆盖行业', value: `${industries.length}`, sub: `${totalStocks} 只股票` },
    ];
  }, [industries]);

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 300 }}>
        <Spin size={40} />
      </div>
    );
  }

  return (
    <div style={{ padding: '20px 24px', maxWidth: 1400, margin: '0 auto' }}>
      {/* Page Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 14, marginBottom: 24 }}>
        <div style={{
          width: 44, height: 44, borderRadius: 10,
          background: 'linear-gradient(135deg, #165DFF, #722ED1)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <Layers size={22} color="#fff" />
        </div>
        <div>
          <h1 style={{ fontSize: 18, fontWeight: 700, margin: 0, color: 'var(--color-text-1)' }}>行业横向对比</h1>
          <p style={{ margin: '2px 0 0', fontSize: 12, color: 'var(--color-text-3)' }}>
            申万行业 PE / PB / PS 中位数对比 & 涨跌排名
          </p>
        </div>
      </div>

      {/* Stats Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 14, marginBottom: 20 }}>
        {statsCards.map((c, i) => (
          <Card key={i} style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <c.icon size={18} color="var(--color-text-2)" />
              <div>
                <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{c.label}</div>
                <div style={{ fontSize: 16, fontWeight: 700, color: 'var(--color-text-1)' }}>{c.value}</div>
                {c.sub && <div style={{ fontSize: 12, color: 'var(--color-primary)' }}>{c.sub}</div>}
              </div>
            </div>
          </Card>
        ))}
      </div>

      {/* Industry Table */}
      <Card
        style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}
        bodyStyle={{ padding: 0 }}
      >
        <Table
          columns={columns}
          data={industries}
          rowKey="industry"
          pagination={{ pageSize: 30, showTotal: true }}
          size="small"
          border={{ wrapper: true, cell: true }}
          rowClassName={(record) => record.industry === expandedIndustry ? 'arco-table-row-selected' : ''}
          expandedRowRender={
            expandedIndustry
              ? () => (
                  <div style={{ padding: '8px 16px 16px', background: 'var(--color-fill-1)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}>
                      <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-1)' }}>
                        {expandedIndustry} — 个股明细
                      </span>
                      <Select
                        size="small"
                        value={stockSort}
                        onChange={handleStockSort}
                        style={{ width: 120 }}
                        options={[
                          { label: 'PE 升序', value: 'pe' },
                          { label: '周涨跌 ↓', value: 'return' },
                          { label: '日涨跌 ↓', value: 'change' },
                        ]}
                      />
                    </div>
                    {stocksLoading ? (
                      <div style={{ textAlign: 'center', padding: 20 }}><Spin /></div>
                    ) : (
                      <Table
                        columns={stockColumns}
                        data={stocks}
                        rowKey="code"
                        pagination={{ pageSize: 15, sizeOptions: [10, 15, 20] }}
                        size="small"
                        border={{ wrapper: true, cell: true }}
                      />
                    )}
                  </div>
                )
              : undefined
          }
        />
      </Card>
    </div>
  );
}
