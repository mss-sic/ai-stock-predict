import { Card, Table, Tag } from '@arco-design/web-react';

const mockHoldings = [
  { id: 1, stockCode: '600519', name: '贵州茅台', costPrice: 1850, currentPrice: 1876, quantity: 100, pnl: 2600, pnlPct: 1.4 },
  { id: 2, stockCode: '000858', name: '五粮液', costPrice: 152, currentPrice: 148.5, quantity: 500, pnl: -1750, pnlPct: -2.3 },
];

export default function HoldingsPage() {
  return (
    <Card title="💼 持仓动态跟踪">
      <Table
        columns={[
          { title: '代码', dataIndex: 'stockCode' },
          { title: '名称', dataIndex: 'name' },
          { title: '成本价', dataIndex: 'costPrice', render: (v: number) => `¥${v.toFixed(2)}` },
          { title: '现价', dataIndex: 'currentPrice', render: (v: number) => `¥${v.toFixed(2)}` },
          { title: '持有', dataIndex: 'quantity', render: (v: number) => `${v}股` },
          { title: '盈亏', dataIndex: 'pnl', render: (v: number) => <span style={{ color: v >= 0 ? '#00b42a' : '#f53f3f' }}>¥{v.toFixed(2)}</span> },
          { title: '盈亏%', dataIndex: 'pnlPct', render: (v: number) => <Tag color={v >= 0 ? 'green' : 'red'}>{v >= 0 ? '+' : ''}{v.toFixed(1)}%</Tag> },
        ]}
        data={mockHoldings}
        rowKey="id"
        pagination={false}
      />
    </Card>
  );
}
