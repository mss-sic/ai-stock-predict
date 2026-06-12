import { useState, useEffect, useMemo } from 'react';
import { Table, Button, Tag } from '@arco-design/web-react';
import { ShieldAlert, AlertTriangle, Info, RefreshCw, EyeOff } from 'lucide-react';
import { fetchRiskAlerts, ignoreRiskAlert } from '../services/api';

const iconMap: Record<string, any> = { high: AlertTriangle, medium: ShieldAlert, low: Info };
const colorMap: Record<string, string> = { high: 'red', medium: 'orange', low: 'green' };
const labels: Record<string, string> = { high: '高风险', medium: '中风险', low: '低风险' };

export default function RiskPage() {
  const [alerts, setAlerts] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [filterLevel, setFilterLevel] = useState<string>('all');
  const [filterType, setFilterType] = useState<string>('all');

  const loadAlerts = async () => {
    setLoading(true);
    try { const res: any = await fetchRiskAlerts(); setAlerts(res.data?.data || []); }
    catch { setAlerts([]); }
    finally { setLoading(false); }
  };

  useEffect(() => { loadAlerts(); }, []);

  const handleIgnore = async (id: number) => {
    try {
      await ignoreRiskAlert(id);
      setAlerts(prev => prev.filter(a => a.id !== id));
    } catch {}
  };

  const filtered = useMemo(() => {
    return alerts.filter((a: any) => {
      if (filterLevel !== 'all' && a.level !== filterLevel) return false;
      if (filterType !== 'all' && a.type !== filterType) return false;
      return true;
    });
  }, [alerts, filterLevel, filterType]);

  const counts = {
    high: alerts.filter((a: any) => a.level === 'high').length,
    medium: alerts.filter((a: any) => a.level === 'medium').length,
    low: alerts.filter((a: any) => a.level === 'low').length,
  };
  const types = [...new Set(alerts.map((a: any) => a.type))];

  return (
    <div>
      <div className="page-header">
        <h2><ShieldAlert size={20} style={{ marginRight: 4 }} />风险预警</h2>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span className="muted">监测 {alerts.length > 0 ? [...new Set(alerts.map((a: any) => a.stockCode))].length : 0} 只持仓股票</span>
          <Button size="small" icon={<RefreshCw size={12} />} loading={loading} onClick={loadAlerts}>刷新</Button>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 16 }}>
        <div
          onClick={() => setFilterLevel(filterLevel === 'high' ? 'all' : 'high')}
          style={{
            background: '#fff', borderRadius: 8, border: filterLevel === 'high' ? '2px solid #f53f3f' : '1px solid #e5e6eb',
            padding: '16px 20px', cursor: 'pointer', transition: 'all 0.15s',
          }}
        >
          <div style={{ fontSize: 13, color: 'var(--color-text-3)', marginBottom: 6 }}>🔴 高风险</div>
          <div style={{ fontSize: 28, fontWeight: 700, color: '#f53f3f' }}>{counts.high}</div>
        </div>
        <div
          onClick={() => setFilterLevel(filterLevel === 'medium' ? 'all' : 'medium')}
          style={{
            background: '#fff', borderRadius: 8, border: filterLevel === 'medium' ? '2px solid #ff7d00' : '1px solid #e5e6eb',
            padding: '16px 20px', cursor: 'pointer', transition: 'all 0.15s',
          }}
        >
          <div style={{ fontSize: 13, color: 'var(--color-text-3)', marginBottom: 6 }}>🟠 中风险</div>
          <div style={{ fontSize: 28, fontWeight: 700, color: '#ff7d00' }}>{counts.medium}</div>
        </div>
        <div
          onClick={() => setFilterLevel(filterLevel === 'low' ? 'all' : 'low')}
          style={{
            background: '#fff', borderRadius: 8, border: filterLevel === 'low' ? '2px solid #00b42a' : '1px solid #e5e6eb',
            padding: '16px 20px', cursor: 'pointer', transition: 'all 0.15s',
          }}
        >
          <div style={{ fontSize: 13, color: 'var(--color-text-3)', marginBottom: 6 }}>🟢 低风险</div>
          <div style={{ fontSize: 28, fontWeight: 700, color: '#00b42a' }}>{counts.low}</div>
        </div>
        <div style={{
          background: '#fff', borderRadius: 8, border: '1px solid #e5e6eb', padding: '16px 20px',
        }}>
          <div style={{ fontSize: 13, color: 'var(--color-text-3)', marginBottom: 6 }}>总计预警</div>
          <div style={{ fontSize: 28, fontWeight: 700, color: 'var(--color-text-1)' }}>{alerts.length}</div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <span style={{ fontSize: 15, fontWeight: 600 }}>预警列表</span>
          <div style={{ display: 'flex', gap: 6 }}>
            {types.map(t => (
              <Button
                key={t}
                size="mini"
                type={filterType === t ? 'primary' : 'outline'}
                onClick={() => setFilterType(filterType === t ? 'all' : t)}
              >
                {t}
              </Button>
            ))}
          </div>
        </div>
        <div className="card-body" style={{ padding: 0 }}>
          {filtered.length === 0 ? (
            <div style={{ padding: 48, textAlign: 'center', color: 'var(--color-text-3)', fontSize: 13 }}>
              {alerts.length === 0 ? '暂无风险预警，持仓安全 🎉' : '该筛选条件下暂无风险预警'}
            </div>
          ) : (
            <Table
              data={filtered}
              rowKey="id"
              size="small"
              columns={[
                {
                  title: '名称', dataIndex: 'stockName', width: 100, ellipsis: true,
                  render: (v: string) => <span style={{ fontSize: 13 }}>{v || '-'}</span>
                },
                {
                  title: '代码', dataIndex: 'stockCode', width: 85,
                  render: (v: string) => <span style={{ fontFamily: 'monospace', fontWeight: 600, fontSize: 12, color: 'var(--color-text-2)' }}>{v}</span>
                },
                {
                  title: '等级', dataIndex: 'level', width: 100,
                  render: (v: string) => {
                    const I = iconMap[v];
                    return <Tag color={colorMap[v]}><I size={11} style={{ marginRight: 2 }} />{labels[v]}</Tag>;
                  }
                },
                { title: '类型', dataIndex: 'type', width: 110 },
                { title: '说明', dataIndex: 'description', ellipsis: true },
                {
                  title: '触发时间', dataIndex: 'hitDate', width: 160,
                  render: (v: string) => <span style={{ fontSize: 12, color: 'var(--color-text-3)' }}>{v ? new Date(v).toLocaleString('zh-CN') : '-'}</span>
                },
                {
                  title: '操作', width: 70,
                  render: (_: any, record: any) => (
                    <Button size="mini" type="text" icon={<EyeOff size={12} />} onClick={() => handleIgnore(record.id)}>忽略</Button>
                  )
                },
              ]}
              pagination={false}
              border={false}
              stripe
            />
          )}
        </div>
      </div>
    </div>
  );
}
