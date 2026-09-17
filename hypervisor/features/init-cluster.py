#!/usr/bin/python3
"""Initialize a new prepared control plane without replacing existing identity."""
import argparse,base64,json,os,pathlib,secrets,subprocess
import yaml


def write_private(path,data):
 descriptor=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
 with os.fdopen(descriptor,'w') as output:output.write(data)


def documents(node,converged=False):
 if node['role']!='control-plane':raise ValueError('A prepared control-plane role is required')
 init={'apiVersion':'kubeadm.k8s.io/v1beta4','kind':'InitConfiguration','bootstrapTokens':[{'ttl':'30m'}],'localAPIEndpoint':{'advertiseAddress':node['node_ip'],'bindPort':6443},'nodeRegistration':{'name':node['name'],'criSocket':'unix:///run/containerd/containerd.sock'}}
 if converged:init['nodeRegistration']['taints']=[]
 cluster={'apiVersion':'kubeadm.k8s.io/v1beta4','kind':'ClusterConfiguration','kubernetesVersion':'v1.36.4','clusterName':'vmalpha','controlPlaneEndpoint':node['api_name']+':6443','networking':{'podSubnet':node['pod_cidr'],'serviceSubnet':node['service_cidr']},'apiServer':{'certSANs':[node['api_name'],node['api_ip'],node['node_ip']],'extraArgs':[{'name':'encryption-provider-config','value':'/etc/kubernetes/encryption.yaml'}],'extraVolumes':[{'name':'encryption-config','hostPath':'/etc/kubernetes/encryption.yaml','mountPath':'/etc/kubernetes/encryption.yaml','readOnly':True,'pathType':'File'}]}}
 kubelet={'apiVersion':'kubelet.config.k8s.io/v1beta1','kind':'KubeletConfiguration','cgroupDriver':'systemd','failSwapOn':False,'memorySwap':{'swapBehavior':'NoSwap'},'serverTLSBootstrap':True,'readOnlyPort':0,'authentication':{'anonymous':{'enabled':False},'webhook':{'enabled':True}},'authorization':{'mode':'Webhook'},'shutdownGracePeriod':'90s','shutdownGracePeriodCriticalPods':'30s'}
 return [init,cluster,kubelet]


def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--run-workloads',action='store_true',help='Explicitly allow workloads on this control-plane node');parser.add_argument('--apply',action='store_true');args=parser.parse_args()
 if os.geteuid()!=0:raise ValueError('Run initialization as root')
 root=pathlib.Path('/etc/kubernetes')
 if any((root/name).exists() for name in ('admin.conf','kubelet.conf','pki/ca.key')) or (pathlib.Path('/var/lib/etcd').exists() and any(pathlib.Path('/var/lib/etcd').iterdir())):
  raise ValueError('Existing cluster identity or etcd data found; refusing initialization or reset')
 node_file=pathlib.Path('/etc/vmalpha/cluster-node.json');info=node_file.lstat()
 if node_file.is_symlink() or info.st_uid!=0 or info.st_mode&0o077:raise ValueError('Prepared node record must be a protected root-owned file')
 node=json.loads(node_file.read_text());config=documents(node,args.run_workloads)
 if not args.apply:print('New-cluster configuration validated; no identity or membership created');return
 root.mkdir(exist_ok=True)
 source=pathlib.Path(__file__).resolve().parent
 subprocess.run(['bash',str(source/'load-images.sh')],check=True)
 encryption=root/'encryption.yaml'
 if not encryption.exists():
  material={'apiVersion':'apiserver.config.k8s.io/v1','kind':'EncryptionConfiguration','resources':[{'resources':['secrets'],'providers':[{'aescbc':{'keys':[{'name':'vmalpha-1','secret':base64.b64encode(secrets.token_bytes(32)).decode()}]}},{'identity':{}}]}]}
  write_private(encryption,json.dumps(material))
 else:
  info=encryption.lstat()
  if encryption.is_symlink() or info.st_uid!=0 or info.st_mode&0o077:raise ValueError('Existing encryption configuration is not protected')
 path=root/'vmalpha-init.yaml'
 write_private(path,yaml.safe_dump_all(config))
 checked=subprocess.run(['/usr/local/bin/kubeadm','config','validate','--config',str(path)],capture_output=True,text=True)
 if checked.returncode:raise ValueError('kubeadm rejected initialization configuration')
 descriptor=os.open('/var/log/vmalpha-kubeadm-init.log',os.O_WRONLY|os.O_CREAT|os.O_TRUNC|os.O_NOFOLLOW,0o600);os.fchmod(descriptor,0o600)
 subprocess.run(['systemctl','enable','--now','kubelet'],check=True,stdout=subprocess.DEVNULL)
 with os.fdopen(descriptor,'w') as output:
  result=subprocess.run(['/usr/local/bin/kubeadm','init','--config',str(path)],stdout=output,stderr=subprocess.STDOUT,timeout=1800)
 if result.returncode:raise ValueError('Initialization failed; inspect protected host log. No reset was performed.')
 manifest=list(yaml.safe_load_all((source/'manifests/kube-flannel-0.28.9.yaml').read_text()))
 for obj in manifest:
  if obj and obj.get('kind')=='ConfigMap' and 'net-conf.json' in obj.get('data',{}):
   network=json.loads(obj['data']['net-conf.json']);network['Network']=node['pod_cidr'];obj['data']['net-conf.json']=json.dumps(network)
 applied=subprocess.run(['/usr/local/bin/kubectl','--kubeconfig=/etc/kubernetes/admin.conf','apply','-f','-'],input=json.dumps({'apiVersion':'v1','kind':'List','items':[x for x in manifest if x]}),text=True,capture_output=True,timeout=120)
 if applied.returncode:raise ValueError('Cluster initialized but network add-on did not complete; inspect it without reinitializing')
 subprocess.run(['python3',str(source/'configure-shutdown.py')],check=True)
 print('New control plane initialized with encrypted Secrets and bundled networking. Verify Ready and workload/network acceptance before use.')

if __name__=='__main__':
 try:main()
 except ValueError as error:raise SystemExit(str(error))
 except (OSError,subprocess.SubprocessError,yaml.YAMLError,KeyError,TypeError,AttributeError):raise SystemExit('Initialization did not complete; inspect protected host configuration/logs. No reset was performed.')
