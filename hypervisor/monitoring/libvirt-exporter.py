#!/usr/bin/python3
"""Bounded authenticated Prometheus exporter for measured libvirt domain counters."""
import concurrent.futures,hmac,http.server,json,pathlib,subprocess,threading,time
TOKEN=pathlib.Path('/etc/vmalpha-monitoring/scrape-token').read_text().strip()
metrics=b'';lock=threading.Lock()
def virsh(*args):return subprocess.check_output(['virsh','-r',*args],text=True,timeout=15)
def collect():
 global metrics
 while True:
  started=time.monotonic();lines=[]
  try:
   blocks=virsh('domstats','--list-active','--cpu-total','--balloon','--block','--interface').strip().split('\n\n')
   for block in blocks[:1000]:
    rows=block.splitlines()
    if not rows or not rows[0].startswith('Domain: '):continue
    name=rows[0][8:].strip().strip("'");uid=virsh('domuuid',name).strip();labels='vm_uuid='+json.dumps(uid)
    values=dict(line.strip().split('=',1) for line in rows[1:] if '=' in line)
    def emit(metric,key,factor=1,extra=''):
     value=values.get(key)
     if value is not None:
      try:number=float(value)*factor
      except ValueError:return
      lines.append(f'vmalpha_vm_{metric}{{{labels}{extra}}} {number}')
    emit('cpu_seconds_total','cpu.time',1e-9);emit('memory_rss_bytes','balloon.rss',1024);emit('memory_allocated_bytes','balloon.current',1024)
    emit('memory_unused_bytes','balloon.unused',1024)
    for i in range(min(int(values.get('net.count',0)),64)):
     extra=',device='+json.dumps(values.get(f'net.{i}.name',str(i)))
     for direction in ('rx','tx'):
      emit('network_'+direction+'_bytes_total',f'net.{i}.{direction}.bytes',extra=extra)
      emit('network_'+direction+'_errors_total',f'net.{i}.{direction}.errs',extra=extra)
    for i in range(min(int(values.get('block.count',0)),128)):
     extra=',device='+json.dumps(values.get(f'block.{i}.name',str(i)))
     for direction in ('rd','wr'):
      emit('disk_'+direction+'_bytes_total',f'block.{i}.{direction}.bytes',extra=extra)
      emit('disk_'+direction+'_requests_total',f'block.{i}.{direction}.reqs',extra=extra)
      emit('disk_'+direction+'_seconds_total',f'block.{i}.{direction}.times',1e-9,extra)
   lines.extend(['vmalpha_libvirt_scrape_success 1',f'vmalpha_libvirt_scrape_timestamp_seconds {time.time()}'])
  except Exception:lines=['vmalpha_libvirt_scrape_success 0']
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
http.server.ThreadingHTTPServer(('127.0.0.1',9177),Handler).serve_forever()
