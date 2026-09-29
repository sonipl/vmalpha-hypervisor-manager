import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {LineChart,Line,XAxis,YAxis,Tooltip,ResponsiveContainer} from 'recharts';
import {nativeAPI} from '@/services/api';
const catalogs:Record<string,string[]>={host:['cpu','memory','network_rx','network_tx','disk_read','disk_write','disk_free'],vm:['cpu','memory','network_rx','network_tx','disk_read','disk_write'],containers:['cpu','memory','network_rx','network_tx','restarts','nodes_ready'],storage:['health','used','capacity','osd_up','read','write','latency']};
export default function NativePerformance({host,vms=[],defaultScope='host'}:{host:string;vms?:string[];defaultScope?:'host'|'vm'|'containers'|'storage'}){
 const [metric,setMetric]=useState('cpu');const [range,setRange]=useState('1h');const [scope,setScope]=useState<'host'|'vm'|'containers'|'storage'>(defaultScope);const [vm,setVM]=useState('');const [seriesIndex,setSeriesIndex]=useState(0);
 const selectedMetric=catalogs[scope].includes(metric)?metric:catalogs[scope][0];
 const selectedVM=vms.includes(vm)?vm:vms[0]||'';
 const noVM=scope==='vm'&&!selectedVM;
 const query=useQuery({queryKey:['native-metrics',host,selectedMetric,range,scope,scope==='vm'?selectedVM:''],queryFn:()=>nativeAPI.metrics(host,selectedMetric,range,scope,scope==='vm'?selectedVM:'').then(r=>r.data),enabled:!noVM,retry:false,refetchInterval:30000});
 const result=query.isError?undefined:query.data;
 const selectedSeries=result?.series[seriesIndex]?seriesIndex:0;
 const values=result?.series[selectedSeries]?.values||[];
 const points=values.map(([time,value])=>({time:time*1000,value}));
 const measured=values.filter(([,v])=>v!==null);const latest=measured[measured.length-1];
 const stale=!!(latest&&result&&result.timestamp-latest[0]>120);
 return <section className="rounded-lg border p-4"><div className="flex flex-wrap items-center gap-3"><h2 className="font-semibold">Measured performance</h2>
  <label>Scope <select className="input" value={scope} onChange={e=>{setScope(e.target.value as 'host'|'vm'|'containers'|'storage');setSeriesIndex(0);}}><option value="host">Host</option><option value="vm">Virtual machine</option><option value="containers">Containers</option><option value="storage">Ceph storage</option></select></label>
  {scope==='vm'&&<label>Guest <select className="input" value={selectedVM} onChange={e=>setVM(e.target.value)}>{vms.map(v=><option key={v}>{v}</option>)}</select></label>}
  <label>Metric <select className="input" value={selectedMetric} onChange={e=>{setMetric(e.target.value);setSeriesIndex(0);}}>{catalogs[scope].map(m=><option key={m} value={m}>{m.replace(/_/g,' ')}</option>)}</select></label>
  <label>History <select className="input" value={range} onChange={e=>setRange(e.target.value)}>{['1h','6h','24h','7d'].map(r=><option key={r}>{r}</option>)}</select></label></div>
  {noVM?<p>No virtual machines on this host.</p>:query.isPending?<p>Loading measured history…</p>:!result?.available?<p role="status">Monitoring data is unavailable.</p>:<>
   <p className="my-2 text-sm">{result.collectorState==='healthy'&&!stale?'Collector healthy':'Collector unavailable or samples stale'} · {result.unit}{latest&&` · Last sample ${new Date(latest[0]*1000).toLocaleString()}`}</p>
   {result.series.length>1&&<label>Series <select className="input max-w-full" value={selectedSeries} onChange={e=>setSeriesIndex(Number(e.target.value))}>{result.series.map((s,i)=><option key={i} value={i}>{Object.entries(s.labels).filter(([k])=>k!=='__name__').map(([k,v])=>`${k}=${v}`).join(', ')||`Series ${i+1}`}</option>)}</select></label>}
   {points.length===0?<p>No measured samples in this range.</p>:<div className="h-56" aria-label={`${selectedMetric} measured history`}><ResponsiveContainer width="100%" height="100%"><LineChart data={points}><XAxis dataKey="time" tickFormatter={v=>new Date(v).toLocaleTimeString()}/><YAxis/><Tooltip labelFormatter={v=>new Date(Number(v)).toLocaleString()}/><Line type="linear" dataKey="value" stroke="#327b70" dot={false} connectNulls={false}/></LineChart></ResponsiveContainer></div>}
  </>}
 </section>;
}
