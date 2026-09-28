import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import api from '@/services/api';

type Spec = { name:string; bridge:string; uplink:string; mtu:number; hosts:string[]; port_groups:{name:string;vlan_id:number}[] };
type Row = {id:string;name:string;revision:number;spec:string;status:string;results:string};
type Review = {id:string;action:string;spec:string;previews:string;expires_at:string};
const fresh = ():Spec => ({name:'',bridge:'',uplink:'',mtu:1500,hosts:[],port_groups:[]});
const parse = (s:string) => { try { return JSON.parse(s); } catch { return {}; } };
const input='border rounded px-3 py-2 bg-white dark:bg-gray-900 dark:border-gray-700 w-full';
export default function DistributedNetworks(){
 const cache=useQueryClient();
 const rows=useQuery({queryKey:['distributed-networks'],queryFn:()=>api.get<{data:Row[]}>('/distributed-networks').then(r=>r.data.data)});
 const targets=useQuery({queryKey:['distributed-network-targets'],queryFn:()=>api.get<string[]>('/distributed-networks/targets').then(r=>r.data)});
 const [editing,setEditing]=useState<Row|null|undefined>(undefined);
 const [spec,setSpec]=useState<Spec>(fresh());
 const [action,setAction]=useState('create');
 const [review,setReview]=useState<Review|null>(null);
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState('');
 const [result,setResult]=useState<unknown>(null);
 const open=(row:Row|null,next:string)=>{setEditing(row);setAction(next);setSpec(row?parse(row.spec):fresh());setReview(null);setError('');setResult(null)};
 const request=async()=>{
  setBusy(true);setError('');
  try{ const {data}=await api.post<Review>('/distributed-networks/review',{id:editing?.id,revision:editing?.revision,action,spec},{timeout:65000});setReview(data); }
  catch(e:any){setError(JSON.stringify(e.response?.data??{error:e.message},null,2));}finally{setBusy(false)}
 };
 const apply=async()=>{
  if(!review)return;setBusy(true);setError('');
  try{const {data}=await api.post('/distributed-networks/apply',{review_id:review.id,confirm:true},{timeout:100000});setResult(data);setReview(null);setEditing(undefined);await cache.invalidateQueries({queryKey:['distributed-networks']});}
  catch(e:any){setError(JSON.stringify(e.response?.data??{error:e.message},null,2));await cache.invalidateQueries({queryKey:['distributed-networks']});setReview(null)}finally{setBusy(false)}
 };
 return <section className="space-y-4">
  <div className="flex justify-between items-center"><div><h2 className="text-lg font-semibold">Distributed switches and port groups</h2><p className="text-sm text-gray-500">Review bridge and VLAN configuration on every selected host before applying.</p></div><button className="btn-primary" onClick={()=>open(null,'create')}>Create switch</button></div>
  {rows.isError&&<p role="alert">Distributed network inventory unavailable.</p>}
  {rows.isLoading?<p>Loading switches…</p>:rows.data?.length===0?<p>No distributed switches configured.</p>:rows.data?.map(row=>{const saved=parse(row.spec) as Spec;return <article key={row.id} className="border rounded-lg p-4 dark:border-gray-700">
   <div className="flex justify-between"><h3 className="font-semibold">{saved.name}</h3><span>{row.status}</span></div>
   <p className="text-sm">Bridge {saved.bridge} · Uplink {saved.uplink} · MTU {saved.mtu}</p><p className="text-sm">Hosts: {saved.hosts?.join(', ')}</p>
   <ul className="my-2">{saved.port_groups?.map(p=><li key={p.name}>{p.name} — {p.vlan_id===0?'Untagged':`VLAN ${p.vlan_id}`}</li>)}</ul>
   <div className="flex gap-3"><button disabled={row.status==='applying'||row.status==='partial'} onClick={()=>open(row,'update')}>Edit switch / port groups</button><button disabled={row.status==='applying'||row.status==='partial'} onClick={()=>open(row,'delete')}>Delete</button></div>
   {(row.status==='partial'||row.status==='applying')&&<p className="text-amber-600">Host reconciliation is required before another change. No automatic retry will run.</p>}
   <details><summary>Host results</summary><pre className="overflow-auto text-xs">{JSON.stringify(parse(row.results),null,2)}</pre></details>
  </article>})}
  {result!==null&&<pre aria-live="polite" className="text-xs whitespace-pre-wrap">{JSON.stringify(result,null,2)}</pre>}
  {editing!==undefined&&<div role="dialog" aria-modal="true" aria-label="Configure distributed network" className="fixed inset-0 z-50 bg-black/50 flex items-center justify-center p-4"><div className="bg-white dark:bg-gray-900 rounded-xl p-6 w-full max-w-3xl max-h-[90vh] overflow-auto space-y-4">
   <h3 className="text-xl font-semibold">{review?'Review changes':action==='delete'?'Delete switch':editing?'Edit switch':'Create distributed switch'}</h3>
   {!review&&action!=='delete'&&<>
    <div className="grid grid-cols-2 gap-3">{(['name','bridge','uplink'] as const).map(key=><label key={key}>{key}<input className={input} value={spec[key]} disabled={busy||(editing!==null&&key!=='name')} onChange={e=>setSpec({...spec,[key]:e.target.value})}/></label>)}<label>MTU<input className={input} type="number" min={1280} max={9000} value={spec.mtu} onChange={e=>setSpec({...spec,mtu:Number(e.target.value)})}/></label></div>
    <fieldset><legend>Target hosts</legend>{targets.data?.map(host=><label className="block" key={host}><input type="checkbox" checked={spec.hosts.includes(host)} disabled={busy||editing!==null} onChange={e=>setSpec({...spec,hosts:e.target.checked?[...spec.hosts,host]:spec.hosts.filter(h=>h!==host)})}/> {host}</label>)}{targets.isError&&<p>Enrolled hosts unavailable.</p>}</fieldset>
    <div><h4 className="font-semibold">Port groups</h4><p className="text-sm text-gray-500">VLAN 0 means untagged. Tagged VLANs use 1–4094.</p>{spec.port_groups.map((p,i)=><div className="flex gap-2 mt-2" key={i}><input aria-label="Port group name" className={input} value={p.name} onChange={e=>setSpec({...spec,port_groups:spec.port_groups.map((v,j)=>j===i?{...v,name:e.target.value}:v)})}/><input aria-label="VLAN ID" className={input} type="number" min={0} max={4094} value={p.vlan_id} onChange={e=>setSpec({...spec,port_groups:spec.port_groups.map((v,j)=>j===i?{...v,vlan_id:Number(e.target.value)}:v)})}/><button onClick={()=>setSpec({...spec,port_groups:spec.port_groups.filter((_,j)=>j!==i)})}>Remove</button></div>)}<button className="mt-2" onClick={()=>setSpec({...spec,port_groups:[...spec.port_groups,{name:'',vlan_id:0}]})}>Add port group</button></div>
   </>}
   {action==='delete'&&!review&&<p>Review removal of {spec.name} from {spec.hosts.join(', ')}. In-use networks must be refused by the host.</p>}
   {review&&<><p>Review expires {new Date(review.expires_at).toLocaleTimeString()}. Applying may partially succeed; inspect each host result.</p><pre className="text-xs overflow-auto">{JSON.stringify({action:review.action,configuration:parse(review.spec),hosts:parse(review.previews)},null,2)}</pre></>}
   {error&&<pre role="alert" className="text-red-600 text-xs whitespace-pre-wrap">{error}</pre>}
   <div className="flex justify-end gap-3"><button disabled={busy} onClick={()=>{setEditing(undefined);setReview(null)}}>Cancel</button>{review?<button className="btn-primary" disabled={busy} onClick={apply}>{busy?'Applying…':'Confirm and apply to hosts'}</button>:<button className="btn-primary" disabled={busy} onClick={request}>{busy?'Checking hosts…':'Review changes'}</button>}</div>
  </div></div>}
 </section>
}
