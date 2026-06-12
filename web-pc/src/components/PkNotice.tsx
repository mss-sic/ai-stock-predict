import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../services/api';
import { Swords, X } from 'lucide-react';

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

  useEffect(() => {
    api.get('/pk/active-notice').then((res: any) => {
      if (res.data?.length > 0) {
        setEvents(res.data);
      }
    }).catch(() => {});
  }, []);

  if (!visible || events.length === 0) return null;

  return (
    <div style={{
      background: 'linear-gradient(135deg, var(--color-primary-6) 0%, #6c5ce7 100%)',
      color: '#fff',
      padding: '8px 20px',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      cursor: 'pointer',
      fontSize: 13,
      position: 'relative',
      zIndex: 10,
    }}>
      <div 
        style={{ display: 'flex', alignItems: 'center', gap: 8, flex: 1 }}
        onClick={() => navigate(`/pk/${events[0].id}`)}
      >
        <Swords size={16} />
        <span style={{ fontWeight: 600 }}>策略PK</span>
        <span style={{ opacity: 0.9 }}>
          {events[0].bannerText || `「${events[0].name}」正在报名中！`}
        </span>
        {events.length > 1 && (
          <span style={{ opacity: 0.7, fontSize: 12 }}>等{events.length}个活动</span>
        )}
      </div>
      <X size={14} style={{ opacity: 0.7, cursor: 'pointer' }} onClick={(e) => { e.stopPropagation(); setVisible(false); }} />
    </div>
  );
}
