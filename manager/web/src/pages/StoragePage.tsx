import { useMemo, useState } from 'react';
import DatastoreBrowser from '@/components/DatastoreBrowser';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import toast from 'react-hot-toast';
import api, { storageAPI } from '@/services/api';
import DataTable, { Column } from '@/components/common/DataTable';
import StatusBadge from '@/components/common/StatusBadge';
import MetricCard from '@/components/common/MetricCard';
import type { Volume, StorageClass, StorageBackend } from '@/types';
import {
  Archive,
  Activity,
  Database,
  HardDrive,
  Layers,
  Plus,
  RefreshCw,
  Server,
  Trash2,
  X,
  Link,
  Unlink,
} from 'lucide-react';

type AddKind = 'local' | 'nfs' | 'ceph';

function apiErrorMessage(e: any, fallback: string) {
  const data = e?.response?.data;
  const parts = [data?.error, data?.message, data?.hint].filter(Boolean);
  if (parts.length) return parts.join(' — ');
  return e?.message || fallback;
}

function fmtBytes(n?: number) {
  if (!n || n <= 0) return '0 B';
  const u = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(i > 1 ? 1 : 0)} ${u[i]}`;
}

function healthTone(h?: string) {
  if (!h) return 'text-gray-500';
  if (h.includes('HEALTH_OK')) return 'text-emerald-600';
  if (h.includes('WARN')) return 'text-amber-600';
  return 'text-red-600';
}

function AddStorageModal({
  open,
  onClose,
  cephAvailable,
}: {
  open: boolean;
  onClose: () => void;
  cephAvailable: boolean;
}) {
  const qc = useQueryClient();
  const [kind, setKind] = useState<AddKind>('local');
  const [localForm, setLocalForm] = useState({
    name: 'local-provisioner',
    device: '',
    mount_path: '/opt/local-path-provisioner',
    fs_type: 'xfs',
    format: false,
    storage_class: '',
  });
  const [nfsForm, setNfsForm] = useState({
    name: 'nfs-data',
    server: '10.0.10.50',
    export_path: '/nfs',
    mount_path: '/var/lib/novasphere/nfs-data',
    mount_options: 'nfsvers=4.1',
    storage_class: '',
  });
  const [cephForm, setCephForm] = useState({
    name: 'ceph-rbd',
    monitors: '10.0.10.51,10.0.10.52,10.0.10.53,10.0.10.54,10.0.10.55',
    pool: 'rbd',
    user: 'rbd',
    key: '',
    cluster_id: '',
    storage_class: '',
    use_cluster: false,
  });

  const addLocal = useMutation({
    mutationFn: () => storageAPI.addLocalStorage({
      ...localForm,
      device: localForm.device.trim(),
      storage_class: localForm.storage_class.trim(),
    }),
    onSuccess: (res) => {
      const backend = res.data?.backend;
      if (backend?.status === 'error') {
        toast.error(backend.message || res.data?.message || 'Failed to add local storage');
        return;
      }
      toast.success(res.data?.message || 'Local storage registered');
      qc.invalidateQueries({ queryKey: ['storage-classes'] });
      onClose();
    },
    onError: (e: any) => toast.error(apiErrorMessage(e, 'Failed to add local storage')),
  });

  const addNfs = useMutation({
    mutationFn: () => storageAPI.addNFSStorage(nfsForm),
    onSuccess: (res) => {
      const backend = res.data?.backend;
      if (backend?.status === 'error' || backend?.status === 'unmounted') {
        toast.error(backend.message || res.data?.message || 'NFS registered but mount failed');
        qc.invalidateQueries({ queryKey: ['storage-classes'] });
        return;
      }
      toast.success(res.data?.message || 'NFS storage registered');
      qc.invalidateQueries({ queryKey: ['storage-classes'] });
      onClose();
    },
    onError: (e: any) => toast.error(apiErrorMessage(e, 'Failed to add NFS storage')),
  });

  const addCeph = useMutation({
    mutationFn: () => storageAPI.addCephStorage(cephForm),
    onSuccess: (res) => {
      const backend = res.data?.backend;
      if (backend?.status === 'error') {
        toast.error(backend.message || res.data?.message || 'Failed to add Ceph storage');
        return;
      }
      toast.success(res.data?.message || 'Ceph storage class created');
      qc.invalidateQueries({ queryKey: ['storage-classes'] });
      onClose();
    },
    onError: (e: any) => toast.error(apiErrorMessage(e, 'Failed to add Ceph storage')),
  });

  if (!open) return null;

  const pending = addLocal.isPending || addNfs.isPending || addCeph.isPending;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="card w-full max-w-2xl max-h-[90vh] overflow-y-auto p-5 space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Add Storage</h2>
          <button type="button" className="btn-ghost p-1" onClick={onClose} aria-label="Close">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="grid grid-cols-3 gap-2">
          {([
            { k: 'local' as const, label: 'Local disk', icon: HardDrive },
            { k: 'nfs' as const, label: 'NFS', icon: Server },
            { k: 'ceph' as const, label: 'Ceph RBD', icon: Database },
          ]).map(({ k, label, icon: Icon }) => (
            <button
              key={k}
              type="button"
              onClick={() => setKind(k)}
              className={`rounded-lg border p-3 text-left transition-colors ${
                kind === k
                  ? 'border-nova-500 bg-nova-50 dark:bg-nova-950/30'
                  : 'border-gray-200 dark:border-gray-800 hover:border-gray-300'
              }`}
            >
              <Icon className="w-5 h-5 mb-1 text-nova-600" />
              <div className="text-sm font-medium">{label}</div>
            </button>
          ))}
        </div>

        {kind === 'local' && (
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault();
              addLocal.mutate();
            }}
          >
            <p className="text-xs text-gray-500">
              Register an existing host mount (leave device empty) or attach a raw disk for local-path provisioning.
              The 500GB disk is usually already mounted at <code className="font-mono">/opt/local-path-provisioner</code>.
            </p>
            <label className="block text-xs text-gray-500">Name
              <input className="input mt-1 w-full" value={localForm.name}
                onChange={(e) => setLocalForm({ ...localForm, name: e.target.value })} required />
            </label>
            <label className="block text-xs text-gray-500">Device (optional — leave empty if already mounted)
              <input className="input mt-1 w-full font-mono" placeholder="/dev/sdb (only for new disks)"
                value={localForm.device}
                onChange={(e) => setLocalForm({ ...localForm, device: e.target.value })} />
            </label>
            <label className="block text-xs text-gray-500">Mount path
              <input className="input mt-1 w-full font-mono" placeholder="/opt/local-path-provisioner"
                value={localForm.mount_path}
                onChange={(e) => setLocalForm({ ...localForm, mount_path: e.target.value })} required />
            </label>
            <div className="grid grid-cols-2 gap-3">
              <label className="block text-xs text-gray-500">Filesystem
                <input className="input mt-1 w-full font-mono" value={localForm.fs_type}
                  onChange={(e) => setLocalForm({ ...localForm, fs_type: e.target.value })} />
              </label>
              <label className="block text-xs text-gray-500">Storage class (optional)
                <input className="input mt-1 w-full font-mono" placeholder="auto"
                  value={localForm.storage_class}
                  onChange={(e) => setLocalForm({ ...localForm, storage_class: e.target.value })} />
              </label>
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={localForm.format}
                onChange={(e) => setLocalForm({ ...localForm, format: e.target.checked })} />
              Format device before mount (destructive)
            </label>
            <button type="submit" className="btn-primary w-full" disabled={pending}>Register local storage</button>
          </form>
        )}

        {kind === 'nfs' && (
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault();
              addNfs.mutate();
            }}
          >
            <p className="text-xs text-gray-500">NFS export mounted on the node; creates a static PV + StorageClass.</p>
            <label className="block text-xs text-gray-500">Name
              <input className="input mt-1 w-full" value={nfsForm.name}
                onChange={(e) => setNfsForm({ ...nfsForm, name: e.target.value })} required />
            </label>
            <div className="grid grid-cols-2 gap-3">
              <label className="block text-xs text-gray-500">NFS server
                <input className="input mt-1 w-full font-mono" value={nfsForm.server}
                  onChange={(e) => setNfsForm({ ...nfsForm, server: e.target.value })} required />
              </label>
              <label className="block text-xs text-gray-500">Export path
                <input className="input mt-1 w-full font-mono" value={nfsForm.export_path}
                  onChange={(e) => setNfsForm({ ...nfsForm, export_path: e.target.value })} required />
              </label>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <label className="block text-xs text-gray-500">Local mount path
                <input className="input mt-1 w-full font-mono" value={nfsForm.mount_path}
                  onChange={(e) => setNfsForm({ ...nfsForm, mount_path: e.target.value })} />
              </label>
              <label className="block text-xs text-gray-500">Mount options
                <input className="input mt-1 w-full font-mono" value={nfsForm.mount_options}
                  onChange={(e) => setNfsForm({ ...nfsForm, mount_options: e.target.value })} />
              </label>
            </div>
            <label className="block text-xs text-gray-500">Storage class (optional)
              <input className="input mt-1 w-full font-mono" placeholder="auto"
                value={nfsForm.storage_class}
                onChange={(e) => setNfsForm({ ...nfsForm, storage_class: e.target.value })} />
            </label>
            <button type="submit" className="btn-primary w-full" disabled={pending}>Add NFS storage</button>
          </form>
        )}

        {kind === 'ceph' && (
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault();
              addCeph.mutate();
            }}
          >
            {!cephAvailable && (
              <div className="rounded-md border border-amber-300 bg-amber-50 dark:bg-amber-950/20 p-3 text-sm text-amber-800 dark:text-amber-200">
                Ceph is not ready on this cluster. Enable Ceph from <strong>Cluster setup</strong> when you have 2+ nodes, then return here.
              </div>
            )}
            <label className="block text-xs text-gray-500">Name
              <input className="input mt-1 w-full" value={cephForm.name}
                onChange={(e) => setCephForm({ ...cephForm, name: e.target.value })} required />
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={cephForm.use_cluster}
                onChange={(e) => setCephForm({ ...cephForm, use_cluster: e.target.checked })} />
              Use existing cluster monitors (auto-discover)
            </label>
            <label className="block text-xs text-gray-500">Monitor IPs (comma-separated)
              <input className="input mt-1 w-full font-mono" placeholder="10.0.15.61,10.0.15.62"
                value={cephForm.monitors} disabled={cephForm.use_cluster}
                onChange={(e) => setCephForm({ ...cephForm, monitors: e.target.value })} />
            </label>
            <div className="grid grid-cols-2 gap-3">
              <label className="block text-xs text-gray-500">RBD pool
                <input className="input mt-1 w-full font-mono" value={cephForm.pool}
                  onChange={(e) => setCephForm({ ...cephForm, pool: e.target.value })} required />
              </label>
              <label className="block text-xs text-gray-500">Ceph user
                <input className="input mt-1 w-full font-mono" value={cephForm.user}
                  onChange={(e) => setCephForm({ ...cephForm, user: e.target.value })} />
              </label>
            </div>
            <label className="block text-xs text-gray-500">Key (required for external Ceph)
              <input type="password" className="input mt-1 w-full font-mono" required
                value={cephForm.key}
                onChange={(e) => setCephForm({ ...cephForm, key: e.target.value })} />
            </label>
            <label className="block text-xs text-gray-500">Cluster ID / FSID (optional — auto-detected when empty)
              <input className="input mt-1 w-full font-mono" placeholder="ceph cluster fsid"
                value={cephForm.cluster_id}
                onChange={(e) => setCephForm({ ...cephForm, cluster_id: e.target.value })} />
            </label>
            <label className="block text-xs text-gray-500">Storage class (optional)
              <input className="input mt-1 w-full font-mono" placeholder="auto"
                value={cephForm.storage_class}
                onChange={(e) => setCephForm({ ...cephForm, storage_class: e.target.value })} />
            </label>
            <button type="submit" className="btn-primary w-full" disabled={pending}>Create Ceph StorageClass</button>
          </form>
        )}
      </div>
    </div>
  );
}

export default function StoragePage() {
 const [browserBackend,setBrowserBackend]=useState<StorageBackend|null>(null);
  const qc = useQueryClient();
  const [tab, setTab] = useState<'cluster' | 'backends' | 'volumes' | 'classes' | 'snapshots'>('backends');
  const [range, setRange] = useState<'15m' | '1h' | '6h' | '24h' | '7d'>('1h');
  const [showAdd, setShowAdd] = useState(false);

  const { data: volumes, isLoading: volLoading } = useQuery({
    queryKey: ['volumes'],
    queryFn: () => storageAPI.listVolumes().then((r) => r.data),
  });

  const { data: classesResp, refetch: refetchClasses, isPending: backendsLoading, isError: backendsError } = useQuery({
    queryKey: ['storage-classes'],
    queryFn: () => storageAPI.listClassesWithBackends().then((r) => r.data),
  });

  const { data: health, refetch: refetchHealth, isFetching: healthFetching } = useQuery({
    queryKey: ['ceph-health'],
    queryFn: () => storageAPI.cephHealth().then((r) => r.data),
    refetchInterval: 15_000,
    retry: false,
  });

  const { data: metrics } = useQuery({
    queryKey: ['ceph-metrics', range],
    queryFn: () => storageAPI.cephMetrics(range).then((r) => r.data),
    refetchInterval: 15_000,
  });

  const removeBackend = useMutation({
    mutationFn: (id: string) => storageAPI.deleteBackend(id),
    onSuccess: () => {
      toast.success('Storage backend removed');
      qc.invalidateQueries({ queryKey: ['storage-classes'] });
    },
    onError: (e: any) => toast.error(apiErrorMessage(e, 'Remove failed')),
  });

  const mountBackend = useMutation({
    mutationFn: (id: string) => storageAPI.mountBackend(id),
    onSuccess: (res) => {
      toast.success(res.data?.message || 'Backend mounted');
      qc.invalidateQueries({ queryKey: ['storage-classes'] });
    },
    onError: (e: any) => toast.error(apiErrorMessage(e, 'Mount failed')),
  });

  const unmountBackend = useMutation({
    mutationFn: (id: string) => storageAPI.unmountBackend(id),
    onSuccess: (res) => {
      toast.success(res.data?.message || 'Backend unmounted');
      qc.invalidateQueries({ queryKey: ['storage-classes'] });
    },
    onError: (e: any) => toast.error(apiErrorMessage(e, 'Unmount failed')),
  });

  const cephAvailable = (health as any)?.available !== false && !!(health as any)?.health;
  const localMode = !cephAvailable;

  const volData = (volumes as any)?.items ?? (volumes as any)?.data ?? [];
  const classData = (classesResp as any)?.items ?? (classesResp as any)?.data ?? [];
  const backends: StorageBackend[] = (classesResp as any)?.backends ?? [];
  const live = (metrics as any)?.live || {};
  const series = useMemo(() => {
    const rows = ((metrics as any)?.series || []) as Array<Record<string, any>>;
    return rows.map((r) => ({
      ...r,
      tLabel: r.t ? new Date(r.t).toLocaleTimeString() : '',
      used_tb: (r.used_bytes || 0) / (1024 ** 4),
      total_tb: (r.total_bytes || 0) / (1024 ** 4),
      iops: (r.read_iops || 0) + (r.write_iops || 0),
      latency: Math.max(r.read_latency_ms || 0, r.write_latency_ms || 0),
    }));
  }, [metrics]);

  const usedPct = live.total_bytes ? Math.round((100 * (live.used_bytes || 0)) / live.total_bytes) : 0;

  const volColumns: Column<Volume>[] = [
    {
      key: 'name', label: 'Volume', sortable: true,
      render: (v) => (
        <div className="flex items-center gap-2">
          <HardDrive className="w-4 h-4 text-gray-400" />
          <div>
            <div className="font-medium text-gray-900 dark:text-white">{v.name}</div>
            <div className="text-[10px] text-gray-400 font-mono">{v.id?.slice(0, 8)}</div>
          </div>
        </div>
      ),
    },
    { key: 'status', label: 'Status', width: '100px', render: (v) => <StatusBadge status={v.status} /> },
    { key: 'size_gb', label: 'Size', width: '80px', sortable: true, render: (v) => <span className="font-mono tabular-nums">{v.size_gb} GB</span> },
    {
      key: 'used_gb', label: 'Used', width: '120px',
      render: (v) => {
        const pct = v.size_gb ? Math.round((v.used_gb / v.size_gb) * 100) : 0;
        return (
          <div className="flex items-center gap-2">
            <div className="w-16 h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
              <div className={`h-full rounded-full ${pct > 85 ? 'bg-red-500' : pct > 60 ? 'bg-amber-500' : 'bg-nova-500'}`} style={{ width: `${pct}%` }} />
            </div>
            <span className="font-mono text-xs tabular-nums">{pct}%</span>
          </div>
        );
      },
    },
    { key: 'storage_class', label: 'Storage Class', render: (v) => <span className="text-xs font-mono text-gray-600 dark:text-gray-400">{v.storage_class}</span> },
    { key: 'vm_id', label: 'Attached To', render: (v) => <span className="text-xs text-gray-600 dark:text-gray-400">{v.vm_id || '—'}</span> },
  ];

  const classColumns: Column<StorageClass>[] = [
    {
      key: 'name', label: 'Class', sortable: true,
      render: (c) => (
        <div className="flex items-center gap-2">
          <Database className="w-4 h-4 text-gray-400" />
          <span className="font-medium font-mono text-gray-900 dark:text-white">{c.name}</span>
        </div>
      ),
    },
    { key: 'status', label: 'Status', width: '100px', render: (c) => <StatusBadge status={c.status ?? 'Unknown'} /> },
    { key: 'provisioner', label: 'Provisioner', render: (c) => <span className="text-xs font-mono">{c.provisioner}</span> },
    { key: 'pool', label: 'Pool', render: (c) => <span className="text-xs font-mono">{c.pool || '—'}</span> },
    { key: 'replication_factor', label: 'Replication', width: '90px', render: (c) => <span className="font-mono">{c.replication_factor ?? 1}x</span> },
  ];

  const backendColumns: Column<StorageBackend>[] = [
    {
      key: 'name', label: 'Backend', sortable: true,
      render: (b) => (
        <div>
          <div className="font-medium text-gray-900 dark:text-white">{b.name}</div>
          <div className="text-[10px] font-mono text-gray-400">{b.storage_class}</div>
        </div>
      ),
    },
    { key: 'type', label: 'Type', width: '80px', render: (b) => <span className="uppercase text-xs font-mono">{b.type}</span> },
    { key: 'status', label: 'Status', width: '100px', render: (b) => <StatusBadge status={b.status} /> },
    { key: 'capacity_bytes', label: 'Capacity', render: (b) => <span className="font-mono text-xs">{fmtBytes(b.capacity_bytes)}</span> },
    {
      key: 'message', label: 'Details',
      render: (b) => <span className="text-xs text-gray-500">{b.message || JSON.stringify(b.config || {}).slice(0, 80)}</span>,
    },
    {
      key: 'actions', label: '', width: '120px',
      render: (b) => (
        <div className="flex items-center gap-1">
          {b.config?.external_native === true && <button className="btn-ghost" onClick={async()=>{try{await api.post('/storage/external-nfs',{id:b.config?.datastore_id,name:b.name,server:b.config?.server,export_path:b.config?.export_path,hosts:b.config?.hosts});await refetchClasses();toast.success('Hypervisor NFS mounts verified');}catch(e:any){toast.error(e.response?.data?.error||'Mount verification failed');}}}>Verify mounts</button>}
          {b.type === "nfs" && (b.config?.external_native || b.config?.native_managed) ? <button className="btn-ghost" onClick={()=>setBrowserBackend(b)}>Browse</button> : <span className="text-xs text-gray-400" title="Folder browsing requires a Manager-mounted NFS datastore">Browse unsupported</span>}
          {b.type !== 'ceph' && !b.config?.native_managed && !b.config?.external_native && (
            <>
              {(b.status === 'unmounted' || b.status === 'offline' || b.status === 'error') && (
                <button
                  type="button"
                  className="btn-ghost p-1 text-emerald-600"
                  title="Mount on host"
                  disabled={mountBackend.isPending}
                  onClick={() => mountBackend.mutate(b.id)}
                >
                  <Link className="w-4 h-4" />
                </button>
              )}
              {(b.status === 'mounted' || b.status === 'online') && (
                <button
                  type="button"
                  className="btn-ghost p-1 text-amber-600"
                  title="Unmount from host"
                  disabled={unmountBackend.isPending}
                  onClick={() => {
                    if (window.confirm(`Unmount "${b.name}" from host? Fails if PVCs are in use.`)) {
                      unmountBackend.mutate(b.id);
                    }
                  }}
                >
                  <Unlink className="w-4 h-4" />
                </button>
              )}
            </>
          )}
          <button
            type="button"
            className="btn-ghost p-1 text-red-600"
            title="Remove registration (data preserved)"
            onClick={() => {
              if (window.confirm(`Remove backend "${b.name}"? Existing volumes are not deleted.`)) {
                removeBackend.mutate(b.id);
              }
            }}
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      ),
    },
  ];

  const tabs = [
    { key: 'backends', label: 'Backends', icon: Layers },
    { key: 'cluster', label: 'Ceph Cluster', icon: Activity },
    { key: 'volumes', label: 'Volumes', icon: HardDrive },
    { key: 'classes', label: 'Storage Classes', icon: Database },
    { key: 'snapshots', label: 'Snapshots', icon: Archive },
  ] as const;

  const hosts = Array.isArray((health as any)?.hosts) ? (health as any).hosts : [];

  return (
    <div className="space-y-4">
      <AddStorageModal open={showAdd} onClose={() => setShowAdd(false)} cephAvailable={cephAvailable} />

      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-display font-semibold text-gray-900 dark:text-white">Storage</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
            {localMode
              ? 'Add local disk or NFS storage — enable Ceph from Cluster setup when you scale out'
              : 'Manage storage backends, Ceph health, and Kubernetes storage classes'}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            className="btn-secondary flex items-center gap-2"
            onClick={() => { refetchHealth(); refetchClasses(); }}
            disabled={healthFetching}
          >
            <RefreshCw className={`w-4 h-4 ${healthFetching ? 'animate-spin' : ''}`} /> Refresh
          </button>
          <button className="btn-primary flex items-center gap-2" onClick={() => setShowAdd(true)}>
            <Plus className="w-4 h-4" /> Add Storage
          </button>
        </div>
      </div>

      <div className="border-b border-gray-200 dark:border-gray-800">
        <div className="flex gap-1 overflow-x-auto">
          {tabs.map((t) => (
            <button key={t.key} onClick={() => { setTab(t.key as typeof tab); const keys = t.key === 'cluster' ? ['ceph-health', 'ceph-metrics'] : t.key === 'volumes' ? ['volumes'] : ['storage-classes']; keys.forEach(key => { void qc.refetchQueries({ queryKey: [key], type: 'active' }, { cancelRefetch: false }); }); }}
              className={`flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors whitespace-nowrap ${
                tab === t.key ? 'border-nova-500 text-nova-600 dark:text-nova-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'
              }`}>
              <t.icon className="w-4 h-4" /> {t.label}
            </button>
          ))}
        </div>
      </div>

      {browserBackend && <DatastoreBrowser backend={browserBackend} close={()=>setBrowserBackend(null)} />}
      {tab === 'backends' && (
        <DataTable
          columns={backendColumns}
          data={backendsError ? [] : backends}
          loading={backendsLoading}
          emptyMessage={backendsError ? "Storage backend inventory unavailable — retry refresh" : "No storage backends registered — click Add Storage"}
        />
      )}

      {tab === 'cluster' && (
        <div className="space-y-4">
          {localMode && (
            <div className="card p-4 text-sm text-gray-600 dark:text-gray-400">
              Ceph cluster metrics appear here after you enable Ceph from Cluster setup (requires 2+ nodes).
            </div>
          )}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
            <MetricCard title="Health" value={(health as any)?.health || live.health || 'UNKNOWN'} icon={Activity} color="nova" />
            <MetricCard title="Capacity" value={fmtBytes(live.total_bytes)} icon={Database} color="blue" />
            <MetricCard title="Used" value={`${fmtBytes(live.used_bytes)} (${usedPct}%)`} icon={Layers} color="copper" />
            <MetricCard title="OSDs up/in" value={`${live.num_osd_up ?? 0}/${live.num_osd_in ?? 0}`} icon={HardDrive} color="green" />
            <MetricCard title="IOPS (r+w)" value={`${Math.round((live.read_iops || 0) + (live.write_iops || 0))}`} icon={Activity} color="nova" />
          </div>

          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-500">History range</span>
            {(['15m', '1h', '6h', '24h', '7d'] as const).map((r) => (
              <button key={r} onClick={() => setRange(r)}
                className={`px-2.5 py-1 text-xs rounded-md border ${range === r ? 'bg-nova-50 border-nova-400 text-nova-700' : 'border-gray-200 text-gray-600'}`}>
                {r}
              </button>
            ))}
            <span className={`ml-auto text-sm font-medium ${healthTone((health as any)?.health || live.health)}`}>
              {(health as any)?.available === false ? 'Ceph not ready yet — bootstrap in progress' : (health as any)?.health || live.health}
            </span>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <div className="card p-4">
              <h3 className="text-sm font-medium mb-3">Capacity usage</h3>
              <div className="h-56">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={series}>
                    <CartesianGrid strokeDasharray="3 3" opacity={0.3} />
                    <XAxis dataKey="tLabel" tick={{ fontSize: 10 }} minTickGap={24} />
                    <YAxis tick={{ fontSize: 10 }} unit=" TB" />
                    <Tooltip />
                    <Legend />
                    <Area type="monotone" dataKey="used_tb" name="Used TB" stroke="#0f766e" fill="#99f6e4" strokeWidth={2} />
                    <Area type="monotone" dataKey="total_tb" name="Total TB" stroke="#64748b" fill="transparent" strokeDasharray="4 4" />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </div>
            <div className="card p-4">
              <h3 className="text-sm font-medium mb-3">IOPS (read + write)</h3>
              <div className="h-56">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={series}>
                    <CartesianGrid strokeDasharray="3 3" opacity={0.3} />
                    <XAxis dataKey="tLabel" tick={{ fontSize: 10 }} minTickGap={24} />
                    <YAxis tick={{ fontSize: 10 }} />
                    <Tooltip />
                    <Legend />
                    <Line type="monotone" dataKey="read_iops" name="Read IOPS" stroke="#2563eb" dot={false} strokeWidth={2} />
                    <Line type="monotone" dataKey="write_iops" name="Write IOPS" stroke="#c2410c" dot={false} strokeWidth={2} />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </div>
          </div>

          <div className="card p-4">
            <h3 className="text-sm font-medium mb-3">Hosts in Ceph pool</h3>
            {hosts.length === 0 ? (
              <p className="text-sm text-gray-500">No hosts reported yet.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full text-sm">
                  <thead>
                    <tr className="text-left text-gray-500 border-b border-gray-200 dark:border-gray-800">
                      <th className="py-2 pr-4">Hostname</th>
                      <th className="py-2 pr-4">Addr</th>
                      <th className="py-2 pr-4">Status</th>
                      <th className="py-2">Labels</th>
                    </tr>
                  </thead>
                  <tbody>
                    {hosts.map((h: any, i: number) => (
                      <tr key={h.hostname || h.addr || i} className="border-b border-gray-100 dark:border-gray-900">
                        <td className="py-2 pr-4 font-mono">{h.hostname || h.host || '—'}</td>
                        <td className="py-2 pr-4 font-mono">{h.addr || h.ip || '—'}</td>
                        <td className="py-2 pr-4">{h.status || (h.labels ? 'online' : '—')}</td>
                        <td className="py-2 font-mono text-xs">{Array.isArray(h.labels) ? h.labels.join(', ') : (h.labels ? JSON.stringify(h.labels) : '—')}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {tab === 'volumes' && <DataTable columns={volColumns} data={volData} loading={volLoading} emptyMessage="No volumes found" />}
      {tab === 'classes' && <DataTable columns={classColumns} data={classData} emptyMessage="No storage classes" />}
      {tab === 'snapshots' && (
        <div className="card p-8 text-center text-gray-500 dark:text-gray-400">
          <Archive className="w-10 h-10 mx-auto mb-3 opacity-40" />
          <p className="font-medium">Snapshot management</p>
          <p className="text-sm mt-1">View and manage VM snapshots across all storage classes.</p>
        </div>
      )}
    </div>
  );
}
