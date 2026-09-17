#!/usr/bin/env python3
"""Fetch pinned upstream payloads and verify published SHA256; never execute downloads."""
import concurrent.futures,hashlib,json,pathlib,sys,urllib.request
root=pathlib.Path(sys.argv[1]);root.mkdir(parents=True,exist_ok=True)
items=[]
for name in ('kubeadm','kubelet','kubectl'):
 url=f'https://dl.k8s.io/release/v1.36.4/bin/linux/amd64/{name}'
 items.append((name,url,url+'.sha256'))
base='https://github.com/containerd/containerd/releases/download/v2.3.5/'
name='containerd-2.3.5-linux-amd64.tar.gz'
items.append((name,base+name,base+name+'.sha256sum'))
items.append(('runc.amd64','https://github.com/opencontainers/runc/releases/download/v1.5.1/runc.amd64','https://github.com/opencontainers/runc/releases/download/v1.5.1/runc.sha256sum'))
name='cni-plugins-linux-amd64-v1.9.1.tgz'
url='https://github.com/containernetworking/plugins/releases/download/v1.9.1/'+name
items.append((name,url,url+'.sha256'))
for project,version in [('prometheus','3.14.0'),('node_exporter','1.12.1')]:
 name=f'{project}-{version}.linux-amd64.tar.gz';base=f'https://github.com/prometheus/{project}/releases/download/v{version}/'
 items.append((name,base+name,base+'sha256sums.txt'))
def fetch(item):
 name,url,checksum_url=item;p=root/name
 with urllib.request.urlopen(checksum_url,timeout=120) as r: checksum_text=r.read().decode()
 candidates=[line.split()[0].lower() for line in checksum_text.splitlines() if line.strip() and (len(line.split())==1 or line.split()[-1].lstrip('*')==name)]
 checksum=next((c for c in candidates if len(c)==64 and all(x in '0123456789abcdef' for x in c)), '')
 if len(checksum)!=64 or any(c not in '0123456789abcdef' for c in checksum):raise ValueError('Invalid upstream checksum for '+name)
 def digest(p):
  h=hashlib.sha256()
  with p.open('rb') as f:
   for b in iter(lambda:f.read(4*1024*1024),b''):h.update(b)
  return h.hexdigest()
 if not p.exists() or digest(p)!=checksum:
  temp=p.with_suffix(p.suffix+'.partial')
  with urllib.request.urlopen(url,timeout=120) as r,temp.open('wb') as f:
   while True:
    b=r.read(4*1024*1024)
    if not b:break
    f.write(b)
  if digest(temp)!=checksum:raise ValueError('Checksum mismatch: '+name)
  temp.replace(p)
 print(name+' SHA256 verified',flush=True)
 return dict(file=name,url=url,checksum_url=checksum_url,sha256=checksum,bytes=p.stat().st_size)
with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:result=list(pool.map(fetch,items))
(root/'binaries-manifest.json').write_text(json.dumps(result,indent=2)+'\n')
