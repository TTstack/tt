"""Launch an isolated systemd fixture, then run its deployment workflow."""
import os,pathlib,pty,subprocess,threading,time
assert b'/tmp/tt-go-port-6426/lab-server.py\x00' in pathlib.Path('/proc/1/cmdline').read_bytes(), 'run only inside the owned lab'
base=pathlib.Path('/tmp/tt-go-port-6426');root=base/'systemd-root';pidfile=base/'systemd-lab.pid'
assert pathlib.Path('/proc/1/comm').read_text().strip()=='python3'
assert not root.exists() and not pidfile.exists()
log=open(base/'systemd-manager.log','w');master,slave=pty.openpty()
manager=subprocess.Popen(['unshare','--mount','--net','--uts','--ipc','/bin/bash',str(base/'systemd-root.sh')],stdin=slave,stdout=slave,stderr=slave,start_new_session=True,env=dict(os.environ,TT_TEST_CONSOLE=os.ttyname(slave)))
os.close(slave)
def capture():
 while True:
  try:data=os.read(master,4096)
  except OSError:return
  if not data:return
  log.write(data.decode(errors='replace'));log.flush()
reader=threading.Thread(target=capture,daemon=True);reader.start()
deadline=time.monotonic()+40
while True:
 if manager.poll() is not None:raise RuntimeError('systemd fixture exited; inspect systemd-manager.log')
 if pidfile.exists():
  pid=int(pidfile.read_text())
  if pathlib.Path(f'/proc/{pid}/comm').read_text().strip()=='systemd':
   assert os.readlink(f'/proc/{pid}/root')==str(root)
   assert os.readlink(f'/proc/{pid}/ns/pid')!=os.readlink('/proc/1/ns/pid')
   result=subprocess.run(['nsenter','--target',str(pid),'--mount','--net','--pid','--cgroup','--root','--wd','systemctl','is-active','tt-validation.target'],capture_output=True)
   if result.returncode==0:break
 if time.monotonic()>deadline:raise RuntimeError('systemd fixture did not become ready')
 time.sleep(.2)
# On failure retain only this owned fixture for inspection, without killing a
# process whose guest/storage operation might still need diagnosis.
subprocess.run(['python3',str(base/'systemd-workflow.py')],check=True)
manager.wait(timeout=20)
reader.join(timeout=2)
os.close(master)
log.close()
