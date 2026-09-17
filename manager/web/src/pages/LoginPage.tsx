import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/context/AuthContext';
import { Server, Eye, EyeOff, AlertCircle } from 'lucide-react';

export default function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await login(username, password);
      navigate('/dashboard');
    } catch {
      setError('Invalid credentials. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex bg-gradient-to-br from-gray-50 via-nova-50/30 to-copper-50/20 dark:from-[#0D1117] dark:via-[#0D1117] dark:to-[#0D1117]">
      {/* Left – Branding */}
      <div className="hidden lg:flex lg:w-1/2 items-center justify-center p-12 relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-br from-nova-600 to-nova-800 dark:from-nova-900 dark:to-[#0D1117]" />
        <div className="absolute inset-0 opacity-10"
          style={{ backgroundImage: 'radial-gradient(circle at 1px 1px, rgba(255,255,255,.15) 1px, transparent 0)', backgroundSize: '32px 32px' }}
        />
        <div className="relative z-10 max-w-md text-white">
          <div className="flex items-center gap-3 mb-8">
            <div className="w-12 h-12 rounded-xl bg-white/20 backdrop-blur flex items-center justify-center">
              <Server className="w-6 h-6 text-white" />
            </div>
            <span className="font-display text-3xl">VM Alpha Manager</span>
          </div>
          <h1 className="text-4xl font-display mb-4 leading-tight">
            Manage your VM Alpha hosts
          </h1>
          <p className="text-nova-100 text-lg leading-relaxed">
            Connect to your hosts, manage native virtual machines, and follow recorded operations.
          </p>
          <div className="mt-10 grid grid-cols-3 gap-4 text-sm">
            {[
              ['Hosts', 'Live inventory'],
              ['Virtual machines', 'Native operations'],
              ['Tasks', 'Recorded outcomes'],
            ].map(([title, sub]) => (
              <div key={title} className="bg-white/10 backdrop-blur rounded-lg p-3">
                <div className="font-semibold">{title}</div>
                <div className="text-nova-200 text-xs mt-0.5">{sub}</div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Right – Form */}
      <div className="flex-1 flex items-center justify-center p-6">
        <div className="w-full max-w-sm">
          <div className="lg:hidden flex items-center gap-2 mb-8 justify-center">
            <div className="w-10 h-10 rounded-xl bg-nova-500 flex items-center justify-center">
              <Server className="w-5 h-5 text-white" />
            </div>
            <span className="font-display text-2xl text-gray-900 dark:text-white">VM Alpha Manager</span>
          </div>

          <h2 className="text-xl font-display text-gray-900 dark:text-white mb-1">Welcome back</h2>
          <p className="text-sm text-gray-500 dark:text-gray-400 mb-6">Sign in to your platform account</p>

          {error && (
            <div className="flex items-center gap-2 p-3 mb-4 bg-red-50 dark:bg-red-950/50 border border-red-200 dark:border-red-800 rounded-lg text-red-700 dark:text-red-400 text-sm">
              <AlertCircle className="w-4 h-4 flex-shrink-0" />
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="username" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                Username
              </label>
              <input
                id="username"
                autoComplete="username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="w-full px-3 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700
                           rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-nova-400 focus:border-transparent"
                placeholder="admin"
                autoFocus
                required
              />
            </div>
            <div>
              <label htmlFor="password" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                Password
              </label>
              <div className="relative">
                <input
                  id="password"
                  autoComplete="current-password"
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full px-3 py-2 pr-10 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700
                             rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-nova-400 focus:border-transparent"
                  placeholder="••••••••"
                  required
                />
                <button
                  type="button"
                  aria-label={showPassword ? "Hide password" : "Show password"}
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
                >
                  {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="btn-primary w-full py-2.5 flex items-center justify-center gap-2"
            >
              {loading ? (
                <>
                  <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                  Signing in…
                </>
              ) : (
                'Sign In'
              )}
            </button>
          </form>

          <div className="mt-6 text-center">
            <p className="text-xs text-gray-400">
              Use the account configured by your platform administrator.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
