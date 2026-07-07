import React, { useState, useCallback } from 'react';
import { Modal, Button } from '@arco-design/web-react';

export interface ConfirmOptions {
  title: string;
  content: React.ReactNode;
  okText?: string;
  cancelText?: string;
  onOk?: () => void | Promise<void>;
  onCancel?: () => void;
}

/**
 * Drop-in replacement for Modal.confirm() that works with React 19.
 * Uses controlled Modal component instead of imperative API.
 *
 * Usage:
 *   const { confirm, ConfirmModal } = useConfirm();
 *   await confirm({ title: '确认?', content: '确定吗?', onOk: () => {...} });
 *   // Render <ConfirmModal /> in your JSX
 */
export function useConfirm() {
  const [opts, setOpts] = useState<ConfirmOptions | null>(null);
  const [loading, setLoading] = useState(false);

  const confirm = useCallback((options: ConfirmOptions) => {
    return new Promise<void>((resolve) => {
      setOpts({
        ...options,
        onOk: async () => {
          setLoading(true);
          try {
            await options.onOk?.();
            resolve();
          } finally {
            setLoading(false);
            setOpts(null);
          }
        },
        onCancel: () => {
          options.onCancel?.();
          setOpts(null);
          resolve();
        },
      });
    });
  }, []);

  const modal = opts ? (
    <Modal
      title={opts.title}
      visible={true}
      onOk={opts.onOk}
      onCancel={opts.onCancel}
      okText={opts.okText || '确定'}
      cancelText={opts.cancelText || '取消'}
      confirmLoading={loading}
      unmountOnExit
    >
      {opts.content}
    </Modal>
  ) : null;

  return { confirm, ConfirmModal: () => modal };
}
