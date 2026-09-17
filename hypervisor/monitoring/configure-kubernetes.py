#!/usr/bin/python3
"""Add authenticated monitoring to an existing cluster without initializing one."""
import crypt,grp,json,os,pathlib,secrets,subprocess,yaml
root=pathlib.Path('/etc/vmalpha-monitoring');ns='vmalpha-monitoring';gid=grp.getgrnam('vmalpha-monitor').gr_gid
def k(*args,input=None):
 p=subprocess.run(['/usr/local/bin/kubectl','--kubeconfig=/etc/kubernetes/admin.conf',*args],input=input,text=True,capture_output=True,timeout=30)
 if p.returncode:raise RuntimeError('Kubernetes monitoring configuration failed; inspect the cluster without printing credential-bearing requests')
 return p.stdout
def apply(obj):k('apply','-f','-',input=json.dumps(obj))
k('get','nodes')
apply({'apiVersion':'v1','kind':'Namespace','metadata':{'name':ns}})
for name in ('vmalpha-prometheus','vmalpha-ksm'):
 apply({'apiVersion':'v1','kind':'ServiceAccount','metadata':{'name':name,'namespace':ns}})
roles={
 'vmalpha-prometheus':[{'apiGroups':[''],'resources':['nodes/metrics'],'verbs':['get']}],
 'vmalpha-ksm':[{'apiGroups':[''],'resources':['nodes','pods','persistentvolumes','persistentvolumeclaims'],'verbs':['list','watch']},{'apiGroups':['apps'],'resources':['deployments','statefulsets','daemonsets'],'verbs':['list','watch']}]
}
for name,rules in roles.items():
 apply({'apiVersion':'rbac.authorization.k8s.io/v1','kind':'ClusterRole','metadata':{'name':name},'rules':rules})
 apply({'apiVersion':'rbac.authorization.k8s.io/v1','kind':'ClusterRoleBinding','metadata':{'name':name},'roleRef':{'apiGroup':'rbac.authorization.k8s.io','kind':'ClusterRole','name':name},'subjects':[{'kind':'ServiceAccount','name':name,'namespace':ns}]})
password=root/'ksm-password'
if not password.exists():
 fd=os.open(password,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o640)
 with os.fdopen(fd,'w') as f:f.write(secrets.token_urlsafe(36)+'\n')
os.chown(password,0,gid)
cert=root/'ksm.crt';key=root/'ksm.key';dns='vmalpha-ksm.'+ns+'.svc'
if not cert.exists():
 subprocess.run(['openssl','req','-x509','-newkey','rsa:3072','-nodes','-days','365','-subj','/CN='+dns,'-addext','subjectAltName=DNS:'+dns,'-addext','basicConstraints=critical,CA:FALSE','-keyout',str(key),'-out',str(cert)],check=True,capture_output=True)
key.chmod(0o600);cert.chmod(0o644)
web={'tls_server_config':{'cert_file':'/etc/metrics/tls.crt','key_file':'/etc/metrics/tls.key','min_version':'TLS12'},'basic_auth_users':{'vmalpha':crypt.crypt(password.read_text().strip(),crypt.mksalt(crypt.METHOD_BLOWFISH))}}
apply({'apiVersion':'v1','kind':'Secret','metadata':{'name':'vmalpha-ksm-tls','namespace':ns},'stringData':{'tls.crt':cert.read_text(),'tls.key':key.read_text(),'web.yml':json.dumps(web)}})
labels={'app':'vmalpha-ksm'}
container={'name':'metrics','image':'registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.20.0','imagePullPolicy':'IfNotPresent','args':['--resources=nodes,pods,persistentvolumes,persistentvolumeclaims,deployments,statefulsets,daemonsets','--tls-config=/etc/metrics/web.yml','--telemetry-host=127.0.0.1'],'ports':[{'name':'https','containerPort':8080}],'resources':{'requests':{'cpu':'20m','memory':'64Mi'},'limits':{'cpu':'250m','memory':'256Mi'}},'securityContext':{'allowPrivilegeEscalation':False,'readOnlyRootFilesystem':True,'capabilities':{'drop':['ALL']}},'volumeMounts':[{'name':'tls','mountPath':'/etc/metrics','readOnly':True}]}
apply({'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'vmalpha-ksm','namespace':ns},'spec':{'replicas':1,'selector':{'matchLabels':labels},'template':{'metadata':{'labels':labels},'spec':{'serviceAccountName':'vmalpha-ksm','securityContext':{'runAsNonRoot':True,'runAsUser':65534,'runAsGroup':65534,'fsGroup':65534,'seccompProfile':{'type':'RuntimeDefault'}},'containers':[container],'volumes':[{'name':'tls','secret':{'secretName':'vmalpha-ksm-tls','defaultMode':288}}]}}}})
apply({'apiVersion':'v1','kind':'Service','metadata':{'name':'vmalpha-ksm','namespace':ns},'spec':{'selector':labels,'ports':[{'port':8080,'targetPort':'https'}]}})
ip=json.loads(k('get','service','vmalpha-ksm','-n',ns,'-o','json'))['spec']['clusterIP']
subprocess.run(['install','-m','0644','/etc/kubernetes/pki/ca.crt',str(root/'kubernetes-ca.crt')],check=True)
source=pathlib.Path(__file__).resolve().parent/'refresh-kubernetes.py'
subprocess.run(['install','-m','0755',str(source),'/usr/libexec/vmalpha-refresh-monitoring'],check=True)
subprocess.run(['/usr/libexec/vmalpha-refresh-monitoring'],check=True)
config=root/'prometheus.yml';data=yaml.safe_load(config.read_text())
data['scrape_configs']=[x for x in data['scrape_configs'] if x['job_name'] not in ('kubelet','kube-state-metrics')]+[
 {'job_name':'kubelet','scheme':'https','metrics_path':'/metrics/cadvisor','sample_limit':100000,'file_sd_configs':[{'files':[str(root/'kubelet-targets.json')],'refresh_interval':'30s'}],'authorization':{'credentials_file':str(root/'kubelet-token')},'tls_config':{'ca_file':str(root/'kubernetes-ca.crt')}},
 {'job_name':'kube-state-metrics','scheme':'https','sample_limit':50000,'static_configs':[{'targets':[ip+':8080']}],'tls_config':{'ca_file':str(cert),'server_name':dns},'basic_auth':{'username':'vmalpha','password_file':str(password)}}]
temporary=config.with_suffix('.new');temporary.write_text(json.dumps(data,indent=2));subprocess.run(['/usr/local/bin/promtool','check','config',str(temporary)],check=True);temporary.replace(config)
pathlib.Path('/etc/systemd/system/vmalpha-refresh-monitoring.service').write_text('''[Unit]
Description=Refresh limited Kubernetes monitoring access
After=network-online.target
[Service]
Type=oneshot
ExecStart=/usr/libexec/vmalpha-refresh-monitoring
''')
pathlib.Path('/etc/systemd/system/vmalpha-refresh-monitoring.timer').write_text('''[Unit]
Description=Discover Kubernetes nodes and rotate monitoring token when due
[Timer]
OnBootSec=45s
OnUnitActiveSec=1min
Persistent=true
[Install]
WantedBy=timers.target
''')
subprocess.run(['restorecon','-RF',str(root),'/usr/libexec/vmalpha-refresh-monitoring','/etc/systemd/system/vmalpha-refresh-monitoring.service','/etc/systemd/system/vmalpha-refresh-monitoring.timer'],check=True)
subprocess.run(['systemctl','daemon-reload'],check=True)
subprocess.run(['systemctl','enable','--now','vmalpha-refresh-monitoring.timer'],check=True)
subprocess.run(['systemctl','restart','vmalpha-prometheus'],check=True)
print('Kubelet and workload-state monitoring configured with verified TLS and limited credentials')
