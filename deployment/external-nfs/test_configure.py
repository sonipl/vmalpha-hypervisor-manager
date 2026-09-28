import importlib.util,pathlib,unittest
spec=importlib.util.spec_from_file_location('installer',pathlib.Path(__file__).parent/'files/configure-external-nfs.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
class Validation(unittest.TestCase):
 def test_opt_in_and_paths(self):
  base=dict(enabled=True,id='external-nfs',server='192.168.71.50',export='/nfs')
  self.assertEqual(m.validate(base),'/var/lib/vmalpha/datastores/external-nfs')
  for delta in ({'enabled':False},{'id':'../etc'},{'server':'host;cmd'},{'export':'/nfs/../etc'},{'version':'2'}):
   with self.assertRaises(ValueError):m.validate(dict(base,**delta))
  m.validate(dict(base,version='4.1'))
if __name__=='__main__':unittest.main()
