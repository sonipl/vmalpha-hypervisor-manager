#!/usr/bin/python3
import subprocess,json,datetime,pathlib,os
cfg=json.loads(pathlib.Path('/etc/vmalpha/storage-workflow.json').read_text())
hosts=dict(zip(cfg['hosts'],cfg['clients']))
vip=cfg['nfs_vip'].split('/')[0]
def run(*a):return subprocess.run(a,check=True,capture_output=True,text=True,timeout=120).stdout
for name,ip in hosts.items():
 if ip==cfg['clients'][0]:run('bash','/usr/local/sbin/vmalpha-storage-verify-client')
 else:run('ssh','-o','BatchMode=yes','-o','ConnectTimeout=10',ip,'bash /usr/local/sbin/vmalpha-storage-verify-client')
manifest={'version':1,'status':'verified','verified_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'fsid':run('ceph','fsid').strip(),'monitors':list(hosts.values()),'placement_nodes':list(hosts),'rbd_pool':'vmalpha-rbd','cephfs':{'name':'vmalpha-fs','data_pool':'vmalpha-cephfs-data','metadata_pool':'vmalpha-cephfs-metadata'},'nfs':{'service':'vmalpha-nfs','export':'/vmalpha','endpoint':vip+':2049'},'checks':{'rbd_read_write':True,'cephfs_read_write':True,'nfs_read_write':True},'host_access':{h:{'rbd_read_write':True,'nfs_read_write':True} for h in hosts}}
p=pathlib.Path('/var/lib/vmalpha/storage-registration.json');tmp=p.with_suffix('.tmp');tmp.write_text(json.dumps(manifest));os.chmod(tmp,0o600);tmp.replace(p)
print('All three hosts verified against scoped RBD and single NFS VIP; manifest refreshed')
