"""VM Alpha Container Platform: fixed, administrator-only controller operations."""
import json,pathlib,re,shutil
OPS={'container-list','container-create','container-action','container-details','container-logs'}
READ_OPS={'container-list','container-details','container-logs'}
KUBECONFIG='/etc/kubernetes/admin.conf'
def ident(value):
 if not isinstance(value,str) or len(value)>63 or not re.fullmatch(r'[a-z0-9](?:[a-z0-9-]*[a-z0-9])?',value):raise ValueError('Use a lowercase Kubernetes name (maximum 63 characters)')
 return value
def handle(op,a,run):
 if not shutil.which('kubectl') or not pathlib.Path(KUBECONFIG).is_file():
  if op=='container-list':return {'ready':False,'message':'VM Alpha Container Platform has not been initialized.','items':[]}
  raise ValueError('Initialize VM Alpha Container Platform before managing workloads')
 def k(*args,input=None):return run('kubectl','--kubeconfig='+KUBECONFIG,'--request-timeout=20s',*args,timeout=30,input=input)
 ns=ident(a.get('namespace','default'))
 if op=='container-list':
  payload=json.loads(k('get','deployments,statefulsets','--all-namespaces','-o','json'))
  return {'ready':True,'items':[{'name':v['metadata']['name'],'namespace':v['metadata']['namespace'],'kind':v['kind'],'replicas':v['spec'].get('replicas',1),'ready':v.get('status',{}).get('readyReplicas',0),'images':[c['image'] for c in v['spec']['template']['spec']['containers']]} for v in payload['items']]}
 n=ident(a['name']);kind=a.get('kind','Deployment')
 if kind not in ('Deployment','StatefulSet'):raise ValueError('Select Deployment or StatefulSet')
 resource=kind.lower()+'/'+n
 if op=='container-create':
  image=a['image']
  if not isinstance(image,str) or len(image)>512 or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._:/@-]*',image):raise ValueError('Invalid container image reference')
  replicas=int(a.get('replicas',1))
  if not 0<=replicas<=100:raise ValueError('Replicas must be between 0 and 100')
  labels={'app.kubernetes.io/name':n,'app.kubernetes.io/managed-by':'vmalpha'}
  spec={'replicas':replicas,'selector':{'matchLabels':labels},'template':{'metadata':{'labels':labels},'spec':{'containers':[{'name':n,'image':image,'resources':{'requests':{'cpu':'100m','memory':'64Mi'},'limits':{'memory':'512Mi'}}}]}}}
  command=a.get('command')
  if isinstance(command,str) and command.strip():
   try:command=json.loads(command)
   except ValueError:raise ValueError('Command must be a JSON string array')
  if command:
   if not isinstance(command,list) or not 1<=len(command)<=32 or any(not isinstance(v,str) or len(v)>4096 or '\x00' in v for v in command):raise ValueError('Command must be a bounded string array')
   spec['template']['spec']['containers'][0]['command']=command
  if kind=='StatefulSet':
   size=int(a.get('storageGiB',1))
   if not 1<=size<=10240:raise ValueError('Storage must be between 1 and 10240 GiB')
   sc=ident(a['storageClass']);k('get','storageclass',sc,'-o','name')
   spec['serviceName']=n;spec['template']['spec']['containers'][0]['volumeMounts']=[{'name':'data','mountPath':'/data'}]
   spec['volumeClaimTemplates']=[{'metadata':{'name':'data'},'spec':{'accessModes':['ReadWriteOnce'],'storageClassName':sc,'resources':{'requests':{'storage':str(size)+'Gi'}}}}]
  manifest={'apiVersion':'apps/v1','kind':kind,'metadata':{'name':n,'namespace':ns,'labels':labels},'spec':spec}
  if kind=='StatefulSet':
   service={'apiVersion':'v1','kind':'Service','metadata':{'name':n,'namespace':ns,'labels':labels},'spec':{'clusterIP':'None','selector':labels,'ports':[{'name':'workload','port':80,'targetPort':80}]}}
   # A headless service provides stable identity. Keep any partial result visible on failure.
   result=k('create','-f','-',input=json.dumps(manifest))
   obj=json.loads(k('-n',ns,'get','statefulset',n,'-o','json'))
   service['metadata']['ownerReferences']=[{'apiVersion':'apps/v1','kind':'StatefulSet','name':n,'uid':obj['metadata']['uid'],'controller':True,'blockOwnerDeletion':True}]
   k('create','-f','-',input=json.dumps(service))
   return result
  return k('create','-f','-',input=json.dumps(manifest))
 if op=='container-action':
  action=a['action']
  if action=='delete':return k('-n',ns,'delete',resource,'--wait=false')+'; persistent volume claims retained'
  if action=='restart':return k('-n',ns,'rollout','restart',resource)
  if action in ('start','stop','scale'):
   replicas=0 if action=='stop' else int(a.get('replicas',1))
   if not 0<=replicas<=100:raise ValueError('Replicas must be between 0 and 100')
   return k('-n',ns,'scale',resource,'--replicas='+str(replicas))
  raise ValueError('Unsupported workload action')
 obj=json.loads(k('-n',ns,'get',resource,'-o','json'))
 if op=='container-details':return obj
 if op=='container-logs':
  # Resolve an actual pod owned by this controller; never execute arbitrary selectors from a caller.
  pods=json.loads(k('-n',ns,'get','pods','-o','json'))['items'];uids={obj['metadata']['uid']}
  if kind=='Deployment':
   rs=json.loads(k('-n',ns,'get','replicasets','-o','json'))['items']
   uids={r['metadata']['uid'] for r in rs if any(o['uid'] in uids for o in r['metadata'].get('ownerReferences',[]))}
  owned=[p for p in pods if any(o['uid'] in uids for o in p['metadata'].get('ownerReferences',[]))]
  if not owned:return 'No pods currently owned by this workload.'
  return k('-n',ns,'logs',owned[0]['metadata']['name'],'--all-containers=true','--tail=200','--limit-bytes=65536')
 raise ValueError('Unsupported container operation')
