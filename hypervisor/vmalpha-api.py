#!/usr/bin/python3
"""VM Alpha Host Client privileged command broker. JSON stdin; fixed operations only."""
import pwd
import fcntl
import datetime, ipaddress, json, os, pathlib, re, shutil, subprocess, sys, time, uuid
import xml.etree.ElementTree as ET
os.environ['PATH']='/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
sys.path.insert(0,'/usr/share/vmalpha')
import vmalpha_auth as access
from vmalpha_security import handle as security_handle, details as security_details, OPS as SECURITY_OPS
from vmalpha_datastore import handle as datastore_handle, OPS as DATASTORE_OPS, READ_OPS as DATASTORE_READ
from vmalpha_monitoring import handle as monitoring_handle
from vmalpha_san import handle as san_handle, OPS as SAN_OPS
from vmalpha_rbd import handle as rbd_handle, OPS as RBD_OPS
from vmalpha_storage import handle as storage_handle, OPS as STORAGE_OPS
from vmalpha_containers import handle as container_handle, OPS as CONTAINER_OPS, READ_OPS as CONTAINER_READ
ROOT=pathlib.Path('/var/lib/libvirt/images')
STATE=pathlib.Path('/var/lib/vmalpha')
SERVICES={'sshd','chronyd','firewalld','cockpit.socket','virtqemud.socket','virtnetworkd.socket','virtstoraged.socket'}
def run(*args,timeout=30,check=True,input=None):
 p=subprocess.run(args,input=input,text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=timeout,env={**os.environ,'LC_ALL':'C'})
 if check and p.returncode: raise ValueError(p.stderr.strip() or p.stdout.strip() or 'Operation failed')
 return p.stdout.strip()
def name(v):
 if not isinstance(v,str) or not re.fullmatch(r'[A-Za-z][A-Za-z0-9_.-]{0,62}',v): raise ValueError('Use a name starting with a letter, followed by letters, numbers, dot, dash or underscore (max 63).')
 return v
def number(v,lo,hi):
 v=int(v)
 if not lo<=v<=hi: raise ValueError(f'Value must be between {lo} and {hi}')
 return v
def path(v,exists=True):
 p=pathlib.Path(v).resolve()
 if not p.is_relative_to(ROOT) or p==ROOT: raise ValueError('Select a file inside the guest datastore.')
 if exists and not p.exists(): raise ValueError('File does not exist')
 return p
def xml(cmd,n): return ET.fromstring(run('virsh',cmd,n))
def info(cmd,n):
 return {a.strip():b.strip() for line in run('virsh',cmd,n).splitlines() if ':' in line for a,b in [line.split(':',1)]}
def textfile(p,default=''):
 try:return pathlib.Path(p).read_text().strip()
 except OSError:return default
def vms():
 result=[]
 for n in run('virsh','list','--all','--name').splitlines():
  x=xml('dumpxml',n); i=info('dominfo',n)
  result.append(dict(name=n,uuid=x.findtext('uuid'),family=x.findtext('metadata/{urn:vmalpha:guest}family','Not reported'),state=i.get('State'),cpu=int(x.findtext('vcpu','0')),memory=int(x.findtext('memory','0'))//1024,autostart=i.get('Autostart')=='enable',disks=[dict(target=d.find('target').get('dev') if d.find('target') is not None else '',path=(d.find('source').get('file') or d.find('source').get('dev') or d.find('source').get('name') or ((d.find('source').get('pool','')+'/'+d.find('source').get('volume','')) if d.find('source').get('pool') else '')) if d.find('source') is not None else '',device=d.get('device')) for d in x.findall('devices/disk')],networks=[dict(mac=d.find('mac').get('address'),network=d.find('source').get('network','') if d.find('source') is not None else '') for d in x.findall('devices/interface')],firmware='UEFI' if x.find('os/loader') is not None else 'BIOS'))
 return result
def pools():
 result=[]
 for n in run('virsh','pool-list','--all','--name').splitlines():
  x=xml('pool-dumpxml',n);i=info('pool-info',n)
  result.append(dict(name=n,path=x.findtext('target/path',''),location=x.findtext('target/path','') or x.findtext('source/name',''),type=x.get('type'),state=i.get('State'),capacity=i.get('Capacity'),available=i.get('Available'),autostart=i.get('Autostart')))
 return result
def networks():
 result=[]
 for n in run('virsh','net-list','--all','--name').splitlines():
  x=xml('net-dumpxml',n);i=info('net-info',n);a=x.find('ip');f=x.find('forward');b=x.find('bridge')
  result.append(dict(name=n,active=i.get('Active'),autostart=i.get('Autostart'),mode=f.get('mode') if f is not None else 'isolated',bridge=b.get('name') if b is not None else '',address=a.attrib if a is not None else {},leases=run('virsh','net-dhcp-leases',n,check=False)))
 return result
def cpu_usage():
 def sample():
  v=list(map(int,textfile('/proc/stat').splitlines()[0].split()[1:9]));return sum(v),v[3]+v[4]
 total,idle=sample();time.sleep(0.15);end,rest=sample();return max(0,min(100,100*(1-(rest-idle)/max(1,end-total))))
def inventory():
 mem={k:int(v.split()[0])*1024 for k,v in [line.split(':',1) for line in textfile('/proc/meminfo').splitlines()]}
 disk=shutil.disk_usage(ROOT); ls=json.loads(run('lsblk','-J','-b','-o','NAME,TYPE,SIZE,FSTYPE,MOUNTPOINT,MODEL'))
 return dict(physicalInterfaces=[p.name for p in pathlib.Path('/sys/class/net').iterdir() if (p/'device').exists()],cpuUsage=cpu_usage(),firmware=['BIOS']+(['UEFI'] if pathlib.Path('/usr/share/edk2/ovmf/OVMF_CODE.fd').is_file() else []),hostname=run('hostname'),version='1.0',os='VM Alpha Linux 1.0',kernel=run('uname','-r'),uptime=float(textfile('/proc/uptime').split()[0]),cpu=os.cpu_count(),cpuModel=next((s.split(':',1)[1].strip() for s in textfile('/proc/cpuinfo').splitlines() if s.startswith('model name')),'Unknown'),load=os.getloadavg()[0],memory=dict(total=mem['MemTotal'],used=mem['MemTotal']-mem['MemAvailable']),storage=dict(total=disk.total,used=disk.used),kvm=os.path.exists('/dev/kvm'),vms=vms(),pools=pools(),networks=networks(),interfaces=json.loads(run('ip','-j','address')),routes=json.loads(run('ip','-j','route')),devices=ls['blockdevices'],manufacturer=textfile('/sys/class/dmi/id/sys_vendor','Unknown'),model=textfile('/sys/class/dmi/id/product_name','Unknown'),maintenance=(STATE/'maintenance').exists())
def security():
 users=[]
 for line in run('getent','passwd').splitlines():
  parts=line.split(':');u,_,uid,gid,gecos,home,shell=parts
  if (1000<=int(uid)<60000 and shell not in ('/sbin/nologin','/usr/sbin/nologin','/bin/false')) or u=='root':
   groups=run('id','-nG',u).split();status=run('passwd','-S',u,check=False).split()
   users.append(dict(name=u,role='Administrator' if 'wheel' in groups or u=='root' else ('VM-scoped user' if 'vmalpha' in groups else 'OS user (no host management)'),locked=len(status)>1 and status[1] in ('LK','L'),shell=shell))
 return dict(**security_details(run),users=users,selinux=run('getenforce'),firewall=run('firewall-cmd','--state',check=False),rules=run('firewall-cmd','--list-all',check=False),ssh=run('systemctl','is-active','sshd',check=False),services=[dict(name=s,state=run('systemctl','is-active',s,check=False),enabled=run('systemctl','is-enabled',s,check=False)) for s in sorted(SERVICES)],banner=textfile('/etc/issue'),lockout=textfile('/etc/security/faillock.conf'),certificates=run('sh','-c','for f in /etc/cockpit/ws-certs.d/*.cert; do test ! -f "$f" || openssl x509 -in "$f" -noout -subject -issuer -dates -fingerprint -sha256; done',check=False),secureboot=run('mokutil','--sb-state',check=False) if shutil.which('mokutil') else 'Not available',ntp=run('timedatectl','show','-p','NTPSynchronized','-p','NTP',check=False))
def tasks():
 try:return json.loads((STATE/'tasks.json').read_text())[-50:]
 except (OSError,ValueError):return []
def audit(op,status,detail='',target='Host',started=None):
 # Never record request bodies, passwords, certificate material or command output.
 row=dict(time=datetime.datetime.now(datetime.timezone.utc).isoformat(),started=started,target=target,operation=op,status=status,user=os.environ.get('SUDO_USER','root'),detail=detail[:120])
 STATE.mkdir(exist_ok=True); f=STATE/'tasks.json'; old=tasks();old.append(row)
 tmp=STATE/('tasks-'+str(uuid.uuid4())+'.tmp');tmp.write_text(json.dumps(old[-100:]));tmp.chmod(0o600);tmp.replace(f)
 run('logger','-t','vmalpha-hostclient',f'{row["user"]} {op} {status}',check=False)
def ceph_telemetry(args):
 if args == {'query':'storage registration'}:
  import stat
  p=pathlib.Path('/var/lib/vmalpha/storage-registration.json')
  st=p.lstat()
  if not stat.S_ISREG(st.st_mode) or st.st_uid!=0 or stat.S_IMODE(st.st_mode)!=0o600 or st.st_size>65536:raise ValueError('Storage manifest unavailable')
  data=json.loads(p.read_text())
  allowed={'version','status','verified_at','fsid','monitors','placement_nodes','rbd_pool','cephfs','nfs','checks','host_access'}
  if not isinstance(data,dict) or set(data)-allowed:raise ValueError('Invalid storage manifest')
  return data
 commands={'status':['status'], 'df':['df'], 'osd perf':['osd','perf'], 'orch host ls':['orch','host','ls'], 'osd tree':['osd','tree']}
 query=args.get('query')
 if set(args)!={'query'} or query not in commands:raise ValueError('Unsupported Ceph telemetry query')
 command=commands[query]+['--format','json']
 binary=shutil.which('ceph')
 if binary: argv=[binary]+command
 else:
  binary=shutil.which('cephadm')
  if not binary:raise ValueError('Ceph telemetry unavailable')
  argv=[binary,'shell','--','ceph']+command
 result=json.loads(run(*argv,timeout=12))
 if not isinstance(result,(dict,list)):raise ValueError('Invalid Ceph telemetry')
 return result

def main(q):
 op=q.get('op','');args=q.get('args',{})
 if op=='metrics':return monitoring_handle(args)
 if op in ('role-create','role-delete','permission-assign','permission-remove'):return access.mutate(op,args,run,name)
 if op in SAN_OPS:return san_handle(op,args,run,name)
 if op in RBD_OPS:return rbd_handle(op,args,run,name)
 if op in STORAGE_OPS:return storage_handle(op,args,run,name)
 if op in CONTAINER_OPS:return container_handle(op,args,run)
 if op in SECURITY_OPS:return security_handle(op,args,run,STATE,name,number)
 if op in DATASTORE_OPS:return datastore_handle(op,args,ROOT,STATE,run,vms,pools)
 if op=='ceph.telemetry':return ceph_telemetry(args)
 if op=='inventory':return inventory()
 if op=='security':return security()
 if op=='tasks':return tasks()
 if op=='logs':return run('journalctl','-n','100','--no-pager','-o','short-iso',timeout=10)
 if op=='files':
  p=ROOT if not args.get('path') else path(args['path']);return [dict(name=f.name,path=str(f),size=f.stat().st_size,directory=f.is_dir()) for f in sorted(p.iterdir()) if not f.is_symlink()][:1000]
 if op=='vm-action':
  n=name(args['name']);a=args['action'];cmd={'start':'start','shutdown':'shutdown','reboot':'reboot','suspend':'suspend','resume':'resume','autostart':'autostart'}.get(a)
  if not cmd:raise ValueError('Unsupported VM action')
  if a=='start' and (STATE/'maintenance').exists():raise ValueError('Exit maintenance mode before starting a guest')
  return run('virsh',cmd,n)
 if op=='vm-edit':
  n=name(args['name'])
  if info('dominfo',n).get('State')!='shut off':raise ValueError('Power off the VM before editing CPU and memory')
  cpu=number(args['cpu'],1,64);ram=number(args['memory'],256,131072)
  x=xml('dumpxml',n);x.find('vcpu').text=str(cpu)
  for tag in ('memory','currentMemory'):
   el=x.find(tag)
   if el is None:el=ET.SubElement(x,tag)
   el.set('unit','MiB');el.text=str(ram)
  return run('virsh','define','/dev/stdin',input=ET.tostring(x,encoding='unicode'))
 if op=='vm-attach-disk':
  n=name(args['name']);p=path(args['path']);target=name(args['target'])
  if not re.fullmatch(r'vd[b-z]',target):raise ValueError('Select a disk target from vdb to vdz')
  if any(d['path']==str(p) for v in vms() for d in v['disks']):raise ValueError('Disk is already attached to a guest')
  cmd=['virsh','attach-disk',n,str(p),target,'--driver','qemu','--subdriver','qcow2','--targetbus','virtio','--config']
  if info('dominfo',n).get('State')=='running':cmd.append('--live')
  return run(*cmd)
 if op=='vm-unregister':
  n=name(args['name'])
  if info('dominfo',n).get('State')!='shut off':raise ValueError('Power off the VM before unregistering')
  folder=STATE/'unregistered';folder.mkdir(mode=0o700,exist_ok=True)
  x=run('virsh','dumpxml',n);(folder/(n+'.xml')).write_text(x)
  return run('virsh','undefine',n,'--keep-nvram')
 if op=='registrations':
  folder=STATE/'unregistered'
  return [p.stem for p in sorted(folder.glob('*.xml'))] if folder.exists() else []
 if op=='vm-register':
  n=name(args['name']);p=STATE/'unregistered'/(n+'.xml')
  if not p.is_file():raise ValueError('Saved VM definition not found')
  if n in [v['name'] for v in vms()]:raise ValueError('A VM with that name is already registered')
  return run('virsh','define',str(p))
 if op=='vm-create':
  if (STATE/'maintenance').exists():raise ValueError('Exit maintenance mode before creating a guest')
  firmware=args.get('firmware','BIOS')
  if firmware not in ('BIOS','UEFI'):raise ValueError('Unsupported firmware')
  if firmware=='UEFI' and not pathlib.Path('/usr/share/edk2/ovmf/OVMF_CODE.fd').is_file():raise ValueError('UEFI firmware is unavailable')
  n=name(args['name']);cpu=number(args['cpu'],1,64);ram=number(args['memory'],256,131072);mode=args['mode'];net=name(args['network']);pool=name(args['pool']);x=xml('pool-dumpxml',pool);base=pathlib.Path(x.findtext('target/path')).resolve()
  if not base.is_relative_to(ROOT):raise ValueError('Use a local guest datastore')
  disk=base/(n+'.qcow2')
  if disk.exists() or n in [v['name'] for v in vms()]:raise ValueError('VM name or disk already exists')
  if args.get('iso'):path(args['iso'])
  if mode=='import':path(args['image'])
  if net not in [n['name'] for n in networks()]:raise ValueError('Guest network does not exist')
  # Build XML through ElementTree, never string interpolation of user input.
  if mode=='import':
   source=path(args['image']);run('qemu-img','convert','-O','qcow2',str(source),str(disk),timeout=180)
  elif mode=='new':run('qemu-img','create','-f','qcow2',str(disk),str(number(args['size'],1,2048))+'G')
  else:raise ValueError('Supported creation types are new or QCOW2/raw import')
  run('restorecon',str(disk));os.chmod(disk,0o660);shutil.chown(disk,user='qemu',group='qemu')
  d=ET.Element('domain',type='kvm');ET.SubElement(d,'name').text=n;ET.SubElement(d,'memory',unit='MiB').text=str(ram);ET.SubElement(d,'vcpu').text=str(cpu)
  ET.SubElement(ET.SubElement(d,'metadata'),'{urn:vmalpha:guest}family').text=args.get('os','Other') if args.get('os','Other') in ('Linux','Other') else 'Other'
  osx=ET.SubElement(d,'os');
  firmware=args.get('firmware','BIOS')
  if firmware not in ('BIOS','UEFI'):raise ValueError('Unsupported firmware')
  if firmware=='UEFI':
   if not pathlib.Path('/usr/share/edk2/ovmf/OVMF_CODE.fd').is_file():raise ValueError('UEFI firmware is unavailable')
   osx.set('firmware','efi');ET.SubElement(ET.SubElement(osx,'firmware'),'feature',name='secure-boot',enabled='no')
  ET.SubElement(osx,'type',arch='x86_64',machine='q35').text='hvm';ET.SubElement(osx,'boot',dev='cdrom' if mode=='new' else 'hd');ET.SubElement(osx,'boot',dev='hd') if mode=='new' else None
  feat=ET.SubElement(d,'features');ET.SubElement(feat,'acpi');ET.SubElement(feat,'apic');ET.SubElement(d,'cpu',mode='host-passthrough')
  dev=ET.SubElement(d,'devices');di=ET.SubElement(dev,'disk',type='file',device='disk');ET.SubElement(di,'driver',name='qemu',type='qcow2');ET.SubElement(di,'source',file=str(disk));ET.SubElement(di,'target',dev='vda',bus='virtio')
  if args.get('iso'):
   iso=path(args['iso']);di=ET.SubElement(dev,'disk',type='file',device='cdrom');ET.SubElement(di,'driver',name='qemu',type='raw');ET.SubElement(di,'source',file=str(iso));ET.SubElement(di,'target',dev='sda',bus='sata');ET.SubElement(di,'readonly')
  ni=ET.SubElement(dev,'interface',type='network');ET.SubElement(ni,'source',network=net);ET.SubElement(ni,'model',type='virtio')
  serial=ET.SubElement(dev,'serial',type='pty');ET.SubElement(serial,'target',port='0');con=ET.SubElement(dev,'console',type='pty');ET.SubElement(con,'target',type='serial',port='0')
  ET.SubElement(ET.SubElement(dev,'graphics',type='vnc'), 'listen', type='socket');video=ET.SubElement(dev,'video');ET.SubElement(video,'model',type='vga');ET.SubElement(dev,'memballoon',model='virtio')
  try:return run('virsh','define','/dev/stdin',input=ET.tostring(d,encoding='unicode'))
  except Exception:
   disk.unlink(missing_ok=True);raise
 if op=='pool-create':
  n=name(args['name']);p=ROOT/n
  if p.exists():raise ValueError('Directory already exists')
  p.mkdir(mode=0o755);run('restorecon',str(p));run('virsh','pool-define-as',n,'dir','--target',str(p));run('virsh','pool-start',n);return run('virsh','pool-autostart',n)
 if op=='volume-create':return run('virsh','vol-create-as',name(args['pool']),name(args['name'])+'.qcow2',str(number(args['size'],1,2048))+'G','--allocation','0','--format','qcow2',timeout=180)
 if op=='network-create':
  n=name(args['name']);sub=ipaddress.ip_network(args['subnet'],strict=True)
  if sub.version!=4 or sub.prefixlen!=24 or not sub.is_private:raise ValueError('Choose a private IPv4 /24 network')
  for route in json.loads(run('ip','-j','route')):
   if route.get('dst','default')!='default' and sub.overlaps(ipaddress.ip_network(route['dst'])):raise ValueError('Subnet overlaps an existing host route')
  root=ET.Element('network');ET.SubElement(root,'name').text=n
  if args.get('mode')=='nat':ET.SubElement(root,'forward',mode='nat')
  elif args.get('mode')!='isolated':raise ValueError('Choose NAT or isolated')
  ip=ET.SubElement(root,'ip',address=str(sub[1]),netmask='255.255.255.0');dh=ET.SubElement(ip,'dhcp');ET.SubElement(dh,'range',start=str(sub[100]),end=str(sub[200]))
  run('virsh','net-define','/dev/stdin',input=ET.tostring(root,encoding='unicode'));run('virsh','net-start',n);return run('virsh','net-autostart',n)
 if op=='network-action':
  n=name(args['name']);a=args['action']
  if a not in ('start','autostart'):raise ValueError('Unsupported network action')
  return run('virsh','net-'+a,n)
 if op=='service':
  s=args['name'];a=args['action']
  if s not in {'sshd','chronyd'} or a not in {'start','stop','restart'}:raise ValueError('Only SSH and time-service lifecycle changes are available')
  return run('systemctl',a,s)
 if op=='banner':
  value=args['text']
  if not isinstance(value,str) or len(value)>2000 or '\x00' in value:raise ValueError('Banner must be at most 2000 characters')
  pathlib.Path('/etc/issue').write_text(value+'\n');return 'Login banner saved'
 if op=='lockout':
  deny=number(args['deny'],3,10);unlock=number(args['unlock'],60,3600)
  # authselect's pam_faillock feature is mandatory for this control to claim enforcement.
  run('authselect','enable-feature','with-faillock');pathlib.Path('/etc/security/faillock.conf').write_text(f'deny = {deny}\nunlock_time = {unlock}\nfail_interval = 900\n')
  return 'PAM account lockout policy enabled'
 if op=='user-create':
  n=name(args['name']);password=args['password']
  if not isinstance(password,str) or len(password)<1 or len(password)>512 or any(c in password for c in '\r\n\x00'):raise ValueError('Invalid password')
  if args['role'] not in ('Administrator','OS user'):raise ValueError('Unsupported role')
  if run('getent','passwd',n,check=False):raise ValueError('User already exists')
  run('useradd','-m',n)
  try:run('passwd','--stdin',n,input=password+'\n')
  except Exception:
   run('usermod','-L',n,check=False);raise
  if args['role']=='Administrator':run('usermod','-aG','wheel',n)
  return 'User created'
 if op=='firewall':
  s=args['service']
  if s not in ('http','https','dns','ntp'):raise ValueError('Unsupported service rule')
  flag='--add-service=' if args.get('enabled') else '--remove-service='
  run('firewall-cmd','--permanent',flag+s);return run('firewall-cmd',flag+s)
 if op=='user-lock':
  n=name(args['name'])
  if n in ('root','admin',os.environ.get('SUDO_USER')):raise ValueError('Primary and current administrator accounts cannot be locked here')
  target=pwd.getpwnam(n)
  if not 1000<=target.pw_uid<60000 or target.pw_shell in ('/sbin/nologin','/usr/sbin/nologin','/bin/false'):raise ValueError('Only interactive local users can be managed here')
  return run('usermod','-L' if args.get('locked') else '-U',n)
 if op=='maintenance':
  if args['enabled'] and any(v['state']=='running' for v in vms()):raise ValueError('Shut down guests before entering maintenance mode')
  if args['enabled']:(STATE/'maintenance').touch()
  else:(STATE/'maintenance').unlink(missing_ok=True)
  return 'Maintenance mode updated (Host Client VM-start gate; not vCenter lockdown)'
 if op=='host-power':
  if args['action'] not in ('reboot','poweroff'):raise ValueError('Unsupported power action')
  if any(v['state']=='running' for v in vms()):raise ValueError('Shut down guests before changing host power')
  return run('systemctl',args['action'])
 raise ValueError('Unsupported operation')
if __name__=='__main__':
 q={}
 started=datetime.datetime.now(datetime.timezone.utc).isoformat()
 try:
  raw=sys.stdin.read(8*1024*1024+1)
  if len(raw)>8*1024*1024:raise ValueError('Request too large')
  q=json.loads(raw)
  if not isinstance(q,dict):raise ValueError('Invalid request')
  if os.geteuid()!=0:raise ValueError('Administrative access is required')
  if q.get('op') not in ('ceph.telemetry','inventory','security','tasks','logs','files','registrations',*CONTAINER_READ,'storage-discover','metrics',*DATASTORE_READ):
   STATE.mkdir(exist_ok=True);lock=open(STATE/'operations.lock','a');fcntl.flock(lock,fcntl.LOCK_EX)
  access.authorize(q)
  result=access.filter_result(q.get('op'),main(q))
  if q.get('op') not in ('ceph.telemetry','inventory','security','tasks','logs','files','registrations',*CONTAINER_READ,'storage-discover','metrics',*DATASTORE_READ,'ds-upload-chunk'):audit(q['op'],'Completed',target=str(q.get('args',{}).get('name') or pathlib.Path(q.get('args',{}).get('path','Host')).name)[:120],started=started)
  print(json.dumps(dict(ok=True,result=result)))
 except Exception as e:
  if q.get('op') not in ('ceph.telemetry','inventory','security','tasks','logs','files','registrations',*CONTAINER_READ,'storage-discover','metrics',*DATASTORE_READ,'ds-upload-chunk'):audit(str(q.get('op','invalid')),'Failed',started=started)
  print(json.dumps(dict(ok=False,error=str(e))))
  sys.exit(1)
