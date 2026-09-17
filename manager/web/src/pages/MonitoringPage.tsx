import { useQuery } from '@tanstack/react-query';
import { monitoringAPI } from '@/services/api';
import MetricCard from '@/components/common/MetricCard';
import StatusBadge from '@/components/common/StatusBadge';
import DataTable, { Column } from '@/components/common/DataTable';
import type { AlertRule, AuditLog } from '@/types';
import { Activity, Bell } from 'lucide-react';

const alertColumns: Column<AlertRule>[] = [
  { key: 'name', label: 'Rule', sortable: true },
  { key: 'expression', label: 'Expression' },
  { key: 'duration', label: 'Duration' },
  { key: 'severity', label: 'Severity', render: (rule) => <StatusBadge status={rule.severity} /> },
  { key: 'enabled', label: 'Configuration', render: (rule) => rule.enabled ? 'Enabled' : 'Disabled' },
];
const eventColumns: Column<AuditLog>[] = [
  { key: 'timestamp', label: 'Time', render: (event) => new Date(event.timestamp).toLocaleString() },
  { key: 'username', label: 'User' },
  { key: 'action', label: 'Action' },
  { key: 'resource', label: 'Resource' },
  { key: 'resource_id', label: 'Resource ID' },
  { key: 'success', label: 'Result', render: (event) => event.success ? 'Succeeded' : 'Failed' },
];

export default function MonitoringPage() {
  const alerts = useQuery({
    queryKey: ['alert-rules'],
    queryFn: () => monitoringAPI.listAlertRules().then((r) => r.data),
    refetchInterval: 30000,
  });
  const events = useQuery({
    queryKey: ['events'],
    queryFn: () => monitoringAPI.getEvents({ per_page: 15 }).then((r) => r.data),
    refetchInterval: 30000,
  });
  return <div className="space-y-5">
    <div><h1 className="text-2xl font-display font-semibold">Monitoring</h1>
      <p className="text-sm text-gray-500 mt-1">Saved alert rules and recent audit events</p></div>
    <div className="card p-4 text-sm text-gray-500">
      Live alert state, event history charts and API latency are unavailable until monitoring is connected.
    </div>
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <MetricCard title="Saved Alert Rules" value={alerts.isError ? '—' : alerts.data?.length ?? '—'} icon={Bell} color="nova" />
      <MetricCard title="Recorded Audit Events" value={events.isError ? '—' : events.data?.total ?? '—'} icon={Activity} color="blue" />
    </div>
    <section>
      <h2 className="text-sm font-semibold mb-3">Alert Rules</h2>
      <DataTable columns={alertColumns} data={alerts.isError ? [] : alerts.data ?? []} loading={alerts.isPending}
        emptyMessage={alerts.isError ? 'Alert rules unavailable. Check your access or retry.' : 'No saved alert rules.'} />
      {alerts.isError && <button className="text-sm text-nova-600 mt-2" onClick={() => alerts.refetch()}>Retry alert rules</button>}
    </section>
    <section>
      <h2 className="text-sm font-semibold mb-3">Recent Audit Events</h2>
      <DataTable columns={eventColumns} data={events.isError ? [] : events.data?.data ?? []} loading={events.isPending}
        emptyMessage={events.isError ? 'Audit events unavailable. Check your access or retry.' : 'No recorded events.'} />
      {events.isError && <button className="text-sm text-nova-600 mt-2" onClick={() => events.refetch()}>Retry audit events</button>}
    </section>
  </div>;
}
