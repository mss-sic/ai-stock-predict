import { useEffect, useState } from 'react';
import { Table, Tag, Button } from '@arco-design/web-react';
import { useNavigate } from 'react-router-dom';
import { fetchTodayBoard } from '../services/api';

interface Stock {
  id: number; stockCode: string; rank: number; score: number; signalTags: string[];
  riskLevel: string; suggestion: string;
}

export default function BoardPage() {
  const [data, setData] = useState<Stock[]>([]);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  useEffect(() => {
    fetchTodayBoard().then((res: any) => {
      setData(res.data || []);
    }).finally(() => setLoading(false));
  }, []);

  const columns = [
    { title: '排名', dataIndex: 'rank', width: 60, sorter: (a: any, b: any) => a.rank - b.rank },
    { title: '代码', dataIndex: 'stockCode', width: 100 },
    {
      title: '概念标签', dataIndex: 'signalTags', render: (tags: string[]) =>
        tags?.map((t) => <Tag key={t} color="arcoblue" style={{ marginRight: 4 }}>{t}</Tag>)
    },
    { title: '评分', dataIndex: 'score', width: 80, render: (v: number) => v?.toFixed(1) },
    {
      title: '信号', dataIndex: 'suggestion', width: 80,
      render: (v: string) => {
        const colors: Record<string, string> = { buy: 'red', hold: 'blue', sell: 'green' };
        const labels: Record<string, string> = { buy: '买入', hold: '持有', sell: '卖出' };
        return <Tag color={colors[v] || 'gray'}>{labels[v] || v}</Tag>;
      },
    },
    { title: '风险', dataIndex: 'riskLevel', width: 80,
      render: (v: string) => <Tag color={v === 'high' ? 'red' : v === 'medium' ? 'orange' : 'green'}>{v}</Tag>
    },
    {
      title: '操作', width: 120,
      render: (_: any, record: Stock) => (
        <Button type="text" size="small" onClick={() => navigate(`/stock/${record.stockCode}`)}>详情</Button>
      ),
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}>📊 今日算法精选榜单</h2>
      <Table columns={columns} data={data} loading={loading} rowKey="id" pagination={false} />
    </div>
  );
}
