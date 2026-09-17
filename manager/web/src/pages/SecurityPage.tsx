import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { userAPI } from '@/services/api';
import DataTable, { Column } from '@/components/common/DataTable';
import MetricCard from '@/components/common/MetricCard';
import type { User, Role, AuditLog } from '@/types';
import { Users, Key, ScrollText } from 'lucide-react';

const userColumns: Column<User>[] = [
  { key: 'username', label: 'Username', sortable: true },
  { key: 'full_name', label: 'Name', sortable: true },
  { key: 'email', label: 'Email' },
  { key: 'role', label: 'Role' },
  { key: 'tenant_id', label: 'Tenant ID' },
];
const roleColumns: Column<Role>[] = [
  { key: 'name', label: 'Role', sortable: true },
  { key: 'description', label: 'Description' },
  { key: 'scope', label: 'Scope' },
  { key: 'is_built_in', label: 'Built in', render: (role) => role.is_built_in ? 'Yes' : 'No' },
  { key: 'permissions', label: 'Stored Permissions', render: (role) => <code className="text-xs break-all">{JSON.stringify(role.permissions)}</code> },
];
const auditColumns: Column<AuditLog>[] = [
  { key: 'timestamp', label: 'Time', render: (event) => new Date(event.timestamp).toLocaleString() },
  { key: 'username', label: 'User' },
  { key: 'action', label: 'Action' },
  { key: 'resource', label: 'Resource' },
  { key: 'resource_id', label: 'Resource ID' },
  { key: 'success', label: 'Result', render: (event) => event.success ? 'Succeeded' : 'Failed' },
];

export default function SecurityPage() {
  const [tab, setTab] = useState<'users' | 'roles' | 'audit'>('users');
  const users = useQuery({ queryKey: ['users'], queryFn: () => userAPI.list().then((r) => r.data) });
  const roles = useQuery({ queryKey: ['roles'], queryFn: () => userAPI.listRoles().then((r) => r.data) });
  const audit = useQuery({ queryKey: ['audit'], queryFn: () => userAPI.listAuditLogs({ per_page: 20 }).then((r) => r.data) });
  const error = tab === 'users' ? users.isError : tab === 'roles' ? roles.isError : audit.isError;
  return <div className="space-y-5">
    <div><h1 className="text-2xl font-display font-semibold">Security</h1><p className="text-sm text-gray-500 mt-1">Users, stored roles and audit records</p></div>
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
      <MetricCard title="Users" value={users.isError ? '—' : users.data?.length ?? '—'} icon={Users} color="nova" />
      <MetricCard title="Roles" value={roles.isError ? '—' : roles.data?.length ?? '—'} icon={Key} color="blue" />
      <MetricCard title="Recorded Audit Events" value={audit.isError ? '—' : audit.data?.total ?? '—'} icon={ScrollText} color="copper" />
    </div>
    <div className="flex gap-3 border-b border-gray-200 pb-3">
      {(['users', 'roles', 'audit'] as const).map((name) => <button key={name} onClick={() => setTab(name)} className={tab === name ? 'font-semibold text-nova-600' : 'text-gray-500'}>{name === 'audit' ? 'Audit Log' : name === 'roles' ? 'Roles' : 'Users'}</button>)}
    </div>
    {error && <p className="text-sm text-red-600">Records unavailable. Check your access or retry. <button className="underline" onClick={() => tab === 'users' ? users.refetch() : tab === 'roles' ? roles.refetch() : audit.refetch()}>Retry</button></p>}
    {tab === 'users' && <DataTable columns={userColumns} data={users.isError ? [] : users.data ?? []} loading={users.isPending} emptyMessage={error ? 'Unavailable' : 'No users found'} />}
    {tab === 'roles' && <DataTable columns={roleColumns} data={roles.isError ? [] : roles.data ?? []} loading={roles.isPending} emptyMessage={error ? 'Unavailable' : 'No roles found'} />}
    {tab === 'audit' && <DataTable columns={auditColumns} data={audit.isError ? [] : audit.data?.data ?? []} loading={audit.isPending} emptyMessage={error ? 'Unavailable' : 'No audit events'} />}
  </div>;
}
