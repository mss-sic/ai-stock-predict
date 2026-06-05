import { useState, useMemo } from 'react';
import { Table } from '@arco-design/web-react';
import { ShieldAlert, AlertTriangle, Info, Filter } from 'lucide-react';

const mockAlerts = [
  { id: 1, code: '000017', level: 'high', type: '连榜过热', desc: '连续上榜 15 个交易日，短线情绪过热', date: '2026-06-03' },
  { id: 2, code: '002456', level: 'medium', type: '技术破位', desc: '跌破 20 日均线支撑位', date: '2026-06-02' },
  { id: 3, code: '300613', level: 'high', type: '质押偏高', desc: '控股股东质押比例超 60%', date: '2026-06-01' },
  { id: 4, code: '688170', level: 'low', type: '流动性弱', desc: '日均成交额低于 5000 万', date: '2026-06-03' },
  { id: 5, code: '600162', level: 'medium', type: '估值偏高', desc: '市盈率高于行业均值 2 倍标准差', date: '2026-05-31' },
  { id: 6, code: '000725', level: 'high', type: '限售解禁', desc: '30 日内有限售股解禁，占总股本 8%', date: '2026-06-05' },
];

const iconMap: Record<string, any> = { high: AlertTriangle, medium: ShieldAlert, low: Info };
const colorMap: Record<string, string> = { high: 'tag-red', medium: 'tag-orange', low: 'tag-green' };
const labels: Record<string, string> = { high: '高风险', medium: '中风险', low: '低风险' };

export default function RiskPage() {
  const [filterLevel, setFilterLevel] = useState<string>('all');
  const [filterType, setFilterType] = useState<string>('all');

  const filtered = useMemo(() => {
    return mockAlerts.filter(a => {
      if (filterLevel !== 'all' && a.level !== filterLevel) return false;
      if (filterType !== 'all' && a.type !== filterType) return false;
      return true;
    });
  }, [filterLevel, filterType]);

  const counts = { high: mockAlerts.filter(a => a.level === 'high').length, medium: mockAlerts.filter(a => a.level === 'medium').length, low: mockAlerts.filter(a => a.level === 'low').length };
  const types = [...new Set(mockAlerts.map(a => a.type))];

  return (
    <div>
      <div className="page-header">
        <h2><ShieldAlert size={20} style={{ marginRight: 4 }} />风险预警</h2>
        <span className="muted">实时监测 · 自动扫描 587 只股票</span>
      </div>

      <div className="stat-grid mb16">
        <div className="stat-card" style={{ cursor: 'pointer', borderColor: filterLevel === 'high' ? 'var(--red-6)' : undefined }} onClick={() => setFilterLevel(filterLevel === 'high' ? 'all' : 'high')}>
          <div className="stat-label">🔴 高风险</div>
          <div className="stat-value" style={{ color: 'var(--red-6)' }}>{counts.high}</div>
        </div>
        <div className="stat-card" style={{ cursor: 'pointer', borderColor: filterLevel === 'medium' ? 'var(--orange-6)' : undefined }} onClick={() => setFilterLevel(filterLevel === 'medium' ? 'all' : 'medium')}>
          <div className="stat-label">🟠 中风险</div>
          <div className="stat-value" style={{ color: 'var(--orange-6)' }}>{counts.medium}</div>
        </div>
        <div className="stat-card" style={{ cursor: 'pointer', borderColor: filterLevel === 'low' ? 'var(--green-6)' : undefined }} onClick={() => setFilterLevel(filterLevel === 'low' ? 'all' : 'low')}>
          <div className="stat-label">🟢 低风险</div>
          <div className="stat-value" style={{ color: 'var(--green-6)' }}>{counts.low}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">总计预警</div>
          <div className="stat-value">{mockAlerts.length}</div>
        </div>
      </div>

      <div className="card mb16">
        <div className="card-header">
          <span className="card-title">预警列表</span>
          <div className="chips">
            {types.map(t => (
              <button key={t} className={`chip ${filterType === t ? 'active' : ''}`} onClick={() => setFilterType(filterType === t ? 'all' : t)}>
                <Filter size={10} style={{ marginRight: 3 }} />{t}
              </button>
            ))}
          </div>
        </div>
        <Table
          columns={[
            { title: '代码', dataIndex: 'code', width: 100, render: (v: string) => <span className="num" style={{ fontWeight: 600 }}>{v}</span> },
            { title: '等级', dataIndex: 'level', width: 90, render: (v: string) => { const I = iconMap[v]; return <span className={`tag ${colorMap[v]}`}><I size={11} />{labels[v]}</span>; } },
            { title: '类型', dataIndex: 'type', width: 120 },
            { title: '说明', dataIndex: 'desc' },
            { title: '日期', dataIndex: 'date', width: 120, render: (v: string) => <span className="muted">{v}</span> },
          ]}
          data={filtered}
          rowKey="id"
          pagination={false}
          empty={() => <div style={{ padding: 32, textAlign: 'center', color: 'var(--color-text-3)' }}>该筛选条件下暂无风险预警</div>}
        />
      </div>
    </div>
  );
}
