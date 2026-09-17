#!/usr/bin/env python3
"""Archive digest-pinned linux/amd64 OCI images with a per-image durable manifest."""
import concurrent.futures,hashlib,json,pathlib,subprocess,sys
root=pathlib.Path(sys.argv[1]);root.mkdir(parents=True,exist_ok=True)
images=pathlib.Path(sys.argv[2]).read_text().splitlines()
def run(*args):return subprocess.check_output(args,text=True)
def fetch(ref):
 if not ref or ref.startswith('#'):return
 key=ref.replace('/','_').replace(':','_');meta=root/(key+'.json');archive=root/(key+'.tar')
 if meta.exists() and archive.exists():print('Already archived '+ref,flush=True);return
 info=json.loads(run('skopeo','inspect','--override-os','linux','--override-arch','amd64','docker://'+ref));digest=info['Digest'];pinned=ref.rsplit(':',1)[0]+'@'+digest
 subprocess.run(['skopeo','copy','--retry-times','3','--override-os','linux','--override-arch','amd64','docker://'+pinned,'oci-archive:'+str(archive)+':'+ref],check=True,stdout=subprocess.DEVNULL)
 h=hashlib.sha256()
 with archive.open('rb') as f:
  for b in iter(lambda:f.read(4*1024*1024),b''):h.update(b)
 meta.write_text(json.dumps(dict(reference=ref,pinned=pinned,file=archive.name,sha256=h.hexdigest(),bytes=archive.stat().st_size),indent=2)+'\n');print('Archived '+ref+' '+digest,flush=True)
with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:list(pool.map(fetch,images))
