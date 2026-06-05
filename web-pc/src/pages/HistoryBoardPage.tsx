import { useState } from 'react';
import { DatePicker, Table, Card, Statistic, Grid } from '@arco-design/web-react';
import { fetchHistoryBoard } from '../services/api';

export default function HistoryBoardPage() {
  const [date, setDate] = useState('');
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  const handleChange = (d: string) => {
    setDate(d);
    setLoading(true);
    fetchHistoryBoard(d).then((res: any) => setData(res.data || [])).finally(() => setLoading(false));
  };

  const columns = [
    { title: '排名', dataIndex: 'rank', width: 60 },
    { title: '代码', dataIndex: 'stockCode', width: 100 },
    { title: '评分', dataIndex: 'score', width: 80, render: (v: number) => v?.toFixed(1) },
    { title: '信号', dataIndex: 'suggestion', width: 80 },
    { title: '风险', dataIndex: 'riskLevel', width: 80 },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}>📅 历史榜单</h2>
      <Grid.Row gutter={16} style={{ marginBottom: 16 }}>
        <Grid.Col span={8}>
          <DatePicker onChange={(_, ds) => handleChange(ds.dateString?.toString() || '')} />
        </Grid.Col>
        <Grid.Col span={4}><Statistic title="上榜股票" value={data.length} /></Grid.Col>
      </Grid.Row>
      <Table columns={columns} data={data} loading={loading} rowKey="id" pagination={false} />
    </div>
  );
}
