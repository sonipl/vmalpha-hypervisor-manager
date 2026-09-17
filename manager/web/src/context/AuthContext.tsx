import React, { createContext, useContext, useState, useCallback, useEffect } from 'react';
import type { User } from '@/types';
import { authAPI } from '@/services/api';

interface AuthContextType {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(() => {
    const stored = localStorage.getItem('nova_user');
    return stored ? JSON.parse(stored) : null;
  });
  const [token, setToken] = useState<string | null>(() =>
    localStorage.getItem('nova_token')
  );

  const login = useCallback(async (username: string, password: string) => {
    const { data } = await authAPI.login(username, password);
    localStorage.setItem('nova_token', data.token);
    localStorage.setItem('nova_user', JSON.stringify(data.user));
    setToken(data.token);
    setUser(data.user);
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem('nova_token');
    localStorage.removeItem('nova_user');
    setToken(null);
    setUser(null);
  }, []);

  // Verify token on mount
  useEffect(() => {
    if (token) {
      authAPI.getProfile().catch(() => logout());
    }
  }, [token, logout]);

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        isAuthenticated: !!token && !!user,
        login,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used within AuthProvider');
  return context;
}
