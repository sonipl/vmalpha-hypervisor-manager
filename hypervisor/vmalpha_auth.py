"""Central authorization for every privileged broker and console entry point."""
import grp,json,os,pathlib,pwd,re,subprocess,uuid
STATE=pathlib.Path('/var/lib/vmalpha')
PRIVILEGES={'vm.view','vm.power','vm.edit','vm.console'}
BUILTIN={'VM Viewer':['vm.view'],'VM Operator':['vm.view','vm.power','vm.edit','vm.console']}
def actor():
 if os.geteuid()!=0:raise ValueError('Privileged broker requires sudo')
 return os.environ.get('SUDO_USER','root')
def admin():
 u=pwd.getpwnam(actor())
 return u.pw_uid==0 or grp.getgrnam('wheel').gr_gid in os.getgrouplist(u.pw_name,u.pw_gid)
def policy():
 try:return json.loads((STATE/'permissions.json').read_text())
 except FileNotFoundError:return {'roles':{},'assignments':[]}
def roles(p=None):return {**BUILTIN,**(p or policy())['roles']}
def save(p):
 STATE.mkdir(mode=0o755,exist_ok=True)
 path=STATE/('permissions-'+uuid.uuid4().hex+'.tmp')
 fd=os.open(path,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
 with os.fdopen(fd,'w') as f:json.dump(p,f)
 path.replace(STATE/'permissions.json')
def rights(vm_uuid):
 if admin():return set(PRIVILEGES)
 p=policy();r=roles(p);result=set()
 for a in p['assignments']:
  if a['user']==actor() and a['scope'] in ('*',vm_uuid):result.update(r.get(a['role'],[]))
 return result

def domain_id(name):
 if not isinstance(name,str) or not re.fullmatch(r'[A-Za-z][A-Za-z0-9_.-]{0,62}',name):raise ValueError('Invalid guest name')
 p=subprocess.run(['virsh','domuuid',name],text=True,capture_output=True)
 if p.returncode:raise ValueError('Guest unavailable or access denied')
 return p.stdout.strip()
def require(privilege,name):
 actor()
 if admin():return
 if privilege not in rights(domain_id(name)):raise ValueError('Permission denied for this virtual machine')
def authorize(q):
 if admin():return
 op=q.get('op');a=q.get('args',{})
 if not isinstance(a,dict):raise ValueError('Invalid operation arguments')
 if op=='metrics' and a.get('scope')=='vm':require('vm.view',a.get('name'));return
 if op in ('inventory','security','tasks') and any(x['user']==actor() for x in policy()['assignments']):return
 perm={'vm-action':'vm.power','vm-edit':'vm.edit','vm-snapshot-create':'vm.snapshot','vm-snapshot-revert':'vm.snapshot','vm-snapshot-list':'vm.view'}.get(op)
 if perm:require(perm,a.get('name'));return
 raise ValueError('Host administrator permission is required')
def filter_result(op,result):
 if op=='inventory':
  result['administrator']=admin()
  for vm in result['vms']:vm['privileges']=sorted(rights(vm['uuid']))
  if not admin():
   result['vms']=[v for v in result['vms'] if 'vm.view' in v['privileges']]
   used={n['network'] for v in result['vms'] for n in v['networks']}
   result['networks']=[n for n in result['networks'] if n['name'] in used];result['pools']=[];result['devices']=[];result['interfaces']=[];result['routes']=[]
 if op=='security':
  p=policy();result['vmRoles']=[{'name':n,'privileges':v,'builtin':n in BUILTIN} for n,v in roles(p).items()]
  result['assignments']=p['assignments'] if admin() else [a for a in p['assignments'] if a['user']==actor()]
  if not admin():
   result['users']=[u for u in result['users'] if u['name']==actor()]
   for k in ('rules','certificates','banner','lockout','passwordPolicy'):result[k]='Host administrator access required'
   result['services']=[]
 if op=='tasks' and not admin():
  result=[t for t in result if t.get('user')==actor()]
 return result

def mutate(op,a,run,name):
 if not admin():raise ValueError('Host administrator permission is required')
 p=policy()
 if op=='role-create':
  n=name(a['name']);privs=a.get('privileges','').split(',') if isinstance(a.get('privileges'),str) else a.get('privileges',[])
  privs=sorted(set(x.strip() for x in privs))
  if n in roles(p):raise ValueError('Role already exists')
  if not privs or not set(privs)<=PRIVILEGES or 'vm.view' not in privs:raise ValueError('Use supported VM privileges and include vm.view')
  p['roles'][n]=privs
 elif op=='role-delete':
  n=a['name']
  if n in BUILTIN:raise ValueError('Built-in roles are protected')
  if any(x['role']==n for x in p['assignments']):raise ValueError('Remove assignments before deleting this role')
  if n not in p['roles']:raise ValueError('Role not found')
  del p['roles'][n]
 elif op=='permission-assign':
  user=name(a['user']);u=pwd.getpwnam(user)
  if not 1000<=u.pw_uid<60000 or u.pw_shell in ('/sbin/nologin','/usr/sbin/nologin','/bin/false'):raise ValueError('Select an interactive local user')
  role=a['role'];vm=a['vm'];scope='*' if vm=='*' else domain_id(vm)
  if role not in roles(p):raise ValueError('Role not found')
  p['assignments']=[x for x in p['assignments'] if not (x['user']==user and x['scope']==scope)]
  p['assignments'].append(dict(id=uuid.uuid4().hex,user=user,scope=scope,vm=vm,role=role))
  run('usermod','-aG','vmalpha',user)
 elif op=='permission-remove':
  token=a['id'];found=[x for x in p['assignments'] if x['id']==token]
  if not found:raise ValueError('Assignment not found')
  p['assignments']=[x for x in p['assignments'] if x['id']!=token]
 else:raise ValueError('Unsupported permission operation')
 save(p);return 'Permissions saved. VM permissions apply immediately; newly assigned users must sign in again to acquire their access group.'
