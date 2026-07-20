/**
 * Toast 通知封装 — 自实现，不依赖 Arco 命令式 API（兼容 React 19）。
 *
 * 用法：import { showToast } from './components/Toast';
 *       showToast('success', '操作成功');
 *       showToast('error', '操作失败: ' + errMsg);
 */

import React, { useState, useCallback, useEffect, useRef } from 'react';
import { createRoot } from 'react-dom/client';
import { CheckCircle, XCircle, AlertTriangle, Info, X } from 'lucide-react';

type ToastType = 'success' | 'error' | 'warning' | 'info';

interface ToastItem {
  id: number;
  type: ToastType;
  message: string;
}

const ICON_MAP: Record<ToastType, React.ReactNode> = {
  success: <CheckCircle size={16} color="#00b42a" />,
  error:   <XCircle size={16} color="#f53f3f" />,
  warning: <AlertTriangle size={16} color="#ff7d00" />,
  info:    <Info size={16} color="#165dff" />,
};

const BG_MAP: Record<ToastType, string> = {
  success: '#f0fff4',
  error:   '#fff2f0',
  warning: '#fff7e6',
  info:    '#e8f3ff',
};

const BORDER_MAP: Record<ToastType, string> = {
  success: '#b7eb8f',
  error:   '#ffccc7',
  warning: '#ffd591',
  info:    '#b7d4ff',
};

let toastId = 0;
let setToastsFn: ((updater: ToastItem[] | ((prev: ToastItem[]) => ToastItem[])) => void) | null = null;

function _ToastContainer() {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const timersRef = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map());

  useEffect(() => {
    setToastsFn = setToasts;
    return () => { setToastsFn = null; };
  }, []);

  const remove = useCallback((id: number) => {
    const timer = timersRef.current.get(id);
    if (timer) { clearTimeout(timer); timersRef.current.delete(id); }
    setToasts(prev => prev.filter(t => t.id !== id));
  }, []);

  useEffect(() => {
    toasts.forEach(t => {
      if (!timersRef.current.has(t.id)) {
        timersRef.current.set(t.id, setTimeout(() => remove(t.id), 4000));
      }
    });
  }, [toasts, remove]);

  if (toasts.length === 0) return null;

  return (
    <div style={{
      position: 'fixed', top: 20, left: '50%', transform: 'translateX(-50%)',
      zIndex: 9999, display: 'flex', flexDirection: 'column', gap: 8,
      pointerEvents: 'none',
    }}>
      {toasts.map(t => (
        <div key={t.id} style={{
          display: 'flex', alignItems: 'center', gap: 8,
          padding: '10px 16px', borderRadius: 8,
          background: BG_MAP[t.type],
          border: `1px solid ${BORDER_MAP[t.type]}`,
          boxShadow: '0 4px 12px rgba(0,0,0,0.1)',
          fontSize: 14, color: 'var(--color-text-1)',
          pointerEvents: 'auto', maxWidth: 480,
          animation: 'toastSlideIn 0.25s ease',
        }}>
          {ICON_MAP[t.type]}
          <span style={{ flex: 1 }}>{t.message}</span>
          <X size={14} color="var(--color-text-3)" style={{ cursor: 'pointer', flexShrink: 0 }}
            onClick={() => remove(t.id)} />
        </div>
      ))}
    </div>
  );
}

// Mount once
let mounted = false;
function ensureMounted() {
  if (mounted || typeof document === 'undefined') return;
  const el = document.createElement('div');
  el.id = 'toast-root';
  document.body.appendChild(el);
  createRoot(el).render(React.createElement(_ToastContainer));
  mounted = true;
}

export function showToast(type: ToastType, message: string) {
  ensureMounted();
  const id = ++toastId;
  if (setToastsFn) {
    setToastsFn(prev => [...prev, { id, type, message }]);
  } else {
    // Fallback: if root not ready yet, queue
    setTimeout(() => {
      if (setToastsFn) setToastsFn(prev => [...prev, { id, type, message }]);
    }, 100);
  }
}

// Inject keyframe once
if (typeof document !== 'undefined' && !document.getElementById('toast-keyframes')) {
  const style = document.createElement('style');
  style.id = 'toast-keyframes';
  style.textContent = `
    @keyframes toastSlideIn {
      from { opacity: 0; transform: translateY(-12px); }
      to   { opacity: 1; transform: translateY(0); }
    }
  `;
  document.head.appendChild(style);
}

export default function __ToastPlaceholder() {
  return null;
}

// Backward-compat export for App.tsx
export const ToastContainer = __ToastPlaceholder;
