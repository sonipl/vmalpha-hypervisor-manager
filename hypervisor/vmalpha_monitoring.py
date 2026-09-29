"""Fixed Prometheus queries with host/per-VM authorization and bounded responses."""
import base64,json,pathlib,re,time,urllib.parse,urllib.request
import vmalpha_auth as access
HOST={
 'cpu':('100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle"}[2m])))','%'),
 'memory':('node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes','bytes'),
 'network_rx':('sum(rate(node_network_receive_bytes_total{device!~"lo|veth.*|vnet.*|virbr.*|cni.*|flannel.*|docker.*"}[2m]))','bytes/s'),
 'network_tx':('sum(rate(node_network_transmit_bytes_total{device!~"lo|veth.*|vnet.*|virbr.*|cni.*|flannel.*|docker.*"}[2m]))','bytes/s'),
 'disk_read':('sum(rate(node_disk_read_bytes_total{device!~"loop.*|dm-.*"}[2m]))','bytes/s'),
 'disk_write':('sum(rate(node_disk_written_bytes_total{device!~"loop.*|dm-.*"}[2m]))','bytes/s'),
 'disk_read_iops':('sum(rate(node_disk_reads_completed_total{device!~"loop.*|dm-.*"}[2m]))','IOPS'),
 'disk_write_iops':('sum(rate(node_disk_writes_completed_total{device!~"loop.*|dm-.*"}[2m]))','IOPS'),
 'disk_read_latency':('sum(rate(node_disk_read_time_seconds_total[2m])) / sum(rate(node_disk_reads_completed_total[2m]))','seconds'),
 'disk_write_latency':('sum(rate(node_disk_write_time_seconds_total[2m])) / sum(rate(node_disk_writes_completed_total[2m]))','seconds'),
 'disk_free':('node_filesystem_avail_bytes{fstype!~"tmpfs|devtmpfs|overlay|squashfs"}','bytes')}
VM={
 'cpu':('100 * rate(vmalpha_vm_cpu_seconds_total{FILTER}[2m])','% of one CPU'),
 'memory':('vmalpha_vm_memory_rss_bytes{FILTER}','bytes (QEMU RSS)'),
 'network_rx':('sum(rate(vmalpha_vm_network_rx_bytes_total{FILTER}[2m]))','bytes/s'),
 'network_tx':('sum(rate(vmalpha_vm_network_tx_bytes_total{FILTER}[2m]))','bytes/s'),
 'disk_read':('sum(rate(vmalpha_vm_disk_rd_bytes_total{FILTER}[2m]))','bytes/s'),
 'disk_write':('sum(rate(vmalpha_vm_disk_wr_bytes_total{FILTER}[2m]))','bytes/s'),
 'disk_read_iops':('sum(rate(vmalpha_vm_disk_rd_requests_total{FILTER}[2m]))','IOPS'),
 'disk_write_iops':('sum(rate(vmalpha_vm_disk_wr_requests_total{FILTER}[2m]))','IOPS'),
 'disk_read_latency':('sum(rate(vmalpha_vm_disk_rd_seconds_total{FILTER}[2m])) / sum(rate(vmalpha_vm_disk_rd_requests_total{FILTER}[2m]))','seconds'),
 'disk_write_latency':('sum(rate(vmalpha_vm_disk_wr_seconds_total{FILTER}[2m])) / sum(rate(vmalpha_vm_disk_wr_requests_total{FILTER}[2m]))','seconds')}
CONTAINERS={
 'cpu':('sum by (namespace,pod) (rate(container_cpu_usage_seconds_total{FILTER,container!="",container!="POD"}[2m]))','CPU cores'),
 'memory':('sum by (namespace,pod) (container_memory_working_set_bytes{FILTER,container!="",container!="POD"})','bytes'),
 'network_rx':('sum by (namespace,pod) (rate(container_network_receive_bytes_total{FILTER}[2m]))','bytes/s'),
 'network_tx':('sum by (namespace,pod) (rate(container_network_transmit_bytes_total{FILTER}[2m]))','bytes/s'),
 'restarts':('sum by (namespace,pod) (kube_pod_container_status_restarts_total{FILTER})','restarts'),
 'nodes_ready':('sum(kube_node_status_condition{condition="Ready",status="true"})','nodes (cluster-wide)')}
STORAGE={'health':('ceph_health_status','0 OK / 1 WARN / 2 ERR'),'used':('ceph_cluster_total_used_bytes','bytes'),'capacity':('ceph_cluster_total_bytes','bytes'),'osd_up':('vmalpha_ceph_osds_up','OSDs'),'read':('sum(rate(vmalpha_ceph_pool_read_bytes_total[2m]))','bytes/s'),'write':('sum(rate(vmalpha_ceph_pool_write_bytes_total[2m]))','bytes/s'),'latency':('vmalpha_ceph_osd_commit_latency_seconds','seconds')}
RANGES={'1h':3600,'6h':21600,'24h':86400,'7d':604800}
def container_query(metric,namespace='',pod=''):
 labels=[]
 for key,value,limit in (('namespace',namespace,63),('pod',pod,253)):
  if not isinstance(value,str) or len(value)>limit:raise ValueError('Invalid workload filter')
  if value:
   if not all(re.fullmatch(r'[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?',part) for part in value.split('.')) or (key=='namespace' and '.' in value):raise ValueError('Invalid workload filter')
   labels.append(key+'='+json.dumps(value))
 selector=','.join(labels)
 query=CONTAINERS[metric][0]
 return query.replace('FILTER,',selector+',' if selector else '').replace('FILTER',selector)

def alerts():
 if not access.admin():raise ValueError('Host administrator permission is required')
 now=time.time()
 try:
  cred=pathlib.Path('/etc/vmalpha-monitoring/query-password').read_text().strip()
  request=urllib.request.Request('http://127.0.0.1:9091/api/v1/alerts',headers={'Authorization':'Basic '+base64.b64encode(('vmalpha:'+cred).encode()).decode()})
  with urllib.request.urlopen(request,timeout=8) as response:raw=response.read(1024*1024+1)
  if len(raw)>1024*1024:raise ValueError('Alert response exceeds limit')
  result=json.loads(raw)
  if result.get('status')!='success':raise ValueError('Alert query failed')
  items=result['data']['alerts']
  if not isinstance(items,list):raise ValueError('Invalid alert response')
  rows=[]
  for item in items[:200]:
   labels=item.get('labels',{});annotations=item.get('annotations',{})
   rows.append({'name':str(labels.get('alertname','Alert'))[:200],
    'severity':str(labels.get('severity','unspecified'))[:40],
    'state':str(item.get('state','unknown'))[:40],
    'activeAt':str(item.get('activeAt',''))[:80],
    'summary':str(annotations.get('summary',''))[:1000],
    'labels':{str(k)[:100]:str(v)[:300] for k,v in list(labels.items())[:30] if k!='alertname'}})
  return {'available':True,'timestamp':now,'alerts':rows,'truncated':len(items)>200}
 except (OSError,ValueError,KeyError,TypeError,AttributeError):
  return {'available':False,'timestamp':now,'alerts':[],
   'message':'Alert evaluation is unavailable. Existing alerts must not be assumed resolved.'}

def handle(a):
 if a.get('metric')=='alerts':return alerts()
 scope=a.get('scope','host');metric=a.get('metric','cpu');window=a.get('range','1h')
 if window not in RANGES:raise ValueError('Unsupported monitoring time range')
 if scope=='vm':
  access.require('vm.view',a.get('name'));uid=access.domain_id(a.get('name'));catalog=VM
 else:
  if not access.admin():raise ValueError('Host administrator permission is required')
  catalog={'host':HOST,'containers':CONTAINERS,'storage':STORAGE}.get(scope)
 if not catalog or metric not in catalog:raise ValueError('Unsupported monitoring metric')
 query,unit=catalog[metric]
 if scope=='containers':query=container_query(metric,a.get('namespace',''),a.get('pod',''))
 if scope=='vm':query=query.replace('FILTER','vm_uuid='+json.dumps(uid))
 now=time.time();params=urllib.parse.urlencode({'query':query,'start':now-RANGES[window],'end':now,'step':max(15,RANGES[window]//240),'timeout':'8s'})
 try:
  cred=pathlib.Path('/etc/vmalpha-monitoring/query-password').read_text().strip()
  request=urllib.request.Request('http://127.0.0.1:9091/api/v1/query_range?'+params,headers={'Authorization':'Basic '+base64.b64encode(('vmalpha:'+cred).encode()).decode()})
  with urllib.request.urlopen(request,timeout=10) as response:raw=response.read(2*1024*1024+1)
  if len(raw)>2*1024*1024:raise ValueError('Monitoring response exceeds limit')
  result=json.loads(raw)
  if result.get('status')!='success':raise ValueError('Monitoring query failed')
  job={'host':'host','vm':'libvirt','containers':'kubelet','storage':'ceph'}[scope]
  if scope=='containers' and metric in ('restarts','nodes_ready'):job='kube-state-metrics'
  health_query='up{job='+json.dumps(job)+'}'
  if scope=='vm':health_query+=' * on(job,instance) vmalpha_libvirt_scrape_success'
  if scope=='storage':health_query+=' * on(job,instance) vmalpha_ceph_scrape_success'
  health_params=urllib.parse.urlencode({'query':health_query,'timeout':'3s'})
  health_request=urllib.request.Request('http://127.0.0.1:9091/api/v1/query?'+health_params,headers=request.headers)
  with urllib.request.urlopen(health_request,timeout=5) as response:health=json.loads(response.read(262144))
  samples=health.get('data',{}).get('result',[])
  collector='healthy' if samples and all(float(s['value'][1])==1 for s in samples) else 'unavailable'
  rows=result['data']['result'][:100];series=[]
  for row in rows:
   values=[]
   for ts,value in row.get('values',[]):
    try:value=float(value);value=value if __import__('math').isfinite(value) else None
    except ValueError:value=None
    values.append([ts,value])
   series.append({'labels':row['metric'],'values':values})
  return {'available':True,'collectorState':collector,'scope':scope,'metric':metric,'unit':unit,'range':window,'timestamp':now,'series':series,'message':'' if series else 'No measured samples in this range. Collector may be unavailable or not initialized.'}
 except (OSError,ValueError,KeyError):return {'available':False,'scope':scope,'metric':metric,'unit':unit,'timestamp':now,'series':[],'message':'Monitoring data unavailable. Check the collector and Prometheus service.'}
