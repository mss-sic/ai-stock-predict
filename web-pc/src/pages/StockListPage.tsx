import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Input, Select, Table, Button, Tag, Pagination, Message, Modal } from '@arco-design/web-react';
import { Star, Search, Eye, StarOff } from 'lucide-react';
import { fetchStocks, addToWatchlist, removeFromWatchlist, fetchWatchlist, fetchWatchlistGroups, createWatchlistGroup } from '../services/api';

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
      setStocks(res.data?.data || []);
      setTotal(res.data?.total || 0);
    } catch { setStocks([]); }
    setLoading(false);
  }, [page, keyword, industry]);

  const loadWatchlist = async () => {
    try {
      const res: any = await fetchWatchlist();
      const codes = new Set((res.data?.data || []).map((i: any) => i.stockCode));
      setWatched(codes);
    } catch {}
  };

  useEffect(() => { loadStocks(); loadWatchlist(); }, [loadStocks]);

  const [addStockCode, setAddStockCode] = useState('');
  const [addGroupId, setAddGroupId] = useState<number>(0);
  const [groups, setGroups] = useState<any[]>([]);
  const [newGroupInput, setNewGroupInput] = useState('');
  const [showAddModal, setShowAddModal] = useState(false);

  useEffect(() => {
    fetchWatchlistGroups().then(({ data }) => setGroups(data.data || [])).catch(() => {});
  }, [watched]);

  const toggleWatch = async (code: string) => {
    if (watched.has(code)) {
      await removeFromWatchlist(code);
      watched.delete(code);
      setWatched(new Set(watched));
      Message.success('已取消自选');
    } else {
      setAddStockCode(code);
      setAddGroupId(0);
      setShowAddModal(true);
    }
  };

  const handleAddWithGroup = async () => {
    if (!addStockCode) return;
    try {
      if (newGroupInput.trim()) {
        const { data } = await createWatchlistGroup(newGroupInput.trim());
        const gid = data.data?.id || 0;
        await addToWatchlist(addStockCode, gid);
        setNewGroupInput('');
      } else {
        await addToWatchlist(addStockCode, addGroupId);
      }
      watched.add(addStockCode);
      setWatched(new Set(watched));
      setShowAddModal(false);
      Message.success('已添加自选');
    } catch (err: any) {
      Message.error(err.response?.data?.message || err.message || '添加失败');
    }
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
            data={stocks || []} columns={columns} rowKey="code"
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
