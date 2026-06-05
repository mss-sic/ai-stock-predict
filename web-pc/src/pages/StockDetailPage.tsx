import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { Card, Descriptions, Spin } from '@arco-design/web-react';
import { fetchStockDetail, fetchKLine } from '../services/api';
import KLineChart from '../components/KLineChart';

export default function StockDetailPage() {
  const { code } = useParams<{ code: string }>();
  const [stock, setStock] = useState<any>(null);
  const [klines, setKlines] = useState<any[]>([]);

  useEffect(() => {
    if (!code) return;
    fetchStockDetail(code).then((res: any) => setStock(res.data));
    fetchKLine(code).then((res: any) => setKlines(res.data || []));
  }, [code]);

  if (!stock) return <Spin loading style={{ display: 'block', margin: '100px auto' }} />;

  return (
    <div>
      <Card style={{ marginBottom: 16 }}>
        <Descriptions title={`${stock.name} (${stock.code})`} column={4} data={[
          { label: '行业', value: stock.industry || '-' },
          { label: '总股本', value: stock.totalShares ? `${(stock.totalShares / 1e8).toFixed(2)}亿` : '-' },
        ]} />
      </Card>
      <Card title="K线图">
        <KLineChart data={klines} />
      </Card>
    </div>
  );
}
