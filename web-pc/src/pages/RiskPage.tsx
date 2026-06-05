import { Card, Table, Tag } from '@arco-design/web-react';

const mockAlerts = [
  { id: 1, stockCode: '000001', level: 'high', type: '连榜过热', description: '连续上榜15个交易日', hitDate: '2026-06-03' },
  { id: 2, stockCode: '002456', level: 'medium', type: '技术破位', description: '跌破20日均线支撑', hitDate: '2026-06-02' },
];

export default function RiskPage() {
  return (
    <Card title="⚠️ 风险预警">
      <Table
        columns={[
          { title: '代码', dataIndex: 'stockCode' },
          { title: '等级', dataIndex: 'level', render: (v: string) => <Tag color={v === 'high' ? 'red' : v === 'medium' ? 'orange' : 'green'}>{v}</Tag> },
          { title: '类型', dataIndex: 'type' },
          { title: '说明', dataIndex: 'description' },
          { title: '日期', dataIndex: 'hitDate' },
        ]}
        data={mockAlerts}
        rowKey="id"
        pagination={false}
      />
    </Card>
  );
}
