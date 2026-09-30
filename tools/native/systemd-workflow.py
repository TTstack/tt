import os,pathlib,shlex,shutil,signal,subprocess,time
assert b'/tmp/tt-go-port-6426/lab-server.py\x00' in pathlib.Path('/proc/1/cmdline').read_bytes(), 'run only inside the owned lab'
base=pathlib.Path('/tmp/tt-go-port-6426');root=base/'systemd-root';pid=int((base/'systemd-lab.pid').read_text())
def owned():
 assert pathlib.Path(f'/proc/{pid}/comm').read_text().strip()=='systemd'
 assert os.readlink(f'/proc/{pid}/root')==str(root)
 assert os.readlink(f'/proc/{pid}/ns/pid')!=os.readlink('/proc/1/ns/pid')
 assert '/systemd-test/' in pathlib.Path(f'/proc/{pid}/cgroup').read_text()
owned()
# A normal root login supplies HOME from passwd; do not repurpose HOME in tests.
(root/'etc/pam.d').mkdir(exist_ok=True)
for name in ['su','su-l']:(root/'etc/pam.d'/name).write_text('auth sufficient pam_rootok.so\naccount required pam_permit.so\nsession required pam_permit.so\n')
def enter(args,key=None):
 owned();environment=dict(os.environ)
 if key:environment['TT_API_KEY']=key
 result=subprocess.run(['nsenter','--target',str(pid),'--mount','--net','--pid','--cgroup','--root','--wd','su','--login','--whitelist-environment=TT_API_KEY','root','--command',shlex.join(args)],env=environment,capture_output=True,text=True,timeout=240)
 if result.returncode!=0:raise RuntimeError((args,result.stdout,result.stderr))
 return result.stdout
def ready(key):
 deadline=time.monotonic()+40
 code="import os,urllib.request; h={'Authorization':'Bearer '+os.environ['TT_API_KEY']}; [urllib.request.urlopen(urllib.request.Request('http://127.0.0.1:'+p+u,headers=h),timeout=2).read() for p,u in [('9100','/api/info'),('9200','/api/status')]]"
 while True:
  try:enter(['python3','-c',code],key);return
  except RuntimeError:
   if time.monotonic()>deadline:raise
   time.sleep(.2)
enter(['/build/bin/tt','deploy','all','--release-dir','/build/bin'])
key=(root/'opt/ttstack/etc/api-key').read_text().strip()
assert (root/'opt/ttstack/etc/api-key').stat().st_mode&0o777==0o600
ready(key)
enter(['/opt/ttstack/bin/tt','config','127.0.0.1:9200'],key)
shutil.copyfile(base/'images/alpine-cloud',root/'home/ttstack/images/alpine-cloud')
enter(['/opt/ttstack/bin/tt','image','create','alpine-cloud'])
enter(['/opt/ttstack/bin/tt','host','add','127.0.0.1:9100'])
deadline=time.monotonic()+50
while 'alpine-cloud' not in enter(['/opt/ttstack/bin/tt','image','list']):
 if time.monotonic()>deadline:raise RuntimeError('image catalog did not refresh')
 time.sleep(.5)
enter(['/opt/ttstack/bin/tt','env','create','demo','--image','alpine-cloud','--engine','qemu','--cpu','1','--mem','256','--disk','2048','--lifetime','0','--ssh-key','/fixture.pub'])
assert 'demo' in enter(['/opt/ttstack/bin/tt','env','show','demo'])
assert enter(['systemctl','show','tt-agent','--property=KillMode','--value']).strip()=='process'
enter(['/build/bin/tt','deploy','all','--release-dir','/build/bin'])
assert (root/'opt/ttstack/etc/api-key').read_text().strip()==key
ready(key)
assert 'demo' in enter(['/opt/ttstack/bin/tt','env','show','demo'])
enter(['/opt/ttstack/bin/tt','env','stop','demo']);enter(['/opt/ttstack/bin/tt','env','start','demo']);enter(['/opt/ttstack/bin/tt','env','delete','demo'])
enter(['systemctl','stop','tt-agent','tt-ctl'])
owned();os.kill(pid,signal.SIGKILL)
print('Private Linux systemd: deploy all, shared key reuse, login config, image validation, host registration, README lifecycle and live deployment restart passed')
