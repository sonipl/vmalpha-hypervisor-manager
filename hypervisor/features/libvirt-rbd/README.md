# Libvirt RBD compatibility build

Input: the signed Rocky source RPM `libvirt-11.10.0-12.3.el9_8.src.rpm`.
The payload source directory retains this exact upstream source package.
The build retains its downstream patches and changes the two string literals
`auth_supported` to `auth_client_required` in `src/storage/storage_backend_rbd.c`.
Ceph 20.2.4 no longer accepts the old option. Cephx remains required.

The included scripts record the actual integration build. Run them only on an
isolated EL9 build machine with the source RPM's build dependencies installed,
including Ceph 20.2.4 development headers. They use
`/var/tmp/vmalpha-libvirt-build` and `/var/tmp/vmalpha-rbd-package`; do not point
them at an existing production installation.

```sh
rpm -K libvirt-11.10.0-12.3.el9_8.src.rpm
rpm -i --define '_topdir /var/tmp/vmalpha-libvirt-build' libvirt-11.10.0-12.3.el9_8.src.rpm
python3 patch-libvirt100.py
nice -n 15 rpmbuild -bb --nocheck \
  --define '_topdir /var/tmp/vmalpha-libvirt-build' \
  --define '_smp_mflags -j2' \
  /var/tmp/vmalpha-libvirt-build/SPECS/libvirt.spec
python3 package100-rbd.py
```

The complete libvirt source build succeeded. Its upstream test suite was not
run (`--nocheck`); real authenticated pool/image creation, VM attachment and
filesystem I/O, including persistence after reboot, were tested separately.

Only the RBD backend is replaced. The package requires the original matching
libvirt core/libraries and provides the original storage-metapackage ABI
dependency. The newer VM Alpha release remains visible in RPM metadata.
Do not force-install this package against another libvirt version.

The delivered RPM signature is checked with the public VM Alpha build key.
The private signing key is not part of this source directory or the payload.
The upstream source RPM and its license text retain original provenance.
