"""VM Alpha Storage discovery and managed network filesystem pools."""
import ipaddress,json,pathlib,re,tempfile,xml.etree.ElementTree as ET
OPS={'storage-discover','storage-nfs-create','storage-disconnect','storage-fc-rescan','storage-iscsi-discover'}
def host(value):
 if not isinstance(value,str) or len(value)>253:raise ValueError('Invalid storage server')
 try:ipaddress.ip_address(value);return value
 except ValueError:pass
 if not re.fullmatch(r'[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?',value):raise ValueError('Invalid storage server')
 return value

def handle(op,a,run,name):
 if op=='storage-discover':
  adapters=[]
  for p in pathlib.Path('/sys/class/fc_host').glob('host*'):
   row={'name':p.name}
   for k in ('port_name','node_name','port_state','speed','supported_speeds','symbolic_name'):
    try:row[k]=(p/k).read_text().strip()
    except OSError:row[k]='Unavailable'
   adapters.append(row)
  return {'fibreChannel':adapters,'iscsi':run('iscsiadm','-m','session',check=False),'multipath':run('multipath','-ll',check=False),'devices':json.loads(run('lsblk','-J','-b','-o','NAME,PATH,TYPE,SIZE,FSTYPE,MOUNTPOINTS,WWN,SERIAL'))['blockdevices']}
 if op=='storage-fc-rescan':
  n=a['adapter']
  if not re.fullmatch(r'host[0-9]+',n) or not (pathlib.Path('/sys/class/fc_host')/n).exists():raise ValueError('Select a detected Fibre Channel adapter')
  (pathlib.Path('/sys/class/scsi_host')/n/'scan').write_text('- - -')
  return 'Fibre Channel discovery requested; no LUN was formatted'
 if op=='storage-iscsi-discover':
  server=host(a['server']);port=int(a.get('port',3260))
  if not 1<=port<=65535:raise ValueError('Invalid TCP port')
  portal=('['+server+']' if ':' in server else server)+':'+str(port)
  # Nonpersistent discovery does not create login/startup records.
  return run('iscsiadm','-m','discovery','-t','sendtargets','-p',portal,'-o','nonpersistent',timeout=60)
 n=name(a['name'])
 if op=='storage-nfs-create':
  server=host(a['server']);export=a['export'];version=str(a.get('version','4'))
  if not isinstance(export,str) or not export.startswith('/') or len(export)>1024 or any(c in export for c in '\x00\n\r'):raise ValueError('Use an absolute NFS export path')
  if version not in ('3','4','4.1','4.2'):raise ValueError('Choose NFS 3, 4, 4.1 or 4.2')
  existing=run('virsh','pool-list','--all','--name').splitlines()
  if n in existing:raise ValueError('Datastore already exists')
  target=pathlib.Path('/var/lib/libvirt/images')/n
  if target.exists():raise ValueError('Datastore mount directory already exists')
  x=ET.Element('pool',{'type':'netfs'});ET.SubElement(x,'name').text=n
  src=ET.SubElement(x,'source');ET.SubElement(src,'host',{'name':server});ET.SubElement(src,'dir',{'path':export});ET.SubElement(src,'format',{'type':'nfs'});ET.SubElement(src,'protocol',{'ver':version})
  ET.SubElement(ET.SubElement(x,'target'),'path').text=str(target)
  with tempfile.NamedTemporaryFile(mode='w',suffix='.xml') as f:
   f.write(ET.tostring(x,encoding='unicode'));f.flush();run('virsh','pool-define',f.name)
  try:
   target.mkdir(mode=0o755);run('virsh','pool-start',n,timeout=60);run('virsh','pool-autostart',n)
  except Exception:
   # Keep the defined pool for inspect/retry; do not remove a potentially mounted path.
   raise ValueError('Datastore defined but activation failed; inspect server export, access and connectivity')
  return 'VM Alpha Storage NFS datastore connected'
 if op=='storage-disconnect':
  x=ET.fromstring(run('virsh','pool-dumpxml',n));target=x.findtext('target/path')
  if x.get('type')!='netfs':raise ValueError('This disconnect operation only accepts NFS datastores')
  if not target:raise ValueError('Datastore target unavailable')
  root=pathlib.Path(target).resolve()
  for vm in run('virsh','list','--all','--name').splitlines():
   dom=ET.fromstring(run('virsh','dumpxml',vm))
   for src in dom.findall('devices/disk/source'):
    if src.get('pool')==n or any(pathlib.Path(src.get(k)).resolve().is_relative_to(root) for k in ('file','dev') if src.get(k)):
     raise ValueError('Datastore is referenced by virtual machine '+vm)
  run('virsh','pool-autostart',n,'--disable');run('virsh','pool-destroy',n)
  return 'NFS datastore disconnected; definition and remote data retained'
 raise ValueError('Unsupported storage operation')
