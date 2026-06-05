import { useState } from 'react';
import { Card, Table, Button } from '@arco-design/web-react';

interface WatchItem { id: number; stockCode: string; }

export default function WatchlistPage() {
  const [list, setList] = useState<WatchItem[]>([]);

  const remove = (id: number) => setList((prev) => prev.filter((i) => i.id !== id));

  return (
    <Card title="⭐ 自选股">
      <Table
        columns={[
          { title: '代码', dataIndex: 'stockCode' },
          { title: '操作', render: (_: any, r: WatchItem) => <Button type="text" status="danger" size="small" onClick={() => remove(r.id)}>删除</Button> },
        ]}
        data={list}
        rowKey="id"
        pagination={false}
      />
    </Card>
  );
}
