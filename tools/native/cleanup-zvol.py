import pathlib,sqlite3,subprocess
assert b'/tmp/tt-go-port-6426/lab-server.py\x00' in pathlib.Path('/proc/1/cmdline').read_bytes(), 'run only inside the owned lab'
root=pathlib.Path('/tmp/tt-go-port-6426')
assert pathlib.Path('/proc/1/comm').read_text().strip()=='python3'
for line in subprocess.check_output(['ps','-eo','stat=,comm='],text=True).splitlines():
 state,name=line.split(maxsplit=1)
 assert state.startswith('Z') or not name.startswith(('qemu-system','firecracker','tt-agent','tt-ctl')),line
assert not list(pathlib.Path('/home/ttstack/run').iterdir())
for name,filename in [('native-agent','agent.db'),('native-ctl','ctl.db')]:
 db=sqlite3.connect('file:'+str(root/name/filename)+'?mode=ro',uri=True)
 assert db.execute('select count(*) from vms').fetchone()[0]==0
names=set(subprocess.check_output(['zfs','list','-H','-o','name','-r','ttgo6426'],text=True).splitlines())
assert names=={'ttgo6426','ttgo6426/images','ttgo6426/images/alpine-cloud','ttgo6426/images/fc-ssh','ttgo6426/images/fc-ssh/rootfs','ttgo6426/runtime'},names
status=subprocess.check_output(['zpool','status','-P','ttgo6426'],text=True)
assert str(root/'zpool.img') in status
subprocess.run(['zpool','destroy','ttgo6426'],check=True)
(root/'zpool.img').unlink()
print('Owned ZFS test pool removed after empty-inventory and vdev checks')
