"""Authenticated libvirt RBD pools and real network-backed VM disks."""
import base64,copy,re,tempfile,uuid,xml.etree.ElementTree as ET
from vmalpha_storage import host
OPS={'storage-rbd-connect','storage-rbd-volumes','storage-rbd-create','storage-rbd-attach'}
def handle(op,a,run,name):
 pool=name(a['pool'])
 if op=='storage-rbd-connect':
  if pool in run('virsh','pool-list','--all','--name').splitlines():raise ValueError('Datastore already exists')
  remote=name(a['remotePool']);user=name(a['username']);key=a['key'].strip()
  try:raw=base64.b64decode(key,validate=True)
  except Exception:raise ValueError('Invalid Ceph key encoding')
  if not 16<=len(raw)<=256:raise ValueError('Invalid Ceph key length')
  monitors=a['monitors'].split(',')
  if not 1<=len(monitors)<=16:raise ValueError('Provide 1 to 16 monitor addresses')
  x=ET.Element('pool',{'type':'rbd'});ET.SubElement(x,'name').text=pool;src=ET.SubElement(x,'source');ET.SubElement(src,'name').text=remote
  for address in monitors:
   address=address.strip();port=6789
   if address.startswith('['):
    m=re.fullmatch(r'\[([^]]+)\](?::([0-9]+))?',address)
    if not m:raise ValueError('Invalid monitor address')
    server=m[1];port=int(m[2] or 6789)
   elif address.count(':')==1:server,p=address.rsplit(':',1);port=int(p)
   else:server=address
   if not 1<=port<=65535:raise ValueError('Invalid monitor port')
   ET.SubElement(src,'host',name=host(server),port=str(port))
  uid=str(uuid.uuid4());secret=ET.Element('secret',ephemeral='no',private='yes');ET.SubElement(secret,'uuid').text=uid;usage=ET.SubElement(secret,'usage',type='ceph');ET.SubElement(usage,'name').text='vmalpha-'+pool
  auth=ET.SubElement(src,'auth',type='ceph',username=user);ET.SubElement(auth,'secret',uuid=uid)
  with tempfile.NamedTemporaryFile(mode='w') as f:
   f.write(ET.tostring(secret,encoding='unicode'));f.flush();run('virsh','secret-define',f.name)
  defined=False
  try:
   with tempfile.NamedTemporaryFile(mode='w') as f:
    f.write(key);f.flush();run('virsh','secret-set-value',uid,'--file',f.name)
   with tempfile.NamedTemporaryFile(mode='w') as f:
    f.write(ET.tostring(x,encoding='unicode'));f.flush();run('virsh','pool-define',f.name);defined=True
   run('virsh','pool-start',pool,timeout=60);run('virsh','pool-autostart',pool)
  except Exception:
   if not defined:run('virsh','secret-undefine',uid,check=False)
   raise ValueError('RBD connection failed; inspect monitor connectivity, pool and Ceph permissions. No secret value is displayed.')
  return 'VM Alpha Storage RBD datastore connected'
 x=ET.fromstring(run('virsh','pool-dumpxml',pool))
 if x.get('type')!='rbd':raise ValueError('Select a Ceph RBD datastore')
 if op=='storage-rbd-volumes':return run('virsh','vol-list',pool,'--details')
 volume=name(a['volume'])
 if op=='storage-rbd-create':
  size=int(a['size'])
  if not 1<=size<=10240:raise ValueError('Size must be between 1 and 10240 GiB')
  return run('virsh','vol-create-as',pool,volume,str(size)+'G','--format','raw',timeout=60)
 if op=='storage-rbd-attach':
  vm=name(a['name']);target=a['target']
  if not re.fullmatch(r'vd[b-z]',target):raise ValueError('Use a disk target from vdb to vdz')
  if run('virsh','domstate',vm)!='shut off':raise ValueError('Shut down the VM before attaching an RBD disk')
  run('virsh','vol-info','--pool',pool,volume)
  remote=x.findtext('source/name')+'/'+volume
  for other in run('virsh','list','--all','--name').splitlines():
   dom=ET.fromstring(run('virsh','dumpxml',other))
   for d in dom.findall('devices/disk'):
    src=d.find('source');t=d.find('target')
    if other==vm and t is not None and t.get('dev')==target:raise ValueError('Disk target already in use')
    if src is not None and src.get('protocol')=='rbd' and src.get('name')==remote:raise ValueError('RBD image already referenced by a VM')
  d=ET.Element('disk',type='network',device='disk');ET.SubElement(d,'driver',name='qemu',type='raw',cache='none')
  auth=copy.deepcopy(x.find('source/auth'))
  if auth is None:raise ValueError('Authenticated RBD pool is required')
  auth.attrib.pop('type',None);auth.find('secret').set('type','ceph');d.append(auth)
  src=ET.SubElement(d,'source',protocol='rbd',name=remote)
  for h in x.findall('source/host'):src.append(copy.deepcopy(h))
  ET.SubElement(d,'target',dev=target,bus='virtio')
  with tempfile.NamedTemporaryFile(mode='w') as f:
   f.write(ET.tostring(d,encoding='unicode'));f.flush();return run('virsh','attach-device',vm,f.name,'--config')
 raise ValueError('Unsupported RBD operation')
