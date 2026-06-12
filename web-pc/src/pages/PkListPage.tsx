import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, Tag, Button, Spin, Empty, Typography, Space } from '@arco-design/web-react';
import { Trophy, Users, Calendar, Play, Plus } from 'lucide-react';
import api from '../services/api';
import { useAuth } from '../services/AuthContext';
import dayjs from 'dayjs';

const { Title, Text } = Typography;

interface PkEvent {
  id: number;
  name: string;
  description: string;
  type: string;
  initialCapital: number;
  startDate: string;
  endDate: string;
  status: string;
  entryCount: number;
  maxEntries: number;
  creatorName: string;
  bannerText: string;
  createdBy: number;
  createdAt: string;
}

const statusMap: Record<string, { color: string; text: string }> = {
  draft: { color: 'gray', text: '草稿' },
  enrolling: { color: 'green', text: '报名中' },
  running: { color: 'blue', text: '进行中' },
  completed: { color: 'red', text: '已结束' },
};

export default function PkListPage() {
  const [events, setEvents] = useState<PkEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();
  const { user } = useAuth();

  const fetchEvents = async () => {
    try {
      const res = await api.get('/pk/events');
      setEvents(res.data.data || []);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchEvents(); }, []);

  if (loading) return <div style={{ padding: 40, textAlign: 'center' }}><Spin size={30} /></div>;

  return (
    <div style={{ padding: '24px 32px', maxWidth: 1200, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 24 }}>
        <Space>
          <Trophy size={28} style={{ color: 'var(--color-primary-6)' }} />
          <Title heading={3} style={{ margin: 0 }}>策略PK</Title>
        </Space>
        <Button type="primary" icon={<Plus size={14} />} onClick={() => navigate('/pk/create')}>创建活动</Button>
      </div>

      {events.length === 0 ? (
        <Empty description="暂无PK活动" />
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(360px, 1fr))', gap: 16 }}>
          {events.map((ev) => {
            const st = statusMap[ev.status] || { color: 'gray', text: ev.status };
            return (
              <Card
                key={ev.id}
                hoverable
                style={{ cursor: 'pointer' }}
                onClick={() => navigate(`/pk/${ev.id}`)}
                title={
                  <Space>
                    <Trophy size={18} style={{ color: 'var(--color-warning-6)' }} />
                    <span style={{ fontWeight: 600 }}>{ev.name}</span>
                    <Tag color={st.color}>{st.text}</Tag>
                  </Space>
                }
              >
                <div style={{ color: 'var(--color-text-2)', fontSize: 13, marginBottom: 12 }}>
                  {ev.description || '暂无描述'}
                </div>
                <div style={{ display: 'flex', gap: 16, fontSize: 12, color: 'var(--color-text-3)' }}>
                  <span><Calendar size={12} style={{ marginRight: 4 }} />{dayjs(ev.startDate).format('MM/DD')} ~ {dayjs(ev.endDate).format('MM/DD')}</span>
                  <span><Users size={12} style={{ marginRight: 4 }} />{ev.entryCount}{ev.maxEntries > 0 ? `/${ev.maxEntries}` : ''}人</span>
                  <span>起始 ¥{(ev.initialCapital || 0).toLocaleString()}</span>
                </div>
                <div style={{ marginTop: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Tag>{ev.type === 'backtest' ? '历史回测' : '实盘PK'}</Tag>
                  <Space size={8}>
                    {ev.status === 'enrolling' && (
                      <Button type="primary" size="small" icon={<Play size={14} />} onClick={(e) => { e.stopPropagation(); navigate(`/pk/${ev.id}`); }}>立即报名</Button>
                    )}
                    {user?.id === ev.createdBy && ev.status === 'draft' && (
                      <Button size="small" onClick={(e) => { e.stopPropagation(); navigate(`/pk/${ev.id}`); }}>管理</Button>
                    )}
                  </Space>
                </div>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
