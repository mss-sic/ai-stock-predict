// ── Arco Design React 19 适配 ──
// Arco 内置了适配器但未自动导入，必须手动引入以启用 createRoot 渲染路径
import '@arco-design/web-react/es/_util/react-19-adapter';

import React from 'react';
import ReactDOM from 'react-dom/client';
import { RouterProvider } from 'react-router-dom';
import { AuthProvider } from './services/AuthContext';
import { ThemeProvider } from './services/ThemeContext';
import router from './router';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeProvider>
      <AuthProvider>
        <RouterProvider router={router} />
      </AuthProvider>
    </ThemeProvider>
  </React.StrictMode>,
);
