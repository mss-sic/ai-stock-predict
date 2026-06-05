import { useState } from 'react';
import { Card, Form, InputNumber, Button, Tabs, Table } from '@arco-design/web-react';

export default function StrategyPage() {
  const [params, setParams] = useState({ stopProfit: 15, stopLoss: -8, maxHoldDays: 20 });

  return (
    <Tabs defaultActiveTab="1">
      <Tabs.TabPane key="1" title="交易计划">
        <Card>
          <Form layout="inline" style={{ marginBottom: 16 }}>
            <Form.Item label="止盈%"><InputNumber value={params.stopProfit} onChange={(v) => setParams({ ...params, stopProfit: v as number })} /></Form.Item>
            <Form.Item label="止损%"><InputNumber value={params.stopLoss} onChange={(v) => setParams({ ...params, stopLoss: v as number })} /></Form.Item>
            <Form.Item label="最大持有日"><InputNumber value={params.maxHoldDays} onChange={(v) => setParams({ ...params, maxHoldDays: v as number })} /></Form.Item>
            <Form.Item><Button type="primary">计算预期收益</Button></Form.Item>
          </Form>
        </Card>
      </Tabs.TabPane>
      <Tabs.TabPane key="2" title="策略回测">
        <Card>
          <p style={{ color: 'var(--color-text-3)' }}>回测功能基于 🚧 构建中，敬请期待。</p>
        </Card>
      </Tabs.TabPane>
    </Tabs>
  );
}
