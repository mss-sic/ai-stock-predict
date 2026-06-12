import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Tag, Button, Spin, Empty, Table, Modal, Select, Typography, Space, Message } from '@arco-design/web-react';
import { Trophy, Users, Calendar, ArrowLeft, Play, TrendingUp, BarChart3 } from 'lucide-react';
import api from '../services/api';
import { useAuth } from '../services/AuthContext';
import dayjs from 'dayjs';

const { Title, Text } = Typography;

interface PkEvent {
  id: number; name: string; description: string; type: string;
  initialCapital: number; startDate: string; endDate: string; status: string;
  entryCount: number; maxEntries: number; creatorName: string;
}

interface PkEntry {
  id: number; userId: number; strategyId: number; strategyName: string;
  username: string; status: string; totalReturn: number; sharpeRatio: number;
  maxDrawdown: number; winRate: number; tradeCount: number; finalRank: number;
  finalEquity: number; joinedAt: string;
}

interface Strategy {
  id: number; name: string;
}

const statusMap: Record<string, { color: string; text: string }> = {
  enrolling: { color: 'green', text: '报名中' },
  running: { color: 'blue', text: '进行中' },
  completed: { color: 'red', text: '已结束' },
};

export default function PkDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { user } = useAuth();
  const navigate = useNavigate();
  const [event, setEvent] = useState<PkEvent | null>(null);
  const [entries, setEntries] = useState<PkEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [joinVisible, setJoinVisible] = useState(false);
  const [strategies, setStrategies] = useState<Strategy[]>([]);
  const [selectedSid, setSelectedSid] = useState<number>(0);
  const [joining, setJoining] = useState(false);

  const fetchData = async () => {
    try {
      const res = await api.get(`/pk/events/${id}`);
      setEvent(res.data.event);
      setEntries(res.data.entries || []);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  const fetchStrategies = async () => {
    try {
      const res = await api.get('/strategies');
      setStrategies(res.data || []);
    } catch (e) {}
  };

  useEffect(() => { fetchData(); }, [id]);

  const handleJoin = async () => {
    if (!selectedSid) return;
    setJoining(true);
    try {
      await api.post(`/pk/events/${id}/join`, { strategyId: selectedSid });
      Message.success('报名成功！');
      setJoinVisible(false);
      fetchData();
    } catch (e: any) {
      Message.error(e?.response?.data?.message || '报名失败');
    } finally {
      setJoining(false);
    }
  };

  if (loading) return <div style={{ padding: 40, textAlign: 'center' }}><Spin size={30} /></div>;
  if (!event) return <Empty description="活动不存在" />;

  const st = statusMap[event.status] || { color: 'gray', text: event.status };
  const myEntry = entries.find((e) => e.userId === user?.uid);

  const columns = [
    { title: '排名', dataIndex: 'finalRank', width: 60, render: (v: number) => v > 0 ? `#${v}` : '-' },
    { title: '选手', dataIndex: 'username', width: 100 },
    { title: '策略', dataIndex: 'strategyName', ellipsis: true },
    { title: '收益率', dataIndex: 'totalReturn', render: (v: number) => <span style={{ color: v >= 0 ? '#f5222d' : '#52c41a', fontWeight: 600 }}>{v > 0 ? '+' : ''}{v?.toFixed(2)}%</span> },
    { title: '夏普', dataIndex: 'sharpeRatio', render: (v: number) => v?.toFixed(2) },
    { title: '最大回撤', dataIndex: 'maxDrawdown', render: (v: number) => `${v?.toFixed(2)}%` },
    { title: '胜率', dataIndex: 'winRate', render: (v: number) => `${v?.toFixed(1)}%` },
    { title: '交易', dataIndex: 'tradeCount' },
    { 
      title: '详情', width: 70, 
      render: (_: any, record: PkEntry) => (
        <Button type="text" size="small" onClick={() => navigate(`/pk/${id}/entry/${record.id}`)}>
          <BarChart3 size={14} />
        </Button>
      )
    },
  ];

  return (
    <div style={{ padding: '24px 32px', maxWidth: 1200, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24 }}>
        <Button type="text" icon={<ArrowLeft size={18} />} onClick={() => navigate('/pk')} />
        <Trophy size={24} style={{ color: 'var(--color-warning-6)' }} />
        <Title heading={3} style={{ margin: 0 }}>{event.name}</Title>
        <Tag color={st.color}>{st.text}</Tag>
      </div>

      <Card style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', gap: 32, fontSize: 13, color: 'var(--color-text-2)' }}>
          <span><Calendar size={14} style={{ marginRight: 4 }} />{dayjs(event.startDate).format('YYYY-MM-DD')} ~ {dayjs(event.endDate).format('YYYY-MM-DD')}</span>
          <span><Users size={14} style={{ marginRight: 4 }} />{entries.length}人参赛</span>
          <span>起始资金 ¥{(event.initialCapital || 0).toLocaleString()}</span>
          <span>类型: {event.type === 'backtest' ? '历史回测' : '实盘PK'}</span>
        </div>
        {event.description && <div style={{ marginTop: 8, color: 'var(--color-text-3)', fontSize: 13 }}>{event.description}</div>}
        {event.status === 'enrolling' && !myEntry && (
          <div style={{ marginTop: 12 }}>
            <Button type="primary" icon={<Play size={14} />} onClick={() => { fetchStrategies(); setJoinVisible(true); }}>
              报名参赛
            </Button>
          </div>
        )}
      </Card>

      {entries.length === 0 ? (
        <Empty description="暂无参赛者" />
      ) : (
        <Table 
          columns={columns} 
          data={entries.map((e, i) => ({ ...e, finalRank: e.finalRank || i + 1 }))} 
          rowKey="id"
          pagination={false}
          scroll={{ x: 700 }}
        />
      )}

      <Modal
        title="报名参赛"
        visible={joinVisible}
        onOk={handleJoin}
        onCancel={() => setJoinVisible(false)}
        confirmLoading={joining}
        okText="确认报名"
      >
        <div style={{ marginBottom: 12 }}>
          <Text>选择参赛策略：</Text>
        </div>
        <Select
          placeholder="请选择策略"
          style={{ width: '100%' }}
          value={selectedSid || undefined}
          onChange={(v) => setSelectedSid(v)}
          options={strategies.map((s) => ({ label: s.name, value: s.id }))}
        />
      </Modal>
    </div>
  );
}
