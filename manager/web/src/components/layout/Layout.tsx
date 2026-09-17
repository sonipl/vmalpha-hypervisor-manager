import React, { useState } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import { useAuth } from '@/context/AuthContext';
import {
  LayoutDashboard, Server, Network, Shield, Brain,
  Activity, Settings, Monitor, ChevronLeft, ChevronRight,
  Search, Moon, Sun, LogOut, Menu,
  Plus, Database, Cpu, Wrench,
} from 'lucide-react';

interface LayoutProps {
  children: React.ReactNode;
}

const navItems = [
  { label: 'Dashboard', icon: LayoutDashboard, path: '/dashboard', group: 'Overview' },
  { label: 'Cluster Setup', icon: Wrench, path: '/setup', group: 'Overview' },
  { label: 'Virtual Machines', icon: Monitor, path: '/vms', group: 'Workloads' },
  { label: 'Hosts', icon: Cpu, path: '/hosts', group: 'Infrastructure' },
 { label: 'Native hosts', icon: Server, path: '/native-hosts', group: 'Infrastructure' },
  { label: 'Storage', icon: Database, path: '/storage', group: 'Infrastructure' },
  { label: 'Networking', icon: Network, path: '/networks', group: 'Infrastructure' },
  { label: 'Monitoring', icon: Activity, path: '/monitoring', group: 'Operations' },
  { label: 'Security', icon: Shield, path: '/security', group: 'Security' },
  { label: 'AI & Automation', icon: Brain, path: '/ai', group: 'AI Engine' },
  { label: 'Settings', icon: Settings, path: '/settings', group: 'Platform' },
];

export default function Layout({ children }: LayoutProps) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const [collapsed, setCollapsed] = useState(false);
  const [darkMode, setDarkMode] = useState(() =>
    document.documentElement.classList.contains('dark')
  );
  const [navigationSearch, setNavigationSearch] = useState('');
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const toggleDark = () => {
    document.documentElement.classList.toggle('dark');
    setDarkMode(!darkMode);
  };

  let currentGroup = '';

  return (
    <div className="flex h-screen overflow-hidden bg-surface-DEFAULT dark:bg-[#0D1117]">
      {/* ── Sidebar ── */}
      <aside
        className={`${collapsed ? 'w-16' : 'w-56'} ${mobileMenuOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'}
          fixed md:relative z-30 h-full flex flex-col bg-white dark:bg-surface-dark
          border-r border-gray-200 dark:border-gray-800 transition-all duration-200`}
      >
        {/* Logo */}
        <div className="flex items-center gap-2 px-4 h-14 border-b border-gray-200 dark:border-gray-800">
          <div className="w-8 h-8 rounded-lg bg-nova-500 flex items-center justify-center flex-shrink-0">
            <Server className="w-4 h-4 text-white" />
          </div>
          {!collapsed && (
            <span className="font-display text-lg text-gray-900 dark:text-white truncate">
              VM Alpha Manager
            </span>
          )}
        </div>

        {/* Quick Create */}
        {!collapsed && (
          <div className="px-3 py-3">
            <button
              onClick={() => navigate('/vms/create')}
              className="btn-primary w-full flex items-center justify-center gap-2 text-xs"
            >
              <Plus className="w-3.5 h-3.5" />
              Create VM
            </button>
          </div>
        )}

        {/* Nav Items */}
        <nav className="flex-1 overflow-y-auto px-2 py-2">
          {navItems.filter(item=>item.label.toLowerCase().includes(navigationSearch.toLowerCase())).map((item) => {
            const showGroup = item.group !== currentGroup;
            currentGroup = item.group;
            return (
              <React.Fragment key={item.path}>
                {showGroup && !collapsed && (
                  <div className="px-3 pt-4 pb-1 text-[10px] font-mono font-medium uppercase tracking-widest text-gray-400 dark:text-gray-600">
                    {item.group}
                  </div>
                )}
                <NavLink
                  to={item.path}
                  className={({ isActive }) =>
                    `sidebar-link ${isActive ? 'active' : ''} ${collapsed ? 'justify-center px-0' : ''}`
                  }
                  title={collapsed ? item.label : undefined}
                  onClick={() => setMobileMenuOpen(false)}
                >
                  <item.icon className="w-4 h-4 flex-shrink-0" />
                  {!collapsed && <span>{item.label}</span>}
                </NavLink>
              </React.Fragment>
            );
          })}
        </nav>

        {/* Collapse Toggle */}
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="hidden md:flex items-center justify-center h-10 border-t border-gray-200 dark:border-gray-800
                     text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
        >
          {collapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
        </button>
      </aside>

      {/* ── Main Area ── */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Top Bar */}
        <header className="h-14 flex items-center justify-between px-4 border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-surface-dark flex-shrink-0">
          <div className="flex items-center gap-3">
            <button
              className="md:hidden text-gray-500"
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
            >
              <Menu className="w-5 h-5" />
            </button>

            {/* Search */}
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
              <input
                type="text"
                aria-label="Find a page"
                placeholder="Find a page..."
                value={navigationSearch}
                onChange={e=>setNavigationSearch(e.target.value)}
                className="pl-9 pr-4 py-1.5 w-64 lg:w-96 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700
                         rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-nova-400 focus:border-transparent
                         placeholder:text-gray-400"
              />

            </div>
          </div>

          <div className="flex items-center gap-2">
            {/* Theme Toggle */}
            <button
              aria-label={darkMode ? "Use light theme" : "Use dark theme"}
              onClick={toggleDark}
              className="p-2 text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 transition-colors rounded-md hover:bg-gray-100 dark:hover:bg-gray-800"
            >
              {darkMode ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />}
            </button>

            {/* User Menu */}
            <div className="flex items-center gap-2 ml-2 pl-2 border-l border-gray-200 dark:border-gray-700">
              <div className="w-7 h-7 rounded-full bg-nova-100 dark:bg-nova-900 flex items-center justify-center">
                <span className="text-xs font-semibold text-nova-600 dark:text-nova-300">
                  {user?.full_name?.charAt(0) || 'U'}
                </span>
              </div>
              <div className="hidden lg:block">
                <div className="text-sm font-medium text-gray-700 dark:text-gray-300">{user?.full_name}</div>
                <div className="text-[10px] text-gray-400 font-mono">{user?.role}</div>
              </div>
              <button
                onClick={logout}
                className="p-1.5 text-gray-400 hover:text-red-500 transition-colors"
                title="Logout"
              >
                <LogOut className="w-4 h-4" />
              </button>
            </div>
          </div>
        </header>

        {/* Content */}
        <main className="flex-1 overflow-y-auto p-6">
          {children}
        </main>
      </div>

      {/* Mobile overlay */}
      {mobileMenuOpen && (
        <div
          className="fixed inset-0 bg-black/30 z-20 md:hidden"
          onClick={() => setMobileMenuOpen(false)}
        />
      )}
    </div>
  );
}
