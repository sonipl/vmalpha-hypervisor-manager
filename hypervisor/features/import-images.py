#!/usr/bin/python3
"""Verify each complete archived image before loading the selected runtime."""
import hashlib,json,pathlib,subprocess,sys
root=pathlib.Path(sys.argv[1]);runtime=sys.argv[2]
if runtime not in ('containerd','podman'):raise SystemExit('Unsupported runtime')
expected={s for s in (pathlib.Path(__file__).resolve().parent/'images.txt').read_text().splitlines() if s and not s.startswith('#')}
present={json.loads(p.read_text())['reference'] for p in root.glob('*.json')}
if present!=expected:raise SystemExit('Feature image archive set is incomplete or unexpected')
for path in sorted(root.glob('*.json')):
 item=json.loads(path.read_text());ref=item['reference']
 if (runtime=='podman')!=(ref.startswith('quay.io/ceph/ceph:')):continue
 archive=root/item['file'];h=hashlib.sha256()
 with archive.open('rb') as f:
  for b in iter(lambda:f.read(4*1024*1024),b''):h.update(b)
 if h.hexdigest()!=item['sha256']:raise SystemExit('Archive checksum mismatch: '+ref)
 if runtime=='containerd':
  subprocess.run(['/usr/local/bin/ctr','-n','k8s.io','images','import','--platform','linux/amd64',str(archive)],check=True,stdout=subprocess.DEVNULL)
 else:subprocess.run(['podman','load','-i',str(archive)],check=True,stdout=subprocess.DEVNULL)
 print('Verified and loaded '+ref,flush=True)
