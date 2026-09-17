import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import toast from 'react-hot-toast';
import { Server, Database, RefreshCw, Plus, Trash2, Zap } from 'lucide-react';
import api from '@/services/api';

type SetupStatus = {
  kubernetes: { available: boolean; ready?: number; total?: number; nodes?: Array<{ name: string; ready: boolean }> };
  kubevirt: { available: boolean };
  ceph: { available: boolean; summary?: string };
  cluster?: {
    size?: string;
    storage_class?: string;
    ceph_enabled?: boolean;
    can_enable_ceph?: boolean;
    ceph_network?: string;
  };
  registered_hosts?: Array<{ id: string; name: string; ip_address: string; labels?: Record<string, string> }>;
};

export default function SetupPage() {
  const qc = useQueryClient();
  const { data, isLoading, refetch, isFetching } = useQuery({
    queryKey: ['setup-status'],
    queryFn: async () => (await api.get<SetupStatus>('/setup/status')).data,
    refetchInterval: 15_000,
  });

  const [nodeForm, setNodeForm] = useState({ name: '', ip: '', role: 'worker', hostname: '' });
  const [cephForm, setCephForm] = useState({ name: '', ip: '', hostname: '', devices: '' });
  const [lastJoin, setLastJoin] = useState('');
  const [lastCeph, setLastCeph] = useState('');

  const addNode = useMutation({
    mutationFn: async () => (await api.post('/setup/nodes', nodeForm)).data,
    onSuccess: (res) => {
      toast.success(`Registered ${res.host?.name || 'node'}`);
      setLastJoin(res.join_command || res.ceph_command || '');
      qc.invalidateQueries({ queryKey: ['setup-status'] });
    },
    onError: (e: any) => toast.error(e?.response?.data?.error || 'Failed to add node'),
  });

  const addCeph = useMutation({
    mutationFn: async () => (await api.post('/setup/ceph/hosts', cephForm)).data,
    onSuccess: (res) => {
      toast.success(res.applied ? 'Ceph host add applied' : 'Ceph command ready');
      setLastCeph(res.ceph_command || '');
      qc.invalidateQueries({ queryKey: ['setup-status'] });
    },
    onError: (e: any) => toast.error(e?.response?.data?.error || 'Failed to add Ceph host'),
  });

  const enableCeph = useMutation({
    mutationFn: async () => (await api.post('/setup/ceph/enable')).data,
    onSuccess: (res) => {
      toast.success(res.message || 'Ceph bootstrap started');
      qc.invalidateQueries({ queryKey: ['setup-status'] });
    },
    onError: (e: any) => toast.error(e?.response?.data?.error || 'Cannot enable Ceph yet'),
  });

  const removeNode = useMutation({
    mutationFn: async (id: string) => (await api.delete(`/setup/nodes/${id}`)).data,
    onSuccess: () => {
      toast.success('Node removed');
      qc.invalidateQueries({ queryKey: ['setup-status'] });
    },
    onError: (e: any) => toast.error(e?.response?.data?.error || 'Remove failed'),
  });

  const nodeTotal = data?.kubernetes?.total ?? 0;
  const storageClass = data?.cluster?.storage_class || 'local-path';
  const cephEnabled = data?.cluster?.ceph_enabled || data?.ceph?.available;

  const cards = useMemo(() => ([
    {
      title: 'Kubernetes',
      ok: !!data?.kubernetes?.available,
      detail: data?.kubernetes?.available
        ? `${data.kubernetes.ready ?? 0}/${data.kubernetes.total ?? 0} nodes Ready`
        : 'Not available yet',
      icon: Server,
    },
    {
      title: 'Storage',
      ok: storageClass === 'local-path' || !!data?.ceph?.available,
      detail: cephEnabled
        ? 'Ceph cluster active'
        : `Local disk (${storageClass}) — enable Ceph when ≥2 nodes`,
      icon: Database,
    },
    {
      title: 'KubeVirt',
      ok: !!data?.kubevirt?.available,
      detail: data?.kubevirt?.available ? 'Namespace present' : 'Not installed yet',
      icon: Server,
    },
  ]), [data, storageClass, cephEnabled]);

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-display text-nova-800 dark:text-nova-200">Cluster setup</h1>
          <p className="text-sm text-gray-500 mt-1">
            Single-node starts on local storage. Add nodes via PXE/join, then enable Ceph from here when ready.
          </p>
        </div>
        <button onClick={() => refetch()} className="btn-secondary inline-flex items-center gap-2" disabled={isFetching}>
          <RefreshCw className={`w-4 h-4 ${isFetching ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      <div className="grid md:grid-cols-3 gap-4">
        {cards.map((c) => (
          <div key={c.title} className="card p-4">
            <div className="flex items-center gap-2 mb-2">
              <c.icon className="w-4 h-4 text-nova-600" />
              <h2 className="font-semibold">{c.title}</h2>
              <span className={`ml-auto status-badge ${c.ok ? 'status-running' : 'status-stopped'}`}>
                {c.ok ? 'OK' : 'Pending'}
              </span>
            </div>
            <p className="text-sm text-gray-500">{isLoading ? 'Loading…' : c.detail}</p>
          </div>
        ))}
      </div>

      {data?.cluster?.can_enable_ceph && !cephEnabled && (
        <div className="card p-4 flex items-center justify-between gap-4 border-amber-200 dark:border-amber-900">
          <div>
            <h2 className="font-semibold flex items-center gap-2">
              <Zap className="w-4 h-4 text-amber-600" /> Enable distributed storage
            </h2>
            <p className="text-sm text-gray-500 mt-1">
              {nodeTotal} nodes registered — bootstrap Ceph on the replication network
              {data.cluster?.ceph_network ? ` (${data.cluster.ceph_network})` : ''}.
            </p>
          </div>
          <button className="btn-primary" onClick={() => enableCeph.mutate()} disabled={enableCeph.isPending}>
            {enableCeph.isPending ? 'Starting…' : 'Enable Ceph'}
          </button>
        </div>
      )}

      {!!data?.kubernetes?.nodes?.length && (
        <div className="card overflow-hidden">
          <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700 font-semibold text-sm">
            Cluster nodes
          </div>
          <table className="data-table">
            <thead>
              <tr><th>Name</th><th>Ready</th></tr>
            </thead>
            <tbody>
              {data!.kubernetes!.nodes!.map((n) => (
                <tr key={n.name} className="border-t border-gray-100 dark:border-gray-800">
                  <td className="px-4 py-2 font-mono text-sm">{n.name}</td>
                  <td className="px-4 py-2">
                    <span className={`status-badge ${n.ready ? 'status-running' : 'status-error'}`}>
                      {n.ready ? 'Ready' : 'NotReady'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {!!data?.registered_hosts?.length && (
        <div className="card overflow-hidden">
          <div className="px-4 py-3 border-b font-semibold text-sm">Registered hosts (pending join)</div>
          <table className="data-table">
            <thead><tr><th>Name</th><th>IP</th><th></th></tr></thead>
            <tbody>
              {data.registered_hosts.map((h) => (
                <tr key={h.id} className="border-t">
                  <td className="px-4 py-2 font-mono text-sm">{h.name}</td>
                  <td className="px-4 py-2 font-mono text-sm">{h.ip_address}</td>
                  <td className="px-4 py-2 text-right">
                    <button className="text-red-600 text-xs inline-flex items-center gap-1" onClick={() => removeNode.mutate(h.id)}>
                      <Trash2 className="w-3 h-3" /> Remove
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="grid lg:grid-cols-2 gap-4">
        <form className="card p-4 space-y-3" onSubmit={(e) => { e.preventDefault(); addNode.mutate(); }}>
          <h2 className="font-semibold flex items-center gap-2"><Plus className="w-4 h-4" /> Add node</h2>
          <p className="text-xs text-gray-500">Install vmalpha-os on the new host, then register and run the join command.</p>
          <label className="block text-xs text-gray-500">Name
            <input className="mt-1 w-full rounded-md border border-gray-300 dark:border-gray-700 bg-transparent px-3 py-2"
              value={nodeForm.name} onChange={(e) => setNodeForm({ ...nodeForm, name: e.target.value })} required />
          </label>
          <label className="block text-xs text-gray-500">IP address
            <input className="mt-1 w-full rounded-md border border-gray-300 dark:border-gray-700 bg-transparent px-3 py-2"
              value={nodeForm.ip} onChange={(e) => setNodeForm({ ...nodeForm, ip: e.target.value })} required />
          </label>
          <label className="block text-xs text-gray-500">Role
            <select className="mt-1 w-full rounded-md border border-gray-300 dark:border-gray-700 bg-transparent px-3 py-2"
              value={nodeForm.role} onChange={(e) => setNodeForm({ ...nodeForm, role: e.target.value })}>
              <option value="worker">worker</option>
              <option value="control-plane">control-plane</option>
            </select>
          </label>
          <button type="submit" className="btn-primary" disabled={addNode.isPending}>
            {addNode.isPending ? 'Registering…' : 'Register & get join command'}
          </button>
          {lastJoin && <pre className="text-xs bg-gray-50 dark:bg-gray-900 p-3 rounded-md overflow-x-auto whitespace-pre-wrap">{lastJoin}</pre>}
        </form>

        <form className="card p-4 space-y-3" onSubmit={(e) => { e.preventDefault(); addCeph.mutate(); }} style={{ opacity: cephEnabled ? 1 : 0.6 }}>
          <h2 className="font-semibold flex items-center gap-2"><Database className="w-4 h-4" /> Add Ceph OSD host</h2>
          <p className="text-xs text-gray-500">After Ceph is enabled, add storage-only hosts with free OSD disks.</p>
          <label className="block text-xs text-gray-500">Name
            <input className="mt-1 w-full rounded-md border border-gray-300 dark:border-gray-700 bg-transparent px-3 py-2"
              value={cephForm.name} onChange={(e) => setCephForm({ ...cephForm, name: e.target.value })} required disabled={!cephEnabled} />
          </label>
          <label className="block text-xs text-gray-500">IP address
            <input className="mt-1 w-full rounded-md border border-gray-300 dark:border-gray-700 bg-transparent px-3 py-2"
              value={cephForm.ip} onChange={(e) => setCephForm({ ...cephForm, ip: e.target.value })} required disabled={!cephEnabled} />
          </label>
          <button type="submit" className="btn-primary" disabled={addCeph.isPending || !cephEnabled}>
            {addCeph.isPending ? 'Adding…' : 'Add Ceph host'}
          </button>
          {lastCeph && <pre className="text-xs bg-gray-50 dark:bg-gray-900 p-3 rounded-md overflow-x-auto whitespace-pre-wrap">{lastCeph}</pre>}
        </form>
      </div>
    </div>
  );
}
