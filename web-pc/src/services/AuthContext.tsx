import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { getMe, setTokens, clearTokens, getAccessToken } from './api';

interface User {
  id: number;
  username: string;
  role: string;
  require2fa: boolean;
  lastLoginAt: string;
  lastLoginIp: string;
}

interface AuthState {
  user: User | null;
  loading: boolean;
  login: (access: string, refresh: string, user: User) => void;
  logout: () => void;
  refreshUser: () => Promise<void>;
  isAdmin: boolean;
}

const AuthContext = createContext<AuthState>({
  user: null,
  loading: true,
  login: () => {},
  logout: () => {},
  refreshUser: async () => {},
  isAdmin: false,
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const refreshUser = useCallback(async () => {
    try {
      const { data } = await getMe();
      setUser(data.data);
    } catch {
      setUser(null);
      clearTokens();
    }
  }, []);

  // On mount, check if we have a token and verify it
  useEffect(() => {
    const token = getAccessToken();
    if (token) {
      refreshUser().finally(() => setLoading(false));
    } else {
      setLoading(false);
    }
  }, [refreshUser]);

  // Listen for forced logout from interceptor
  useEffect(() => {
    const handler = () => {
      setUser(null);
      clearTokens();
    };
    window.addEventListener('auth:logout', handler);
    return () => window.removeEventListener('auth:logout', handler);
  }, []);

  const login = useCallback((access: string, refresh: string, u: User) => {
    setTokens(access, refresh);
    setUser(u);
  }, []);

  const logout = useCallback(() => {
    setUser(null);
    clearTokens();
  }, []);

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, refreshUser, isAdmin: user?.role === 'admin' }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
