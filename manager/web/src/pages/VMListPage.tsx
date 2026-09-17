import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { vmAPI } from '@/services/api';
import DataTable, { Column } from '@/components/common/DataTable';
import StatusBadge from '@/components/common/StatusBadge';
import type { VirtualMachine } from '@/types';
import {
  Plus, Search, Monitor, Play, Square, RotateCcw,
  MoreVertical,
} from 'lucide-react';
import toast from 'react-hot-toast';

export default function VMListPage() {
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [page, setPage] = useState(1);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['vms', page, statusFilter, search],
    queryFn: () =>
      vmAPI.list({
        page,
        per_page: 20,
        ...(statusFilter !== 'all' && { status: statusFilter }),
        ...(search && { search }),
      }).then((r) => r.data),
  });

  const handleAction = async (id: string, action: string) => {
    try {
      await vmAPI.action(id, action);
      toast.success(`VM ${action} initiated`);
      refetch();
    } catch {
      toast.error(`Failed to ${action} VM`);
    }
  };

  const columns: Column<VirtualMachine>[] = [
    {
      key: 'name', label: 'Name', sortable: true,
      render: (vm) => (
        <div className="flex items-center gap-2.5">
          <Monitor className="w-4 h-4 text-gray-400 flex-shrink-0" />
          <div>
            <div className="font-medium text-gray-900 dark:text-white">{vm.name}</div>
            <div className="text-[10px] text-gray-400 font-mono">{vm.id?.slice(0, 8)}</div>
          </div>
        </div>
      ),
    },
    {
      key: 'status', label: 'Status', sortable: true, width: '110px',
      render: (vm) => <StatusBadge status={vm.status} />,
    },
    { key: 'os', label: 'OS', sortable: true },
    {
      key: 'vcpus', label: 'vCPU', sortable: true, width: '70px',
      render: (vm) => <span className="font-mono tabular-nums">{vm.vcpus}</span>,
    },
    {
      key: 'memory_mb', label: 'Memory', sortable: true, width: '90px',
      render: (vm) => <span className="font-mono tabular-nums">{(vm.memory_mb / 1024).toFixed(0)} GB</span>,
    },
    {
      key: 'ip_address', label: 'IP Address', width: '130px',
      render: (vm) => (
        <span className="font-mono text-xs text-gray-600 dark:text-gray-400">
          {vm.ip_address || '—'}
        </span>
      ),
    },
    {
      key: 'host_name', label: 'Host', sortable: true,
      render: (vm) => (
        <span className="text-xs text-gray-600 dark:text-gray-400">{vm.host_node || '—'}</span>
      ),
    },
    {
      key: 'actions', label: '', width: '120px',
      render: (vm) => (
        <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          {vm.status === 'Stopped' && (
            <button
              onClick={(e) => { e.stopPropagation(); handleAction(vm.id, 'start'); }}
              className="p-1.5 text-emerald-600 hover:bg-emerald-50 dark:hover:bg-emerald-950 rounded"
              title="Start"
            >
              <Play className="w-3.5 h-3.5" />
            </button>
          )}
          {vm.status === 'Running' && (
            <>
              <button
                onClick={(e) => { e.stopPropagation(); handleAction(vm.id, 'stop'); }}
                className="p-1.5 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 rounded"
                title="Stop"
              >
                <Square className="w-3.5 h-3.5" />
              </button>
              <button
                onClick={(e) => { e.stopPropagation(); handleAction(vm.id, 'restart'); }}
                className="p-1.5 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 rounded"
                title="Restart"
              >
                <RotateCcw className="w-3.5 h-3.5" />
              </button>
            </>
          )}
          <button
            onClick={(e) => { e.stopPropagation(); }}
            className="p-1.5 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 rounded"
            title="More"
          >
            <MoreVertical className="w-3.5 h-3.5" />
          </button>
        </div>
      ),
    },
  ];


  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-display font-semibold text-gray-900 dark:text-white">Virtual Machines</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
            {isError ? 'Inventory unavailable' : isLoading ? 'Loading inventory…' : `${data?.total ?? 0} machines · ${(data?.data ?? []).filter((v) => v.status === 'Running').length} running on this page`}
          </p>
        </div>
        <button
          onClick={() => navigate('/vms/create')}
          className="btn-primary flex items-center gap-2"
        >
          <Plus className="w-4 h-4" />
          Create VM
        </button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            type="text"
            value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1); }}
            placeholder="Search VMs…"
            className="w-full pl-9 pr-3 py-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700
                       rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-nova-400"
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
          className="px-3 py-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700
                     rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-nova-400"
        >
          <option value="all">All Status</option>
          <option value="Running">Running</option>
          <option value="Stopped">Stopped</option>
          <option value="Paused">Paused</option>
          <option value="Error">Error</option>
        </select>
      </div>

      {/* Table */}
      <DataTable
        columns={columns}
        data={isError ? [] : data?.data ?? []}
        loading={isLoading}
        onRowClick={(vm) => navigate(`/vms/${vm.id}`)}
        emptyMessage={isError ? "VM inventory unavailable. Check your connection or access." : "No virtual machines found"}
      />

      {/* Pagination */}
      {(data?.total ?? 0) > 20 && (
        <div className="flex items-center justify-between text-sm">
          <span className="text-gray-500">
            Page {page} of {Math.ceil((data?.total ?? 0) / 20)}
          </span>
          <div className="flex gap-2">
            <button
              disabled={page <= 1}
              onClick={() => setPage(page - 1)}
              className="btn-secondary px-3 py-1.5 text-xs disabled:opacity-50"
            >
              Previous
            </button>
            <button
              disabled={page >= Math.ceil((data?.total ?? 0) / 20)}
              onClick={() => setPage(page + 1)}
              className="btn-secondary px-3 py-1.5 text-xs disabled:opacity-50"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
