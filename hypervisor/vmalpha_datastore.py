"""Contained datastore operations, called only by the root command broker."""
import base64, json, os, pathlib, shutil, stat, time, uuid

READ_OPS={'ds-list','ds-read','ds-trash-list'}
OPS=READ_OPS|{'ds-mkdir','ds-copy','ds-move','ds-trash','ds-restore','ds-upload-begin','ds-upload-chunk','ds-upload-finish','ds-upload-cancel'}

def handle(op,a,root,state,run,vms,pools):
 root=root.resolve()
 def checked(value,exists=True,allow_root=False):
  p=pathlib.Path(value)
  if not p.is_absolute() or '..' in p.parts:raise ValueError('Use an absolute datastore path without parent traversal')
  if any(x.startswith('.vmalpha-') for x in p.parts):raise ValueError('Reserved datastore directory')
  if not p.is_relative_to(root) or (p==root and not allow_root):raise ValueError('Path is outside the datastore')
  for q in (p,*p.parents):
   if q.is_symlink():raise ValueError('Symbolic links are not supported')
   if q==root:break
  if exists and not p.exists():raise ValueError('File does not exist')
  if p.exists() and not (p.is_file() or p.is_dir()):raise ValueError('Only regular files and directories are supported')
  return p
 def protect(p,copy=False):
  for vm in vms():
   for d in vm['disks']:
    if d['path'] and pathlib.Path(d['path']).is_relative_to(p):
     if not copy or vm['state']!='shut off':raise ValueError('File is referenced by a VM; detach it first (copy requires the VM powered off)')
  if not copy and any(x['path'] and pathlib.Path(x['path']).is_relative_to(p) for x in pools()):raise ValueError('Cannot move or delete a datastore root')
 def destination(value):
  p=checked(value,False)
  checked(str(p.parent),True,True)
  if p.exists():raise ValueError('Destination already exists; files are never overwritten')
  if not p.parent.is_dir():raise ValueError('Destination folder does not exist')
  return p
 def metadata(p):
  s=p.stat();return dict(name=p.name,path=str(p),directory=p.is_dir(),size=s.st_size,modified=s.st_mtime,identity=f'{s.st_dev}:{s.st_ino}:{s.st_size}:{s.st_mtime_ns}')
 if op=='ds-trash-list':
  trash=root/'.vmalpha-trash'
  if trash.is_symlink():raise ValueError('Invalid trash directory')
  return [dict(token=p.stem,**json.loads(p.read_text())) for p in trash.glob('*.json') if not p.is_symlink()]
 if op=='ds-restore':
  token=a.get('token','')
  if not re_token(token):raise ValueError('Invalid trash item')
  trash=root/'.vmalpha-trash';meta=trash/(token+'.json');item=trash/(token+'.item')
  if trash.is_symlink() or meta.is_symlink() or item.is_symlink():raise ValueError('Invalid trash item')
  m=json.loads(meta.read_text());dest=destination(m['path']);item.rename(dest);meta.unlink();run('restorecon','-R',str(dest));return 'Item restored'
 if op=='ds-list':
  p=checked(a.get('path',str(root)),True,True)
  if not p.is_dir():raise ValueError('Select a directory')
  entries=[metadata(f) for f in p.iterdir() if not f.is_symlink() and not f.name.startswith('.vmalpha-') and (f.is_file() or f.is_dir())]
  entries.sort(key=lambda f:(not f['directory'],f['name'].casefold()))
  usage=shutil.disk_usage(p)
  return dict(path=str(p),entries=entries[:2000],truncated=len(entries)>2000,total=usage.total,free=usage.free)
 if op=='ds-mkdir':
  p=destination(a['path']);p.mkdir(mode=0o755);run('restorecon',str(p));return 'Directory created'
 if op in ('ds-copy','ds-move','ds-trash'):
  p=checked(a['path']);protect(p,op=='ds-copy')
  if op=='ds-trash':
   trash=root/'.vmalpha-trash';trash.mkdir(mode=0o700,exist_ok=True)
   if trash.is_symlink():raise ValueError('Invalid trash directory')
   token=uuid.uuid4().hex;dest=trash/(token+'.item')
   meta=trash/(token+'.json');meta.write_text(json.dumps(dict(path=str(p),name=p.name,time=time.time())));meta.chmod(0o600)
   try:p.rename(dest)
   except Exception:meta.unlink();raise
   return 'Moved to recoverable datastore trash'
  dest=destination(a['destination'])
  if dest.is_relative_to(p):raise ValueError('Destination cannot be inside the source')
  if op=='ds-copy':
   if not p.is_file():raise ValueError('Copy supports files; use Move for folders')
   if p.stat().st_blocks*512>shutil.disk_usage(dest.parent).free:raise ValueError('Insufficient free space')
   # Exclusive placeholder prevents overwrite; cp preserves sparse images.
   fd=os.open(dest,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o640);os.close(fd)
   try:run('cp','--sparse=always','--reflink=auto','--',str(p),str(dest),timeout=600)
   except Exception:dest.unlink(missing_ok=True);raise
  else:p.rename(dest)
  run('restorecon','-R',str(dest));return 'File copied' if op=='ds-copy' else 'Item moved'
 if op=='ds-read':
  p=checked(a['path']);protect(p,True)
  if not p.is_file():raise ValueError('Select a file')
  fd=os.open(p,os.O_RDONLY|os.O_NOFOLLOW)
  with os.fdopen(fd,'rb') as f:
   s=os.fstat(f.fileno());identity=f'{s.st_dev}:{s.st_ino}:{s.st_size}:{s.st_mtime_ns}'
   if identity!=a['identity']:raise ValueError('File changed; refresh before downloading')
   if s.st_size>256*1024*1024:raise ValueError('Browser download limit is 256 MiB; use SSH for larger files')
   offset=int(a.get('offset',0))
   if not 0<=offset<=s.st_size:raise ValueError('Invalid offset')
   f.seek(offset);chunk=f.read(4*1024*1024)
   return dict(data=base64.b64encode(chunk).decode(),offset=offset+len(chunk),size=s.st_size)
 transfer=root/'.vmalpha-transfers';transfer.mkdir(mode=0o700,exist_ok=True)
 if transfer.is_symlink():raise ValueError('Invalid transfer directory')
 owner=os.environ.get('SUDO_USER','root')
 if op=='ds-upload-begin':
  dest=destination(a['path']);size=int(a['size'])
  if not 0<=size<=8*1024**3 or size>shutil.disk_usage(dest.parent).free:raise ValueError('Upload exceeds 8 GiB or available space')
  token=uuid.uuid4().hex;meta=transfer/(token+'.json');part=transfer/(token+'.part')
  meta.write_text(json.dumps(dict(path=str(dest),size=size,user=owner)));meta.chmod(0o600)
  part.touch(mode=0o600,exist_ok=False);return dict(token=token)
 token=a.get('token','')
 if not isinstance(token,str) or len(token)!=32 or any(c not in '0123456789abcdef' for c in token):raise ValueError('Invalid transfer')
 meta=transfer/(token+'.json');part=transfer/(token+'.part')
 if meta.is_symlink() or part.is_symlink():raise ValueError('Invalid transfer')
 m=json.loads(meta.read_text())
 if m['user']!=owner:raise ValueError('Transfer belongs to another account')
 if op=='ds-upload-cancel':part.unlink(missing_ok=True);meta.unlink();return 'Upload cancelled'
 if op=='ds-upload-chunk':
  chunk=base64.b64decode(a['data'],validate=True)
  if len(chunk)>4*1024*1024 or part.stat().st_size!=int(a['offset']) or part.stat().st_size+len(chunk)>m['size']:raise ValueError('Invalid upload chunk or offset')
  fd=os.open(part,os.O_WRONLY|os.O_APPEND|os.O_NOFOLLOW)
  with os.fdopen(fd,'ab') as f:f.write(chunk)
  return dict(offset=part.stat().st_size)
 if op=='ds-upload-finish':
  if part.stat().st_size!=m['size']:raise ValueError('Upload is incomplete')
  dest=destination(m['path']);os.link(part,dest,follow_symlinks=False);part.unlink();meta.unlink();dest.chmod(0o644);run('restorecon',str(dest));return 'Upload completed'
 raise ValueError('Unsupported datastore operation')

def re_token(token):
 return isinstance(token,str) and len(token)==32 and all(c in '0123456789abcdef' for c in token)
