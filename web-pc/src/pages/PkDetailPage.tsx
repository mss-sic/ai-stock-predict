import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Tag, Button, Spin, Empty, Table, Modal, Select, Typography, Space, Message, Input, InputNumber, DatePicker, Popconfirm } from '@arco-design/web-react';
import { Trophy, Users, Calendar, ArrowLeft, Play, TrendingUp, BarChart3, Edit3, Power, StopCircle, Trash2, Medal } from 'lucide-react';
import api from '../services/api';
import { useAuth } from '../services/AuthContext';
import dayjs from 'dayjs';

const { Title, Text } = Typography;

interface PkEvent {
  id: number; name: string; description: string; type: string;
  initialCapital: number; startDate: string; endDate: string; status: string;
  entryCount: number; maxEntries: number; creatorName: string; createdBy: number;
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
  draft: { color: 'gray', text: '草稿' },
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
  const [editVisible, setEditVisible] = useState(false);
  const [editName, setEditName] = useState('');
  const [editDesc, setEditDesc] = useState('');
  const [editCapital, setEditCapital] = useState(100000);
  const [editMax, setEditMax] = useState(0);
  const [editBanner, setEditBanner] = useState('');
  const [actionLoading, setActionLoading] = useState(false);

  const fetchData = async () => {
    try {
      const res = await api.get(`/pk/events/${id}`);
      const d = res.data?.data || {};
      setEvent(d.event || null);
      setEntries(Array.isArray(d.entries) ? d.entries : []);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  const fetchStrategies = async () => {
    try {
      const res = await api.get('/strategies', { params: { exclude_pk: 'true' } });
      const list = res.data?.data;
      setStrategies(Array.isArray(list) ? list : []);
    } catch (e) {}
  };

  useEffect(() => { fetchData(); }, [id]);

  const handleStart = async () => {
    setActionLoading(true);
    try { await api.post(`/pk/events/${id}/start`); Message.success('活动已开启！'); fetchData(); }
    catch (e: any) { Message.error(e?.response?.data?.message || '操作失败'); }
    finally { setActionLoading(false); }
  };
  const handleClose = async () => {
    setActionLoading(true);
    try { await api.post(`/pk/events/${id}/close`); Message.success('活动已关闭'); fetchData(); }
    catch (e: any) { Message.error(e?.response?.data?.message || '操作失败'); }
    finally { setActionLoading(false); }
  };
  const handleEdit = async () => {
    setActionLoading(true);
    try {
      await api.put(`/pk/events/${id}`, {
        name: editName, description: editDesc, initialCapital: editCapital,
        maxEntries: editMax, bannerText: editBanner,
      });
      Message.success('已更新'); setEditVisible(false); fetchData();
    } catch (e: any) { Message.error(e?.response?.data?.message || '保存失败'); }
    finally { setActionLoading(false); }
  };
  const openEdit = () => {
    if (!event) return;
    setEditName(event.name); setEditDesc(event.description || '');
    setEditCapital(event.initialCapital); setEditMax(event.maxEntries);
    setEditBanner(event.bannerText || ''); setEditVisible(true);
  };
  const handleDelete = async () => {
    setActionLoading(true);
    try { await api.delete(`/pk/events/${id}`); Message.success('已删除'); navigate('/pk'); }
    catch (e: any) { Message.error(e?.response?.data?.message || '删除失败'); }
    finally { setActionLoading(false); }
  };

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

  const st = statusMap[event.status] || statusMap.draft;
  const myEntry = entries.find(e => e.userId === user?.id);

  const columns = [
    { title: '排名', dataIndex: 'finalRank', width: 60, render: (v: number) => <span style={{ fontWeight: 700, color: v <= 3 ? 'var(--color-warning-text)' : 'var(--color-text-2)' }}>#{v}</span> },
    {
      title: '选手 / 策略', width: 160,
      render: (_: any, record: PkEntry) => (
        <div>
          <div style={{ fontWeight: 600 }}>{record.username || `选手 #${record.userId}`}</div>
          <div style={{ fontSize: 11, color: 'var(--color-text-3)' }}>{record.strategyName || '未知'}</div>
        </div>
      ),
    },
    { title: '总收益', dataIndex: 'totalReturn', render: (v: number) => <span style={{ color: v >= 0 ? 'var(--stock-up)' : 'var(--stock-down)', fontWeight: 600 }}>{v > 0 ? '+' : ''}{v?.toFixed(2)}%</span> },
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

  // ── Podium: top 3 ──
  const displayedEntries = (Array.isArray(entries) ? entries : []).map((e: any, i: number) => ({ ...e, finalRank: e?.finalRank || i + 1 }));
  const tableData = (Array.isArray(displayedEntries) ? displayedEntries : []).slice(3, 100);

  const podiumConfig = [
    { label: '🥇', color: '#f5a623', bg: 'linear-gradient(135deg, #fffdf5 0%, #fef6e0 40%, #ffeaa7 100%)', border: '#e8b800', shadow: '0 4px 20px rgba(245,166,35,0.25)' },
    { label: '🥈', color: '#a0aab4', bg: 'linear-gradient(135deg, #fafbfc 0%, #edf0f4 40%, #d5dbe3 100%)', border: '#b0b8c0', shadow: '0 4px 20px rgba(160,170,180,0.22)' },
    { label: '🥉', color: '#d4884a', bg: 'linear-gradient(135deg, #fdf6f0 0%, #fae9d8 40%, #e8c99b 100%)', border: '#c07a30', shadow: '0 4px 20px rgba(212,136,74,0.22)' },
  ];

  // Order: 2nd (left) · 1st (center, tallest) · 3rd (right)
  const podiumOrdered: { entry: PkEntry; place: number; cfg: typeof podiumConfig[0]; height: number }[] = [];
  if (displayedEntries[1]) podiumOrdered.push({ entry: displayedEntries[1], place: 2, cfg: podiumConfig[1], height: 140 });
  if (displayedEntries[0]) podiumOrdered.push({ entry: displayedEntries[0], place: 1, cfg: podiumConfig[0], height: 170 });
  if (displayedEntries[2]) podiumOrdered.push({ entry: displayedEntries[2], place: 3, cfg: podiumConfig[2], height: 120 });

  return (
    <div style={{ padding: '24px 32px', maxWidth: 1200, margin: '0 auto' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24 }}>
        <Button type="text" icon={<ArrowLeft size={18} />} onClick={() => navigate('/pk')} />
        <Trophy size={24} style={{ color: 'var(--color-warning-6)' }} />
        <Title heading={3} style={{ margin: 0 }}>{event.name}</Title>
        <Tag color={st.color}>{st.text}</Tag>
        {user?.id === event.createdBy && (
          <Space size={8} style={{ marginLeft: 8 }}>
            {event.status === 'draft' && (
              <>
                <Button size="small" icon={<Edit3 size={12} />} onClick={openEdit}>编辑</Button>
                <Button size="small" type="primary" icon={<Power size={12} />} onClick={handleStart} loading={actionLoading}>开启</Button>
                <Popconfirm title="确定删除该活动？" onOk={handleDelete}>
                  <Button size="small" status="danger" icon={<Trash2 size={12} />} loading={actionLoading}>删除</Button>
                </Popconfirm>
              </>
            )}
            {(event.status === 'enrolling' || event.status === 'running') && (
              <Button size="small" status="danger" icon={<StopCircle size={12} />} onClick={handleClose} loading={actionLoading}>关闭</Button>
            )}
          </Space>
        )}
      </div>

      <Card style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', gap: 32, fontSize: 13, color: 'var(--color-text-2)' }}>
          <span><Calendar size={14} style={{ marginRight: 4 }} />{dayjs(event.startDate).format('YYYY-MM-DD')} ~ {dayjs(event.endDate).format('YYYY-MM-DD')}</span>
          <span><Users size={14} style={{ marginRight: 4 }} />{entries.length}人参赛</span>
          <span>起始资金 ¥{(event.initialCapital || 0).toLocaleString()}</span>
          <span>类型: {event.type === 'backtest' ? '历史回测' : '实盘PK'}</span>
        </div>
        {event.description && <div style={{ marginTop: 8, color: 'var(--color-text-3)', fontSize: 13 }}>{event.description}</div>}
        {event.status === 'enrolling' && (
          <div style={{ marginTop: 12 }}>
            <Button type="primary" icon={<Play size={14} />} onClick={() => { fetchStrategies(); setJoinVisible(true); }}>
              报名参赛
            </Button>
          </div>
        )}
      </Card>

      {/* ── Podium: Top 3 ── */}
      {podiumOrdered.length > 0 && (
        <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'flex-end', gap: 12, marginBottom: 20 }}>
          {podiumOrdered.map(({ entry, place, cfg, height }) => (
            <div
              key={entry.id}
              style={{
                flex: '0 1 200px',
                minHeight: height,
                background: cfg.bg,
                border: `1.5px solid ${cfg.border}`,
                borderRadius: 14,
                padding: '12px 10px',
                boxShadow: cfg.shadow,
                cursor: 'pointer',
                transition: 'transform 0.2s, box-shadow 0.2s',
                position: 'relative',
                overflow: 'hidden',
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'flex-start',
              }}
              onClick={() => navigate(`/pk/${id}/entry/${entry.id}`)}
              onMouseEnter={(e) => {
                e.currentTarget.style.transform = 'translateY(-3px)';
                e.currentTarget.style.boxShadow = cfg.shadow.replace('0.22)', '0.45)').replace('0.25)', '0.45)');
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.transform = 'translateY(0)';
                e.currentTarget.style.boxShadow = cfg.shadow;
              }}
            >
              <div style={{ fontSize: 22, textAlign: 'center', marginBottom: 2, lineHeight: 1 }}>{cfg.label}</div>
              <div style={{ textAlign: 'center', marginBottom: 8 }}>
                <div style={{ fontSize: 13, fontWeight: 700, color: '#1d2129' }}>{entry.username || `选手 #${entry.userId}`}</div>
                <div style={{ fontSize: 10, color: '#4e5969', marginTop: 1 }}>{entry.strategyName || '未知策略'}</div>
              </div>
              <div style={{ textAlign: 'center', marginBottom: 8 }}>
                <div style={{ fontSize: 24, fontWeight: 900, color: entry.totalReturn >= 0 ? '#e0584c' : '#2ba471', lineHeight: 1.1 }}>
                  {entry.totalReturn >= 0 ? '+' : ''}{entry.totalReturn?.toFixed(1)}%
                </div>
                <div style={{ fontSize: 10, color: '#86909c', marginTop: 2 }}>总收益率</div>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-around', borderTop: `1px solid ${cfg.border}44`, paddingTop: 8, marginTop: 'auto' }}>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 12, fontWeight: 700, color: '#4e5969' }}>
                    ¥{((entry.finalEquity || 0) / 10000).toFixed(1)}<span style={{ fontSize: 9, fontWeight: 400 }}>万</span>
                  </div>
                  <div style={{ fontSize: 9, color: '#86909c', marginTop: 1 }}>最终权益</div>
                </div>
                <div style={{ textAlign: 'center' }}>
                  <div style={{ fontSize: 12, fontWeight: 700, color: '#c4564a' }}>
                    {entry.maxDrawdown?.toFixed(1)}%
                  </div>
                  <div style={{ fontSize: 9, color: '#86909c', marginTop: 1 }}>最大回撤</div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {entries.length === 0 ? (
        <Empty description="暂无参赛者" />
      ) : (
        <>
          {tableData.length > 0 && (
            <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-2)', marginBottom: 12, display: 'flex', alignItems: 'center', gap: 6 }}>
              <Medal size={14} /> 排行榜 <span style={{ fontSize: 11, color: 'var(--color-text-3)', fontWeight: 400 }}>(第4-{Math.min(displayedEntries.length, 100)}名)</span>
            </div>
          )}
          <Table 
            columns={columns} 
            data={tableData}
            rowKey="id"
            pagination={false}
            scroll={{ x: 700 }}
          />
        </>
      )}

      <Modal
        title="编辑活动"
        visible={editVisible}
        onOk={handleEdit}
        onCancel={() => setEditVisible(false)}
        confirmLoading={actionLoading}
        okText="保存"
      >
        <Space direction="vertical" style={{ width: '100%' }} size={12}>
          <div><Text>活动名称</Text><Input value={editName} onChange={setEditName} /></div>
          <div><Text>描述</Text><Input.TextArea value={editDesc} onChange={setEditDesc} rows={2} /></div>
          <div><Text>起始资金</Text><InputNumber value={editCapital} onChange={(v) => setEditCapital(v || 0)} min={10000} style={{ width: '100%' }} suffix="元" /></div>
          <div><Text>报名上限</Text><InputNumber value={editMax} onChange={(v) => setEditMax(v || 0)} min={0} style={{ width: '100%' }} suffix="人 (0=不限)" /></div>
          <div><Text>通知文案</Text><Input value={editBanner} onChange={setEditBanner} /></div>
        </Space>
      </Modal>

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
          options={(Array.isArray(strategies) ? strategies : []).map((s: any) => ({ label: s.name, value: s.id }))}
        />
      </Modal>
    </div>
  );
}
