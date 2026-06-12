import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../services/api';
import { Trophy, X } from 'lucide-react';

interface PkEvent {
  id: number;
  name: string;
  type: string;
  bannerText: string;
}

export default function PkNotice() {
  const [events, setEvents] = useState<PkEvent[]>([]);
  const [visible, setVisible] = useState(true);
  const navigate = useNavigate();

  const fetchNotice = () => {
    api.get('/pk/active-notice').then((res: any) => {
      const list = res.data.data || [];
      setEvents(list);
    }).catch(() => {});
  };

  useEffect(() => {
    fetchNotice();
    const timer = setInterval(fetchNotice, 30000);
    return () => clearInterval(timer);
  }, []);

  if (!visible || events.length === 0) return null;

  return (
    <div style={{
      background: 'var(--color-bg-1)',
      color: 'var(--color-text-1)',
      padding: '10px 24px',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      cursor: 'pointer',
      fontSize: 13,
      borderBottom: '2px solid var(--color-primary)',
      position: 'relative',
      zIndex: 10,
    }}>
      <div
        style={{ display: 'flex', alignItems: 'center', gap: 10, flex: 1 }}
        onClick={() => navigate(`/pk/${events[0].id}`)}
      >
        <Trophy size={18} style={{ color: 'var(--color-warning)' }} />
        <span style={{ fontWeight: 600, color: 'var(--color-primary)' }}>策略PK</span>
        <span style={{ color: 'var(--color-text-2)' }}>
          {events[0].bannerText || `「${events[0].name}」正在报名中！`}
        </span>
        {events.length > 1 && (
          <span style={{ color: 'var(--color-text-3)', fontSize: 12 }}>等{events.length}个活动</span>
        )}
      </div>
      <X
        size={14}
        style={{ color: 'var(--color-text-3)', cursor: 'pointer' }}
        onClick={(e) => { e.stopPropagation(); setVisible(false); }}
      />
    </div>
  );
}
