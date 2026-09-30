"""Run inside the dedicated tt-port-6426 VNET jail; one guest at a time."""
import contextlib,http.client,json,os,pathlib,subprocess,time,uuid
import socket
assert socket.gethostname()=='tt-port-6426', 'run only inside the owned FreeBSD VNET'
root=pathlib.Path('/tmp/tt-go-port-6426');base=root/'native';key=uuid.uuid4().hex
processes=[]
def call(port,method,path,body=None):
 connection=http.client.HTTPConnection('127.0.0.1',port,timeout=240)
 headers={'Authorization':'Bearer '+key}
 if body is not None:headers['Content-Type']='application/json';body=json.dumps(body)
 connection.request(method,path,body,headers);response=connection.getresponse();data=response.read();connection.close()
 result=json.loads(data)
 if not (200<=response.status<300 and result['ok']):raise RuntimeError((method,path,response.status,result))
 return result.get('data')
def start(binary,args,port,ready,label):
 log=open(base/(label+'.log'),'w')
 process=subprocess.Popen([str(binary)]+args,env=dict(os.environ,TT_API_KEY=key,GOMAXPROCS='1'),stdout=log,stderr=log)
 processes.append((process,log));deadline=time.monotonic()+30
 while True:
  if process.poll() is not None:raise RuntimeError(label+' exited; inspect its log')
  try:call(port,'GET',ready);return process
  except (OSError,http.client.HTTPException):pass
  if time.monotonic()>deadline:raise RuntimeError(label+' not ready')
  time.sleep(.2)
def stop(process):
 process.terminate();process.wait(timeout=40)
 if process.returncode!=0:raise RuntimeError('service shutdown failed')
def cli(binary,command):
 result=subprocess.run([str(binary),'--server','127.0.0.1:19200']+command,env=dict(os.environ,TT_API_KEY=key,GOMAXPROCS='1'),capture_output=True,text=True,timeout=240)
 if result.returncode!=0:raise RuntimeError((command,result.stderr))
 return result.stdout
def guest_ssh(ip,command):
 deadline=time.monotonic()+90
 while True:
  result=subprocess.run(['ssh','-i',str(base/'guest-key'),'-o','IdentitiesOnly=yes','-o','BatchMode=yes','-o','ConnectTimeout=3','-o','StrictHostKeyChecking=no','-o','UserKnownHostsFile=/dev/null','root@'+ip,command],capture_output=True,text=True,timeout=10)
  if result.returncode==0:return result.stdout
  if time.monotonic()>deadline:raise RuntimeError('guest SSH failed: '+result.stderr)
  time.sleep(.5)
for name in ['agent','ctl','runtime']:(base/name).mkdir(exist_ok=True)
if not (base/'guest-key').exists():subprocess.run(['ssh-keygen','-q','-t','ed25519','-N','','-f',str(base/'guest-key')],check=True)
agent_args=['--listen','127.0.0.1:19100','--host-id','native-freebsd','--data-dir',str(base/'agent'),'--image-dir',str(base/'images'),'--runtime-dir',str(base/'runtime'),'--cpu-total','2','--mem-total','2048','--disk-total','16384']
ctl_args=['--listen','127.0.0.1:19200','--data-dir',str(base/'ctl')]
go=root/'bin';rust=root/'rust-target/debug'
try:
 agent=start(go/'tt-agent',agent_args,19100,'/api/info','go-agent')
 controller=start(go/'tt-ctl',ctl_args,19200,'/api/status','go-ctl')
 for existing in call(19200,'GET','/api/envs'):
  if existing['id']=='native-bhyve-0':cli(go/'tt',['env','delete',existing['id']])
 cli(go/'tt',['host','add','127.0.0.1:19100'])
 assert 'bhyve-freebsd' in cli(go/'tt',['image','list'])
 for index,(agent_bin,controller_bin,cli_bin) in enumerate([(go/'tt-agent',go/'tt-ctl',go/'tt'),(go/'tt-agent',rust/'tt-ctl',rust/'tt'),(rust/'tt-agent',go/'tt-ctl',go/'tt')]):
  if index:
   stop(controller);stop(agent)
   agent=start(agent_bin,agent_args,19100,'/api/info','agent-'+str(index))
   controller=start(controller_bin,ctl_args,19200,'/api/status','ctl-'+str(index))
  name='native-bhyve-'+str(index)
  print('Creating '+name,flush=True)
  cli(cli_bin,['env','create',name,'--image','bhyve-freebsd','--engine','bhyve','--cpu','1','--mem','512','--lifetime','0'])
  detail=call(19200,'GET','/api/envs/'+name);vm=detail['vms'][0];vmid=vm['id'];ip=vm['ip']
  assert vm['state']=='running'
  assert guest_ssh(ip,'printf retained > /root/tt-marker; cat /root/tt-marker')=='retained'
  # Restart each control-plane service while the actual guest keeps running.
  stop(controller);stop(agent)
  takeover_agent=rust/'tt-agent' if agent_bin==go/'tt-agent' else go/'tt-agent'
  takeover_ctl=rust/'tt-ctl' if controller_bin==go/'tt-ctl' else go/'tt-ctl'
  agent=start(takeover_agent,agent_args,19100,'/api/info','agent-restart-'+str(index))
  controller=start(takeover_ctl,ctl_args,19200,'/api/status','ctl-restart-'+str(index))
  assert guest_ssh(ip,'cat /root/tt-marker')=='retained'
  cli(cli_bin,['env','stop',name]);detail=call(19200,'GET','/api/envs/'+name);assert detail['vms'][0]['state']=='stopped'
  assert (base/'runtime'/('clone-'+vmid)).is_file()
  cli(cli_bin,['env','start',name]);assert guest_ssh(ip,'cat /root/tt-marker')=='retained'
  cli(cli_bin,['env','delete',name]);assert not (base/'runtime'/('clone-'+vmid)).exists()
  assert not pathlib.Path('/home/ttstack/run/bhyve-'+vmid+'.disk').exists()
  print(name+': real SSH, Go/Rust live takeover, cold stop/start and deletion passed',flush=True)
 print('FreeBSD native bhyve matrix passed',flush=True)
finally:
 for process,log in reversed(processes):
  if process.poll() is None:
   process.terminate()
   try:process.wait(timeout=40)
   except subprocess.TimeoutExpired:process.kill();process.wait()
  log.close()
