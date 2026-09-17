#!/usr/bin/python3
"""Prepare an explicitly selected new node; never initialize or reset a cluster."""
import argparse,ipaddress,json,os,pathlib,re,subprocess


def configuration(args):
 for value in (args.name,args.api_name):
  if len(value)>253 or not all(re.fullmatch(r'[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?',label) for label in value.split('.')):
   raise ValueError('Node and API names must be valid lowercase DNS names')
 networks=[ipaddress.ip_network(v,strict=True) for v in (args.node_cidr,args.pod_cidr,args.service_cidr)]
 node_ip=ipaddress.ip_address(args.node_ip);api_ip=ipaddress.ip_address(args.api_ip)
 if any(n.version!=4 for n in networks) or node_ip.version!=4 or api_ip.version!=4:raise ValueError('This preparation workflow currently supports IPv4 only')
 if networks[0].prefixlen<16:raise ValueError('Select a bounded node network (/16 or narrower)')
 if node_ip not in networks[0] or api_ip not in networks[0]:raise ValueError('Node and API addresses must be in the selected node network')
 if any(networks[i].overlaps(networks[j]) for i in range(3) for j in range(i+1,3)):raise ValueError('Node, pod and service networks must not overlap')
 return dict(name=args.name,node_ip=str(node_ip),node_cidr=str(networks[0]),pod_cidr=str(networks[1]),service_cidr=str(networks[2]),api_name=args.api_name,api_ip=str(api_ip),role=args.role,storage=args.storage)


def run(*args):return subprocess.run(args,check=True,text=True,capture_output=True).stdout.strip()


def prepare(config):
 if os.geteuid()!=0:raise ValueError('Run preparation as root')
 if pathlib.Path('/etc/kubernetes/kubelet.conf').exists():raise ValueError('Node already has cluster membership; use the reviewed maintenance/rejoin workflow')
 local=json.loads(run('ip','-j','address'))
 addresses={a['local'] for interface in local for a in interface.get('addr_info',[]) if a.get('family')=='inet'}
 if config['node_ip'] not in addresses:raise ValueError('Selected node address is not assigned locally')
 if run('getenforce')!='Enforcing':raise ValueError('SELinux must be enforcing')
 run('systemctl','is-active','firewalld')
 if run('timedatectl','show','-p','NTPSynchronized','--value')!='yes':raise ValueError('Synchronize the host clock before enrollment')
 for binary in ('kubeadm','kubelet','kubectl','containerd'):
  if not pathlib.Path('/usr/local/bin',binary).is_file():raise ValueError('Install the verified offline runtime payload first')
 hosts=pathlib.Path('/etc/hosts');lines=hosts.read_text().splitlines()
 for line in lines:
  fields=line.split('#',1)[0].split()
  if config['api_name'] in fields[1:] and fields[0]!=config['api_ip']:raise ValueError('Existing API endpoint mapping conflicts with selection')
 if not any(config['api_name'] in line.split('#',1)[0].split()[1:] for line in lines):
  hosts.write_text('\n'.join(lines)+'\n'+config['api_ip']+' '+config['api_name']+' # VM Alpha cluster endpoint\n')
 run('hostnamectl','set-hostname',config['name'])
 for module in ('br_netfilter','overlay'):run('modprobe',module)
 pathlib.Path('/etc/modules-load.d/vmalpha-kubernetes.conf').write_text('br_netfilter\noverlay\n')
 sysctl=pathlib.Path('/etc/sysctl.d/90-vmalpha-kubernetes.conf')
 sysctl.write_text('net.ipv4.ip_forward = 1\nnet.bridge.bridge-nf-call-iptables = 1\nnet.bridge.bridge-nf-call-ip6tables = 1\n')
 run('sysctl','--load',str(sysctl))
 ports=[('10250','tcp'),('8472','udp')]
 if config['role']=='control-plane':ports += [('6443','tcp'),('2379-2380','tcp')]
 if config['storage']:ports += [('3300','tcp'),('6789','tcp'),('6800-7568','tcp')]
 for port,protocol in ports:
  rule='rule family="ipv4" source address="'+config['node_cidr']+'" port port="'+port+'" protocol="'+protocol+'" accept'
  for permanent in ([],['--permanent']):run('firewall-cmd',*permanent,'--add-rich-rule='+rule)
 zones=run('firewall-cmd','--permanent','--get-zones').split()
 if 'vmalpha-pods' not in zones:run('firewall-cmd','--permanent','--new-zone=vmalpha-pods')
 run('firewall-cmd','--permanent','--zone=vmalpha-pods','--add-source='+config['pod_cidr'])
 run('firewall-cmd','--permanent','--zone=vmalpha-pods','--add-forward')
 for port,protocol in [('6443','tcp')]+([('3300','tcp'),('6789','tcp'),('6800-7568','tcp')] if config['storage'] else []):
  run('firewall-cmd','--permanent','--zone=vmalpha-pods','--add-port='+port+'/'+protocol)
 policies=run('firewall-cmd','--permanent','--get-policies').split()
 if 'va-pods-egress' not in policies:run('firewall-cmd','--permanent','--new-policy=va-pods-egress')
 for flag in ('--add-ingress-zone=vmalpha-pods','--add-egress-zone=ANY','--set-target=ACCEPT'):
  run('firewall-cmd','--permanent','--policy=va-pods-egress',flag)
 run('firewall-cmd','--reload')
 root=pathlib.Path('/etc/vmalpha');root.mkdir(mode=0o750,exist_ok=True)
 record=root/'cluster-node.json';record.write_text(json.dumps(config,indent=2)+'\n');record.chmod(0o600)
 run('restorecon','-RF','/etc/hosts','/etc/vmalpha','/etc/modules-load.d/vmalpha-kubernetes.conf',str(sysctl))
 run('systemctl','enable','--now','containerd')
 print('Node prerequisites applied. No cluster was created, joined, reset or removed.')


def main():
 parser=argparse.ArgumentParser(description=__doc__)
 for option in ('name','node-ip','node-cidr','pod-cidr','service-cidr','api-name','api-ip'):parser.add_argument('--'+option,required=True)
 parser.add_argument('--role',choices=('control-plane','worker'),required=True)
 parser.add_argument('--storage',action='store_true')
 parser.add_argument('--apply',action='store_true',help='Apply prerequisites to this new node; otherwise validate inputs only')
 args=parser.parse_args();config=configuration(args)
 if args.apply:prepare(config)
 else:print(json.dumps({'validated':True,'configuration':config,'applied':False},indent=2))

if __name__=='__main__':
 try:main()
 except (ValueError,subprocess.CalledProcessError) as error:raise SystemExit(str(error))
