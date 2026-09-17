import { useState } from 'react';
import {isAxiosError} from 'axios';
import { useQuery } from '@tanstack/react-query';
import { nativeAPI, type NativeTask } from '@/services/api';
import NativePerformance from '@/components/NativePerformance';
import NativeStorageNetwork from '@/components/NativeStorageNetwork';
import { useAuth } from '@/context/AuthContext';

export default function NativeHostsPage() {
 const {user}=useAuth();
 const [chosen,setChosen]=useState('');
 const taskKey='vmalpha_native_task_'+(user?.id||'');
 const [task,setTask]=useState<NativeTask|null>(()=>{try{return JSON.parse(localStorage.getItem(taskKey)||'null');}catch{return null;}});
 function remember(next:NativeTask){localStorage.setItem(taskKey,JSON.stringify(next));setTask(next);}
 const [message,setMessage]=useState('');
 const [pending,setPending]=useState(false);
 const [name,setName]=useState('');
 const [image,setImage]=useState('');
 const [pool,setPool]=useState('default');
 const [network,setNetwork]=useState('default');
 const allowed=user?.role==='Platform Admin';
 const hosts=useQuery({queryKey:['native-hosts'],queryFn:()=>nativeAPI.hosts().then(r=>r.data.hosts),enabled:allowed});
 const selected=chosen||hosts.data?.[0]||'';
 const inventory=useQuery({queryKey:['native-inventory',selected],queryFn:()=>nativeAPI.inventory(selected).then(r=>r.data),enabled:allowed&&!!selected,retry:false,refetchInterval:15000});
 const live=inventory.isError?undefined:inventory.data;
 const uncertain=task?.status==='Unknown'||task?.status==='Running';
 const blocked=pending||uncertain||!live;
 async function operate(op:string,args:Record<string,unknown>){
  if(blocked)return;
  const id=crypto.randomUUID();
  setPending(true);setMessage('');
  try{remember({id,host:selected,operation:op,status:'Running',created_at:new Date().toISOString()});}catch{setPending(false);setMessage('Unable to save the operation identifier in this browser. No request was sent.');return;}
  try{const r=await nativeAPI.operate(selected,op,args,id);remember(r.data.task);if(op==='vm-action'&&args.action==='shutdown')setMessage('Shutdown requested. Live inventory will show when the guest stops.');await inventory.refetch();}
  catch(error){
   const response=isAxiosError(error)?error.response:undefined;
   const recorded=response?.data?.task as NativeTask|undefined;
   if(response?.data?.outcome==='not_started'||response?.status===401||response?.status===403){localStorage.removeItem(taskKey);setTask(null);setMessage('The operation did not start. Check your inputs and access before trying again.');}
   else if(recorded?.id===id&&recorded.status==='Failed'){remember(recorded);setMessage('Command failed. Inspect live host state before retrying; partial changes may exist.');}
   else{remember({id,host:selected,operation:op,status:'Unknown',created_at:new Date().toISOString()});setMessage('Outcome is not confirmed. Check the saved task and live host state before trying again.');}
  }
  finally{setPending(false);}
 }
 async function checkTask(){
  if(!task)return;
  try{const r=await nativeAPI.task(task.id);remember(r.data);setMessage(r.data.status==='Succeeded'?'Completed.':'This task still requires host readback.');await inventory.refetch();}
  catch{setMessage('Task status is unavailable. Do not repeat the operation until its outcome is established.');}
 }
 if(!allowed)return <p className="p-6">Native host management currently requires Platform Admin access.</p>;
 return <div className="space-y-6 p-6">
  <div><h1 className="text-2xl font-semibold">Native hosts</h1><p className="text-sm text-gray-500">Live VM Alpha hosts and their virtual machines.</p></div>
  {hosts.isError?<p role="alert">Unable to load enrolled hosts.</p>:hosts.isLoading?<p>Loading hosts…</p>:hosts.data?.length===0?<p>No hosts enrolled.</p>:<label className="block">Host <select className="input ml-3" value={selected} disabled={pending} onChange={e=>setChosen(e.target.value)}>{hosts.data?.map(h=><option key={h}>{h}</option>)}</select></label>}
  {inventory.isError&&<p role="alert">Live inventory is unavailable. Cached values are hidden.</p>}
  {selected&&inventory.isLoading&&<p>Reading host…</p>}
  {live&&<>
   <section className="rounded-lg border p-4"><h2 className="font-semibold">{live.hostname}</h2><p>VM Alpha {live.version} · {live.cpu} CPUs · {(live.memory.total/1024**3).toFixed(1)} GiB memory · KVM {live.kvm?'available':'unavailable'}</p><p>{live.maintenance?'Maintenance enabled':'Available for placement'} · CPU {live.cpuUsage.toFixed(1)}%</p></section>
   <NativePerformance host={selected} vms={live.vms.map(v=>v.name)}/>
   <NativeStorageNetwork key={selected} host={selected} inventory={live} blocked={blocked} operate={operate} selectImage={setImage}/>
   <section className="overflow-x-auto rounded-lg border p-4"><h2 className="mb-3 font-semibold">Virtual machines</h2>{live.vms.length===0?<p>No virtual machines on this host.</p>:<table className="w-full text-left text-sm"><thead><tr><th>Name</th><th>State</th><th>CPU / memory</th><th>Actions</th></tr></thead><tbody>{live.vms.map(vm=><tr key={vm.uuid} className="border-t"><td className="py-3">{vm.name}</td><td>{vm.state}</td><td>{vm.cpu} / {vm.memory} MiB</td><td className="space-x-2">{vm.state==='shut off'&&<button className="btn-secondary" disabled={blocked||live.maintenance} onClick={()=>operate('vm-action',{name:vm.name,action:'start'})}>Start</button>}{vm.state==='running'&&<button className="btn-secondary" disabled={blocked} onClick={()=>operate('vm-action',{name:vm.name,action:'shutdown'})}>Shut down</button>}{vm.state==='paused'&&<button className="btn-secondary" disabled={blocked} onClick={()=>operate('vm-action',{name:vm.name,action:'resume'})}>Resume</button>}</td></tr>)}</tbody></table>}</section>
   <form className="space-y-3 rounded-lg border p-4" onSubmit={e=>{e.preventDefault();void operate('vm-create',{name,mode:'import',image,pool,network,cpu:1,memory:512,size:1,os:'Linux',firmware:'BIOS'});}}>
    <h2 className="font-semibold">Import a guest</h2><p className="text-sm text-gray-500">Creates a separate copy of a QCOW2 or raw disk already in this host’s guest datastore. Initial guest size: 1 CPU, 512 MiB, BIOS.</p>
    <label className="block">VM name <input className="input" required pattern="[A-Za-z][A-Za-z0-9_.-]{0,62}" value={name} onChange={e=>setName(e.target.value)}/></label>
    <label className="block">Source disk path <input className="input w-full" required value={image} onChange={e=>setImage(e.target.value)}/></label>
    <label className="block">Datastore <select className="input" value={pool} onChange={e=>setPool(e.target.value)}>{live.pools.filter(p=>p.type==='dir').map(p=><option key={p.name}>{p.name}</option>)}</select></label>
    <label className="block">Network <select className="input" value={network} onChange={e=>setNetwork(e.target.value)}>{live.networks.map(n=><option key={n.name}>{n.name}</option>)}</select></label>
    <button className="btn-primary" disabled={blocked||live.maintenance}>Import VM</button>
   </form>
  </>}
  {task&&<section aria-live="polite" className="rounded-lg border p-4"><h2 className="font-semibold">Latest operation: {task.status}</h2><p>{task.operation} · {task.host}</p><p className="font-mono text-xs">{task.id}</p><button className="btn-secondary mt-2" disabled={pending} onClick={checkTask}>Check saved task</button></section>}
  {message&&<p role="status">{message}</p>}
 </div>;
}
