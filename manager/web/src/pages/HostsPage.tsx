import { useQuery } from '@tanstack/react-query';
import { hostAPI } from '@/services/api';
import DataTable, { Column } from '@/components/common/DataTable';
import StatusBadge from '@/components/common/StatusBadge';
import MetricCard from '@/components/common/MetricCard';
import type { Host } from '@/types';
import { Server, Cpu, MemoryStick, Wrench } from 'lucide-react';
import toast from 'react-hot-toast';

function isHealthy(status: string) {
  const s = status?.toLowerCase();
  return s === 'ready' || s === 'healthy';
}

export default function HostsPage() {
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['hosts'],
    queryFn: () => hostAPI.list().then((r) => r.data),
    refetchInterval: 30000,
  });

  const hosts = isError ? [] : data?.data ?? [];

  const toggleMaintenance = async (id: string, enable: boolean) => {
    try {
      await hostAPI.maintenance(id, enable);
      toast.success(enable ? 'Entering maintenance mode' : 'Exiting maintenance mode');
      refetch();
    } catch {
      toast.error('Failed to toggle maintenance mode');
    }
  };

  const columns: Column<Host>[] = [
    {
      key: 'name', label: 'Hostname', sortable: true,
      render: (h) => (
        <div className="flex items-center gap-2.5">
          <Server className="w-4 h-4 text-gray-400" />
          <div>
            <div className="font-medium text-gray-900 dark:text-white">{h.name}</div>
            <div className="text-[10px] text-gray-400 font-mono">{h.ip_address || '—'}</div>
          </div>
        </div>
      ),
    },
    { key: 'status', label: 'Status', sortable: true, width: '110px', render: (h) => <StatusBadge status={h.status} /> },
    {
      key: 'os_version', label: 'OS', sortable: true,
      render: (h) => <span className="text-xs text-gray-600 dark:text-gray-400">{h.os_version || '—'}</span>,
    },
    {
      key: 'cpu_cores', label: 'CPU', width: '100px', sortable: true,
      render: (h) => <span className="font-mono tabular-nums">{h.cpu_cores > 0 ? `${h.cpu_cores} cores` : '—'}</span>,
    },
    {
      key: 'memory_mb', label: 'Memory', width: '100px', sortable: true,
      render: (h) => (
        <span className="font-mono tabular-nums">
          {h.memory_mb ? `${(h.memory_mb / 1024).toFixed(0)} GB` : '—'}
        </span>
      ),
    },
    {
      key: 'kernel_version', label: 'Kernel', sortable: true,
      render: (h) => <span className="text-xs font-mono text-gray-500">{h.kernel_version || '—'}</span>,
    },
    {
      key: 'actions', label: '', width: '90px',
      render: (h) => (
        <button
          onClick={(e) => { e.stopPropagation(); toggleMaintenance(h.id, h.status !== 'Maintenance' && h.status !== 'Draining'); }}
          className="text-xs text-gray-500 hover:text-nova-600 flex items-center gap-1"
          title={h.status === 'Maintenance' || h.status === 'Draining' ? 'Exit Maintenance' : 'Enter Maintenance'}
        >
          <Wrench className="w-3.5 h-3.5" />
          {h.status === 'Maintenance' || h.status === 'Draining' ? 'Ready' : 'Maintain'}
        </button>
      ),
    },
  ];

  const healthyCount = hosts.filter((h) => isHealthy(h.status)).length;

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-display font-semibold text-gray-900 dark:text-white">Hosts</h1>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
          {hosts.length} compute nodes · {healthyCount} healthy
        </p>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <MetricCard title="Total Hosts" value={hosts.length} icon={Server} color="blue" />
        <MetricCard title="Total vCPUs" value={hosts.reduce((s, h) => s + (h.cpu_cores ?? 0), 0)} icon={Cpu} color="nova" />
        <MetricCard title="Total Memory" value={`${(hosts.reduce((s, h) => s + (h.memory_mb ?? 0), 0) / 1024).toFixed(0)} GB`} icon={MemoryStick} color="copper" />
      </div>

      {isError && <p role="alert" className="text-sm text-red-600">Host inventory refresh failed. <button onClick={() => refetch()}>Retry</button></p>}
      <DataTable columns={columns} data={hosts} loading={isLoading} emptyMessage="No enrolled host inventory is available." />
    </div>
  );
}
