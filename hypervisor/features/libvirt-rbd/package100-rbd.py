"""Package only the patched RBD backend against the fixture's existing libvirt ABI."""
from pathlib import Path
import subprocess

top = Path('/var/tmp/vmalpha-rbd-package')
for directory in ('SOURCES', 'SPECS', 'BUILD', 'RPMS', 'SRPMS'):
    (top / directory).mkdir(parents=True, exist_ok=True)
rpm = '/var/tmp/vmalpha-libvirt-build/RPMS/x86_64/libvirt-daemon-driver-storage-rbd-11.10.0-12.3.el9.vmalpha1.x86_64.rpm'
payload = subprocess.Popen(['rpm2cpio', rpm], stdout=subprocess.PIPE)
subprocess.run(['cpio', '-idmu', './usr/lib64/libvirt/storage-backend/libvirt_storage_backend_rbd.so'], cwd=top/'SOURCES', stdin=payload.stdout, check=True)
payload.stdout.close()
assert payload.wait() == 0
(top/'SOURCES/libvirt_storage_backend_rbd.so').write_bytes((top/'SOURCES/usr/lib64/libvirt/storage-backend/libvirt_storage_backend_rbd.so').read_bytes())
spec = '''Name: libvirt-daemon-driver-storage-rbd
Version: 11.10.0
Release: 12.3.el9_8.vmalpha1
Summary: Libvirt RBD storage backend with Ceph 20 authentication compatibility
License: LGPL-2.1-or-later
URL: https://libvirt.org/
Source0: usr/lib64/libvirt/storage-backend/libvirt_storage_backend_rbd.so
Requires: libvirt-daemon-driver-storage-core = 11.10.0-12.3.el9_8
Requires: libvirt-libs = 11.10.0-12.3.el9_8
# Preserve the storage metapackage's exact dependency on the original ABI.
Provides: libvirt-daemon-driver-storage-rbd = 11.10.0-12.3.el9_8
%global debug_package %{nil}
%description
RBD backend built from Rocky libvirt 11.10.0-12.3.el9_8 source,
including its downstream patches. The two auth_supported option names
are replaced by auth_client_required, retaining cephx authentication.
This development package replaces only the RBD plugin.
%install
install -D -m 0755 %{SOURCE0} %{buildroot}/usr/lib64/libvirt/storage-backend/libvirt_storage_backend_rbd.so
%files
/usr/lib64/libvirt/storage-backend/libvirt_storage_backend_rbd.so
'''
path = top/'SPECS/rbd.spec'
path.write_text(spec)
subprocess.run(['rpmbuild', '-bb', '--define', '_topdir '+str(top), str(path)], check=True)
