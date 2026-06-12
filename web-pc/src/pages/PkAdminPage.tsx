import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, Form, Input, InputNumber, Select, DatePicker, Button, Message, Typography, Space, Divider, Tag } from '@arco-design/web-react';
import { Trophy, ArrowLeft, Send, Calendar, Clock } from 'lucide-react';
import api from '../services/api';
import dayjs from 'dayjs';

const { Title } = Typography;
const FormItem = Form.Item;

const timePresets = [
  { label: '近1月', value: '1m' },
  { label: '近3月', value: '3m' },
  { label: '近6月', value: '6m' },
  { label: '近1年', value: '1y' },
  { label: '自定义', value: 'custom' },
];

export default function PkAdminPage() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [submitting, setSubmitting] = useState(false);
  const [preset, setPreset] = useState('custom');

  const handlePreset = (val: string) => {
    setPreset(val);
    const now = dayjs();
    const map: Record<string, [dayjs.Dayjs, dayjs.Dayjs]> = {
      '1m': [now.subtract(30, 'day'), now],
      '3m': [now.subtract(90, 'day'), now],
      '6m': [now.subtract(180, 'day'), now],
      '1y': [now.subtract(365, 'day'), now],
    };
    if (map[val]) {
      form.setFieldsValue({ startDate: map[val][0].format('YYYY-MM-DD'), endDate: map[val][1].format('YYYY-MM-DD') });
    }
  };

  const handleSubmit = async (values: any) => {
    setSubmitting(true);
    try {
      await api.post('/pk/events', {
        name: values.name,
        description: values.description || '',
        type: values.type || 'backtest',
        initialCapital: values.initialCapital || 100000,
        startDate: values.startDate,
        endDate: values.endDate,
        stockPool: values.stockPool || 'all',
        stockPoolParams: '',
        maxEntries: values.maxEntries || 0,
        bannerText: values.bannerText || '',
      });
      Message.success('活动创建成功！');
      navigate('/pk');
    } catch (e: any) {
      Message.error(e?.response?.data?.message || '创建失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div style={{ padding: '24px 32px', maxWidth: 720, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24 }}>
        <Button type="text" icon={<ArrowLeft size={18} />} onClick={() => navigate('/pk')} />
        <Trophy size={22} style={{ color: 'var(--color-warning-6)' }} />
        <Title heading={3} style={{ margin: 0 }}>创建PK活动</Title>
      </div>

      <Card>
        <Form
          form={form}
          layout="vertical"
          onSubmit={handleSubmit}
          initialValues={{
            type: 'backtest',
            initialCapital: 100000,
            stockPool: 'all',
            maxEntries: 0,
          }}
        >
          <FormItem label="活动名称" field="name" rules={[{ required: true, message: '请输入活动名称' }]}>
            <Input placeholder="如：2026春季策略PK大赛" maxLength={50} />
          </FormItem>

          <FormItem label="活动描述" field="description">
            <Input.TextArea placeholder="活动简介（选填）" maxLength={200} rows={2} />
          </FormItem>

          <FormItem label="PK类型" field="type">
            <Select
              options={[
                { label: '历史回测', value: 'backtest' },
                { label: '实盘PK', value: 'live' },
              ]}
            />
          </FormItem>

          <FormItem label="起始资金" field="initialCapital">
            <InputNumber min={10000} max={10000000} step={10000} style={{ width: '100%' }} suffix="元" />
          </FormItem>

          <Divider />
          <div style={{ color: 'var(--color-text-2)', fontSize: 13, marginBottom: 12, display: 'flex', alignItems: 'center', gap: 6 }}>
            <Calendar size={14} /> 时间范围
          </div>

          <FormItem style={{ marginBottom: 8 }}>
            <Select
              value={preset}
              onChange={handlePreset}
              options={timePresets}
              style={{ width: 160 }}
            />
          </FormItem>

          <div style={{ display: 'flex', gap: 16 }}>
            <FormItem label="开始日期" field="startDate" rules={[{ required: true, message: '请选择' }]}>
              <Input placeholder="2026-01-01" />
            </FormItem>
            <FormItem label="结束日期" field="endDate" rules={[{ required: true, message: '请选择' }]}>
              <Input placeholder="2026-06-01" />
            </FormItem>
          </div>

          <Divider />
          <div style={{ display: 'flex', gap: 16 }}>
            <FormItem label="股票池" field="stockPool" style={{ flex: 1 }}>
              <Select
                options={[
                  { label: '全部A股', value: 'all' },
                ]}
              />
            </FormItem>
            <FormItem label="报名上限" field="maxEntries" style={{ flex: 1 }}>
              <InputNumber min={0} max={1000} style={{ width: '100%' }} suffix="人 (0=不限)" />
            </FormItem>
          </div>

          <FormItem label="首页通知文案" field="bannerText">
            <Input placeholder="如：策略PK大赛正在报名中！快来参赛吧" maxLength={100} />
          </FormItem>

          <Divider />
          <FormItem>
            <Space>
              <Button type="primary" htmlType="submit" loading={submitting} icon={<Send size={14} />}>
                创建活动
              </Button>
              <Button onClick={() => navigate('/pk')}>取消</Button>
            </Space>
          </FormItem>
        </Form>
      </Card>
    </div>
  );
}
