import { useState, useEffect } from 'react';
import { Card, Spin, Statistic, Table, Tag, Typography } from '@arco-design/web-react';
import { BarChart3, Database, AlertTriangle, Clock, TrendingDown, DollarSign } from 'lucide-react';
import { fetchStatsDashboard } from '../services/api';

const { Title, Text } = Typography;

interface KLineStats {
    totalRows: number;
    totalStocks: number;
    minDate: string;
    maxDate: string;
    qualityOk: number;
    qualitySuspect: number;
    qualityBad: number;
    staleStocks: number;
    sparseStocks: number;
}

interface FinancialStats {
    totalRows: number;
    totalStocks: number;
    hasCashFlow: number;
    cashFlowPct: number;
}

interface DashboardData {
    kline: KLineStats;
    financials: FinancialStats;
}

const formatNumber = (n: number) => n?.toLocaleString() ?? '0';

export default function DataDashboardPage() {
    const [data, setData] = useState<DashboardData | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');

    useEffect(() => {
        fetchStatsDashboard()
            .then((res: any) => setData(res.data?.data))
            .catch((err: any) => { console.error('[Dashboard] fetch failed:', err); setError('加载失败'); })
            .finally(() => setLoading(false));
    }, []);

    if (loading) return <div style={{ display: 'flex', justifyContent: 'center', padding: 120 }}><Spin size={40} /></div>;

    if (error || !data) return <div style={{ textAlign: 'center', padding: 80, color: 'var(--color-text-3)' }}>{error || '暂无数据'}</div>;

    const { kline, financials } = data;
    const qualityColumns = [
        { title: '质量等级', dataIndex: 'level', width: 120 },
        { title: '数量', dataIndex: 'count', width: 100, render: (v: number) => formatNumber(v) },
        { title: '占比', dataIndex: 'pct', width: 80, render: (v: string) => v },
    ];
    const qualityData = [
        { key: 'ok', level: <Tag color="green">ok</Tag>, count: kline.qualityOk, pct: (kline.qualityOk / kline.totalRows * 100).toFixed(2) + '%' },
        { key: 'suspect', level: <Tag color="orange">suspect</Tag>, count: kline.qualitySuspect, pct: (kline.qualitySuspect / kline.totalRows * 100).toFixed(2) + '%' },
        { key: 'bad', level: <Tag color="red">bad</Tag>, count: kline.qualityBad, pct: (kline.qualityBad / kline.totalRows * 100).toFixed(2) + '%' },
    ];

    return (
        <div style={{ padding: '24px 28px', maxWidth: 1200 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24 }}>
                <div style={{
                    background: 'linear-gradient(135deg, #165DFF, #722ED1)',
                    borderRadius: 10, width: 40, height: 40,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>
                    <BarChart3 size={20} color="#fff" />
                </div>
                <Title heading={4} style={{ margin: 0 }}>数据质量看板</Title>
            </div>

            {/* KLine summary */}
            <Card style={{ marginBottom: 20, background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16 }}>
                    <Database size={16} color="var(--color-text-2)" />
                    <Text bold>K线数据概览</Text>
                </div>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 24, marginBottom: 20 }}>
                    <Statistic title="总行数" value={formatNumber(kline.totalRows)} />
                    <Statistic title="股票数" value={formatNumber(kline.totalStocks)} />
                    <Statistic title="最早日期" value={kline.minDate} />
                    <Statistic title="最晚日期" value={kline.maxDate} />
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 24, marginBottom: 20 }}>
                    <div style={{
                        background: 'var(--color-fill-1)', borderRadius: 8, padding: '16px 20px',
                        display: 'flex', alignItems: 'center', gap: 12,
                    }}>
                        <Clock size={18} color="#faad14" />
                        <div>
                            <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>数据滞后 3 天以上</div>
                            <div style={{ fontSize: 22, fontWeight: 700, color: '#faad14' }}>{formatNumber(kline.staleStocks)} <span style={{ fontSize: 12, fontWeight: 400 }}>只</span></div>
                        </div>
                    </div>
                    <div style={{
                        background: 'var(--color-fill-1)', borderRadius: 8, padding: '16px 20px',
                        display: 'flex', alignItems: 'center', gap: 12,
                    }}>
                        <TrendingDown size={18} color="#ff7d00" />
                        <div>
                            <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>数据稀疏 {'<'}100 条</div>
                            <div style={{ fontSize: 22, fontWeight: 700, color: '#ff7d00' }}>{formatNumber(kline.sparseStocks)} <span style={{ fontSize: 12, fontWeight: 400 }}>只</span></div>
                        </div>
                    </div>
                    <div style={{
                        background: 'var(--color-fill-1)', borderRadius: 8, padding: '16px 20px',
                        display: 'flex', alignItems: 'center', gap: 12,
                    }}>
                        <AlertTriangle size={18} color="#f53f3f" />
                        <div>
                            <div style={{ fontSize: 12, color: 'var(--color-text-3)' }}>异常数据 (suspect+bad)</div>
                            <div style={{ fontSize: 22, fontWeight: 700, color: '#f53f3f' }}>{formatNumber(kline.qualitySuspect + kline.qualityBad)} <span style={{ fontSize: 12, fontWeight: 400 }}>条</span></div>
                        </div>
                    </div>
                </div>

                <Text bold style={{ fontSize: 13, marginBottom: 8, display: 'block' }}>数据质量分布</Text>
                <Table
                    columns={qualityColumns}
                    data={qualityData}
                    size="small"
                    pagination={false}
                    style={{ maxWidth: 400 }}
                />
            </Card>

            {/* Financial summary */}
            <Card style={{ background: 'var(--color-bg-2)', borderRadius: 10, border: '1px solid var(--color-border-2)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16 }}>
                    <DollarSign size={16} color="var(--color-text-2)" />
                    <Text bold>财务数据概览</Text>
                </div>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 24 }}>
                    <Statistic title="总记录数" value={formatNumber(financials.totalRows)} />
                    <Statistic title="股票数" value={formatNumber(financials.totalStocks)} />
                    <Statistic title="含现金流" value={formatNumber(financials.hasCashFlow)} />
                    <Statistic title="现金流覆盖率" value={financials.cashFlowPct.toFixed(1) + '%'} />
                </div>
            </Card>
        </div>
    );
}
