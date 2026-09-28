#!/usr/bin/python3
import subprocess,json,sys,ipaddress,pathlib,tempfile,os,time
if len(sys.argv)!=2:raise SystemExit("Pass explicit opt-in configuration JSON")
cfg=json.loads(pathlib.Path(sys.argv[1]).read_text())
if cfg.get("enabled") is not True:raise SystemExit("Ceph shared storage requires explicit opt-in")
hosts=cfg["hosts"]; clients=cfg["clients"]
if len(hosts)!=3 or len(set(hosts))!=3 or len(clients)!=3:raise SystemExit("Exactly three hosts and managed-client addresses required")
for h in hosts:
 if not h or any(c not in "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-" for c in h):raise SystemExit("Invalid hostname")
clients=[str(ipaddress.IPv4Address(a)) for a in clients]
placement="3 "+" ".join(hosts)
vip=str(ipaddress.IPv4Interface(cfg["nfs_vip"]))
if cfg.get("vip_allocation_verified") is not True:raise SystemExit("Verify and reserve VIP before provisioning")

def cmd(*a):
 p=subprocess.run(a,check=True,capture_output=True,text=True,timeout=90);return p.stdout

def ceph(*a):return cmd('ceph',*a)
pools=json.loads(ceph('osd','pool','ls','--format','json'))
for p in ['vmalpha-rbd','vmalpha-cephfs-data','vmalpha-cephfs-metadata']:
 if p not in pools:ceph('osd','pool','create',p)
 ceph('osd','pool','set',p,'size','3');ceph('osd','pool','set',p,'min_size','2');ceph('osd','pool','set',p,'pg_autoscale_mode','on')
ceph('osd','pool','application','enable','vmalpha-rbd','rbd');cmd('rbd','pool','init','vmalpha-rbd')
filesystems=json.loads(ceph('fs','ls','--format','json'))
existing=next((f for f in filesystems if f['name']=='vmalpha-fs'),None)
if existing and (existing.get('metadata_pool')!='vmalpha-cephfs-metadata' or existing.get('data_pools')!=['vmalpha-cephfs-data']):raise SystemExit('Existing filesystem uses different pools; review required')
if not existing:ceph('fs','new','vmalpha-fs','vmalpha-cephfs-metadata','vmalpha-cephfs-data')
ceph('orch','apply','mds','vmalpha-fs',placement)
ceph('fs','set','vmalpha-fs','standby_count_wanted','2')
if 'vmalpha-nfs' not in json.loads(ceph('nfs','cluster','ls')):ceph('nfs','cluster','create','vmalpha-nfs',placement)
print('Named pools and filesystem configured; three-host MDS/NFS placement submitted')

# Wait for active filesystem before mounting, with a bounded deadline.
for attempt in range(36):
 fs=json.loads(ceph('fs','dump','--format','json'))
 if any(f['mdsmap']['fs_name']=='vmalpha-fs' and f['mdsmap'].get('up') for f in fs['filesystems']):break
 time.sleep(5)
else:raise SystemExit('MDS readiness timeout; rerun after resolving service health')
# Prepare only the dedicated export directory. Never change an existing directory.
with tempfile.TemporaryDirectory(prefix='vmalpha-fs-',dir='/run') as mountpoint:
 with tempfile.NamedTemporaryFile(mode='w',prefix='vmalpha-key-',dir='/run') as key:
  os.chmod(key.name,0o600);key.write(ceph('auth','get-key','client.admin').strip());key.flush()
  cmd('mount','-t','ceph',','.join(a+':6789' for a in clients)+':/',mountpoint,'-o','name=admin,secretfile='+key.name+',mds_namespace=vmalpha-fs')
  try:
   directory=pathlib.Path(mountpoint)/'vmalpha'
   if not directory.exists():
    directory.mkdir(mode=0o770);os.chown(directory,4294967294,4294967294);os.chmod(directory,0o770)
  finally:cmd('umount',mountpoint)
# Root-squashed access is restricted to these exact enrolled clients.
exports=json.loads(ceph('nfs','export','ls','vmalpha-nfs'))
if '/vmalpha' not in exports:
 ceph('nfs','export','create','cephfs','--cluster-id','vmalpha-nfs','--pseudo-path','/vmalpha','--fsname','vmalpha-fs','--path','/vmalpha','--client_addr='+','.join(a+'/32' for a in clients),'--squash','root_squash')
else:
 export=json.loads(ceph('nfs','export','get','vmalpha-nfs','/vmalpha'))
 expected={a+'/32' for a in clients}
 actual={a for c in export.get('clients',[]) for a in c.get('addresses',[])}
 if actual!=expected or export.get('access_type','').lower()!='none':raise SystemExit('Existing export ACL differs; review required')
print('Configuration submitted; mark ready only after service and client read/write validation')

# Convert existing NFS service in place; retain pool, filesystem and export.
specs=[{'service_type':'nfs','service_id':'vmalpha-nfs','placement':{'count':3,'hosts':hosts},'spec':{'port':12049,'enable_haproxy_protocol':True}}, {'service_type':'ingress','service_id':'nfs.vmalpha-nfs','placement':{'count':3,'hosts':hosts},'spec':{'backend_service':'nfs.vmalpha-nfs','frontend_port':2049,'monitor_port':9049,'virtual_ip':vip,'enable_haproxy_protocol':True,'use_keepalived_multicast':False}}]
with tempfile.NamedTemporaryFile(mode='w',dir='/run',suffix='.json') as spec:
 for entry in specs:
  spec.seek(0);spec.truncate();json.dump(entry,spec);spec.flush();ceph('orch','apply','-i',spec.name)
print('Single NFS VIP submitted; validate from every selected host before registration')
