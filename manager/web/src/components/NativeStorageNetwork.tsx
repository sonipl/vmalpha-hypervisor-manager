import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {nativeAPI,type NativeInventory} from '@/services/api';

type Props={host:string;inventory:NativeInventory;blocked:boolean;operate:(op:string,args:Record<string,unknown>)=>Promise<void>;selectImage:(path:string)=>void};
export default function NativeStorageNetwork({host,inventory,blocked,operate,selectImage}:Props){
 const [poolName,setPoolName]=useState('');
 const [volumeName,setVolumeName]=useState('');
 const [chosenPool,setChosenPool]=useState('');
 const [size,setSize]=useState(1);
 const [networkName,setNetworkName]=useState('');
 const [mode,setMode]=useState('nat');
 const [subnet,setSubnet]=useState('');
 const [directory,setDirectory]=useState('');
 const [browse,setBrowse]=useState(false);
 const pools=inventory.pools.filter(p=>p.type==='dir'&&p.state==='running');
 const pool=pools.some(p=>p.name===chosenPool)?chosenPool:pools[0]?.name||'';
 const files=useQuery({queryKey:['native-files',host,directory],queryFn:()=>nativeAPI.files(host,directory).then(r=>r.data),enabled:browse,retry:false});
 const entries=files.isError?undefined:files.data;
 const namePattern='[A-Za-z][A-Za-z0-9_.-]{0,62}';
 return <>
  <section className="space-y-4 rounded-lg border p-4"><h2 className="font-semibold">Local datastores</h2>
   <table className="w-full text-left text-sm"><thead><tr><th>Name</th><th>Type</th><th>State</th><th>Available</th></tr></thead><tbody>{inventory.pools.map(p=><tr key={p.name}><td>{p.name}</td><td>{p.type}</td><td>{p.state}</td><td>{p.available}</td></tr>)}</tbody></table>
   <form className="space-y-2" onSubmit={e=>{e.preventDefault();void operate('pool-create',{name:poolName});}}>
    <label className="block">New local datastore name <input className="input" required pattern={namePattern} value={poolName} onChange={e=>setPoolName(e.target.value)}/></label>
    <button className="btn-secondary" disabled={blocked}>Create datastore</button>
   </form>
   <form className="space-y-2" onSubmit={e=>{e.preventDefault();void operate('volume-create',{pool,name:volumeName,size});}}>
    <label className="block">Volume datastore <select className="input" required value={pool} onChange={e=>setChosenPool(e.target.value)}>{pools.map(p=><option key={p.name}>{p.name}</option>)}</select></label>
    <label className="block">New volume name <input className="input" required pattern={namePattern} value={volumeName} onChange={e=>setVolumeName(e.target.value)}/></label>
    <p className="text-sm text-gray-500">A .qcow2 extension is added to the name.</p>
    <label className="block">Capacity (GiB) <input className="input" type="number" required min={1} max={2048} value={size} onChange={e=>setSize(Number(e.target.value))}/></label>
    <button className="btn-secondary" disabled={blocked||!pool}>Create volume</button>
   </form>
  </section>
  <section className="space-y-3 rounded-lg border p-4"><h2 className="font-semibold">Guest networks</h2>
   <table className="w-full text-left text-sm"><thead><tr><th>Name</th><th>Mode</th><th>Active</th><th>Bridge</th></tr></thead><tbody>{inventory.networks.map(n=><tr key={n.name}><td>{n.name}</td><td>{n.mode}</td><td>{n.active}</td><td>{n.bridge}</td></tr>)}</tbody></table>
   <form className="space-y-2" onSubmit={e=>{e.preventDefault();void operate('network-create',{name:networkName,mode,subnet});}}>
    <label className="block">New network name <input className="input" required pattern={namePattern} value={networkName} onChange={e=>setNetworkName(e.target.value)}/></label>
    <label className="block">Network mode <select className="input" value={mode} onChange={e=>setMode(e.target.value)}><option value="nat">NAT</option><option value="isolated">Isolated</option></select></label>
    <label className="block">Private IPv4 subnet (/24) <input className="input" required placeholder="192.168.150.0/24" value={subnet} onChange={e=>setSubnet(e.target.value)}/></label>
    <p className="text-sm text-gray-500">The host checks for overlapping routes. New networks include DHCP and start automatically.</p>
    <button className="btn-secondary" disabled={blocked}>Create network</button>
   </form>
  </section>
  <section className="space-y-3 rounded-lg border p-4"><h2 className="font-semibold">Datastore files</h2>
   <button className="btn-secondary" onClick={()=>{setDirectory('');setBrowse(true);}}>Browse guest datastore</button>
   {browse&&<><p className="break-all">{directory||'/var/lib/libvirt/images'}</p>
    {directory&&<button className="btn-secondary" onClick={()=>setDirectory('')}>Datastore root</button>}
    {files.isFetching&&<p>Reading files…</p>}{files.isError&&<p role="alert">Files are unavailable. Cached entries are hidden.</p>}
    {entries?.length===0&&<p>This directory is empty.</p>}
    <ul>{entries?.map(f=><li className="flex gap-3 py-1" key={f.path}><span>{f.name}</span>{f.directory?<button className="btn-secondary" onClick={()=>setDirectory(f.path)}>Open {f.name}</button>:<><span>{(f.size/1024**2).toFixed(1)} MiB</span>{/\.(qcow2|raw|img)$/i.test(f.name)&&<button className="btn-secondary" onClick={()=>selectImage(f.path)}>Use as import source</button>}</>}</li>)}</ul>
   </>}
  </section>
 </>;
}
