#!/usr/bin/python3
"""Rotate a short-lived, metrics-only token and discover existing cluster nodes."""
import grp,ipaddress,json,os,pathlib,subprocess,time
root=pathlib.Path('/etc/vmalpha-monitoring');gid=grp.getgrnam('vmalpha-monitor').gr_gid
def kubectl(*args):return subprocess.check_output(['/usr/local/bin/kubectl','--kubeconfig=/etc/kubernetes/admin.conf',*args],text=True,timeout=20)
def save(name,text):
 p=root/(name+'.new');fd=os.open(p,os.O_CREAT|os.O_TRUNC|os.O_WRONLY,0o640)
 with os.fdopen(fd,'w') as f:f.write(text)
 os.chown(p,0,gid);p.replace(root/name)
token_file=root/'kubelet-token'
if not token_file.exists() or time.time()-token_file.stat().st_mtime >= 20*60:
 token=kubectl('create','token','vmalpha-prometheus','-n','vmalpha-monitoring','--duration=1h').strip()
 save('kubelet-token',token+'\n')
nodes=json.loads(kubectl('get','nodes','-o','json'))['items'];targets=[]
for node in nodes[:1000]:
 addresses=[a['address'] for a in node['status']['addresses'] if a['type']=='InternalIP']
 if not addresses:continue
 address=ipaddress.ip_address(addresses[0]);host='['+str(address)+']' if address.version==6 else str(address)
 targets.append({'targets':[host+':10250'],'labels':{'node':node['metadata']['name']}})
save('kubelet-targets.json',json.dumps(targets))
print('Refreshed node scrape targets; rotated metrics-only credentials if due')
