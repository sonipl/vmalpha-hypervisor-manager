import { useEffect, useRef, useState } from 'react';
import RFB from '@novnc/novnc';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { vmAPI, storageAPI } from '@/services/api';
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
    async function connect() {
      try {
        const { data } = await vmAPI.createConsole(vmID);
        if (cancelled || !target.current) return;
        const endpoint = new URL(data.endpoint, window.location.origin);
        endpoint.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        rfb = new RFB(target.current, endpoint.href);
        rfb.scaleViewport = true;
        rfb.resizeSession = false;
        rfb.addEventListener('connect', () => { if (!cancelled) setConnecting(false); });
        rfb.addEventListener('disconnect', (event: Event & { detail: { clean: boolean } }) => {
          if (!cancelled && !event.detail.clean) setError('The console connection was closed unexpectedly.');
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
  const vm = useQuery({ queryKey: ['vm', id], queryFn: () => vmAPI.get(id!).then((r) => r.data), enabled: !!id });
  const snapshots = useQuery({ queryKey: ['vm-snapshots', id], queryFn: () => storageAPI.listSnapshots({ vm_id: id! }).then((r) => r.data), enabled: !!id && tab === 'snapshots' });
  async function action(name: string) {
    if (!id || pending) return;
    setPending(true);
    try { await vmAPI.action(id, name); toast.success(`${name} request accepted`); await vm.refetch(); }
    catch { toast.error(`${name} request failed`); }
    finally { setPending(false); }
  }
  if (vm.isPending) return <p className="text-gray-500">Loading virtual machine…</p>;
  if (vm.isError || !vm.data) return <div className="card p-5"><p>Virtual machine unavailable.</p><button className="text-nova-600 mt-2" onClick={() => vm.refetch()}>Retry</button></div>;
  const machine = vm.data;
  return <div className="space-y-5">
    <div className="flex items-center gap-3">
      <button onClick={() => navigate('/vms')} aria-label="Back to VMs"><ArrowLeft className="w-5 h-5" /></button>
      <h1 className="text-2xl font-display font-semibold">{machine.name}</h1><StatusBadge status={machine.status} />
      <div className="ml-auto flex gap-2">
        {machine.status === 'Stopped' && <button disabled={pending} onClick={() => action('start')} className="btn-primary">Start</button>}
        {machine.status === 'Running' && <>
          <button disabled={pending} onClick={() => action('pause')} className="btn-secondary">Pause</button>
          <button disabled={pending} onClick={() => action('restart')} className="btn-secondary">Restart</button>
          <button disabled={pending} onClick={() => action('stop')} className="btn-secondary">Stop</button>
        </>}
        {machine.status === 'Paused' && <button disabled={pending} onClick={() => action('unpause')} className="btn-primary">Resume</button>}
      </div>
    </div>
    <div className="flex gap-4 border-b border-gray-200 pb-3">
      {(['overview', 'performance', 'snapshots', 'console'] as const).map((name) => <button key={name} onClick={() => setTab(name)} className={tab === name ? 'text-nova-600 font-semibold capitalize' : 'text-gray-500 capitalize'}>{name}</button>)}
    </div>
    {tab === 'overview' && <>
      <dl className="card p-5 grid grid-cols-2 gap-5 text-sm">
        {[
          ['OS', machine.os], ['vCPUs', String(machine.vcpus)], ['Memory', `${machine.memory_mb} MB`],
          ['IP Address', machine.ip_address || '—'], ['Host', machine.host_node || '—'],
          ['Description', machine.description || '—'], ['Secure Boot', machine.secure_boot ? 'Enabled' : 'Disabled'],
          ['vTPM', machine.vtpm ? 'Enabled' : 'Disabled'],
        ].map(([label, value]) => <div key={label}><dt className="text-gray-500">{label}</dt><dd className="mt-1">{value}</dd></div>)}
      </dl>
      <section className="card p-5"><h2 className="font-semibold mb-3">Disks</h2>
        {machine.disks?.length ? machine.disks.map((disk) => <p key={disk.id} className="text-sm py-2">{disk.name} · {disk.size_gb} GB · {disk.storage_class} · {disk.bus}</p>) : <p className="text-sm text-gray-500">No disks reported.</p>}
      </section>
      <section className="card p-5"><h2 className="font-semibold mb-3">Network Interfaces</h2>
        {machine.nics?.length ? machine.nics.map((nic) => <p key={nic.id} className="text-sm py-2">{nic.name} · {nic.mac_address} · {nic.ip_address || 'No address reported'} · Network {nic.network_id}</p>) : <p className="text-sm text-gray-500">No interfaces reported.</p>}
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
