import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import toast from 'react-hot-toast';
import { Shield, Save, Plug } from 'lucide-react';
import api from '@/services/api';

type LDAPSettings = {
  enabled: boolean;
  url: string;
  base_dn: string;
  bind_dn: string;
  bind_secret?: string;
  user_filter: string;
  group_base: string;
  domain: string;
  use_tls: boolean;
};

export default function SettingsPage() {
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ['settings-ldap'],
    queryFn: async () => (await api.get<{ data: LDAPSettings }>('/settings/ldap')).data.data,
  });

  const [form, setForm] = useState<LDAPSettings | null>(null);
  const current = form ?? data;

  const save = useMutation({
    mutationFn: async () => api.put('/settings/ldap', current),
    onSuccess: () => {
      toast.success('LDAP settings saved');
      qc.invalidateQueries({ queryKey: ['settings-ldap'] });
    },
    onError: (e: any) => toast.error(e?.response?.data?.error || 'Save failed'),
  });

  const test = useMutation({
    mutationFn: async () => (await api.post('/settings/ldap/test')).data,
    onSuccess: (res) => toast.success(res.message || 'Test complete'),
    onError: () => toast.error('LDAP test failed'),
  });

  if (isLoading || !current) {
    return <div className="p-6 text-sm text-gray-500">Loading settings…</div>;
  }

  const set = (k: keyof LDAPSettings, v: string | boolean) =>
    setForm({ ...current, [k]: v });

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-display font-semibold text-gray-900 dark:text-white">Settings</h1>
        <p className="text-sm text-gray-500 mt-1">
          Configure LDAP now — VM Alpha Manager will use these settings when directory authentication is enabled.
        </p>
      </div>

      <form
        className="card p-5 space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate();
        }}
      >
        <h2 className="font-semibold flex items-center gap-2">
          <Shield className="w-4 h-4 text-nova-600" /> LDAP / directory
        </h2>

        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={current.enabled}
            onChange={(e) => set('enabled', e.target.checked)}
          />
          Enable LDAP authentication (login integration pending)
        </label>

        <label className="block text-xs text-gray-500">Server URL
          <input className="mt-1 w-full rounded-md border border-gray-300 dark:border-gray-700 bg-transparent px-3 py-2 font-mono text-sm"
            value={current.url} onChange={(e) => set('url', e.target.value)} />
        </label>
        <label className="block text-xs text-gray-500">Base DN
          <input className="mt-1 w-full rounded-md border border-gray-300 dark:border-gray-700 bg-transparent px-3 py-2 font-mono text-sm"
            value={current.base_dn} onChange={(e) => set('base_dn', e.target.value)} />
        </label>
        <label className="block text-xs text-gray-500">Bind DN
          <input className="mt-1 w-full rounded-md border border-gray-300 dark:border-gray-700 bg-transparent px-3 py-2 font-mono text-sm"
            value={current.bind_dn} onChange={(e) => set('bind_dn', e.target.value)} />
        </label>
        <label className="block text-xs text-gray-500">Bind password
          <input type="password" className="mt-1 w-full rounded-md border border-gray-300 dark:border-gray-700 bg-transparent px-3 py-2"
            placeholder="********"
            onChange={(e) => set('bind_secret', e.target.value)} />
        </label>
        <label className="block text-xs text-gray-500">User filter
          <input className="mt-1 w-full rounded-md border border-gray-300 dark:border-gray-700 bg-transparent px-3 py-2 font-mono text-sm"
            value={current.user_filter} onChange={(e) => set('user_filter', e.target.value)} />
        </label>
        <label className="block text-xs text-gray-500">Group base
          <input className="mt-1 w-full rounded-md border border-gray-300 dark:border-gray-700 bg-transparent px-3 py-2 font-mono text-sm"
            value={current.group_base} onChange={(e) => set('group_base', e.target.value)} />
        </label>
        <label className="block text-xs text-gray-500">Domain
          <input className="mt-1 w-full rounded-md border border-gray-300 dark:border-gray-700 bg-transparent px-3 py-2"
            value={current.domain} onChange={(e) => set('domain', e.target.value)} />
        </label>

        <div className="flex gap-2 pt-2">
          <button type="submit" className="btn-primary inline-flex items-center gap-2" disabled={save.isPending}>
            <Save className="w-4 h-4" /> {save.isPending ? 'Saving…' : 'Save'}
          </button>
          <button type="button" className="btn-secondary inline-flex items-center gap-2" onClick={() => test.mutate()} disabled={test.isPending}>
            <Plug className="w-4 h-4" /> Test connection
          </button>
        </div>
      </form>
    </div>
  );
}
