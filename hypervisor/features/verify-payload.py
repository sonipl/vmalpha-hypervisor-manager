#!/usr/bin/python3
"""Verify a complete signed feature manifest before offline installation."""
import hashlib,json,pathlib,subprocess,sys,tempfile
root=pathlib.Path(sys.argv[1]).resolve()
manifest=root/'payload-manifest.json'
with tempfile.TemporaryDirectory() as temporary:
 subprocess.run(['gpg','--homedir',temporary,'--batch','--import',str(root/'keys/RPM-GPG-KEY-VMAlpha')],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 subprocess.run(['gpg','--homedir',temporary,'--batch','--verify',str(root/'payload-manifest.json.asc'),str(manifest)],check=True)
data=json.loads(manifest.read_text())
for row in data['files']:
 p=(root/row['path']).resolve()
 if not p.is_relative_to(root) or not p.is_file():raise SystemExit('Missing or unsafe payload path')
 h=hashlib.sha256()
 with p.open('rb') as f:
  for block in iter(lambda:f.read(4*1024*1024),b''):h.update(block)
 if h.hexdigest()!=row['sha256'] or p.stat().st_size!=row['bytes']:raise SystemExit('Payload integrity failure: '+row['path'])
print('Complete signed feature payload verified')
