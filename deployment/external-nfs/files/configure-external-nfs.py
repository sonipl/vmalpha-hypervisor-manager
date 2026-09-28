#!/usr/bin/python3
"""Opt-in managed NFSv3 automount. Never formats storage or changes networking."""
import json,pathlib,re,subprocess,sys,os,datetime,tempfile

def validate(c):
 if c.get('enabled') is not True:raise ValueError('Explicit external NFS opt-in required')
 if not re.fullmatch(r'[a-z][a-z0-9-]{0,40}',c.get('id','')):raise ValueError('Invalid datastore ID')
 if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9.-]{0,252}',c.get('server','')):raise ValueError('Invalid server')
 if not re.fullmatch(r'/[A-Za-z0-9_./-]*',c.get('export','')) or '..' in pathlib.PurePosixPath(c['export']).parts:raise ValueError('Invalid export path')
 if str(c.get('version','3')) not in ('3','4.1'):raise ValueError('Unsupported NFS version')
 return '/var/lib/vmalpha/datastores/'+c['id']
def run(*a):return subprocess.run(a,check=True,capture_output=True,text=True,timeout=45).stdout

def configure(c):
 target=validate(c);source=c['server']+':'+c['export'];p=pathlib.Path(target);version=str(c.get('version','3'))
 if any(v.is_symlink() for v in (p,*p.parents)):raise ValueError('Symlink mount targets refused')
 existing=subprocess.run(['findmnt','-rn','-M',target,'-o','SOURCE,FSTYPE'],capture_output=True,text=True).stdout.strip()
 if existing and existing.split() not in ([source,'nfs'],[source,'nfs4'],['systemd-1','autofs']):raise ValueError('Existing mount conflicts')
 if p.exists() and not existing and any(p.iterdir()):raise ValueError('Mount target must be empty')
 fstab=pathlib.Path('/etc/fstab');text=fstab.read_text();marker='# VMALPHA external-nfs '+c['id']
 lines=text.splitlines();desired=source+' '+target+' nfs vers='+version+',soft,nofail,_netdev,x-systemd.automount,x-systemd.mount-timeout=30s 0 0'
 for line in lines:
  fields=line.split()
  if line.strip().startswith('#') or len(fields)<2:continue
  if fields[1]==target and line!=desired:raise ValueError('Existing fstab mount differs; migration required')
 if desired not in lines:
  backup=fstab.with_name('fstab.vmalpha-'+datetime.datetime.now().strftime('%Y%m%d%H%M%S'));backup.write_text(text);os.chmod(backup,0o600)
  fd,tmp=tempfile.mkstemp(prefix='.fstab-vmalpha-',dir='/etc')
  try:
   with os.fdopen(fd,'w') as f:
    f.write(text.rstrip()+'\n'+marker+'\n'+desired+'\n');f.flush();os.fsync(f.fileno())
   os.chmod(tmp,fstab.stat().st_mode&0o777);os.replace(tmp,fstab)
  finally:
   if os.path.exists(tmp):os.unlink(tmp)
 p.mkdir(parents=True,exist_ok=True)
 run('systemctl','daemon-reload');unit=run('systemd-escape','--path','--suffix=automount',target).strip();run('systemctl','start',unit)
 run('timeout','35','stat','--',target+'/.' )
 result=json.loads(run('findmnt','-J','-M',target,'-o','SOURCE,FSTYPE,OPTIONS'))['filesystems'][0]
 if result['source']!=source or result['fstype'] not in ('nfs','nfs4') or not {'vers='+version,'soft'}<=set(result['options'].split(',')):raise ValueError('Mount verification failed')
 print(json.dumps({'id':c['id'],'source':source,'mount_path':target,'verified':True}))
if __name__=='__main__':configure(json.loads(pathlib.Path(sys.argv[1]).read_text()))
