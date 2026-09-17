#!/usr/bin/python3
"""Enable monitoring for an existing authorized Ceph cluster; never creates OSDs."""
import grp,json,os,pathlib,subprocess
root=pathlib.Path('/etc/vmalpha-monitoring')
key=root/'ceph.client.vmalpha-monitoring.keyring'
# Keep key material in memory and a protected file, never stdout or argv.
p=subprocess.run(['ceph','auth','get-or-create','client.vmalpha-monitoring','mon','allow r','mgr','allow r'],capture_output=True,text=True,check=True)
fd=os.open(key,os.O_WRONLY|os.O_CREAT|os.O_TRUNC,0o640)
with os.fdopen(fd,'w') as f:f.write(p.stdout)
os.chown(key,0,grp.getgrnam('vmalpha-monitor').gr_gid)
source=pathlib.Path(__file__).resolve().parent/'ceph-exporter.py'
subprocess.run(['install','-m','0755',str(source),'/usr/libexec/vmalpha-ceph-exporter'],check=True)
unit=pathlib.Path('/etc/systemd/system/vmalpha-ceph-exporter.service')
unit.write_text('''[Unit]
Description=VM Alpha Storage metrics collector
After=network-online.target
[Service]
User=vmalpha-monitor
Group=vmalpha-monitor
ExecStart=/usr/libexec/vmalpha-ceph-exporter
Restart=on-failure
MemoryMax=192M
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
[Install]
WantedBy=multi-user.target
''')
# JSON is valid YAML; keep all existing scrape jobs when adding this collector.
import yaml
config=root/'prometheus.yml';data=yaml.safe_load(config.read_text())
data['scrape_configs']=[x for x in data['scrape_configs'] if x['job_name']!='ceph']+[{'job_name':'ceph','sample_limit':20000,'static_configs':[{'targets':['127.0.0.1:9178']}],'authorization':{'credentials_file':str(root/'scrape-token')}}]
temporary=config.with_suffix('.new');temporary.write_text(json.dumps(data,indent=2))
subprocess.run(['/usr/local/bin/promtool','check','config',str(temporary)],check=True)
temporary.replace(config)
subprocess.run(['restorecon','-RF',str(root),str(unit),'/usr/libexec/vmalpha-ceph-exporter'],check=True)
subprocess.run(['systemctl','daemon-reload'],check=True)
subprocess.run(['systemctl','enable','--now','vmalpha-ceph-exporter'],check=True)
subprocess.run(['systemctl','restart','vmalpha-prometheus'],check=True)
print('Ceph monitoring configured with read-only cluster credentials')
