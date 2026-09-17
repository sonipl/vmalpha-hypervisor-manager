#!/usr/bin/python3
"""Authenticated bounded collector of actual Ceph monitor statistics."""
import hmac,http.server,json,pathlib,subprocess,threading,time
TOKEN=pathlib.Path('/etc/vmalpha-monitoring/scrape-token').read_text().strip()
KEY='/etc/vmalpha-monitoring/ceph.client.vmalpha-monitoring.keyring'
lock=threading.Lock();metrics=b''
def command(binary,*args):
 p=subprocess.run([binary,'--name','client.vmalpha-monitoring','--keyring',KEY,*args,'--format','json'],capture_output=True,text=True,timeout=10)
 if p.returncode or len(p.stdout)>2*1024*1024:raise ValueError('Ceph collector unavailable')
 return json.loads(p.stdout)
def collect():
 global metrics
 while True:
  started=time.monotonic();lines=[]
  try:
   status=command('ceph','status');capacity=command('ceph','df');io=command('rados','df');perf=command('ceph','osd','perf')
   def emit(metric,value,labels=None):
    label='{'+','.join(k+'='+json.dumps(str(v)) for k,v in labels.items())+'}' if labels else ''
    lines.append(metric+label+' '+str(float(value)))
   emit('ceph_health_status',{'HEALTH_OK':0,'HEALTH_WARN':1,'HEALTH_ERR':2}[status['health']['status']])
   for key,suffix in [('total_bytes','bytes'),('total_used_bytes','used_bytes'),('total_avail_bytes','available_bytes')]:
    emit('ceph_cluster_total_'+suffix,capacity['stats'][key])
   emit('vmalpha_ceph_osds_up',status['osdmap']['num_up_osds'])
   emit('vmalpha_ceph_osds_total',status['osdmap']['num_osds'])
   for pool in io['pools'][:1024]:
    for key in ('read_bytes','write_bytes','read_ops','write_ops'):
     emit('vmalpha_ceph_pool_'+key+'_total',pool[key],{'pool':pool['name']})
   for row in perf['osdstats']['osd_perf_infos'][:2048]:
    for operation in ('commit','apply'):
     emit('vmalpha_ceph_osd_'+operation+'_latency_seconds',row['perf_stats'][operation+'_latency_ns']/1e9,{'osd':row['id']})
   for name,check in status['health']['checks'].items():
    emit('vmalpha_ceph_health_check',1,{'check':name,'severity':check['severity']})
   lines.extend(['vmalpha_ceph_scrape_success 1',f'vmalpha_ceph_scrape_timestamp_seconds {time.time()}'])
  except Exception:lines=['vmalpha_ceph_scrape_success 0']
  with lock:metrics=('\n'.join(lines)+'\n').encode()
  time.sleep(max(1,15-(time.monotonic()-started)))
class Handler(http.server.BaseHTTPRequestHandler):
 def do_GET(self):
  if self.path!='/metrics':self.send_error(404);return
  if not hmac.compare_digest(self.headers.get('Authorization',''),'Bearer '+TOKEN):self.send_error(401);return
  with lock:body=metrics
  self.send_response(200);self.send_header('Content-Type','text/plain; version=0.0.4');self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)
 def log_message(self,*args):pass
threading.Thread(target=collect,daemon=True).start()
http.server.ThreadingHTTPServer(('127.0.0.1',9178),Handler).serve_forever()
