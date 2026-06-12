import { useEffect, useState, useCallback } from 'react';
import { AlertCircle, CheckCircle, Info, X } from 'lucide-react';

interface ToastItem {
  id: number;
  type: 'error' | 'warning' | 'success' | 'info';
  message: string;
}

let toastId = 0;
let addToastFn: ((item: ToastItem) => void) | null = null;

export function showToast(type: ToastItem['type'], message: string) {
  if (addToastFn) {
    addToastFn({ id: ++toastId, type, message });
  } else {
    // Fallback: show alert if React isn't mounted yet
    console.error(`[${type.toUpperCase()}] ${message}`);
  }
}

export default function ToastContainer() {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const addToast = useCallback((item: ToastItem) => {
    setToasts(prev => [...prev, item]);
    setTimeout(() => {
      setToasts(prev => prev.filter(t => t.id !== item.id));
    }, 5000);
  }, []);

  useEffect(() => {
    addToastFn = addToast;
    return () => { addToastFn = null; };
  }, [addToast]);

  const remove = (id: number) => {
    setToasts(prev => prev.filter(t => t.id !== id));
  };

  if (toasts.length === 0) return null;

  const colors: Record<string, { bg: string; border: string; icon: any; color: string }> = {
    error:   { bg: 'var(--color-danger-bg)',   border: 'var(--color-danger-border)',   icon: AlertCircle, color: 'var(--color-danger-text)' },
    warning: { bg: 'var(--color-warning-bg)',   border: 'var(--color-warning-border)',   icon: AlertCircle, color: 'var(--color-warning-text)' },
    success: { bg: 'var(--color-success-bg)',   border: 'var(--color-success-border)',   icon: CheckCircle,  color: 'var(--color-success-text)' },
    info:    { bg: 'var(--color-info-bg)',      border: 'var(--color-info-border)',      icon: Info,         color: 'var(--color-primary)' },
  };

  return (
    <div style={{
      position: 'fixed', top: 16, left: '50%', transform: 'translateX(-50%)',
      zIndex: 10000, display: 'flex', flexDirection: 'column', gap: 8,
      pointerEvents: 'none',
    }}>
      {toasts.map(t => {
        const c = colors[t.type] || colors.info;
        const Icon = c.icon;
        return (
          <div key={t.id} style={{
            display: 'flex', alignItems: 'center', gap: 8,
            padding: '10px 16px', borderRadius: 8,
            background: c.bg, border: `1px solid ${c.border}`,
            boxShadow: '0 4px 12px rgba(0,0,0,0.08)',
            fontSize: 13, color: 'var(--color-text-1)',
            pointerEvents: 'auto',
            animation: 'toastSlideIn 0.3s ease',
            maxWidth: 480,
          }}>
            <Icon size={16} color={c.color} style={{ flexShrink: 0 }} />
            <span style={{ flex: 1, lineHeight: 1.4 }}>{t.message}</span>
            <button onClick={() => remove(t.id)} style={{
              background: 'none', border: 'none', cursor: 'pointer', padding: 2,
              color: 'var(--color-text-3)', flexShrink: 0,
            }}><X size={14} /></button>
          </div>
        );
      })}
    </div>
  );
}
