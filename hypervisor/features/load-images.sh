#!/bin/bash
set -euo pipefail
SOURCE=$(cd -- "$(dirname -- "$0")" && pwd)
PAYLOAD=${1:-/usr/share/vmalpha/features}
install -d /var/lib/vmalpha
exec 9>/var/lib/vmalpha/feature-images.lock
flock 9
HASH=$(sha256sum "$PAYLOAD/payload-manifest.json" | cut -d' ' -f1)
if test -f /var/lib/vmalpha/feature-images.sha256 && test "$(cat /var/lib/vmalpha/feature-images.sha256)" = "$HASH"; then
 # Garbage collection may remove unused images after the previous import.
 if python3 - "$SOURCE/images.txt" <<'PY_CHECK'
import pathlib,subprocess,sys
expected={s for s in pathlib.Path(sys.argv[1]).read_text().splitlines() if s and not s.startswith('#')}
ctr=set(subprocess.check_output(['/usr/local/bin/ctr','-n','k8s.io','images','list','--quiet'],text=True).splitlines())
podman=set(subprocess.check_output(['podman','images','--format','{{.Repository}}:{{.Tag}}'],text=True).splitlines())
raise SystemExit(0 if all(ref in (podman if ref.startswith('quay.io/ceph/ceph:') else ctr) for ref in expected) else 1)
PY_CHECK
 then
  echo 'Feature image references are present for this payload.'
  exit 0
 fi
fi
python3 "$SOURCE/verify-payload.py" "$PAYLOAD"
python3 "$SOURCE/import-images.py" "$PAYLOAD/images" containerd
python3 "$SOURCE/import-images.py" "$PAYLOAD/images" podman
printf '%s\n' "$HASH" > /var/lib/vmalpha/feature-images.sha256
