import io,json,pathlib,sys,unittest
from unittest.mock import patch
sys.path.insert(0,str(pathlib.Path(__file__).resolve().parents[1]))
import vmalpha_monitoring as monitoring

class AlertBoundaryTests(unittest.TestCase):
 def test_vm_reader_cannot_access_host_alerts(self):
  with patch.object(monitoring.access,'admin',return_value=False),patch.object(monitoring.urllib.request,'urlopen') as fetch:
   with self.assertRaises(ValueError):monitoring.handle({'scope':'vm','metric':'alerts','name':'owned-guest'})
   fetch.assert_not_called()
 def check_response(self,response):
  with patch.object(monitoring.access,'admin',return_value=True),patch.object(pathlib.Path,'read_text',return_value='test-only'),patch.object(monitoring.urllib.request,'urlopen',return_value=io.BytesIO(response)):
   return monitoring.alerts()
 def test_backend_failure_is_not_resolved_alerts(self):
  for body in (b'{"status":"error"}',b'{"status":"success","data":{"alerts":null}}',b'x'*(1024*1024+1)):
   result=self.check_response(body);self.assertFalse(result['available']);self.assertIn('not be assumed resolved',result['message'])
 def test_real_pending_and_firing_states_are_preserved(self):
  raw={'status':'success','data':{'alerts':[{'state':'pending','labels':{'alertname':'CollectorUnavailable','instance':'node02'},'annotations':{'summary':'collector down'},'activeAt':'2026-09-16T17:00:00Z'},{'state':'firing','labels':{'alertname':'DatastoreLowSpace'},'annotations':{}}]}}
  result=self.check_response(json.dumps(raw).encode());self.assertTrue(result['available']);self.assertEqual([a['state'] for a in result['alerts']],['pending','firing']);self.assertEqual(result['alerts'][0]['labels']['instance'],'node02')
 def test_network_error_is_unavailable(self):
  with patch.object(monitoring.access,'admin',return_value=True),patch.object(pathlib.Path,'read_text',return_value='test-only'),patch.object(monitoring.urllib.request,'urlopen',side_effect=OSError('unreachable')):
   self.assertFalse(monitoring.alerts()['available'])

class WorkloadFilterTests(unittest.TestCase):
 def test_selected_workload_is_literal_match(self):
  query=monitoring.container_query('cpu','default','alpha-storage-test-0')
  self.assertIn('namespace="default",pod="alpha-storage-test-0",container!=""',query)
  self.assertNotIn('FILTER',query)
 def test_query_injection_is_rejected(self):
  for value in ('default"} or vector(1)', 'default\n', 'a'*254):
   with self.assertRaises(ValueError):monitoring.container_query('cpu',value,'')
 def test_all_workloads_preserve_required_filters(self):
  query=monitoring.container_query('memory')
  self.assertIn('{container!="",container!="POD"}',query)
 def test_cluster_ready_count_remains_cluster_wide(self):
  self.assertEqual(monitoring.container_query('nodes_ready','default'),monitoring.CONTAINERS['nodes_ready'][0])

if __name__=='__main__':unittest.main()
