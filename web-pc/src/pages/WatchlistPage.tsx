import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Table, Button, Tag, Message, Empty } from '@arco-design/web-react';
import { Star, Trash2, TrendingUp, TrendingDown, Plus } from 'lucide-react';
import { fetchWatchlist, removeWatchlist } from '../services/api';

export default function WatchlistPage() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const load = async () => {
    setLoading(true);
    try {
      const res: any = await fetchWatchlist();
      setList(res.data || []);
    } catch { setList([]); }
    setLoading(false);
  };

  useEffect(() => { load(); }, []);

  const handleRemove = async (code: string) => {
    await removeWatchlist(code);
    setList(p => p.filter(i => i.stockCode !== code));
    Message.success('已移除自选');
  };

  const columns = [
    {
      title: '代码', dataIndex: 'stockCode', width: 100,
      render: (v: string) => <span style={{ fontFamily: 'monospace', fontWeight: 600 }}>{v}</span>,
    },
    {
      title: '名称', dataIndex: 'stockName', width: 120,
      render: (v: string, r: any) => (
        <span style={{ cursor: 'pointer', color: '#165dff' }} onClick={() => navigate(`/stock/${r.stockCode}`)}>{v || r.stockCode}</span>
      ),
    },
    {
      title: '行业', dataIndex: 'industry', width: 100,
      render: (v: string) => v ? <Tag size="small" style={{ background: '#f2f3f5', border: 'none' }}>{v}</Tag> : '—',
    },
    {
      title: '最新价', dataIndex: 'price', width: 90,
      render: (v: number) => v > 0 ? <span style={{ fontWeight: 600, fontFamily: 'monospace' }}>{v.toFixed(2)}</span> : '—',
    },
    {
      title: '涨跌幅', dataIndex: 'chgPct', width: 90,
      render: (v: number) => {
        if (!v) return <span className="muted">—</span>;
        const up = v >= 0;
        return (
          <span style={{ color: up ? '#f53f3f' : '#00b42a', fontWeight: 600, fontFamily: 'monospace', display: 'flex', alignItems: 'center', gap: 2 }}>
            {up ? <TrendingUp size={12} /> : <TrendingDown size={12} />}
            {up ? '+' : ''}{v.toFixed(2)}%
          </span>
        );
      },
    },
    {
      title: '市盈率', dataIndex: 'pe', width: 80,
      render: (v: number) => v > 0 ? <span style={{ fontFamily: 'monospace' }}>{v.toFixed(1)}</span> : '—',
    },
    {
      title: '市净率', dataIndex: 'pb', width: 80,
      render: (v: number) => v > 0 ? <span style={{ fontFamily: 'monospace' }}>{v.toFixed(2)}</span> : '—',
    },
    {
      title: '加入日期', dataIndex: 'addedAt', width: 110,
      render: (v: string) => <span className="muted" style={{ fontSize: 12 }}>{v}</span>,
    },
    {
      title: '操作', width: 70,
      render: (_: any, r: any) => (
        <Button type="text" size="mini" icon={<Trash2 size={13} />}
          onClick={() => handleRemove(r.stockCode)} style={{ color: 'var(--color-text-3)' }} />
      ),
    },
  ];

  return (
    <div>
      <div className="page-header">
        <h2><Star size={20} style={{ marginRight: 8 }} />自选股</h2>
        <span className="muted">{list.length} 只</span>
        <div style={{ flex: 1 }} />
        <Button type="primary" size="small" icon={<Plus size={14} />} onClick={() => navigate('/stocks')}>
          添加股票
        </Button>
      </div>

      <div className="card">
        {list.length === 0 && !loading ? (
          <div style={{ padding: 60, textAlign: 'center' }}>
            <Empty description="暂无自选股，去股票列表添加" />
            <Button type="primary" style={{ marginTop: 16 }} onClick={() => navigate('/stocks')}>
              <Plus size={14} style={{ marginRight: 4 }} />浏览股票
            </Button>
          </div>
        ) : (
          <Table
            data={list} columns={columns} rowKey="stockCode"
            loading={loading} pagination={false} border={false} stripe size="small"
            onRow={(r) => ({
              style: { cursor: 'pointer' },
              onDoubleClick: () => navigate(`/stock/${r.stockCode}`),
            })}
          />
        )}
      </div>
    </div>
  );
}
