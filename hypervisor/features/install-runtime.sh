#!/bin/bash
# Install verified feature payloads. Does not initialize a cluster or consume disks.
set -euo pipefail
PAYLOAD=${1:-/usr/share/vmalpha/features}
python3 - "$PAYLOAD/binaries" <<'PY'
import hashlib,json,pathlib,sys
root=pathlib.Path(sys.argv[1]);items=json.loads((root/'binaries-manifest.json').read_text())
expected={'kubeadm','kubelet','kubectl','containerd-2.3.5-linux-amd64.tar.gz','runc.amd64','cni-plugins-linux-amd64-v1.9.1.tgz'}
if not expected<={x['file'] for x in items}:raise SystemExit('Feature binary manifest is incomplete')
for x in items:
 p=root/x['file'];h=hashlib.sha256()
 with p.open('rb') as f:
  for b in iter(lambda:f.read(4*1024*1024),b''):h.update(b)
 if h.hexdigest()!=x['sha256']:raise SystemExit('Payload checksum mismatch: '+x['file'])
PY
install -d /usr/local/bin /usr/local/sbin /opt/cni/bin /etc/containerd /etc/cni/net.d /var/lib/kubelet /etc/kubernetes
install -m 0755 "$PAYLOAD/binaries/kubeadm" "$PAYLOAD/binaries/kubelet" "$PAYLOAD/binaries/kubectl" /usr/local/bin/
install -m 0755 "$PAYLOAD/binaries/runc.amd64" /usr/local/sbin/runc
tar --no-same-owner -xzf "$PAYLOAD/binaries/containerd-2.3.5-linux-amd64.tar.gz" -C /usr/local
tar --no-same-owner -xzf "$PAYLOAD/binaries/cni-plugins-linux-amd64-v1.9.1.tgz" -C /opt/cni/bin
if ! test -f /etc/containerd/config.toml; then
 /usr/local/bin/containerd config default > /etc/containerd/config.toml
 sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
fi
cat > /etc/systemd/system/containerd.service <<'UNIT'
[Unit]
Description=VM Alpha Container Platform runtime (containerd)
After=network.target
[Service]
ExecStart=/usr/local/bin/containerd
Restart=always
RestartSec=5
Delegate=yes
KillMode=process
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
OOMScoreAdjust=-999
[Install]
WantedBy=multi-user.target
UNIT
cat > /etc/systemd/system/kubelet.service <<'UNIT'
[Unit]
Description=VM Alpha Container Platform node agent (kubelet)
Wants=network-online.target
After=network-online.target containerd.service
[Service]
Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf"
Environment="KUBELET_CONFIG_ARGS=--config=/var/lib/kubelet/config.yaml"
EnvironmentFile=-/var/lib/kubelet/kubeadm-flags.env
EnvironmentFile=-/etc/sysconfig/kubelet
ExecStart=/usr/local/bin/kubelet $KUBELET_KUBECONFIG_ARGS $KUBELET_CONFIG_ARGS $KUBELET_KUBEADM_ARGS $KUBELET_EXTRA_ARGS
Restart=always
RestartSec=10
[Install]
WantedBy=multi-user.target
UNIT
restorecon -RF /usr/local/bin /usr/local/sbin /opt/cni /etc/containerd /etc/kubernetes /var/lib/kubelet /etc/systemd/system/containerd.service /etc/systemd/system/kubelet.service
systemctl daemon-reload
printf '%s\n' 'VM Alpha Container Platform runtime installed; cluster initialization is separate.'
