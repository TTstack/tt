import pathlib,sqlite3,subprocess
assert b'/tmp/tt-go-port-6426/lab-server.py\x00' in pathlib.Path('/proc/1/cmdline').read_bytes(), 'run only inside the owned lab'
root=pathlib.Path('/tmp/tt-go-port-6426')
for line in subprocess.check_output(['ps','-eo','stat=,comm='],text=True).splitlines():
 state,name=line.split(maxsplit=1)
 assert state.startswith('Z') or not name.startswith(('qemu-system','firecracker','tt-agent','tt-ctl','dockerd','containerd')),line
for directory in ['/home/ttstack/run',str(root/'agent2-run')]:assert not list(pathlib.Path(directory).iterdir()),directory
for directory,filename in [('native-agent','agent.db'),('agent2-data','agent.db'),('native-ctl','ctl.db'),('mixed-ctl','ctl.db'),('docker-ctl','ctl.db'),('systemd-root/home/ttstack/data','agent.db'),('systemd-root/home/ttstack/ctl','ctl.db')]:
 path=root/directory/filename
 if not path.exists():continue
 db=sqlite3.connect('file:'+str(path)+'?mode=ro',uri=True)
 assert db.execute('select count(*) from vms').fetchone()[0]==0,str(path)
 db.close()
for directory in ['/home/ttstack/store/run1','/home/ttstack/store/run2']:
 assert not list(pathlib.Path(directory).glob('clone-*')),directory
subprocess.run(['umount','/home/ttstack/store'],check=True)
loops=subprocess.check_output(['losetup','--noheadings','--output','NAME','--associated',str(root/'reflink.img')],text=True).strip()
assert not loops,'owned loop device still attached'
(root/'reflink.img').unlink()
print('Owned Btrfs image unmounted and loop device removed after empty-inventory checks')
print(pathlib.Path('/sys/fs/cgroup/memory.events').read_text())
