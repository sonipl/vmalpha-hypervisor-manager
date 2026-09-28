import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { monitoringAPI, vmAPI } from '@/services/api';
import MetricCard from '@/components/common/MetricCard';
import StatusBadge from '@/components/common/StatusBadge';
import UtilizationGauge from '@/components/common/UtilizationGauge';
import { Monitor, Server, ArrowRight, Activity } from 'lucide-react';
import { ResponsiveContainer, PieChart, Pie, Cell, Tooltip } from 'recharts';

const COLORS: Record<string, string> = {
  Running: '#14b8a6', Stopped: '#6b7280', Paused: '#f59e0b', Error: '#ef4444',
  Migrating: '#3b82f6', Provisioning: '#8b5cf6', Other: '#94a3b8',
};

export default function DashboardPage() {
  const navigate = useNavigate();
  const metricsQuery = useQuery({
    queryKey: ['dashboard-metrics'],
    queryFn: () => monitoringAPI.getDashboard().then((r) => r.data),
    refetchInterval: 30000,
  });
  const vmsQuery = useQuery({
    queryKey: ['vms-summary'],
    queryFn: () => vmAPI.list({ per_page: '5', sort: '-created_at' }).then((r) => r.data),
    refetchInterval: 30000,
  });
  // Do not present cached inventory as current after a failed refresh.
  const metrics = metricsQuery.isError ? undefined : metricsQuery.data;
  const vms = vmsQuery.isError ? undefined : vmsQuery.data;
  const healthTone = metrics?.cluster_health === 'Healthy' ? 'green' : metrics?.cluster_health === 'Warning' ? 'amber' : ['Degraded', 'Critical', 'Unhealthy'].includes(metrics?.cluster_health ?? '') ? 'red' : 'neutral';
  const statuses = metrics ? [
    { name: 'Running', value: metrics.vms.running },
    { name: 'Stopped', value: metrics.vms.stopped },
    { name: 'Paused', value: metrics.vms.paused },
    { name: 'Error', value: metrics.vms.error },
    { name: 'Migrating', value: metrics.vms.migrating },
    { name: 'Provisioning', value: metrics.vms.provisioning },
  ] : [];
  const other = metrics ? Math.max(0, metrics.vms.total - statuses.reduce((sum, s) => sum + s.value, 0)) : 0;
  if (other) statuses.push({ name: 'Other', value: other });
  const state = metricsQuery.isPending ? 'Loading inventory…' : metricsQuery.isError ? 'Inventory unavailable. Retry or check your access.' : 'Latest inventory';

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-display font-semibold text-gray-900 dark:text-white">Dashboard</h1>
        <p className="text-sm text-gray-500 mt-1">{state}</p>
        {metricsQuery.isError && <button className="text-sm text-nova-600 mt-2" onClick={() => metricsQuery.refetch()}>Retry inventory</button>}
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <MetricCard title="Virtual Machines" value={metrics?.vms.total ?? '—'}
          subtitle={metrics ? `${metrics.vms.running} running` : 'Unavailable'} icon={Monitor} color="nova" />
        <MetricCard title="Hosts" value={metrics?.hosts.total ?? '—'}
          subtitle={metrics ? `${metrics.hosts.ready} ready in inventory` : 'Unavailable'} icon={Server} color="blue" />
        <MetricCard title="Cluster Health" value={metricsQuery.isPending ? 'Checking…' : metricsQuery.isError ? 'Unavailable' : metrics?.cluster_health ?? 'Unknown'}
          subtitle={metricsQuery.isPending ? "Reading live host and Ceph health" : metricsQuery.isError ? "Health request failed; retrying on refresh" : metrics?.cluster_health === "Healthy" ? "Live host and Ceph checks passed" : metrics?.cluster_health_scope ?? "Live cluster health is unavailable"} icon={Activity} color={healthTone} statusTone={healthTone} />
      </div>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div className="card p-5">
          <h2 className="text-sm font-semibold mb-4">VM Status Distribution</h2>
          {metrics && metrics.vms.total > 0 ? <>
            <ResponsiveContainer width="100%" height={200}>
              <PieChart><Pie data={statuses} dataKey="value" innerRadius={50} outerRadius={80}>
                {statuses.map((s) => <Cell key={s.name} fill={COLORS[s.name]} />)}
              </Pie><Tooltip /></PieChart>
            </ResponsiveContainer>
            <div className="flex flex-wrap gap-3 justify-center text-xs">
              {statuses.map((s) => <span key={s.name}>{s.name}: {s.value}</span>)}
            </div>
          </> : <p className="text-sm text-gray-500">{metrics ? 'No virtual machines in inventory.' : state}</p>}
        </div>
        <div className="card p-5">
          <h2 className="text-sm font-semibold mb-4">Resource Utilization</h2>
          <div className="space-y-4">
            {([
              ['CPU', metrics?.utilization.cpu_percent],
              ['Memory', metrics?.utilization.memory_percent],
              ['Host root filesystems', metrics?.utilization.storage_percent],
            ] as const).map(([label, value]) => typeof value === 'number' && Number.isFinite(value)
              ? <UtilizationGauge key={label} label={label} value={value} />
              : <p key={label} className="text-sm text-gray-500">{label}: unavailable</p>)}
            <p className="text-sm text-gray-500">{metrics?.telemetry_status === 'live' ? 'Live host samples. Filesystem usage excludes shared Ceph capacity.' : metrics?.telemetry_status === 'partial' ? 'Some hosts are unavailable; aggregate utilization is withheld.' : 'Live host telemetry is unavailable.'}</p>
          </div>
        </div>
      </div>
      <div className="card p-5">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-sm font-semibold">Recent VMs</h2>
          <button onClick={() => navigate('/vms')} className="text-xs text-nova-600 flex items-center gap-1">View all <ArrowRight className="w-3 h-3" /></button>
        </div>
        {vmsQuery.isPending ? <p className="text-sm text-gray-500">Loading virtual machines…</p>
          : vmsQuery.isError ? <p className="text-sm text-gray-500">VM inventory unavailable.</p>
          : !vms?.data?.length ? <p className="text-sm text-gray-500">No virtual machines in inventory.</p>
          : vms.data.slice(0, 5).map((vm) => <button key={vm.id} onClick={() => navigate(`/vms/${vm.id}`)}
              className="w-full text-left flex items-center justify-between p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800">
              <div><div className="text-sm font-medium">{vm.name}</div><div className="text-xs text-gray-500">{vm.vcpus} vCPU · {(vm.memory_mb / 1024).toFixed(1)} GB</div></div>
              <StatusBadge status={vm.status} />
            </button>)}
      </div>
    </div>
  );
}
