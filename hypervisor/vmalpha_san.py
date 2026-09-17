"""VM Alpha Storage SAN pools; explicit targets, no automatic formatting."""
import base64,pathlib,re,tempfile,uuid,xml.etree.ElementTree as ET
from vmalpha_storage import host
OPS={'storage-iscsi-connect','storage-fc-connect','storage-san-volumes','storage-san-disconnect','storage-san-attach'}
def define(x,run,command='pool-define'):
 with tempfile.NamedTemporaryFile(mode='w') as f:
  f.write(ET.tostring(x,encoding='unicode'));f.flush();return run('virsh',command,f.name)
def paths(pool,run):
 # virsh vol-list has no --name option. SAN volume paths are absolute devices.
 result=set()
 for line in run('virsh','vol-list',pool).splitlines():
  fields=line.strip().split(None,1)
  if len(fields)==2 and fields[1].startswith('/'):
   result.add(str(pathlib.Path(fields[1]).resolve()))
 return result
def idle(devices,run):
 for device in devices:
  p=pathlib.Path(device)
  if not p.is_block_device():raise ValueError('Storage device is unavailable')
  if run('lsblk','-n','-o','MOUNTPOINTS',device).strip():raise ValueError('LUN has a host mount or active swap')
  if list((pathlib.Path('/sys/class/block')/p.name/'holders').glob('*')):raise ValueError('LUN has active device-mapper or multipath consumers; detach those consumers first')
  if run('fuser',device,check=False).strip():raise ValueError('LUN is open by a host process')
 for vm in run('virsh','list','--all','--name').splitlines():
  d=ET.fromstring(run('virsh','dumpxml',vm))
  for src in d.findall('devices/disk/source'):
   if src.get('pool') and src.get('volume'):
    device=run('virsh','vol-path','--pool',src.get('pool'),src.get('volume'))
    if str(pathlib.Path(device).resolve()) in devices:raise ValueError('LUN is referenced by virtual machine '+vm)
   for k in ('dev','file'):
    if src.get(k) and str(pathlib.Path(src.get(k)).resolve()) in devices:raise ValueError('LUN is referenced by virtual machine '+vm)
def handle(op,a,run,name):
 pool=name(a['pool'])
 if op in ('storage-iscsi-connect','storage-fc-connect'):
  if pool in run('virsh','pool-list','--all','--name').splitlines():raise ValueError('Datastore name already exists')
  kind='iscsi' if op=='storage-iscsi-connect' else 'scsi'
  x=ET.Element('pool',type=kind);ET.SubElement(x,'name').text=pool;src=ET.SubElement(x,'source');uid=None
  if kind=='iscsi':
   # The distro unit initializes a unique initiator IQN on first use.
   run('systemctl','start','iscsid.service')
   server=host(a['server']);port=int(a.get('port',3260));target=a['iqn']
   if not 1<=port<=65535:raise ValueError('Invalid iSCSI port')
   if not isinstance(target,str) or len(target)>223 or not re.fullmatch(r'(?:iqn\.|eui\.|naa\.)[A-Za-z0-9.:-]+',target):raise ValueError('Select a discovered iSCSI target IQN')
   ET.SubElement(src,'host',name=server,port=str(port));ET.SubElement(src,'device',path=target)
   if a.get('username'):
    username=a['username'];password=a.get('password','')
    if not isinstance(username,str) or len(username)>255 or any(c in username for c in '\n\r\x00'):raise ValueError('Invalid CHAP username')
    if not isinstance(password,str) or not 12<=len(password)<=16:raise ValueError('CHAP secret must be 12–16 characters')
    uid=str(uuid.uuid4());secret=ET.Element('secret',ephemeral='no',private='yes');ET.SubElement(secret,'uuid').text=uid;ET.SubElement(ET.SubElement(secret,'usage',type='iscsi'),'target').text='vmalpha-'+pool
    define(secret,run,'secret-define')
    try:
     with tempfile.NamedTemporaryFile(mode='w') as f:
      f.write(base64.b64encode(password.encode()).decode());f.flush();run('virsh','secret-set-value',uid,'--file',f.name)
    except Exception:
     run('virsh','secret-undefine',uid,check=False);raise ValueError('Could not store CHAP credential')
    ET.SubElement(ET.SubElement(src,'auth',type='chap',username=username),'secret',uuid=uid)
  else:
   adapter=a['adapter']
   if not re.fullmatch(r'host[0-9]+',adapter) or not (pathlib.Path('/sys/class/fc_host')/adapter).exists():raise ValueError('Select a detected Fibre Channel adapter')
   ET.SubElement(src,'adapter',name=adapter)
  ET.SubElement(ET.SubElement(x,'target'),'path').text='/dev/disk/by-path'
  try:define(x,run)
  except Exception:
   if uid:run('virsh','secret-undefine',uid,check=False)
   raise
  try:run('virsh','pool-start',pool,timeout=60);run('virsh','pool-autostart',pool)
  except Exception:raise ValueError('SAN pool defined but connection failed; inspect target access and connectivity')
  return 'SAN datastore connected; no LUN was formatted'
 x=ET.fromstring(run('virsh','pool-dumpxml',pool))
 if x.get('type') not in ('iscsi','scsi'):raise ValueError('Select an iSCSI or Fibre Channel datastore')
 if op=='storage-san-volumes':return run('virsh','vol-list',pool,'--details')
 if op=='storage-san-disconnect':
  idle(paths(pool,run),run);run('virsh','pool-autostart',pool,'--disable');run('virsh','pool-destroy',pool)
  return 'SAN datastore disconnected; definition retained and LUN data unchanged'
 if op=='storage-san-attach':
  volume=a['volume'];vm=name(a['name']);target=a['target']
  if not isinstance(volume,str) or not volume or len(volume)>255 or '\x00' in volume:raise ValueError('Invalid LUN selection')
  device=run('virsh','vol-path','--pool',pool,volume);canonical=str(pathlib.Path(device).resolve())
  if canonical not in paths(pool,run):raise ValueError('Select a discovered LUN')
  if run('virsh','domstate',vm)!='shut off':raise ValueError('Shut down the VM before adding a SAN LUN')
  if not re.fullmatch(r'vd[b-z]',target):raise ValueError('Use disk target vdb through vdz')
  d=ET.fromstring(run('virsh','dumpxml',vm))
  if any(t.get('dev')==target for t in d.findall('devices/disk/target')):raise ValueError('Disk target is already used')
  idle({canonical},run)
  disk=ET.Element('disk',type='block',device='disk');ET.SubElement(disk,'driver',name='qemu',type='raw',cache='none');ET.SubElement(disk,'source',dev=device);ET.SubElement(disk,'target',dev=target,bus='virtio')
  with tempfile.NamedTemporaryFile(mode='w') as f:
   f.write(ET.tostring(disk,encoding='unicode'));f.flush();return run('virsh','attach-device',vm,f.name,'--config')
 raise ValueError('Unsupported SAN operation')
