#!/usr/bin/python3
"""Approve only an explicitly identified node's verified kubelet-serving CSR."""
import argparse,base64,copy,datetime,ipaddress,json,re,subprocess,urllib.parse


def matches(csr,node,address):
 spec=csr.get('spec',{})
 if spec.get('signerName')!='kubernetes.io/kubelet-serving' or spec.get('username')!='system:node:'+node or csr.get('status',{}).get('conditions'):return False
 if set(spec.get('groups',[]))!={'system:nodes','system:authenticated'}:raise ValueError('Unexpected requesting groups')
 usages=set(spec.get('usages',[]))
 if 'server auth' not in usages or not usages<={'server auth','digital signature','key encipherment'}:raise ValueError('Unexpected serving certificate usage')
 request=base64.b64decode(spec['request'],validate=True)
 if len(request)>32768:raise ValueError('CSR exceeds size limit')
 result=subprocess.run(['openssl','req','-verify','-noout','-subject','-text'],input=request,capture_output=True,check=True)
 lines=result.stdout.decode().splitlines()
 subjects=[line.strip() for line in lines if line.strip().startswith('Subject:')]
 if subjects!=['Subject: O=system:nodes, CN=system:node:'+node]:raise ValueError('CSR subject differs from selected node identity')
 sans=[i for i,line in enumerate(lines) if 'X509v3 Subject Alternative Name:' in line]
 if len(sans)!=1 or sans[0]+1>=len(lines):raise ValueError('Exactly one SAN extension is required')
 if set(lines[sans[0]+1].strip().split(', '))!={'DNS:'+node,'IP Address:'+address}:raise ValueError('CSR SANs differ from selected node address/name')
 return True


def approval_document(csr):
 metadata=csr.get('metadata',{})
 if not metadata.get('uid') or not metadata.get('resourceVersion'):raise ValueError('Versioned CSR identity is required')
 result=copy.deepcopy(csr)
 result.setdefault('status',{})['conditions']=[{'type':'Approved','status':'True','reason':'VMAlphaVerifiedNode','message':'Verified requested node identity, address and server usage','lastUpdateTime':datetime.datetime.now(datetime.timezone.utc).isoformat()}]
 return result


def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--node',required=True);parser.add_argument('--address',required=True);parser.add_argument('--approve',action='store_true');args=parser.parse_args()
 if not re.fullmatch(r'[a-z0-9][a-z0-9.-]{0,251}[a-z0-9]|[a-z0-9]',args.node):raise ValueError('Invalid node name')
 address=str(ipaddress.ip_address(args.address));k=['/usr/local/bin/kubectl','--kubeconfig=/etc/kubernetes/admin.conf']
 node=json.loads(subprocess.check_output(k+['get','node',args.node,'-o','json']))
 ips={v['address'] for v in node.get('status',{}).get('addresses',[]) if v['type']=='InternalIP'}
 if ips!={address}:raise ValueError('Registered node addresses differ from selected address')
 csrs=json.loads(subprocess.check_output(k+['get','csr','-o','json']))['items'];count=0
 for csr in csrs:
  if not matches(csr,args.node,address):continue
  name=csr['metadata']['name'];count+=1
  if args.approve:
   # PUT preserves the UID/resourceVersion just inspected; replacement races fail closed.
   resource='/apis/certificates.k8s.io/v1/certificatesigningrequests/'+urllib.parse.quote(name,safe='')+'/approval'
   subprocess.run(k+['replace','--raw',resource,'-f','-'],input=json.dumps(approval_document(csr)),text=True,capture_output=True,check=True)
   print('Approved verified CSR:',name)
  else:print('Verified pending CSR:',name)
 print(str(count)+' matching serving CSR(s) '+('approved' if args.approve else 'checked; no approval performed'))

if __name__=='__main__':
 try:main()
 except (ValueError,KeyError,TypeError,subprocess.SubprocessError) as error:raise SystemExit('Serving CSR verification failed; no unverified request was approved')
