#!/usr/bin/python3
"""Bootstrap explicitly selected Ceph monitor/manager roles without consuming disks."""
import argparse,json,os,pathlib,subprocess


def command(node,single_host,config):
 if not node.get('storage'):raise ValueError('Prepare this node with the storage role first')
 result=['cephadm','--image','quay.io/ceph/ceph:v20.2.4','bootstrap','--mon-ip',node['node_ip'],'--skip-pull','--skip-dashboard','--skip-monitoring-stack','--skip-firewalld','--config',str(config)]
 if single_host:result.append('--single-host-defaults')
 return result


def main():
 parser=argparse.ArgumentParser(description=__doc__)
 parser.add_argument('--single-host',action='store_true',help='Explicit non-HA single-host replication defaults')
 parser.add_argument('--osd-memory-mib',type=int,default=4096)
 parser.add_argument('--apply',action='store_true');args=parser.parse_args()
 if os.geteuid()!=0:raise ValueError('Run storage initialization as root')
 if not 2048<=args.osd_memory_mib<=32768:raise ValueError('OSD target must be between 2048 and 32768 MiB')
 record=pathlib.Path('/etc/vmalpha/cluster-node.json');info=record.lstat()
 if record.is_symlink() or info.st_uid!=0 or info.st_mode&0o077:raise ValueError('Node preparation record must be protected and root-owned')
 node=json.loads(record.read_text());config=pathlib.Path('/etc/vmalpha/ceph-initial.conf')
 invocation=command(node,args.single_host,config)
 if pathlib.Path('/etc/ceph/ceph.conf').exists() or (pathlib.Path('/var/lib/ceph').exists() and any(pathlib.Path('/var/lib/ceph').iterdir())):
  raise ValueError('Existing Ceph state found; refusing initialization or reset')
 if not args.apply:print('Storage bootstrap selection validated; no services or OSDs created');return
 subprocess.run(['bash',str(pathlib.Path(__file__).with_name('load-images.sh'))],check=True)
 size=2 if args.single_host else 3;minimum=1 if args.single_host else 2
 descriptor=os.open(config,os.O_CREAT|os.O_EXCL|os.O_WRONLY|os.O_NOFOLLOW,0o600)
 with os.fdopen(descriptor,'w') as output:output.write('[global]\n  osd_pool_default_size = '+str(size)+'\n  osd_pool_default_min_size = '+str(minimum)+'\n[osd]\n  osd_memory_target = '+str(args.osd_memory_mib*1024*1024)+'\n  osd_memory_target_autotune = false\n')
 descriptor=os.open('/var/log/vmalpha-ceph-bootstrap.log',os.O_CREAT|os.O_TRUNC|os.O_WRONLY|os.O_NOFOLLOW,0o600);os.fchmod(descriptor,0o600)
 with os.fdopen(descriptor,'w') as output:result=subprocess.run(invocation,stdout=output,stderr=subprocess.STDOUT,timeout=1800)
 if result.returncode:raise ValueError('Storage bootstrap failed; inspect protected host log. No reset was performed.')
 print('Storage monitor/manager initialized; no OSD disks were selected or consumed. Verify membership and explicitly select new disks next.')

if __name__=='__main__':
 try:main()
 except ValueError as error:raise SystemExit(str(error))
 except (OSError,subprocess.SubprocessError,KeyError,TypeError):raise SystemExit('Storage initialization did not complete; inspect protected host configuration/logs.')
