"""Fixed administrative security operations for the isolated Host Client."""
import configparser, os, pathlib, pwd, re, shutil, tempfile, datetime
OPS={'user-role','session-policy','password-policy','certificate-import','certificate-restore'}

def details(run):
 c=configparser.ConfigParser();c.read('/etc/cockpit/cockpit.conf')
 policy=pathlib.Path('/etc/security/pwquality.conf.d/90-vmalpha.conf')
 return dict(idleTimeout=c.getint('Session','IdleTimeout',fallback=15),passwordPolicy=policy.read_text() if policy.exists() else 'System PAM password-quality defaults',certificateRollback=pathlib.Path('/var/lib/vmalpha/certificate-backup/current').is_dir())

def handle(op,a,run,state,name,number):
 if op=='user-role':
  user=name(a['name']);role=a['role']
  if user in ('root','admin',os.environ.get('SUDO_USER')):raise ValueError('Primary and current administrator roles are protected')
  u=pwd.getpwnam(user)
  if not 1000<=u.pw_uid<60000 or u.pw_shell in ('/sbin/nologin','/usr/sbin/nologin','/bin/false'):raise ValueError('Select an interactive local user')
  if role not in ('Administrator','OS user'):raise ValueError('Unsupported role')
  groups=run('id','-nG',user).split()
  if role=='Administrator':run('usermod','-aG','wheel',user)
  elif 'wheel' in groups:run('gpasswd','-d',user,'wheel')
  # Existing authenticated processes retain Unix groups; make this explicit.
  return 'Role updated. The new role applies to new login sessions; existing sessions must log out.'
 if op=='session-policy':
  minutes=number(a['minutes'],1,60);p=pathlib.Path('/etc/cockpit/cockpit.conf')
  c=configparser.ConfigParser();c.optionxform=str;c.read(p)
  if not c.has_section('Session'):c.add_section('Session')
  c.set('Session','IdleTimeout',str(minutes))
  with p.open('w') as f:c.write(f)
  return 'Idle timeout saved for new login sessions'
 if op=='password-policy':
  length=number(a['minlen'],8,64);classes=number(a['minclass'],1,4)
  p=pathlib.Path('/etc/security/pwquality.conf.d/90-vmalpha.conf');p.parent.mkdir(exist_ok=True)
  if 'pam_pwquality.so' not in pathlib.Path('/etc/pam.d/system-auth').read_text():raise ValueError('PAM password-quality enforcement is not configured')
  p.write_text(f'minlen = {length}\nminclass = {classes}\nenforce_for_root\n');p.chmod(0o644)
  return 'Password quality policy saved for future password changes. Existing passwords are unchanged.'
 if op=='certificate-import':
  cert=a.get('certificate','');key=a.get('privateKey','')
  if not isinstance(cert,str) or not isinstance(key,str) or len(cert)>131072 or len(key)>32768:raise ValueError('Invalid PEM input size')
  if not cert.startswith('-----BEGIN CERTIFICATE-----') or 'PRIVATE KEY-----' not in key:raise ValueError('Provide a PEM certificate chain and unencrypted private key')
  with tempfile.TemporaryDirectory(prefix='vmalpha-cert-') as td:
   td=pathlib.Path(td);cp=td/'leaf.cert';kp=td/'leaf.key';cp.write_text(cert);kp.write_text(key);kp.chmod(0o600)
   run('openssl','x509','-in',str(cp),'-noout','-checkend','0')
   end=run('openssl','x509','-in',str(cp),'-noout','-startdate').split('=',1)[1]
   start=datetime.datetime.strptime(end,'%b %d %H:%M:%S %Y %Z').replace(tzinfo=datetime.timezone.utc)
   if start>datetime.datetime.now(datetime.timezone.utc):raise ValueError('Certificate is not yet valid')
   pubcert=run('openssl','x509','-in',str(cp),'-pubkey','-noout')
   pubkey=run('openssl','pkey','-in',str(kp),'-passin','pass:','-pubout')
   if pubcert.strip()!=pubkey.strip():raise ValueError('Certificate does not match the private key')
   # Parsing the entire chain rejects broken trailing certificate material.
   run('openssl','crl2pkcs7','-nocrl','-certfile',str(cp),'-out',str(td/'chain.p7b'))
   folder=pathlib.Path('/etc/cockpit/ws-certs.d');folder.mkdir(mode=0o755,exist_ok=True)
   backup=state/'certificate-backup';backup.mkdir(mode=0o700,parents=True,exist_ok=True)
   current=backup/'current'
   if current.exists():current.rename(backup/('prior-'+datetime.datetime.now().strftime('%Y%m%d%H%M%S%f')))
   current.mkdir(mode=0o700)
   for p in folder.iterdir():
    if p.suffix in ('.cert','.key') and p.is_file() and not p.is_symlink():shutil.copy2(p,current/p.name)
   # Publish key first, then the certificate as the activation marker.
   stem='zz-vmalpha-'+datetime.datetime.now().strftime('%Y%m%d%H%M%S%f')
   dest=folder/(stem+'.cert');keydest=folder/(stem+'.key')
   kp.chmod(0o600);cp.chmod(0o644)
   shutil.copy2(kp,keydest);shutil.copy2(cp,dest)
   run('restorecon',str(dest),str(keydest))
   try:
    active=run('/usr/libexec/cockpit-certificate-ensure','--check')
    if str(dest) not in active:raise ValueError('Cockpit did not select the imported certificate')
   except Exception:
    dest.unlink(missing_ok=True);keydest.unlink(missing_ok=True);raise
  run('systemd-run','--quiet','--collect','--on-active=3s','--timer-property=AccuracySec=1s','/usr/bin/systemctl','try-restart','cockpit.service')
  return 'Certificate installed; management sessions restart in 3 seconds. Reconnect to use it. Verify its CA trust on your client. Previous certificate is available for rollback.'
 if op=='certificate-restore':
  folder=pathlib.Path('/etc/cockpit/ws-certs.d');current=state/'certificate-backup/current'
  if not current.is_dir():raise ValueError('No certificate backup is available')
  archive=state/'certificate-backup'/('replaced-'+datetime.datetime.now().strftime('%Y%m%d%H%M%S%f'));archive.mkdir(mode=0o700)
  for active in folder.glob('zz-vmalpha*'):
   if active.suffix in ('.cert','.key'):active.rename(archive/active.name)
  for p in current.iterdir():
   if p.is_file() and p.suffix in ('.cert','.key'):shutil.copy2(p,folder/p.name)
  run('restorecon','-R',str(folder));run('systemd-run','--quiet','--collect','--on-active=3s','--timer-property=AccuracySec=1s','/usr/bin/systemctl','try-restart','cockpit.service');return 'Previous certificate restored; management sessions restart in 3 seconds. Reconnect to use it'
 raise ValueError('Unsupported security operation')
