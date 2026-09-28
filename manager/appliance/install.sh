#!/bin/bash
# Run only inside the separate uninitialized appliance builder guest.
set -euo pipefail
test "$(id -u)" = 0
test -f /var/lib/vmalpha-manager-build-ready
test ! -e /var/lib/pgsql/data/PG_VERSION
test ! -e /var/lib/vmalpha-manager/initialized
package_dir=$(cd -- "$(dirname -- "$0")" && pwd)
getent passwd vmalpha >/dev/null || useradd --system --home-dir /opt/vmalpha-manager --shell /sbin/nologin vmalpha
install -m 0755 "$package_dir/vmalpha-manager" /usr/local/bin/vmalpha-manager
install -m 0755 "$package_dir/firstboot.py" /usr/libexec/vmalpha-manager-firstboot
install -m 0700 "$package_dir/scripts/register-ceph-backends.py" /usr/libexec/vmalpha-register-ceph-backends
install -m 0644 "$package_dir/vmalpha-manager.service" "$package_dir/vmalpha-manager-firstboot.service" /etc/systemd/system/
mkdir -p /opt/vmalpha-manager/web /etc/nginx/conf.d /etc/systemd/system/nginx.service.d
cp -a "$package_dir/dist" /opt/vmalpha-manager/web/
chown -R root:root /opt/vmalpha-manager
install -m 0644 "$package_dir/nginx.conf" /etc/nginx/conf.d/vmalpha-manager.conf
cat >/etc/nginx/nginx.conf <<'EOF'
user nginx;
worker_processes auto;
error_log /var/log/nginx/error.log warn;
pid /run/nginx.pid;
include /usr/share/nginx/modules/*.conf;
events { worker_connections 1024; }
http {
    map $http_upgrade $connection_upgrade { default upgrade; '' close; }
    include /etc/nginx/mime.types;
    default_type application/octet-stream;
    sendfile on;
    server_tokens off;
    access_log /var/log/nginx/access.log;
    include /etc/nginx/conf.d/*.conf;
}
EOF
cat >/etc/systemd/system/nginx.service.d/vmalpha.conf <<'EOF'
[Unit]
Requires=vmalpha-manager-firstboot.service
After=vmalpha-manager-firstboot.service
EOF
restorecon -RF /usr/local/bin/vmalpha-manager /usr/libexec/vmalpha-manager-firstboot /opt/vmalpha-manager /etc/nginx /etc/systemd/system
systemctl daemon-reload
systemctl enable vmalpha-manager-firstboot vmalpha-manager nginx
# Do not initialize database, administrator, JWT, or TLS in the reusable image.
echo 'Manager files installed; first-boot initialization remains pending.'
