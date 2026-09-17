#!/usr/bin/python3
"""Enable kubelet graceful shutdown while local storage services remain available."""
import json,pathlib,subprocess,yaml
p=pathlib.Path('/var/lib/kubelet/config.yaml')
if not p.exists():raise SystemExit('Initialize or join the cluster first')
config=yaml.safe_load(p.read_text());config['shutdownGracePeriod']='90s';config['shutdownGracePeriodCriticalPods']='30s'
p.write_text(yaml.safe_dump(config));p.chmod(0o600)
for unit in ('kubelet','containerd'):
 directory=pathlib.Path('/etc/systemd/system')/(unit+'.service.d');directory.mkdir(exist_ok=True)
 (directory/'vmalpha-storage.conf').write_text('[Unit]\nAfter=ceph.target\n')
directory=pathlib.Path('/etc/systemd/logind.conf.d');directory.mkdir(exist_ok=True)
(directory/'vmalpha-kubernetes.conf').write_text('[Login]\nInhibitDelayMaxSec=120\n')
subprocess.run(['systemctl','daemon-reload'],check=True)
subprocess.run(['systemctl','restart','systemd-logind'],check=True)
subprocess.run(['systemctl','restart','kubelet'],check=True)
if pathlib.Path('/etc/kubernetes/admin.conf').exists():
 # Keep CSI available during ordinary workload termination.
 patch={'spec':{'nodePlugin':{'priorityClassName':'system-node-critical'},'controllerPlugin':{'priorityClassName':'system-cluster-critical'}}}
 k=['/usr/local/bin/kubectl','--kubeconfig=/etc/kubernetes/admin.conf']
 crd=subprocess.run(k+['get','crd','drivers.csi.ceph.io','--ignore-not-found','-o','name'],capture_output=True,text=True,check=True).stdout.strip()
 if crd:
  driver=subprocess.run(k+['-n','ceph-csi-operator-system','get','driver','rbd.csi.ceph.com','--ignore-not-found','-o','name'],capture_output=True,text=True,check=True).stdout.strip()
  if driver:subprocess.run(k+['-n','ceph-csi-operator-system','patch','driver','rbd.csi.ceph.com','--type=merge','-p',json.dumps(patch)],check=True)
print('Graceful shutdown configured; reboot acceptance still required')
