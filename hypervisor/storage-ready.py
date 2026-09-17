#!/usr/bin/python3
"""Wait for explicitly autostarted network datastores before QEMU autostart."""
import subprocess,time,xml.etree.ElementTree as ET
def virsh(*args):
 p=subprocess.run(['virsh','-c','storage:///system',*args],text=True,capture_output=True,timeout=20)
 if p.returncode:raise RuntimeError('Storage operation failed')
 return p.stdout.strip()
deadline=time.monotonic()+150
while True:
 try:pool_names=virsh('pool-list','--all','--name').splitlines();break
 except (RuntimeError,subprocess.TimeoutExpired):
  if time.monotonic()>=deadline:raise SystemExit('Storage service did not become available')
  time.sleep(5)
pending=set()
for pool in pool_names:
 data=dict(line.split(':',1) for line in virsh('pool-info',pool).splitlines() if ':' in line)
 data={k.strip():v.strip() for k,v in data.items()}
 if data.get('Autostart')=='yes' and ET.fromstring(virsh('pool-dumpxml',pool)).get('type') in ('netfs','rbd','iscsi','scsi'):
  pending.add(pool)
while pending and time.monotonic()<deadline:
 for pool in list(pending):
  try:
   info=virsh('pool-info',pool)
   if not any(line.startswith('State:') and line.split(':',1)[1].strip()=='running' for line in info.splitlines()):virsh('pool-start',pool)
   pending.remove(pool)
   print('Network datastore ready: '+pool,flush=True)
  except (RuntimeError,subprocess.TimeoutExpired):pass
 if pending:time.sleep(5)
if pending:raise SystemExit('Autostart datastores unavailable: '+', '.join(sorted(pending)))
