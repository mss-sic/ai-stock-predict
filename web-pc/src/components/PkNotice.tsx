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

  const bannerText = events[0].bannerText || `「${events[0].name}」正在报名中！`;
  const suffix = events.length > 1 ? `  等${events.length}个活动` : '';

  return (
    <div style={{
      background: 'var(--color-bg-1)',
      padding: '8px 24px',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      cursor: 'pointer',
      fontSize: 13,
      position: 'relative',
      zIndex: 10,
    }}>
      <style>{`
        @keyframes pkNoticeScroll {
          0% { transform: translateX(0); }
          100% { transform: translateX(-50%); }
        }
        .pk-notice-track {
          display: inline-flex;
          white-space: nowrap;
          animation: pkNoticeScroll 28s linear infinite;
        }
        .pk-notice-track:hover {
          animation-play-state: paused;
        }
      `}</style>

      <div
        onClick={() => navigate(`/pk/${events[0].id}`)}
        style={{
          display: 'flex', alignItems: 'center', gap: 10, flex: 1,
          overflow: 'hidden', minWidth: 0,
        }}
      >
        <Trophy size={18} style={{ color: 'var(--color-warning)', flexShrink: 0 }} />
        <span style={{ fontWeight: 600, color: 'var(--color-primary)', flexShrink: 0 }}>策略PK</span>

        <div style={{ flex: 1, overflow: 'hidden', minWidth: 0 }}>
          <div className="pk-notice-track">
            <span style={{ color: 'var(--color-text-2)', paddingRight: 48 }}>
              {bannerText}{suffix}
            </span>
            <span style={{ color: 'var(--color-text-2)', paddingRight: 48 }}>
              {bannerText}{suffix}
            </span>
          </div>
        </div>
      </div>

      <X
        size={14}
        style={{ color: 'var(--color-text-3)', cursor: 'pointer', flexShrink: 0 }}
        onClick={(e) => { e.stopPropagation(); setVisible(false); }}
      />
    </div>
  );
}
