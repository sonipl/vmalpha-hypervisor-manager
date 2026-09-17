#!/usr/bin/python3
"""Join one prepared node with protected kubeadm configuration; never reset it."""
import argparse,json,os,pathlib,re,stat,subprocess
import yaml


def protected_file(path):
 p=pathlib.Path(path);info=p.lstat()
 if not stat.S_ISREG(info.st_mode) or info.st_uid!=0 or info.st_mode & 0o077:
  raise ValueError('Enrollment files must be regular root-owned files with mode 0600 or stricter')
 if info.st_size>65536:raise ValueError('Enrollment configuration is too large')
 return p


def validated(data,node):
 if not isinstance(data,dict) or data.get('apiVersion')!='kubeadm.k8s.io/v1beta4' or data.get('kind')!='JoinConfiguration':raise ValueError('A kubeadm v1beta4 JoinConfiguration is required')
 if set(data)-{'apiVersion','kind','discovery','nodeRegistration','controlPlane'}:raise ValueError('Unsupported enrollment configuration fields')
 discovery=data.get('discovery',{});token=discovery.get('bootstrapToken',{})
 if set(discovery)-{'bootstrapToken'}:raise ValueError('Only pinned bootstrap-token discovery is supported')
 if set(token)-{'apiServerEndpoint','token','caCertHashes'}:raise ValueError('Unsupported bootstrap-token fields')
 if token.get('apiServerEndpoint')!=node['api_name']+':6443':raise ValueError('Enrollment endpoint differs from prepared endpoint')
 if not re.fullmatch(r'[a-z0-9]{6}\.[a-z0-9]{16}',token.get('token','')):raise ValueError('Invalid bootstrap token format')
 hashes=token.get('caCertHashes',[])
 if not isinstance(hashes,list) or not hashes or not all(isinstance(h,str) and re.fullmatch(r'sha256:[a-f0-9]{64}',h) for h in hashes):raise ValueError('Pinned SHA256 CA hashes are required')
 registration=data.get('nodeRegistration',{})
 if set(registration)-{'name','criSocket'}:raise ValueError('Unsupported node registration fields')
 if registration.get('name')!=node['name'] or registration.get('criSocket')!='unix:///run/containerd/containerd.sock':raise ValueError('Enrollment node/runtime identity differs from prepared node')
 control=data.get('controlPlane')
 if node['role']=='control-plane':
  if not isinstance(control,dict) or set(control)-{'localAPIEndpoint','certificateKey'}:raise ValueError('Control-plane enrollment configuration is required')
  if control.get('localAPIEndpoint')!={'advertiseAddress':node['node_ip'],'bindPort':6443}:raise ValueError('Control-plane address differs from selected local address')
  if not re.fullmatch(r'[a-f0-9]{64}',control.get('certificateKey','')):raise ValueError('Protected certificate transfer key is required')
 elif control is not None:raise ValueError('Worker enrollment cannot add a control-plane role')
 return data


def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--config-file',required=True);parser.add_argument('--apply',action='store_true');args=parser.parse_args()
 if os.geteuid()!=0:raise ValueError('Run enrollment as root')
 if pathlib.Path('/etc/kubernetes/kubelet.conf').exists():raise ValueError('Existing cluster identity found; refusing to join or reset')
 node=json.loads(protected_file('/etc/vmalpha/cluster-node.json').read_text())
 config=protected_file(args.config_file)
 data=validated(yaml.safe_load(config.read_text()),node)
 if node['role']=='control-plane':
  encryption=protected_file('/etc/kubernetes/encryption.yaml')
  if yaml.safe_load(encryption.read_text()).get('kind')!='EncryptionConfiguration':raise ValueError('Shared encryption-provider configuration is required')
 # Validation output never includes enrollment material.
 checked=subprocess.run(['/usr/local/bin/kubeadm','config','validate','--config',str(config)],capture_output=True,text=True)
 if checked.returncode:raise ValueError('kubeadm rejected enrollment configuration')
 if not args.apply:print('Protected enrollment configuration validated; no membership change');return
 log=pathlib.Path('/var/log/vmalpha-kubeadm-join.log')
 descriptor=os.open(log,os.O_WRONLY|os.O_CREAT|os.O_TRUNC|os.O_NOFOLLOW,0o600);os.fchmod(descriptor,0o600)
 subprocess.run(['systemctl','enable','--now','kubelet'],check=True,stdout=subprocess.DEVNULL)
 with os.fdopen(descriptor,'w') as output:
  result=subprocess.run(['/usr/local/bin/kubeadm','join','--config',str(config)],stdout=output,stderr=subprocess.STDOUT,timeout=1800)
 if result.returncode:raise ValueError('Join failed; inspect the protected host log. No reset was performed.')
 subprocess.run(['python3',str(pathlib.Path(__file__).with_name('configure-shutdown.py'))],check=True)
 print('Existing-cluster join completed. Verify Ready, serving certificate, networking and storage before adding another node.')

if __name__=='__main__':
 try:main()
 except ValueError as error:
  raise SystemExit(str(error))
 except (OSError,subprocess.SubprocessError,yaml.YAMLError,KeyError,TypeError,AttributeError):
  raise SystemExit('Enrollment did not complete. Check prerequisites and protected host configuration/logs; secrets are not printed.')
