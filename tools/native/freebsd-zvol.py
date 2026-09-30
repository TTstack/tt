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
for name in ['agent-zvol','ctl-zvol']:(base/name).mkdir(exist_ok=True)
agent_args=['--listen','127.0.0.1:19100','--host-id','native-freebsd-zvol','--data-dir',str(base/'agent-zvol'),'--image-dir','zroot/tt-go-port-6426-native/images','--runtime-dir','zroot/tt-go-port-6426-native/runtime','--storage','zvol','--enable-disk-backup','--cpu-total','2','--mem-total','2048','--disk-total','16384']
ctl_args=['--listen','127.0.0.1:19200','--data-dir',str(base/'ctl-zvol')]
go=root/'bin';rust=root/'rust-target/debug'
try:
 agent=start(go/'tt-agent',agent_args,19100,'/api/info','go-zvol-agent')
 controller=start(rust/'tt-ctl',ctl_args,19200,'/api/status','rust-zvol-ctl')
 cli(go/'tt',['host','add','127.0.0.1:19100'])
 name='native-zvol-backup'
 cli(go/'tt',['env','create',name,'--image','bhyve-freebsd','--engine','bhyve','--cpu','1','--mem','512','--lifetime','0'])
 vm=call(19200,'GET','/api/envs/'+name)['vms'][0];vmid=vm['id'];ip=vm['ip']
 assert guest_ssh(ip,'printf first > /root/tt-marker; sync; cat /root/tt-marker')=='first'
 cli(go/'tt',['env','stop',name])
 cli(go/'tt',['env','backup','create',vmid])
 first=call(19200,'GET','/api/vms/'+vmid+'/backup')['vm']['backup']['current']['id']
 cli(go/'tt',['env','start',name]);assert guest_ssh(ip,'printf second > /root/tt-marker; sync; cat /root/tt-marker')=='second'
 cli(go/'tt',['env','stop',name]);cli(go/'tt',['env','backup','create',vmid])
 state=call(19200,'GET','/api/vms/'+vmid+'/backup')['vm']['backup'];generation=state['current']['id'];assert generation!=first
 cli(go/'tt',['env','start',name]);assert guest_ssh(ip,'printf third > /root/tt-marker; sync; cat /root/tt-marker')=='third';cli(go/'tt',['env','stop',name])
 state=call(19200,'GET','/api/vms/'+vmid+'/backup')['vm']['backup']
 command=['env','backup','restore',vmid,'--generation',generation,'--operation-id',str(uuid.uuid4()),'--expected-revision',state['revision']]
 cli(go/'tt',command)
 # Swap both services while retaining the completed restore receipt and disk.
 stop(controller);stop(agent)
 agent=start(rust/'tt-agent',agent_args,19100,'/api/info','rust-zvol-agent')
 controller=start(go/'tt-ctl',ctl_args,19200,'/api/status','go-zvol-ctl')
 cli(rust/'tt',['env','start',name]);assert guest_ssh(ip,'cat /root/tt-marker')=='second'
 assert guest_ssh(ip,'printf after-replay > /root/tt-marker; sync; cat /root/tt-marker')=='after-replay'
 cli(rust/'tt',['env','stop',name]);cli(rust/'tt',command)
 cli(rust/'tt',['env','start',name]);assert guest_ssh(ip,'cat /root/tt-marker')=='after-replay';cli(rust/'tt',['env','stop',name])
 cli(rust/'tt',['env','backup','delete',vmid]);cli(rust/'tt',['env','delete',name])
 remaining=subprocess.check_output(['zfs','list','-H','-o','name','-r','zroot/tt-go-port-6426-native/runtime'],text=True).splitlines()
 assert remaining==['zroot/tt-go-port-6426-native/runtime'],remaining
 print('bhyve zvol: real SSH, refresh, restore, cross-language takeover, exact replay preserving later writes and cleanup passed',flush=True)
finally:
 for process,log in reversed(processes):
  if process.poll() is None:
   process.terminate()
   try:process.wait(timeout=40)
   except subprocess.TimeoutExpired:process.kill();process.wait()
  log.close()
