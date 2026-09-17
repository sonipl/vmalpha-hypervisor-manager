# VM Alpha Manager appliance build (development)

This package is under integration testing and is not an accepted release image.
Use the official Rocky 9.8 GenericCloud Base x86_64 image, verify its detached
checksum signature using the Rocky release key, and verify the image SHA256.
The recorded base is Rocky-9-GenericCloud-Base-9.8-20260525.0.x86_64.qcow2,
SHA256 92c206cc6f790c61583247eefe87890f8828420662c17cacf247cec78ab4eec8.
Release key fingerprint: 21CB256AE16FC54C6E652949702D426D350D275D.
Source: https://download.rockylinux.org/pub/rocky/9/images/x86_64/

Build in a separate guest with a new disk. Install a supported PostgreSQL module,
nginx, firewalld, Python 3, OpenSSL and SELinux management tools. Build the Go API
for linux/amd64 with CGO disabled and build the frontend after type checking.
Package those outputs alongside these scripts. install.sh only installs the
application and enables first-boot services; it must not initialize the image's
database or create operator credentials. A test builder marker is required.

First boot creates a local PostgreSQL database using Unix-socket peer authentication,
a unique JWT secret, a random administrator password and a unique TLS key and
certificate with the instance hostname/IP. The API listens only on loopback;
nginx serves HTTPS. No live host enrollment is shipped. The initial password is
root-readable at /var/lib/vmalpha-manager/bootstrap-password. Supply an operator
SSH public key and unique hostname/network with cloud-init when deploying.
Verify the TLS fingerprint via that trusted console/SSH channel before login.

An interrupted initialization stops for inspection instead of silently replacing
credentials or an existing database. Examine the first-boot unit journal and
/var/lib/vmalpha-manager/initializing; preserve state before recovery. Never remove
that marker blindly on an initialized system. Credentials and backups must not
be included in a reusable template.

Before export: shut down cleanly, remove builder access/seed/cache/logs, clear
machine identity for regeneration, ensure no PostgreSQL data/JWT/TLS/bootstrap
state remains, detach build media, and export a standalone QCOW2 without backing
files. Deploy two copies with distinct cloud-init instance IDs, then verify
machine/SSH/TLS/JWT/database identity separation, login, reboot persistence,
authenticated host enrollment and native operations. Also test isolated backup
restore. No reusable-appliance acceptance is implied until those gates pass.
