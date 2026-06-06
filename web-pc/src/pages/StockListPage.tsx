import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Input, Select, Table, Button, Tag, Pagination, Message } from '@arco-design/web-react';
import { Star, Search, Eye, StarOff } from 'lucide-react';
import { fetchStocks, addWatchlist, removeWatchlist, fetchWatchlist } from '../services/api';

export default function StockListPage() {
  const [stocks, setStocks] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [industry, setIndustry] = useState('');
  const [page, setPage] = useState(1);
  const [watched, setWatched] = useState<Set<string>>(new Set());
  const navigate = useNavigate();

  const loadStocks = useCallback(async () => {
    setLoading(true);
    try {
      const res: any = await fetchStocks({ page, pageSize: 20, keyword, industry });
      setStocks(res.data || []);
      setTotal(res.total || 0);
    } catch { setStocks([]); }
    setLoading(false);
  }, [page, keyword, industry]);

  const loadWatchlist = async () => {
    try {
      const res: any = await fetchWatchlist();
      const codes = new Set((res.data || []).map((i: any) => i.stockCode));
      setWatched(codes);
    } catch {}
  };

  useEffect(() => { loadStocks(); loadWatchlist(); }, [loadStocks]);

  const toggleWatch = async (code: string) => {
    if (watched.has(code)) {
      await removeWatchlist(code);
      watched.delete(code);
      Message.success('已取消自选');
    } else {
      await addWatchlist(code);
      watched.add(code);
      Message.success('已添加自选');
    }
    setWatched(new Set(watched));
  };

  const columns = [
    {
      title: '代码', dataIndex: 'code', width: 100,
      render: (v: string) => <span style={{ fontFamily: 'monospace', fontWeight: 600, color: '#1d2129' }}>{v}</span>,
    },
    {
      title: '名称', dataIndex: 'name', width: 160,
      render: (v: string, r: any) => (
        <span style={{ cursor: 'pointer', color: '#165dff' }} onClick={() => navigate(`/stock/${r.code}`)}>{v}</span>
      ),
    },
    {
      title: '行业', dataIndex: 'industry', width: 120,
      render: (v: string) => v ? <Tag size="small" style={{ background: '#f2f3f5', border: 'none' }}>{v}</Tag> : <span className="muted">—</span>,
    },
    {
      title: '操作', width: 80,
      render: (_: any, r: any) => (
        <Button
          type="text" size="mini"
          icon={watched.has(r.code) ? <StarOff size={14} /> : <Star size={14} />}
          onClick={() => toggleWatch(r.code)}
          style={{ color: watched.has(r.code) ? '#f7ba1e' : 'var(--color-text-3)' }}
        />
      ),
    },
  ];

  return (
    <div>
      <div className="page-header">
        <h2><Search size={20} style={{ marginRight: 8 }} />股票列表</h2>
        <span className="muted">{total} 只个股</span>
      </div>

      <div className="card mb16">
        <div className="card-header" style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <Input
            prefix={<Search size={14} />}
            placeholder="代码 / 名称搜索"
            value={keyword}
            onChange={(v) => { setKeyword(v); setPage(1); }}
            style={{ width: 220 }}
            allowClear
          />
          <Select
            placeholder="行业筛选"
            value={industry || undefined}
            onChange={(v) => { setIndustry(v || ''); setPage(1); }}
            style={{ width: 160 }}
            allowClear
            options={[
              '电子','机械设备','通信','基础化工','有色金属','公用事业','电力设备','医药生物',
              '环保','汽车','计算机','食品饮料','建筑材料','交通运输','房地产','银行','商贸零售',
              '轻工制造','国防军工','家用电器','农林牧渔','石油石化','美容护理','社会服务',
              '传媒','煤炭','钢铁','综合','纺织服饰',
            ].map(i => ({ label: i, value: i }))}
          />
          <span style={{ flex: 1 }} />
          <Pagination
            current={page} total={total} pageSize={20} size="small" simple
            onChange={(p) => setPage(p)}
          />
        </div>
        <div style={{ padding: 0 }}>
          <Table
            data={stocks} columns={columns} rowKey="code"
            loading={loading} pagination={false} border={false} stripe
            size="small"
            onRow={(r) => ({
              style: { cursor: 'pointer' },
              onDoubleClick: () => navigate(`/stock/${r.code}`),
            })}
          />
        </div>
        <div style={{ padding: '10px 16px', display: 'flex', justifyContent: 'flex-end' }}>
          <Pagination current={page} total={total} pageSize={20}
            onChange={(p) => setPage(p)} showTotal size="small" />
        </div>
      </div>
    </div>
  );
}
