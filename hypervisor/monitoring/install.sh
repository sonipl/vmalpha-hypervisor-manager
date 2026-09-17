#!/bin/bash
set -euo pipefail
PAYLOAD=${1:-/usr/share/vmalpha/features}
ACTIVATE=${2:-auto}
case "$ACTIVATE" in auto|--defer-start) ;; *) echo "Unsupported activation mode"; exit 2;; esac
SOURCE=$(cd -- "$(dirname -- "$0")" && pwd)
# Verify published digests recorded during payload assembly before extraction.
python3 - "$PAYLOAD/binaries" <<'PY'
import hashlib,json,pathlib,sys
p=pathlib.Path(sys.argv[1]);manifest=json.loads((p/'binaries-manifest.json').read_text())
for n in ('prometheus-3.14.0.linux-amd64.tar.gz','node_exporter-1.12.1.linux-amd64.tar.gz'):
 row=next(x for x in manifest if x['file']==n)
 if hashlib.sha256((p/n).read_bytes()).hexdigest()!=row['sha256']:raise SystemExit('Monitoring payload checksum failed')
PY
getent passwd vmalpha-monitor >/dev/null || useradd --system --no-create-home --shell /sbin/nologin vmalpha-monitor
install -d -m 0750 -o root -g vmalpha-monitor /etc/vmalpha-monitoring
install -d -m 0750 -o vmalpha-monitor -g vmalpha-monitor /var/lib/vmalpha-prometheus
TMP_MONITOR=$(mktemp -d)
trap 'rm -rf "$TMP_MONITOR"' EXIT
tar --no-same-owner -xzf "$PAYLOAD/binaries/prometheus-3.14.0.linux-amd64.tar.gz" -C "$TMP_MONITOR"
tar --no-same-owner -xzf "$PAYLOAD/binaries/node_exporter-1.12.1.linux-amd64.tar.gz" -C "$TMP_MONITOR"
install -m 0755 "$TMP_MONITOR/prometheus-3.14.0.linux-amd64/prometheus" "$TMP_MONITOR/prometheus-3.14.0.linux-amd64/promtool" "$TMP_MONITOR/node_exporter-1.12.1.linux-amd64/node_exporter" /usr/local/bin/
install -m 0755 "$SOURCE/libvirt-exporter.py" /usr/libexec/vmalpha-libvirt-exporter
python3 - <<'PY'
import crypt,grp,os,pathlib,secrets
root=pathlib.Path('/etc/vmalpha-monitoring');gid=grp.getgrnam('vmalpha-monitor').gr_gid
for name in ('query-password','scrape-token'):
 p=root/name
 if not p.exists():
  fd=os.open(p,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o640)
  with os.fdopen(fd,'w') as f:f.write(secrets.token_urlsafe(36)+'\n')
 os.chown(p,0,gid)
p=root/'web.yml';password=(root/'query-password').read_text().strip();hashed=crypt.crypt(password,crypt.mksalt(crypt.METHOD_BLOWFISH))
if not hashed.startswith('$2'):raise SystemExit('bcrypt support is required')
p.write_text('basic_auth_users:\n  vmalpha: "'+hashed+'"\n');p.chmod(0o640);os.chown(p,0,gid)
PY
# Preserve enrolled Kubernetes/Ceph jobs when updating an existing installation.
if ! test -f /etc/vmalpha-monitoring/prometheus.yml; then
cat > /etc/vmalpha-monitoring/prometheus.yml <<'YAML'
global:
  scrape_interval: 15s
  evaluation_interval: 30s
rule_files:
  - /etc/vmalpha-monitoring/alerts.yml
scrape_configs:
  - job_name: host
    sample_limit: 10000
    static_configs:
      - targets: ['127.0.0.1:9100']
    basic_auth:
      username: vmalpha
      password_file: /etc/vmalpha-monitoring/query-password
  - job_name: libvirt
    sample_limit: 50000
    static_configs:
      - targets: ['127.0.0.1:9177']
    authorization:
      credentials_file: /etc/vmalpha-monitoring/scrape-token
YAML
fi
cat > /etc/vmalpha-monitoring/alerts.yml <<'YAML'
groups:
  - name: vmalpha-host
    rules:
      - alert: CollectorUnavailable
        expr: up == 0
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: 'A monitoring collector is unavailable; inspect its service.'
      - alert: HostMemoryPressure
        expr: node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes < 0.10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: 'Available host memory is below 10%; review VM and workload allocations.'
      - alert: DatastoreLowSpace
        expr: node_filesystem_avail_bytes{fstype!~"tmpfs|devtmpfs|overlay|squashfs"} / node_filesystem_size_bytes < 0.10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: 'Filesystem free space is below 10%; review capacity before provisioning.'
YAML
cat > /etc/systemd/system/vmalpha-prometheus.service <<'UNIT'
[Unit]
Description=VM Alpha performance history (Prometheus)
After=network.target
[Service]
User=vmalpha-monitor
Group=vmalpha-monitor
Environment=GOMEMLIMIT=512MiB
ExecStart=/usr/local/bin/prometheus --config.file=/etc/vmalpha-monitoring/prometheus.yml --web.listen-address=127.0.0.1:9091 --web.config.file=/etc/vmalpha-monitoring/web.yml --storage.tsdb.path=/var/lib/vmalpha-prometheus --storage.tsdb.retention.time=7d --storage.tsdb.retention.size=2GB --query.max-concurrency=4 --query.max-samples=100000
Restart=on-failure
MemoryMax=1G
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/vmalpha-prometheus
[Install]
WantedBy=multi-user.target
UNIT
cat > /etc/systemd/system/vmalpha-node-exporter.service <<'UNIT'
[Unit]
Description=VM Alpha host metrics (Prometheus node exporter)
After=network.target
[Service]
User=vmalpha-monitor
Group=vmalpha-monitor
ExecStart=/usr/local/bin/node_exporter --web.listen-address=127.0.0.1:9100 --web.config.file=/etc/vmalpha-monitoring/web.yml
Restart=on-failure
MemoryMax=128M
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
[Install]
WantedBy=multi-user.target
UNIT
cat > /etc/systemd/system/vmalpha-libvirt-exporter.service <<'UNIT'
[Unit]
Description=VM Alpha VM performance collector
After=virtqemud.socket
[Service]
ExecStart=/usr/libexec/vmalpha-libvirt-exporter
Restart=on-failure
MemoryMax=128M
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
[Install]
WantedBy=multi-user.target
UNIT
/usr/local/bin/promtool check config /etc/vmalpha-monitoring/prometheus.yml
restorecon -RF /etc/vmalpha-monitoring /var/lib/vmalpha-prometheus /usr/libexec/vmalpha-libvirt-exporter /usr/local/bin /etc/systemd/system/vmalpha-prometheus.service /etc/systemd/system/vmalpha-node-exporter.service /etc/systemd/system/vmalpha-libvirt-exporter.service
systemctl enable vmalpha-node-exporter vmalpha-libvirt-exporter vmalpha-prometheus
# Anaconda's chroot cannot start units in the installed system. Start only on a
# running host; enabled units start normally on the first installed boot.
if [ "$ACTIVATE" != "--defer-start" ] && ! systemd-detect-virt --chroot >/dev/null 2>&1; then
 systemctl daemon-reload
 systemctl start vmalpha-node-exporter vmalpha-libvirt-exporter vmalpha-prometheus
fi
