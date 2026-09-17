#!/usr/bin/env python3
"""Exercise initialization branches with fake libvirt/systemctl, never a live host."""
import os
import pathlib
import subprocess
import tempfile

source = pathlib.Path(__file__).resolve().parent
mock = '''#!/usr/bin/env python3
import os,pathlib,sys
r=pathlib.Path(os.environ['MOCK_ROOT']); args=sys.argv[1:]
with (r/'calls').open('a') as f: f.write(pathlib.Path(sys.argv[0]).name+' '+' '.join(args)+'\\n')
if pathlib.Path(sys.argv[0]).name!='virsh': sys.exit(0)
if args[:2]==['-c','qemu:///system']: args=args[2:]
cmd=args[0]
if cmd=='list': sys.exit(0)
if cmd=='net-info':
    if not (r/'net').exists(): sys.exit(1)
    print('Active: yes' if (r/'net-active').exists() else 'Active: no')
elif cmd=='net-define': (r/'net').touch()
elif cmd=='net-start': (r/'net-active').touch()
elif cmd=='pool-info':
    if not (r/'pool').exists(): sys.exit(1)
    print('State: running' if (r/'pool-active').exists() else 'State: inactive')
elif cmd=='pool-define-as': (r/'pool').touch()
elif cmd=='pool-start': (r/'pool-active').touch()
'''
for existing in (False, True):
    with tempfile.TemporaryDirectory(prefix='vmalpha-firstboot-') as temporary:
        root = pathlib.Path(temporary)
        (root/'bin').mkdir()
        for name in ('systemctl', 'virsh', 'restorecon'):
            path = root/'bin'/name
            path.write_text(mock)
            path.chmod(0o755)
        release = root/'etc/vmalpha-release'
        release.parent.mkdir()
        release.touch()
        if existing:
            for name in ('net','net-active','pool','pool-active'): (root/name).touch()
        script = (source/'firstboot.sh').read_text()
        for prefix in ('/etc/vmalpha-release','/var/lib/vmalpha','/var/lib/libvirt/images','/usr/share/vmalpha'):
            script = script.replace(prefix, str(root)+prefix)
        env = dict(os.environ, PATH=str(root/'bin')+os.pathsep+os.environ['PATH'], MOCK_ROOT=str(root))
        subprocess.run(['bash','-c',script], env=env, check=True, capture_output=True)
        assert (root/'var/lib/vmalpha/initialized').exists()
        calls = (root/'calls').read_text()
        if existing:
            assert 'net-define' not in calls and 'pool-define-as' not in calls
            assert 'net-start' not in calls and 'pool-start' not in calls
        else:
            assert 'net-define' in calls and 'pool-define-as' in calls
            assert (root/'net-active').exists() and (root/'pool-active').exists()
        assert 'net-autostart' in calls and 'pool-autostart' in calls
        again = subprocess.run(['bash','-c',script], env=env, capture_output=True)
        assert again.returncode != 0 and (root/'calls').read_text() == calls
        print('PASS:', 'preserve existing active resources' if existing else 'initialize missing resources', 'and completion guard')
