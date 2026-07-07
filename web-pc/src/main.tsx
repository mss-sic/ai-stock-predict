import React from 'react';
import ReactDOM from 'react-dom/client';
import { RouterProvider } from 'react-router-dom';
import { AuthProvider } from './services/AuthContext';
import { ThemeProvider } from './services/ThemeContext';
import router from './router';

// ── React 19 + Arco Design compatibility polyfill ──
// Arco Design internally uses ReactDOM.render / unmountComponentAtNode
// (removed in React 19) for imperative APIs like Modal.confirm, Message.xxx.
// This polyfill restores them via createRoot.
import * as _ReactDOM from 'react-dom';

const _rootMap = new WeakMap<Element, ReactDOM.Root>();

const _render = (element: React.ReactNode, container: Element, callback?: () => void) => {
  let root = _rootMap.get(container);
  if (!root) {
    root = ReactDOM.createRoot(container);
    _rootMap.set(container, root);
  }
  root.render(element as any);
  callback?.();
};

const _unmountComponentAtNode = (container: Element) => {
  const root = _rootMap.get(container);
  if (root) {
    root.unmount();
    _rootMap.delete(container);
  }
  return true;
};

// Patch — use Object.assign to avoid rolldown [ASSIGN_TO_IMPORT] error
Object.assign(_ReactDOM, { render: _render, unmountComponentAtNode: _unmountComponentAtNode });

// ── App Bootstrap ──

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeProvider>
      <AuthProvider>
        <RouterProvider router={router} />
      </AuthProvider>
    </ThemeProvider>
  </React.StrictMode>,
);
