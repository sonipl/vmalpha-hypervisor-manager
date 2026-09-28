import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { networkAPI, storageAPI, templateAPI, vmAPI } from '@/services/api';
import type { Network, StorageClass, VMTemplate } from '@/types';
import { ArrowLeft, Plus, Trash2, HardDrive, Network as NetworkIcon } from 'lucide-react';
import toast from 'react-hot-toast';

interface DiskEntry { name: string; size_gb: number; storage_class: string; }
interface NICEntry { name: string; network_id: string; }

function parseOS(label: string): { os: string; os_version: string } {
  const m = label.trim().match(/^(.+?)\s+(\d+(?:\.\d+)*)$/);
  if (!m) return { os: label.toLowerCase().replace(/\s+/g, ''), os_version: '' };
  return { os: m[1].toLowerCase().replace(/\s+/g, ''), os_version: m[2] };
}

export default function VMCreatePage() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [networks, setNetworks] = useState<Network[]>([]);
  const [storageClasses, setStorageClasses] = useState<StorageClass[]>([]);
  const [templates, setTemplates] = useState<VMTemplate[]>([]);

  const [form, setForm] = useState({
    name: '',
    description: '',
    os_type: 'Ubuntu 22.04',
    vcpus: 2,
    memory_mb: 4096,
    template_id: '',
  });

  const [disks, setDisks] = useState<DiskEntry[]>([
    { name: 'root', size_gb: 50, storage_class: 'local-path' },
  ]);

  const [nics, setNics] = useState<NICEntry[]>([
    { name: 'eth0', network_id: '' },
  ]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [netRes, scRes, templateRes] = await Promise.all([
          networkAPI.list({ per_page: 100 }),
          storageAPI.listClasses(),
          templateAPI.list(),
        ]);
        if (cancelled) return;
        const nets = (netRes.data as { data?: Network[] }).data ?? (netRes.data as unknown as Network[]) ?? [];
        const classes = Array.isArray(scRes.data)
          ? scRes.data
          : ((scRes.data as { data?: StorageClass[] }).data ?? []);
        setNetworks(nets);
        setStorageClasses(classes);
        setTemplates(templateRes.data.data ?? templateRes.data.items ?? []);

        const preferred =
          nets.find((n) => (n.subnet || '').startsWith('10.0.10.') || n.name.includes('10.0.10')) ||
          nets[0];
        if (preferred) {
          setNics((prev) => prev.map((n, i) => (i === 0 ? { ...n, network_id: preferred.id } : n)));
        }

        const preferredSC =
          classes.find((c) => c.name === 'local-path') ||
          classes.find((c) => c.name === 'local') ||
          classes.find((c) => c.name === 'ceph-rbd-ssd') ||
          classes[0];
        if (preferredSC) {
          setDisks((prev) => prev.map((d, i) => (i === 0 ? { ...d, storage_class: preferredSC.name } : d)));
        }
      } catch (err) {
        console.error(err);
        toast.error('Failed to load networks/storage classes');
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!nics.every((n) => n.network_id)) {
      toast.error('Select a network for each NIC');
      return;
    }
    setLoading(true);
    try {
      const { os, os_version } = parseOS(form.os_type);
      const payload = {
        name: form.name,
        namespace: 'default',
        description: form.description,
        os,
        os_version,
        vcpus: form.vcpus,
        memory_mb: form.memory_mb,
        ...(form.template_id ? { template_id: form.template_id } : {}),
        disks: disks.map((d) => ({
          name: d.name,
          size_gb: d.size_gb,
          storage_class: d.storage_class,
          bus: 'virtio',
          bootable: d.name === 'root',
        })),
        nics: nics.map((n) => ({
          name: n.name,
          network_id: n.network_id,
          model: 'virtio',
          type: 'bridge',
        })),
      };
      const res = await vmAPI.create(payload as never);
      toast.success('VM created successfully');
      navigate(`/vms/${res.data.id}`);
    } catch (err: unknown) {
      const msg =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ||
        'Failed to create VM';
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  };

  const osOptions = [
    'Ubuntu 22.04', 'Ubuntu 24.04', 'RHEL 9', 'RHEL 8', 'CentOS Stream 9',
    'Debian 12', 'Fedora 39', 'Windows Server 2022', 'Windows Server 2019',
    'Rocky Linux 9', 'AlmaLinux 9', 'openSUSE Leap 15',
  ];

  const scOptions = storageClasses.length
    ? storageClasses.map((c) => c.name)
    : ['local-path', 'ceph-rbd-ssd', 'local'];

  const netLabel = (n: Network) =>
    n.subnet ? `${n.name} (${n.subnet})` : n.name;

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <div className="flex items-center gap-3">
        <button onClick={() => navigate('/vms')} className="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800">
          <ArrowLeft className="w-5 h-5" />
        </button>
        <div>
          <h1 className="text-2xl font-display font-semibold text-gray-900 dark:text-white">Create Virtual Machine</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">Configure and deploy a new KVM instance</p>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        <div className="card p-5 space-y-4">
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Basic Configuration</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name</label>
              <input type="text" required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="vm-web-01"
                className="w-full px-3 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-lg text-sm focus:ring-2 focus:ring-nova-400 focus:outline-none" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Boot template</label>
              <select value={form.template_id} onChange={(e) => setForm({ ...form, template_id: e.target.value })}
                className="w-full px-3 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-lg text-sm focus:ring-2 focus:ring-nova-400 focus:outline-none">
                <option value="">No catalog template</option>
                {templates.map((template) => <option key={template.id} value={template.id}>{template.name} · {template.disk_gb} GB</option>)}
              </select>
              <p className="mt-1 text-xs text-gray-500">Templates are verified QCOW2 images in an External NFS datastore.</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Operating System</label>
              <select value={form.os_type} onChange={(e) => setForm({ ...form, os_type: e.target.value })}
                className="w-full px-3 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-lg text-sm focus:ring-2 focus:ring-nova-400 focus:outline-none">
                {osOptions.map((os) => <option key={os} value={os}>{os}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">vCPUs</label>
              <input type="number" min={1} max={128} value={form.vcpus} onChange={(e) => setForm({ ...form, vcpus: +e.target.value })}
                className="w-full px-3 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-lg text-sm focus:ring-2 focus:ring-nova-400 focus:outline-none" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Memory (MB)</label>
              <select value={form.memory_mb} onChange={(e) => setForm({ ...form, memory_mb: +e.target.value })}
                className="w-full px-3 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-lg text-sm focus:ring-2 focus:ring-nova-400 focus:outline-none">
                {[1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072].map((m) => (
                  <option key={m} value={m}>{(m / 1024).toFixed(0)} GB</option>
                ))}
              </select>
            </div>
            <div className="md:col-span-2">
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Description</label>
              <input type="text" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })}
                placeholder="Optional description"
                className="w-full px-3 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded-lg text-sm focus:ring-2 focus:ring-nova-400 focus:outline-none" />
            </div>
          </div>
        </div>

        <div className="card p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 flex items-center gap-2">
              <HardDrive className="w-4 h-4" /> Storage
            </h3>
            <button type="button" onClick={() => setDisks([...disks, { name: `disk-${disks.length}`, size_gb: 50, storage_class: scOptions[0] }])}
              className="text-xs text-nova-600 dark:text-nova-400 hover:underline flex items-center gap-1">
              <Plus className="w-3 h-3" /> Add Disk
            </button>
          </div>
          {disks.map((disk, i) => (
            <div key={i} className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800/50 rounded-lg">
              <input type="text" value={disk.name} onChange={(e) => { const d = [...disks]; d[i].name = e.target.value; setDisks(d); }}
                className="w-32 px-2 py-1.5 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded text-sm" placeholder="Name" />
              <input type="number" min={1} value={disk.size_gb} onChange={(e) => { const d = [...disks]; d[i].size_gb = +e.target.value; setDisks(d); }}
                className="w-24 px-2 py-1.5 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded text-sm" />
              <span className="text-xs text-gray-500">GB</span>
              <select value={disk.storage_class} onChange={(e) => { const d = [...disks]; d[i].storage_class = e.target.value; setDisks(d); }}
                className="flex-1 px-2 py-1.5 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded text-sm">
                {scOptions.map((sc) => <option key={sc} value={sc}>{sc}</option>)}
              </select>
              {disks.length > 1 && (
                <button type="button" onClick={() => setDisks(disks.filter((_, j) => j !== i))}
                  className="p-1 text-red-400 hover:text-red-600"><Trash2 className="w-4 h-4" /></button>
              )}
            </div>
          ))}
        </div>

        <div className="card p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 flex items-center gap-2">
              <NetworkIcon className="w-4 h-4" /> Network Interfaces
            </h3>
            <button type="button" onClick={() => setNics([...nics, { name: `eth${nics.length}`, network_id: networks[0]?.id || '' }])}
              className="text-xs text-nova-600 dark:text-nova-400 hover:underline flex items-center gap-1">
              <Plus className="w-3 h-3" /> Add NIC
            </button>
          </div>
          {networks.length === 0 && (
            <p className="text-sm text-amber-600 dark:text-amber-400">No networks configured in VM Alpha Manager yet.</p>
          )}
          {nics.map((nic, i) => (
            <div key={i} className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800/50 rounded-lg">
              <input type="text" value={nic.name} onChange={(e) => { const n = [...nics]; n[i].name = e.target.value; setNics(n); }}
                className="w-32 px-2 py-1.5 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded text-sm" placeholder="Name" />
              <select value={nic.network_id} onChange={(e) => { const n = [...nics]; n[i].network_id = e.target.value; setNics(n); }}
                className="flex-1 px-2 py-1.5 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded text-sm" required>
                <option value="" disabled>Select network…</option>
                {networks.map((net) => <option key={net.id} value={net.id}>{netLabel(net)}</option>)}
              </select>
              {nics.length > 1 && (
                <button type="button" onClick={() => setNics(nics.filter((_, j) => j !== i))}
                  className="p-1 text-red-400 hover:text-red-600"><Trash2 className="w-4 h-4" /></button>
              )}
            </div>
          ))}
        </div>

        <div className="flex items-center gap-3">
          <button type="submit" disabled={loading || networks.length === 0} className="btn-primary flex items-center gap-2">
            {loading ? (
              <><div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" /> Creating…</>
            ) : (
              <><Plus className="w-4 h-4" /> Create VM</>
            )}
          </button>
          <button type="button" onClick={() => navigate('/vms')} className="btn-secondary">Cancel</button>
        </div>
      </form>
    </div>
  );
}
