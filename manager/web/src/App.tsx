import { Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from '@/context/AuthContext';
import Layout from '@/components/layout/Layout';
import LoginPage from '@/pages/LoginPage';
import DashboardPage from '@/pages/DashboardPage';
import VMListPage from '@/pages/VMListPage';
import VMDetailPage from '@/pages/VMDetailPage';
import VMCreatePage from '@/pages/VMCreatePage';
import HostsPage from '@/pages/HostsPage';
import NativeHostsPage from '@/pages/NativeHostsPage';
import StoragePage from '@/pages/StoragePage';
import NetworksPage from '@/pages/NetworksPage';
import SecurityPage from '@/pages/SecurityPage';
import AIConsolePage from '@/pages/AIConsolePage';
import MonitoringPage from '@/pages/MonitoringPage';
import SettingsPage from '@/pages/SettingsPage';
import SetupPage from '@/pages/SetupPage';

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuth();
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/*"
          element={
            <ProtectedRoute>
              <Layout>
                <Routes>
                  <Route path="/" element={<Navigate to="/dashboard" replace />} />
                  <Route path="/dashboard" element={<DashboardPage />} />
                  <Route path="/setup" element={<SetupPage />} />
                  <Route path="/vms" element={<VMListPage />} />
                  <Route path="/vms/create" element={<VMCreatePage />} />
                  <Route path="/vms/:id" element={<VMDetailPage />} />
                  <Route path="/hosts" element={<HostsPage />} />
<Route path="/native-hosts" element={<NativeHostsPage />} />
                  <Route path="/storage" element={<StoragePage />} />
                  <Route path="/networks" element={<NetworksPage />} />
                  <Route path="/security" element={<SecurityPage />} />
                  <Route path="/ai" element={<AIConsolePage />} />
                  <Route path="/monitoring" element={<MonitoringPage />} />
                  <Route path="/settings" element={<SettingsPage />} />
                </Routes>
              </Layout>
            </ProtectedRoute>
          }
        />
      </Routes>
    </AuthProvider>
  );
}
