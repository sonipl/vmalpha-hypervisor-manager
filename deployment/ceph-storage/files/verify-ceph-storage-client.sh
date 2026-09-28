#!/bin/bash
set -euo pipefail
umask 077
base=$(mktemp -d /run/vmalpha-storage-test.XXXXXX)
image="verify-$(hostname -s)-$(date +%s)-$$"
cleanup(){ mountpoint -q "$base/nfs" && umount "$base/nfs"; rm -rf "$base"; }
trap cleanup EXIT
mkdir "$base/nfs"
printf 'VM Alpha storage validation %s' "$image" > "$base/input"
rbd --id vmalpha-storage --conf /etc/ceph/vmalpha-storage.conf --keyring /etc/ceph/ceph.client.vmalpha-storage.keyring import "$base/input" "vmalpha-rbd/$image"
rbd --id vmalpha-storage --conf /etc/ceph/vmalpha-storage.conf --keyring /etc/ceph/ceph.client.vmalpha-storage.keyring export "vmalpha-rbd/$image" "$base/output"
cmp -n "$(stat -c%s "$base/input")" "$base/input" "$base/output"
rbd --id vmalpha-storage --conf /etc/ceph/vmalpha-storage.conf --keyring /etc/ceph/ceph.client.vmalpha-storage.keyring rm "vmalpha-rbd/$image"
server=$(python3 -c "import json;print(json.load(open('/etc/vmalpha/storage-workflow.json'))['nfs_vip'].split('/')[0])")
for server in "$server"; do
 timeout 30 mount -t nfs -o vers=4.1,soft,timeo=20,retrans=2 "$server:/vmalpha" "$base/nfs"
 cp "$base/input" "$base/nfs/$image"
 cmp "$base/input" "$base/nfs/$image"
 rm "$base/nfs/$image"
 umount "$base/nfs"
done
echo "$(hostname -f) RBD_AND_ALL_NFS_ENDPOINTS_RW_VERIFIED"
