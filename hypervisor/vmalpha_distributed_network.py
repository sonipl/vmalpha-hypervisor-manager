"""Reviewed NetworkManager-only distributed bridges. No discovery-time mutation."""
import hashlib,json,os,pathlib,re,socket,tempfile,xml.etree.ElementTree as ET
OPS={'network.distributed.discover','network.distributed.preview','network.distributed.apply'}
READ_OPS=OPS-{'network.distributed.apply'}
STATE=pathlib.Path('/var/lib/vmalpha/distributed-networks')
IFACE=re.compile(r'^[a-zA-Z][a-zA-Z0-9_-]{0,14}$')
DISPLAY=re.compile(r'^[a-zA-Z0-9][a-zA-Z0-9 _.-]{0,62}$')
IDENT=re.compile(r'^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$')
def digest(v):return hashlib.sha256(json.dumps(v,sort_keys=True,separators=(',',':')).encode()).hexdigest()
def save(p,data):
 STATE.mkdir(mode=0o700,parents=True,exist_ok=True)
 fd,tmp=tempfile.mkstemp(dir=STATE,prefix='.network-')
 try:
  with os.fdopen(fd,'w') as f:json.dump(data,f,sort_keys=True);f.flush();os.fsync(f.fileno())
  os.replace(tmp,p)
 finally:
  if os.path.exists(tmp):os.unlink(tmp)
def load(ident):
 p=STATE/(ident+'.json')
 if p.is_symlink():raise ValueError('State symlink refused')
 return json.loads(p.read_text()) if p.exists() else None
def validate(args):
 ident=args.get('id','');action=args.get('action');s=args.get('spec',{})
 if not IDENT.fullmatch(ident) or action not in ('create','update','delete'):raise ValueError('Invalid network identity/action')
 if not DISPLAY.fullmatch(s.get('name','')) or not IFACE.fullmatch(s.get('bridge','')) or not IFACE.fullmatch(s.get('uplink','')) or s['bridge']==s['uplink']:raise ValueError('Invalid bridge/uplink/name')
 if type(s.get('mtu')) is not int or not 1280<=s['mtu']<=9000:raise ValueError('Invalid MTU')
 hosts=s.get('hosts',[])
 if not isinstance(hosts,list) or not hosts or len(hosts)>64 or len(set(hosts))!=len(hosts):raise ValueError('Invalid target hosts')
 local={socket.gethostname(),socket.getfqdn(),socket.gethostname().split('.')[0],socket.getfqdn().split('.')[0]}
 if not any(h in local for h in hosts):raise ValueError('This host was not selected')
 groups=s.get('port_groups',[]);names=set();vlans=set()
 if not isinstance(groups,list) or len(groups)>64:raise ValueError('Invalid port groups')
 for p in groups:
  n=p.get('name','');v=p.get('vlan_id')
  if not DISPLAY.fullmatch(n) or type(v) is not int or not 0<=v<=4094 or n in names or v in vlans:raise ValueError('Invalid/duplicate port group or VLAN')
  names.add(n);vlans.add(v)
 return ident,action,s

def profiles(ident,s):
 prefix='vmalpha-dvs-'+ident
 rows=[{'id':prefix+'-bridge','type':'bridge','ifname':s['bridge']}, {'id':prefix+'-uplink','type':'ethernet','ifname':s['uplink'],'master':s['bridge']}]
 for p in s['port_groups']:
  if p['vlan_id']==0:continue
  suffix=hashlib.sha256((ident+':'+str(p['vlan_id'])).encode()).hexdigest()[:11]
  bridge='vp'+suffix;vlan='vv'+suffix
  rows.extend([{'id':prefix+'-pg-'+str(p['vlan_id']),'type':'bridge','ifname':bridge}, {'id':prefix+'-vlan-'+str(p['vlan_id']),'type':'vlan','ifname':vlan,'master':bridge,'parent':s['bridge'],'vlan':p['vlan_id']}])
 return rows

def snapshot(run):
 links=json.loads(run('ip','-j','-d','link','show',timeout=10));addresses=json.loads(run('ip','-j','address','show',timeout=10));routes=[]
 for family in ('-4','-6'):routes+=json.loads(run('ip','-j',family,'route','show','table','all',timeout=10))
 # Persistent profiles and inactive domains are included: an unused running link
 # is not necessarily available for takeover.
 connections=run('nmcli','-t','--escape','no','-f','UUID,NAME,TYPE,DEVICE','connection','show',timeout=10)
 refs=set()
 for domain in run('virsh','list','--all','--uuid',timeout=10).splitlines():
  if not domain:continue
  tree=ET.fromstring(run('virsh','dumpxml',domain,timeout=10))
  for source in tree.findall('./devices/interface/source'):
   for key in ('bridge','dev'):
    if source.get(key):refs.add(source.get(key))
 for net in run('virsh','net-list','--all','--uuid',timeout=10).splitlines():
  if net:
   bridge=ET.fromstring(run('virsh','net-dumpxml',net,timeout=10)).find('bridge')
   if bridge is not None and bridge.get('name'):refs.add(bridge.get('name'))
 addr={a['ifname']:[{k:v.get(k) for k in ('family','local','prefixlen','scope')} for v in a.get('addr_info',[]) if v.get('scope')!='link'] for a in addresses}
 route_if={r.get('dev') for r in routes if r.get('dst')!='ff00::/8'}
 normalized=[]
 for link in links:
  name=link['ifname'];physical=(pathlib.Path('/sys/class/net')/name/'device').exists()
  normalized.append({'name':name,'kind':link.get('linkinfo',{}).get('info_kind','physical' if physical else 'other'),'physical':physical,'master':link.get('master'),'mtu':link.get('mtu'),'active':link.get('operstate')=='UP','flags':link.get('flags',[]),'addresses':addr.get(name,[]),'routed':name in route_if,'vm_referenced':name in refs})
 profile_interfaces={}
 for line in connections.splitlines():
  fields=line.split(':')
  if len(fields)<4:raise ValueError('Unparseable NetworkManager profile')
  profile_interfaces[fields[0]]=run('nmcli','-g','connection.interface-name','connection','show','uuid',fields[0],timeout=10)
 return {'interfaces':sorted(normalized,key=lambda v:v['name']),'connections':connections,'profile_interfaces':profile_interfaces,'vm_references':sorted(refs)}

def discover(run):
 data=snapshot(run)
 for row in data['interfaces']:
  row['eligible_uplink']=row['physical'] and not row['master'] and not row['addresses'] and not row['routed'] and not row['vm_referenced'] and row['name'] not in data['profile_interfaces'].values() and not any(l.split(':')[-1]==row['name'] for l in data['connections'].splitlines())
  row['protection']='management/routed, addressed, member, VM use or existing profile' if not row['eligible_uplink'] else ''
 return {'supported':True,'hostname':socket.getfqdn(),'interfaces':data['interfaces']}

def preflight(args,run):
 ident,action,s=validate(args);old=load(ident);data=snapshot(run);links={r['name']:r for r in data['interfaces']}
 if old and old.get('state')!='applied':raise ValueError('Prior incomplete operation requires manual reconciliation')
 if action=='create' and old:raise ValueError('Network identity already exists')
 if action!='create' and not old:raise ValueError('Network is not managed on this host')
 if old and any(old['spec'][k]!=s[k] for k in ('bridge','uplink','hosts')):raise ValueError('Bridge/uplink/host migration is not supported')
 owned=profiles(ident,old['spec']) if old else [];owned_names={p['ifname'] for p in owned};owned_ids={p['id'] for p in owned};wanted=profiles(ident,s)
 if s['uplink'] not in links or not links[s['uplink']]['physical']:raise ValueError('Uplink must be an existing physical NIC')
 touched=owned_names|{p['ifname'] for p in wanted}
 for n in touched:
  row=links.get(n)
  if not row:continue
  if row['addresses'] or row['routed'] or row['vm_referenced']:raise ValueError('Management/addressed/routed/in-use interface protected: '+n)
  if row['master'] and row['master'] not in owned_names:raise ValueError('Interface belongs to another bridge/bond: '+n)
  if n!=s['uplink'] and n not in owned_names:raise ValueError('Existing unmanaged interface protected: '+n)
 for row in data['interfaces']:
  if row['master'] in touched and row['name'] not in owned_names:raise ValueError('Unmanaged bridge member protected: '+row['name'])
 for line in data['connections'].splitlines():
  fields=line.split(':');connection=fields[1] if len(fields)>2 else '';device=fields[-1]
  if device in touched and connection not in owned_ids:raise ValueError('Existing NetworkManager connection protected: '+connection)
  if connection.startswith('vmalpha-dvs-'+ident) and connection not in owned_ids:raise ValueError('Orphaned profile requires reconciliation')
 # Reject dormant profiles with the same interface too (not shown in DEVICE).
 for line in data['connections'].splitlines():
  fields=line.split(':')
  if len(fields)<4:raise ValueError('Unparseable NetworkManager profile')
  uuid,name=fields[:2]
  if name in owned_ids:
   data.setdefault('owned_settings',{})[uuid]=run('nmcli','-t','connection','show','uuid',uuid,timeout=10)
   continue
  bound=data['profile_interfaces'][uuid]
  if bound in touched:raise ValueError('Persistent interface profile protected: '+name)
 fingerprint=digest({'id':ident,'action':action,'spec':s,'old':old,'snapshot':data})
 mapping=[]
 for p in s['port_groups']:
  interface=s['bridge'] if p['vlan_id']==0 else next(v['ifname'] for v in wanted if v['id'].endswith('-pg-'+str(p['vlan_id'])))
  mapping.append({**p,'bridge':interface})
 return {'supported':True,'fingerprint':fingerprint,'host':socket.getfqdn(),'action':action,'port_groups':mapping,'plan':wanted,'old':old,'original_uplink_mtu':old.get('original_uplink_mtu',links[s['uplink']]['mtu']) if old else links[s['uplink']]['mtu']}

def remove(rows,run):
 for p in reversed(rows):run('nmcli','connection','delete','id',p['id'],timeout=15)
def create(ident,s,run,created):
 for p in profiles(ident,s):
  args=['nmcli','connection','add','type',p['type'],'con-name',p['id'],'ifname',p['ifname'],'connection.autoconnect','yes']
  if p.get('master'):args+=['master',p['master'],'slave-type','bridge']
  else:args+=['ipv4.method','disabled','ipv6.method','disabled','bridge.stp','no']
  if p['type']=='vlan':args+=['dev',p['parent'],'id',str(p['vlan'])]
  args+=['802-3-ethernet.mtu',str(s['mtu'])]
  created.append(p)  # Track before execution: a timeout may have created it.
  run(*args,timeout=15)
 for p in profiles(ident,s):run('nmcli','--wait','10','connection','up','id',p['id'],timeout=15)

def handle(op,args,run):
 if op=='network.distributed.discover':
  if args:raise ValueError('Discovery takes no arguments')
  return discover(run)
 preview=preflight(args,run)
 if op=='network.distributed.preview':return {k:v for k,v in preview.items() if k!='old'}
 if op!='network.distributed.apply':raise ValueError('Unsupported network operation')
 if args.get('expected_fingerprint')!=preview['fingerprint']:raise ValueError('Host network state changed after review')
 ident,action,s=validate(args);old=preview['old'];p=STATE/(ident+'.json');created=[];removed=False
 save(p,{'state':'applying','spec':s,'previous':old})
 try:
  if old:remove(profiles(ident,old['spec']),run);removed=True
  if action!='delete':create(ident,s,run,created)
  # Verify actual links and persistent connection activation before success.
  state=snapshot(run);present={v['name']:v for v in state['interfaces']}
  if action!='delete':
   for row in profiles(ident,s):
    if row['ifname'] not in present or present[row['ifname']]['mtu']!=s['mtu'] or (row.get('master') and present[row['ifname']]['master']!=row['master']):raise ValueError('Post-apply link verification failed')
    if not any(line.split(':')[1]==row['id'] and line.split(':')[-1]==row['ifname'] for line in state['connections'].splitlines()):raise ValueError('Profile activation verification failed')
  if action=='delete':
   if any(row['ifname'] in present for row in profiles(ident,s) if row['ifname']!=s['uplink']):raise ValueError('Deleted interface still present')
   run('ip','link','set','dev',s['uplink'],'mtu',str(preview['original_uplink_mtu']),timeout=10);p.unlink()
  else:save(p,{'state':'applied','spec':s,'original_uplink_mtu':preview['original_uplink_mtu']})
  return {'applied':True,'state':'deleted' if action=='delete' else 'applied','host':socket.getfqdn(),'port_groups':preview['port_groups']}
 except Exception as error:
  rollback='manual_reconciliation_required'
  try:
   remove(created,run)
   if old and removed:create(ident,old['spec'],run,[]);save(p,old);rollback='restored_previous_configuration'
   elif not old:
    run('ip','link','set','dev',s['uplink'],'mtu',str(preview['original_uplink_mtu']),timeout=10);p.unlink();rollback='removed_created_profiles'
  except Exception:pass
  return {'applied':False,'state':'failed','host':socket.getfqdn(),'error':str(error),'rollback':rollback}
