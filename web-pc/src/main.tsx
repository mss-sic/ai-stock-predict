import React from 'react';
import ReactDOM from 'react-dom/client';
import { RouterProvider } from 'react-router-dom';
import { AuthProvider } from './services/AuthContext';
import { ThemeProvider } from './services/ThemeContext';
import router from './router';
// Suppress React 19 deprecation warning from @arco-design/web-react (third-party library)
// "Accessing element.ref was removed in React 19. ref is now a regular prop."
const _origConsoleError = console.error.bind(console);
console.error = (...args: any[]) => {
  const msg = typeof args[0] === 'string' ? args[0] : '';
  if (msg.includes('element.ref was removed') || msg.includes('Accessing element.ref')) return;
  _origConsoleError(...args);
};


ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeProvider><AuthProvider>
      <RouterProvider router={router} />
    </AuthProvider></ThemeProvider>
  </React.StrictMode>
);
