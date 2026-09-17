#!/usr/bin/python3
"""Create instance-local identity and database on an uninitialized appliance."""
import fcntl
import ipaddress
import json
import os
import pathlib
import pwd
import secrets
import socket
import subprocess

ROOT = pathlib.Path('/var/lib/vmalpha-manager')
CONFIG = pathlib.Path('/etc/novasphere')

def run(*args, **kwargs):
    return subprocess.run(args, check=True, text=True, **kwargs)

def private(path, content, owner=None):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as stream:
        stream.write(content)
    if owner:
        os.chown(path, owner.pw_uid, owner.pw_gid)

def main():
    if os.geteuid() != 0:
        raise SystemExit('Run as root')
    ROOT.mkdir(mode=0o700, exist_ok=True)
    with (ROOT/'initialize.lock').open('w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        if (ROOT/'initialized').exists():
            return
        # A failed attempt must be inspected, never silently regenerate credentials.
        if (ROOT/'initializing').exists():
            raise SystemExit('Incomplete initialization: inspect the journal before recovery')
        private(ROOT/'initializing', 'Initialization started\n')
        user = pwd.getpwnam('vmalpha')
        CONFIG.mkdir(mode=0o755, exist_ok=True)
        if pathlib.Path('/var/lib/pgsql/data/PG_VERSION').exists():
            raise SystemExit('Refusing to initialize an image containing a database')
        run('postgresql-setup', '--initdb')
        run('systemctl', 'enable', '--now', 'postgresql')
        run('runuser', '-u', 'postgres', '--', 'psql', '-v', 'ON_ERROR_STOP=1',
            '-c', 'CREATE ROLE vmalpha LOGIN;', '-c', 'CREATE DATABASE vmalpha OWNER vmalpha;')
        password = secrets.token_urlsafe(30)
        config = {
            'listen_address': '127.0.0.1', 'port': 8080, 'env': 'production',
            'database': {'host': '/var/run/postgresql', 'port': 5432, 'user': 'vmalpha',
                         'dbname': 'vmalpha', 'password': '', 'sslmode': 'disable'},
            'auth': {'jwt_secret': secrets.token_urlsafe(48), 'token_expiry_hours': 8},
            'ai': {'enabled': False}, 'native_hosts': {},
        }
        private(CONFIG/'config.yaml', json.dumps(config, indent=2)+'\n', user)
        private(ROOT/'bootstrap-password', password+'\n')
        private(ROOT/'bootstrap.env', 'NOVA_BOOTSTRAP_PASSWORD='+password+'\n')
        tls = pathlib.Path('/etc/pki/vmalpha-manager')
        tls.mkdir(mode=0o700, exist_ok=True)
        hostname = socket.getfqdn()
        if not hostname or any(c not in 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-' for c in hostname):
            raise SystemExit('Set a valid DNS hostname before initialization')
        sans = ['DNS:'+hostname, 'DNS:localhost', 'IP:127.0.0.1']
        addresses = json.loads(subprocess.check_output(['ip','-j','address'], text=True))
        for interface in addresses:
            for address in interface.get('addr_info', []):
                if address.get('scope') == 'global':
                    sans.append('IP:'+str(ipaddress.ip_address(address['local'])))
        run('openssl','req','-x509','-newkey','rsa:3072','-nodes','-sha256','-days','365',
            '-keyout',str(tls/'server.key'),'-out',str(tls/'server.crt'),
            '-subj','/CN='+hostname,'-addext','subjectAltName='+','.join(sans),
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        os.chmod(tls/'server.key', 0o600)
        run('restorecon','-RF',str(CONFIG),str(tls))
        run('setsebool','-P','httpd_can_network_connect','on')
        run('systemctl','enable','--now','firewalld')
        run('firewall-cmd','--permanent','--add-service=https')
        run('firewall-cmd','--reload')
        private(ROOT/'initialized', 'Instance identity and database initialized\n')
        (ROOT/'initializing').unlink()
        print('VM Alpha Manager initialized. Read the initial administrator password with:')
        print('sudo cat /var/lib/vmalpha-manager/bootstrap-password')
        print('Verify the TLS certificate fingerprint before signing in.')

if __name__ == '__main__':
    main()
