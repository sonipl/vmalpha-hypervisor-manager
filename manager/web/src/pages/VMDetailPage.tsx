import { useEffect, useRef, useState, type FormEvent } from 'react';
import RFB from '@novnc/novnc';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { vmAPI, storageAPI, nativeAPI } from '@/services/api';
import StatusBadge from '@/components/common/StatusBadge';
import NativePerformance from '@/components/NativePerformance';
import MetricsMatrix from '@/components/MetricsMatrix';
import { ArrowLeft } from 'lucide-react';
import toast from 'react-hot-toast';

function ConsolePanel({ vmID }: { vmID: string }) {
  const target = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string>();
  const [connecting, setConnecting] = useState(true);
  useEffect(() => {
    let rfb: RFB | undefined;
    let cancelled = false;
    let connected = false;
    async function connect() {
      try {
        const { data } = await vmAPI.createConsole(vmID);
        if (cancelled || !target.current) return;
        const endpoint = new URL(data.endpoint, window.location.origin);
        endpoint.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        rfb = new RFB(target.current, endpoint.href);
        rfb.scaleViewport = true;
        rfb.resizeSession = false;
        rfb.addEventListener('connect', () => { connected = true; if (!cancelled) setConnecting(false); });
        rfb.addEventListener('disconnect', (event: Event & { detail: { clean: boolean } }) => {
          if (!cancelled) {
            setConnecting(false);
            setError(!connected
              ? 'The host closed the console before sending a display. Check that this VM has a VNC device and the host console proxy is healthy.'
              : event.detail.clean ? 'The host closed the console session.' : 'The console connection was interrupted.');
          }
        });
      } catch (err: any) {
        if (!cancelled) setError(err.response?.data?.error || 'Unable to open this VM console.');
      }
    }
    connect();
    return () => { cancelled = true; rfb?.disconnect(); };
  }, [vmID]);
  if (error) return <div className="card p-5 text-sm text-red-700">{error}</div>;
  return <section className="card p-3"><div ref={target} className="min-h-[480px] bg-black" aria-label="Virtual machine console" />{connecting && <p className="p-2 text-sm text-gray-500">Connecting to the host console…</p>}</section>;
}

export default function VMDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [tab, setTab] = useState<'overview' | 'performance' | 'snapshots' | 'console'>('overview');
  const [pending, setPending] = useState(false);
  const [editingHardware, setEditingHardware] = useState(false);
  const [vcpus, setVcpus] = useState('');
  const [memoryMB, setMemoryMB] = useState('');
  const [attachingDisk, setAttachingDisk] = useState(false);
  const [diskPath, setDiskPath] = useState('');
  const [diskTarget, setDiskTarget] = useState('vdb');
  const vm = useQuery({ queryKey: ['vm', id], queryFn: () => vmAPI.get(id!).then((r) => r.data), enabled: !!id });
  const nativeInventory = useQuery({
    queryKey: ['native-inventory', vm.data?.host_node],
    queryFn: () => nativeAPI.inventory(vm.data!.host_node!).then((r) => r.data),
    enabled: !!vm.data?.host_node,
    refetchInterval: 15000,
    retry: false,
  });
  const snapshots = useQuery({ queryKey: ['vm-snapshots', id], queryFn: () => storageAPI.listSnapshots({ vm_id: id! }).then((r) => r.data), enabled: !!id && tab === 'snapshots' });
  async function action(name: string) {
    if (!id || pending) return;
    setPending(true);
    try {
      const nativeAction: Record<string, string> = { start: 'start', stop: 'shutdown', restart: 'reboot', pause: 'suspend', unpause: 'resume' };
      if (machine.host_node && nativeAction[name]) {
        await nativeAPI.operate(machine.host_node, 'vm-action', { name: machine.name, action: nativeAction[name] }, crypto.randomUUID());
        toast.success(`${name} request accepted by ${machine.host_node}`);
      } else {
        await vmAPI.action(id, name);
        toast.success(`${name} request accepted`);
      }
      await Promise.all([vm.refetch(), nativeInventory.refetch()]);
    }
    catch { toast.error(`${name} request failed`); }
    finally { setPending(false); }
  }
  async function requestGuestAgent() {
    if (!id || pending) return;
    setPending(true);
    try { const response = await vmAPI.requestGuestAgent(id); toast.success(response.data.message || 'Guest agent requested'); await vm.refetch(); }
    catch (error: any) { toast.error(error.response?.data?.error || 'Guest agent request failed'); }
    finally { setPending(false); }
  }
  if (vm.isPending) return <p className="text-gray-500">Loading virtual machine…</p>;
  if (vm.isError || !vm.data) return <div className="card p-5"><p>Virtual machine unavailable.</p><button className="text-nova-600 mt-2" onClick={() => vm.refetch()}>Retry</button></div>;
  const machine = vm.data;
  const liveMachine = nativeInventory.data?.vms.find((guest) => guest.name === machine.name);
  const liveState = liveMachine?.state.toLowerCase();
  const liveStatus = liveState === 'running' ? 'Running' : liveState === 'paused' ? 'Paused' : liveState === 'shut off' ? 'Stopped' : machine.status;
  const liveUnavailable = !!machine.host_node && !nativeInventory.isPending && (!nativeInventory.data || !liveMachine);
  const operationBlocked = pending || (!!machine.host_node && (!!nativeInventory.isPending || liveUnavailable));
  async function editHardware(event: FormEvent) {
    event.preventDefault();
    if (!machine.host_node || pending) return;
    const cpu = Number(vcpus);
    const memory = Number(memoryMB);
    if (!Number.isInteger(cpu) || cpu < 1 || !Number.isInteger(memory) || memory < 256) {
      toast.error('Enter at least one vCPU and 256 MB of memory.');
      return;
    }
    setPending(true);
    try {
      await nativeAPI.operate(machine.host_node, 'vm-edit', { name: machine.name, cpu, memory }, crypto.randomUUID());
      toast.success('Hardware update accepted by the native Hypervisor.');
      setEditingHardware(false);
      await Promise.all([vm.refetch(), nativeInventory.refetch()]);
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Hardware update failed. Ensure the VM is powered off.');
    } finally { setPending(false); }
  }
  async function attachDisk(event: FormEvent) {
    event.preventDefault();
    if (!machine.host_node || pending) return;
    if (!diskPath.trim() || !/^vd[b-z]$/.test(diskTarget)) {
      toast.error('Provide a datastore disk path and a target from vdb through vdz.');
      return;
    }
    setPending(true);
    try {
      await nativeAPI.operate(machine.host_node, 'vm-attach-disk', { name: machine.name, path: diskPath.trim(), target: diskTarget }, crypto.randomUUID());
      toast.success('Disk attach request accepted by the native Hypervisor.');
      setAttachingDisk(false);
      setDiskPath('');
      await Promise.all([vm.refetch(), nativeInventory.refetch()]);
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Disk attach failed.');
    } finally { setPending(false); }
  }
  return <div className="space-y-5">
    <div className="flex items-center gap-3">
      <button onClick={() => navigate('/vms')} aria-label="Back to VMs"><ArrowLeft className="w-5 h-5" /></button>
      <h1 className="text-2xl font-display font-semibold">{machine.name}</h1><StatusBadge status={liveStatus} />
      <div className="ml-auto flex gap-2">
        <button disabled={operationBlocked} onClick={requestGuestAgent} className="btn-secondary">Install Guest Agent</button>
        {liveStatus === 'Stopped' && machine.host_node && <button disabled={operationBlocked} onClick={() => { setVcpus(String(liveMachine?.cpu ?? machine.vcpus)); setMemoryMB(String(liveMachine?.memory ?? machine.memory_mb)); setEditingHardware(true); }} className="btn-secondary">Edit Hardware</button>}
        {machine.host_node && <button disabled={operationBlocked} onClick={() => setAttachingDisk(true)} className="btn-secondary">Attach Disk</button>}
        {liveStatus === 'Stopped' && <button disabled={operationBlocked} onClick={() => action('start')} className="btn-primary">Start</button>}
        {liveStatus === 'Running' && <>
          <button disabled={operationBlocked} onClick={() => action('pause')} className="btn-secondary">Pause</button>
          <button disabled={operationBlocked} onClick={() => action('restart')} className="btn-secondary">Restart</button>
          <button disabled={operationBlocked} onClick={() => action('stop')} className="btn-secondary">Stop</button>
        </>}
        {liveStatus === 'Paused' && <button disabled={operationBlocked} onClick={() => action('unpause')} className="btn-primary">Resume</button>}
      </div>
    </div>
    {machine.host_node && <div role={liveUnavailable ? 'alert' : 'status'} className={`rounded border p-3 text-sm ${liveUnavailable ? 'border-red-200 bg-red-50 text-red-800' : 'border-blue-200 bg-blue-50 text-blue-800'}`}>
      {liveUnavailable
        ? `Live state from ${machine.host_node} is unavailable. Host actions are disabled to avoid changing a VM using stale Manager status.`
        : nativeInventory.isPending ? `Reading live VM state from ${machine.host_node}…`
          : `Live state from ${machine.host_node}${nativeInventory.data?.version ? ` · ${nativeInventory.data.version}` : ''}.`}
    </div>}
    {editingHardware && <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" role="dialog" aria-modal="true" aria-labelledby="edit-hardware-title">
      <form onSubmit={editHardware} className="card w-full max-w-md space-y-4 p-6 shadow-xl">
        <div><h2 id="edit-hardware-title" className="text-lg font-semibold">Edit hardware</h2><p className="mt-1 text-sm text-gray-500">Changes apply through {machine.host_node} while this VM is powered off.</p></div>
        <label className="block text-sm font-medium">vCPUs<input value={vcpus} onChange={(event) => setVcpus(event.target.value)} type="number" min="1" max="64" required className="mt-1 w-full rounded border p-2" /></label>
        <label className="block text-sm font-medium">Memory (MiB)<input value={memoryMB} onChange={(event) => setMemoryMB(event.target.value)} type="number" min="256" max="131072" required className="mt-1 w-full rounded border p-2" /></label>
        <div className="flex justify-end gap-2"><button type="button" onClick={() => setEditingHardware(false)} disabled={pending} className="btn-secondary">Cancel</button><button type="submit" disabled={pending} className="btn-primary">Apply</button></div>
      </form>
    </div>}
    {attachingDisk && <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" role="dialog" aria-modal="true" aria-labelledby="attach-disk-title">
      <form onSubmit={attachDisk} className="card w-full max-w-md space-y-4 p-6 shadow-xl">
        <div><h2 id="attach-disk-title" className="text-lg font-semibold">Attach existing disk</h2><p className="mt-1 text-sm text-gray-500">Use a QCOW2 disk file from a registered local datastore on {machine.host_node}.</p></div>
        <label className="block text-sm font-medium">Disk path<input value={diskPath} onChange={(event) => setDiskPath(event.target.value)} placeholder="/var/lib/libvirt/images/data.qcow2" required className="mt-1 w-full rounded border p-2" /></label>
        <label className="block text-sm font-medium">Target device<input value={diskTarget} onChange={(event) => setDiskTarget(event.target.value)} pattern="vd[b-z]" required className="mt-1 w-full rounded border p-2" /></label>
        <div className="flex justify-end gap-2"><button type="button" onClick={() => setAttachingDisk(false)} disabled={pending} className="btn-secondary">Cancel</button><button type="submit" disabled={pending} className="btn-primary">Attach</button></div>
      </form>
    </div>}
    <div className="flex gap-4 border-b border-gray-200 pb-3">
      {(['overview', 'performance', 'snapshots', 'console'] as const).map((name) => <button key={name} onClick={() => setTab(name)} className={tab === name ? 'text-nova-600 font-semibold capitalize' : 'text-gray-500 capitalize'}>{name}</button>)}
    </div>
    {tab === 'overview' && <>
      <dl className="card p-5 grid grid-cols-2 gap-5 text-sm">
        {[
          ['OS', machine.os], ['vCPUs', String(liveMachine?.cpu ?? machine.vcpus)], ['Memory', `${liveMachine?.memory ?? machine.memory_mb} MB`],
          ['IP Address', machine.ip_address || '—'], ['Host', machine.host_node || '—'],
          ['Description', machine.description || '—'], ['Secure Boot', machine.secure_boot ? 'Enabled' : 'Disabled'],
          ['vTPM', machine.vtpm ? 'Enabled' : 'Disabled'],
        ].map(([label, value]) => <div key={label}><dt className="text-gray-500">{label}</dt><dd className="mt-1">{value}</dd></div>)}
      </dl>
      <section className="card p-5"><h2 className="font-semibold mb-3">Disks</h2>
        {machine.host_node ? liveMachine?.disks?.length ? liveMachine.disks.map((disk) => <p key={`${disk.target}:${disk.path}`} className="text-sm py-2">{disk.target || disk.device} · {disk.path || 'Non-file device'}{disk.device && disk.device !== 'disk' ? ` · ${disk.device}` : ''}</p>) : <p className="text-sm text-gray-500">{liveUnavailable ? 'Live disk inventory is unavailable.' : 'No disks reported by the host.'}</p> : machine.disks?.length ? machine.disks.map((disk) => <p key={disk.id} className="text-sm py-2">{disk.name} · {disk.size_gb} GB · {disk.storage_class} · {disk.bus}</p>) : <p className="text-sm text-gray-500">No disks reported.</p>}
      </section>
      <section className="card p-5"><h2 className="font-semibold mb-3">Network Interfaces</h2>
        {machine.host_node ? liveMachine?.networks?.length ? liveMachine.networks.map((nic) => <p key={nic.mac} className="text-sm py-2">{nic.mac} · Network {nic.network || 'not reported'}</p>) : <p className="text-sm text-gray-500">{liveUnavailable ? 'Live network inventory is unavailable.' : 'No interfaces reported by the host.'}</p> : machine.nics?.length ? machine.nics.map((nic) => <p key={nic.id} className="text-sm py-2">{nic.name} · {nic.mac_address} · {nic.ip_address || 'No address reported'} · Network {nic.network_id}</p>) : <p className="text-sm text-gray-500">No interfaces reported.</p>}
      </section>
    </>}
    {tab === 'performance' && (machine.host_node
      ? <div className="space-y-5"><MetricsMatrix host={machine.host_node} scope="vm" vm={machine.name} /><NativePerformance host={machine.host_node} vms={[machine.name]} defaultScope="vm" /></div>
      : <p className="card p-5 text-sm text-gray-500">Performance is unavailable until this VM is assigned to an enrolled host.</p>)}
    {tab === 'console' && <ConsolePanel vmID={machine.id} />}
    {tab === 'snapshots' && <section className="card p-5"><h2 className="font-semibold mb-3">Recorded Snapshots</h2>
      {snapshots.isPending ? <p>Loading snapshots…</p> : snapshots.isError ? <p>Snapshots unavailable.</p> : !snapshots.data?.length ? <p>No snapshots recorded.</p> : snapshots.data.map((snapshot) => <div key={snapshot.id} className="py-3 border-b border-gray-100"><p className="font-medium">{snapshot.name}</p><p className="text-sm text-gray-500">{new Date(snapshot.created_at).toLocaleString()} · {snapshot.size_mb} MB · {snapshot.status}</p></div>)}
    </section>}
  </div>;
}
