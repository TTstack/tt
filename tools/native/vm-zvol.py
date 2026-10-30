"""Bounded native VM/reflink acceptance inside the private Linux lab."""
import http.client,json,os,pathlib,subprocess,time,uuid
assert b'/tmp/tt-go-port-6426/lab-server.py\x00' in pathlib.Path('/proc/1/cmdline').read_bytes(), 'run only inside the owned lab'
root=pathlib.Path('/tmp/tt-go-port-6426');go=root/'bin';rust=go if os.environ.get('TT_NATIVE_GO_ONLY')=='1' else root/'rust-target/debug';key=uuid.uuid4().hex;processes=[]
def call(port,method,path,body=None):
 connection=http.client.HTTPConnection('127.0.0.1',port,timeout=240);headers={'Authorization':'Bearer '+key}
 if body is not None:headers['Content-Type']='application/json';body=json.dumps(body)
 connection.request(method,path,body,headers);response=connection.getresponse();data=response.read();connection.close();result=json.loads(data)
 if not (200<=response.status<300 and result['ok']):raise RuntimeError((method,path,response.status,result))
 return result.get('data')
def start(binary,args,port,ready,label):
 log=open(root/(label+'.log'),'w');process=subprocess.Popen([str(binary)]+args,env=dict(os.environ,TT_API_KEY=key,GOMAXPROCS='2',DOCKER_HOST='unix:///run/tt-native-docker.sock'),stdout=log,stderr=log);processes.append((process,log));deadline=time.monotonic()+40
 while True:
  if process.poll() is not None:raise RuntimeError(label+' exited; inspect its log')
  try:call(port,'GET',ready);return process
  except (OSError,http.client.HTTPException):pass
  if time.monotonic()>deadline:raise RuntimeError(label+' not ready')
  time.sleep(.2)
def stop(process):
 process.terminate();process.wait(timeout=45)
 if process.returncode!=0:raise RuntimeError('service shutdown failed')
def cli(binary,command):
 for attempt in range(4):
  result=subprocess.run([str(binary),'--server','127.0.0.1:19200']+command,env=dict(os.environ,TT_API_KEY=key,GOMAXPROCS='2'),capture_output=True,text=True,timeout=240)
  if result.returncode==0:return result.stdout
  if command[:2]==['env','stop'] and 'has no readable identity; resources retained' in result.stderr and attempt<3:
   call(19200,'GET','/api/envs/'+command[2])
   print('Stop retained an uncertain process identity; inspect and retry the same stop',flush=True)
   time.sleep(.25)
   continue
  raise RuntimeError((command,result.stderr))

def ssh(vm,command,external=False,allow_failure=False):
 prefix=['ip','netns','exec','tt-peer'] if external else []
 address='198.18.0.1' if external else vm['ip'];port=vm['ssh']['port'] if external else 22
 args=prefix+['ssh','-i',str(root/'guest-key'),'-p',str(port),'-o','IdentitiesOnly=yes','-o','BatchMode=yes','-o','ConnectTimeout=3','-o','StrictHostKeyChecking=no','-o','UserKnownHostsFile=/dev/null','tester@'+address,command]
 deadline=time.monotonic()+60
 while True:
  result=subprocess.run(args,capture_output=True,text=True,timeout=15)
  if result.returncode==0 or allow_failure:return result
  if time.monotonic()>deadline:raise RuntimeError('guest SSH failed: '+result.stderr)
  time.sleep(.5)
def marker(vm,value=None):
 command='cat /home/tester/tt-marker' if value is None else 'printf '+value+' > /home/tester/tt-marker; sync; cat /home/tester/tt-marker'
 return ssh(vm,command).stdout
agent_args=['--listen','127.0.0.1:19100','--host-id','native-linux-zvol','--data-dir',str(root/'native-agent'),'--image-dir','ttgo6426/images','--runtime-dir','ttgo6426/runtime','--storage','zvol','--enable-disk-backup','--cpu-total','4','--mem-total','4096','--disk-total','32768','--ssh-public-address','198.18.0.1','--port-start','21000','--port-end','21199']
ctl_args=['--listen','127.0.0.1:19200','--data-dir',str(root/'native-ctl')]
try:
 peer_log=open(root/'peer-http.log','w');peer=subprocess.Popen(['ip','netns','exec','tt-peer','python3','-m','http.server','18080','--bind','198.18.0.2','--directory',str(root/'http-fixture')],stdout=peer_log,stderr=peer_log);processes.append((peer,peer_log))
 agent=start(go/'tt-agent',agent_args,19100,'/api/info','go-file-agent');controller=start(rust/'tt-ctl',ctl_args,19200,'/api/status','rust-file-ctl')
 for existing in call(19200,'GET','/api/envs'):
  if existing['id'] in ['native-qemu','native-firecracker']:cli(go/'tt',['env','delete',existing['id']])
 cli(go/'tt',['host','add','127.0.0.1:19100'])
 for index,engine in enumerate(os.environ.get('TT_NATIVE_ENGINES','qemu,firecracker').split(',')):
  name='native-'+engine;image='alpine-cloud' if engine=='qemu' else 'fc-ssh';disk=4096 if engine=='qemu' else 512
  args=['env','create',name,'--image',image,'--engine',engine,'--cpu','1','--mem','512','--disk',str(disk),'--lifetime','0','--ssh-key',str(root/'guest-key.pub'),'--ssh-user','tester','-p','80']
  if engine=='firecracker':args+=['--ssh-sudo','--guest-config',str(root/'guest-config.json'),'--isolated-network']
  print('Creating '+name,flush=True);cli(go/'tt',args)
  vm=call(19200,'GET','/api/envs/'+name)['vms'][0];vmid=vm['id'];assert vm['state']=='running'
  ssh(vm,'true')
  deadline=time.monotonic()+45
  while not call(19100,'GET','/api/vms/'+vmid)['ssh']['ready']:
   if time.monotonic()>deadline:raise RuntimeError('SSH observation did not become ready')
   time.sleep(.5)
  assert ssh(vm,'id -un').stdout.strip()=='tester'
  if engine=='firecracker':assert ssh(vm,'sudo -n id -u').stdout.strip()=='0'
  assert ssh(vm,'echo external',external=True).stdout.strip()=='external'
  assert marker(vm,'first')=='first'
  assert ssh(vm,'wget -qO- -T 3 http://198.18.0.2:18080/').stdout.strip()=='tt-native-egress'
  if engine=='firecracker':
   assert ssh(vm,'sudo cat /run/ttstack-config/opaque.txt').stdout=='opaque fixture data\n'
   assert ssh(vm,'sudo sh -c "echo forbidden >> /run/ttstack-config/opaque.txt"',allow_failure=True).returncode!=0
   sandbox=json.loads(pathlib.Path('/home/ttstack/run/fc-'+vmid+'.sandbox.json').read_text())
   assert pathlib.Path(sandbox['cgroup']+'/memory.max').read_text().strip()==str(640*1024*1024)
   assert pathlib.Path(sandbox['cgroup']+'/cpu.max').read_text().strip()=='100000 100000'
  # A stale PID must not kill the lab's PID 1, or lose the actual VMM.
  pid_path=pathlib.Path('/home/ttstack/run/'+('qemu-' if engine=='qemu' else 'fc-')+vmid+'.pid');pid_path.write_text('1')
  stop(controller);stop(agent)
  agent=start(rust/'tt-agent',agent_args,19100,'/api/info','rust-file-agent-'+engine);controller=start(go/'tt-ctl',ctl_args,19200,'/api/status','go-file-ctl-'+engine)
  assert marker(vm)=='first';cli(rust/'tt',['env','stop',name]);assert pathlib.Path('/proc/1/comm').read_text().strip()=='python3'
  cli(rust/'tt',['env','resize',vmid,'--cpu','2','--mem','640','--disk',str(disk+1024 if engine=='qemu' else disk+256)])
  cli(rust/'tt',['env','start',name]);assert marker(vm)=='first';cli(rust/'tt',['env','stop',name])
  cli(rust/'tt',['env','backup','create',vmid]);first=call(19200,'GET','/api/vms/'+vmid+'/backup')['vm']['backup']['current']['id']
  cli(rust/'tt',['env','start',name]);assert marker(vm,'second')=='second';cli(rust/'tt',['env','stop',name]);cli(rust/'tt',['env','backup','create',vmid])
  state=call(19200,'GET','/api/vms/'+vmid+'/backup')['vm']['backup'];generation=state['current']['id'];assert generation!=first
  cli(rust/'tt',['env','start',name]);assert marker(vm,'third')=='third';cli(rust/'tt',['env','stop',name])
  state=call(19200,'GET','/api/vms/'+vmid+'/backup')['vm']['backup'];retry=['env','backup','restore',vmid,'--generation',generation,'--operation-id',str(uuid.uuid4()),'--expected-revision',state['revision']]
  cli(rust/'tt',retry)
  stop(controller);stop(agent)
  agent=start(go/'tt-agent',agent_args,19100,'/api/info','go-file-recovery-'+engine);controller=start(rust/'tt-ctl',ctl_args,19200,'/api/status','rust-file-recovery-'+engine)
  cli(go/'tt',['env','start',name]);assert marker(vm)=='second';assert marker(vm,'after-replay')=='after-replay';cli(go/'tt',['env','stop',name]);cli(go/'tt',retry)
  cli(go/'tt',['env','start',name]);assert marker(vm)=='after-replay';cli(go/'tt',['env','stop',name]);cli(go/'tt',['env','backup','delete',vmid]);cli(go/'tt',['env','delete',name])
  assert subprocess.run(['zfs','list','ttgo6426/runtime/clone-'+vmid],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode!=0
  print(engine+': SSH account/external port, egress, stale PID, service takeover, cold boot, resize, zvol refresh/restore/exact replay and deletion passed',flush=True)
 print('Linux zvol VM matrix passed',flush=True)
finally:
 for process,log in reversed(processes):
  if process.poll() is None:
   process.terminate()
   try:process.wait(timeout=45)
   except subprocess.TimeoutExpired:process.kill();process.wait()
  log.close()
