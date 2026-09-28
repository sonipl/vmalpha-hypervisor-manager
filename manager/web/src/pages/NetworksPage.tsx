import { useState } from 'react';
import DistributedNetworks from '@/components/DistributedNetworks';
import { useQuery } from '@tanstack/react-query';
import { networkAPI } from '@/services/api';
import DataTable, { Column } from '@/components/common/DataTable';
import MetricCard from '@/components/common/MetricCard';
import type { Network, FirewallRule } from '@/types';
import { Network as NetworkIcon, Shield } from 'lucide-react';

export default function NetworksPage() {
  const [tab, setTab] = useState<'networks' | 'firewalls'>('networks');

  const { data: networks, isLoading, isError: networkError } = useQuery({
    queryKey: ['networks'],
    queryFn: () => networkAPI.list().then((r) => r.data),
  });

  const { data: firewalls, isError: firewallError, isLoading: firewallsLoading } = useQuery({
    queryKey: ['firewall-rules'],
    queryFn: () => networkAPI.listFirewallRules().then((r) => r.data),
  });

  const netData = networkError ? [] : networks?.data ?? [];
  const fwData = firewallError ? [] : firewalls ?? [];

  const netColumns: Column<Network>[] = [
    {
      key: 'name', label: 'Network', sortable: true,
      render: (n) => (
        <div className="flex items-center gap-2">
          <NetworkIcon className="w-4 h-4 text-gray-400" />
          <div>
            <div className="font-medium text-gray-900 dark:text-white">{n.name}</div>
            <div className="text-[10px] text-gray-400 font-mono">{n.type}</div>
          </div>
        </div>
      ),
    },
    { key: 'vlan_id', label: 'VLAN', width: '80px', render: (n) => <span className="font-mono tabular-nums">{n.vlan_id || '—'}</span> },
    { key: 'subnet', label: 'Subnet', render: (n) => <span className="font-mono text-xs">{n.subnet}</span> },
    { key: 'gateway', label: 'Gateway', render: (n) => <span className="font-mono text-xs">{n.gateway}</span> },
  ];

  const fwColumns: Column<FirewallRule>[] = [
    {
      key: 'name', label: 'Rule', sortable: true,
      render: (r) => <span className="font-medium text-gray-900 dark:text-white">{r.name}</span>,
    },
    {
      key: 'direction', label: 'Direction', width: '100px',
      render: (r) => (
        <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${
          r.direction === 'Ingress' ? 'bg-blue-50 dark:bg-blue-950 text-blue-700 dark:text-blue-400' : 'bg-purple-50 dark:bg-purple-950 text-purple-700 dark:text-purple-400'
        }`}>
          {r.direction}
        </span>
      ),
    },
    { key: 'protocol', label: 'Protocol', width: '90px', render: (r) => <span className="font-mono text-xs uppercase">{r.protocol}</span> },
    { key: 'port_range', label: 'Port(s)', width: '100px', render: (r) => <span className="font-mono text-xs">{r.port_range}</span> },
    { key: 'source', label: 'Source', render: (r) => <span className="font-mono text-xs">{r.source}</span> },
    {
      key: 'action', label: 'Action', width: '90px',
      render: (r) => (
        <span className={`text-xs font-semibold ${r.action === 'Allow' ? 'text-emerald-600' : 'text-red-600'}`}>
          {r.action?.toUpperCase()}
        </span>
      ),
    },
    { key: 'priority', label: 'Priority', width: '80px', render: (r) => <span className="font-mono tabular-nums">{r.priority}</span> },
  ];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-display font-semibold text-gray-900 dark:text-white">Networking</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">Registered networks and stored firewall rules</p>
        </div>

      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-2 gap-4">
        <MetricCard title="Networks" value={networkError || isLoading ? '—' : networks?.total ?? '—'} icon={NetworkIcon} color="nova" />
        <MetricCard title="Firewall Rules" value={firewallError || firewallsLoading ? '—' : fwData.length} icon={Shield} color="red" />
      </div>

      <div className="border-b border-gray-200 dark:border-gray-800">
        <div className="flex gap-1">
          {[
            { key: 'networks' as const, label: 'Networks', icon: NetworkIcon },
            { key: 'firewalls' as const, label: 'Firewall Rules', icon: Shield },
          ].map((t) => (
            <button key={t.key} onClick={() => setTab(t.key)}
              className={`flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${
                tab === t.key ? 'border-nova-500 text-nova-600 dark:text-nova-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'
              }`}>
              <t.icon className="w-4 h-4" /> {t.label}
            </button>
          ))}
        </div>
      </div>

      {tab === 'networks' && <DistributedNetworks />}
      {tab === 'networks' && <DataTable columns={netColumns} data={netData} loading={isLoading} emptyMessage={networkError ? "Network inventory unavailable" : "No networks found"} />}
      {tab === 'firewalls' && <DataTable columns={fwColumns} data={fwData} loading={firewallsLoading} emptyMessage={firewallError ? "Firewall rules unavailable" : "No firewall rules"} />}
    </div>
  );
}
