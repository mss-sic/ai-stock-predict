/**
 * Toast 通知封装 — 基于 Arco Design Message。
 *
 * 用法：import { showToast } from './components/Toast';
 *       showToast('success', '操作成功');
 *       showToast('error', '操作失败: ' + errMsg);
 */

import { Message } from '@arco-design/web-react';

type ToastType = 'success' | 'error' | 'warning' | 'info';

export function showToast(type: ToastType, message: string) {
  Message[type]({ content: message, duration: 4000, position: 'top' });
}

export default function ToastContainer() {
  // Arco Message 是命令式调用，不需要 DOM 容器
  return null;
}
