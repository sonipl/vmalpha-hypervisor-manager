import { useEffect, useState } from 'react';
import api from '@/services/api';
import type { StorageBackend } from '@/types';

type Entry = { name: string; directory: boolean; size: number };
export default function DatastoreBrowser({ backend, close }: { backend: StorageBackend; close: () => void }) {
 const [path, setPath] = useState(''); const [entries,setEntries]=useState<Entry[]>([]); const [error,setError]=useState(''); const [busy,setBusy]=useState(false);
 const base=`/storage/backends/${encodeURIComponent(backend.id)}/browser`;
 const child=(name:string)=>path?`${path}/${name}`:name;
 const refresh=async()=>{setError('');setBusy(true);try{const r=await api.get(base,{params:{path}});setEntries(r.data.entries);}catch(e:any){setEntries([]);setError(e.response?.data?.error||'Datastore unavailable');}finally{setBusy(false);}};
 useEffect(()=>{void refresh();},[path,backend.id]);
 const action=async(fn:()=>Promise<unknown>)=>{setBusy(true);setError('');try{await fn();await refresh();}catch(e:any){setError(e.response?.data?.error||'Operation failed');}finally{setBusy(false);}};
 return <div className="fixed inset-0 z-50 bg-black/50 flex items-center justify-center"><div className="card p-6 w-full max-w-3xl max-h-[85vh] overflow-auto space-y-4">
 <div className="flex justify-between"><h2 className="text-lg font-semibold">Datastore browser · {backend.name}</h2><button onClick={close}>Close</button></div>
 <div className="flex gap-3 items-center"><button disabled={busy||!path} onClick={()=>setPath(path.split('/').slice(0,-1).join('/'))}>Up</button><code>/{path}</code><button disabled={busy} onClick={()=>void refresh()}>Refresh</button></div>
 <div className="flex gap-4"><button disabled={busy} onClick={()=>{const name=window.prompt('New folder name');if(name&&!name.includes('/')&&name!=='.'&&name!=='..')void action(()=>api.post(`${base}/folder`,null,{params:{path:child(name)}}));}}>New folder</button>
 <label>Upload file (maximum 512 MiB)<input type="file" disabled={busy} onChange={e=>{const f=e.target.files?.[0];if(f){if(f.size>512*1024*1024){setError('Upload exceeds 512 MiB');return;}void action(()=>api.post(`${base}/upload`,f,{params:{path:child(f.name)},headers:{'Content-Type':'application/octet-stream'},timeout:0}));}e.target.value='';}} /></label></div>
 <p className="text-xs text-gray-500">Existing files are never overwritten. Delete removes only a file or an empty folder. Do not remove files used by virtual machines.</p>
 {error&&<p role="alert" className="text-red-600">{error}</p>}{busy&&<p>Working…</p>}
 <table className="w-full"><thead><tr><th className="text-left">Name</th><th>Size</th><th>Actions</th></tr></thead><tbody>{entries.map(e=><tr key={e.name}><td><button disabled={busy||!e.directory} onClick={()=>setPath(child(e.name))}>{e.directory?'📁 ':''}{e.name}</button></td><td>{e.directory?'—':e.size}</td><td className="flex gap-3">{!e.directory&&<button disabled={busy} onClick={()=>void action(async()=>{const r=await api.get(`${base}/download`,{params:{path:child(e.name)},responseType:'blob'});const u=URL.createObjectURL(r.data);const a=document.createElement('a');a.href=u;a.download=e.name;a.click();setTimeout(()=>URL.revokeObjectURL(u),1000);})}>Download</button>}<button disabled={busy} onClick={()=>{if(window.confirm(`Delete ${e.name}? This cannot be undone.`))void action(()=>api.delete(`${base}/entry`,{params:{path:child(e.name)}}));}}>Delete</button></td></tr>)}</tbody></table>
 </div></div>;
}
