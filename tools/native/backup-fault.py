"""Bounded native VM/reflink acceptance inside the private Linux lab."""
import http.client,json,os,pathlib,socket,subprocess,time,uuid
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
 agent=start(go/'tt-agent',agent_args,19100,'/api/info','fault-go-agent');controller=start(go/'tt-ctl',ctl_args,19200,'/api/status','fault-go-ctl')
 cli(go/'tt',['host','add','127.0.0.1:19100']);name='native-backup-fault'
 cli(go/'tt',['env','create',name,'--image','fc-ssh','--engine','firecracker','--cpu','1','--mem','512','--disk','512','--lifetime','0','--ssh-key',str(root/'guest-key.pub'),'--ssh-user','tester','--ssh-sudo'])
 vm=call(19200,'GET','/api/envs/'+name)['vms'][0];vmid=vm['id'];assert marker(vm,'first')=='first';cli(go/'tt',['env','stop',name])
 state=call(19200,'GET','/api/vms/'+vmid+'/backup')['vm']['backup'];operation=str(uuid.uuid4());revision=state['revision']
 request=f'PUT /api/vms/{vmid}/backup HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {key}\r\nIdempotency-Key: {operation}\r\nIf-Match: "{revision}"\r\nContent-Length: 0\r\nConnection: close\r\n\r\n'
 connection=socket.create_connection(('127.0.0.1',19200));connection.sendall(request.encode())
 # Closing before HTTP admission can legitimately cancel an unread request.
 # First establish durable acceptance without consuming the submitter's reply.
 deadline=time.monotonic()+60
 while True:
  admitted=call(19200,'GET','/api/vms/'+vmid+'/backup')['vm']['backup']
  pending=admitted.get('pending') or {};last=admitted.get('last_result') or {}
  if pending.get('request',{}).get('operation_id')==operation or last.get('request',{}).get('operation_id')==operation or admitted.get('current'):break
  if time.monotonic()>deadline:raise RuntimeError('backup submission was not durably admitted')
  time.sleep(.02)
 connection.close()
 deadline=time.monotonic()+60
 while True:
  state=call(19200,'GET','/api/vms/'+vmid+'/backup')['vm']['backup']
  if state.get('current') and not state.get('pending'):break
  if time.monotonic()>deadline:raise RuntimeError('disconnected backup did not settle')
  time.sleep(.2)
 generation=state['current']['id'];stop(controller);stop(agent)
 agent=start(rust/'tt-agent',agent_args,19100,'/api/info','fault-rust-agent');controller=start(rust/'tt-ctl',ctl_args,19200,'/api/status','fault-rust-ctl')
 cli(rust/'tt',['env','backup','create',vmid,'--operation-id',operation,'--expected-revision',revision])
 assert call(19200,'GET','/api/vms/'+vmid+'/backup')['vm']['backup']['current']['id']==generation
 snapshot='ttgo6426/runtime/clone-'+vmid+'/rootfs@ttbackup-'+generation
 subprocess.run(['zfs','hold','ttgo6426-test',snapshot],check=True)
 cli(rust/'tt',['env','start',name]);assert marker(vm,'second')=='second';cli(rust/'tt',['env','stop',name]);cli(rust/'tt',['env','backup','create',vmid])
 state=call(19200,'GET','/api/vms/'+vmid+'/backup')['vm']['backup'];assert state['current']['id']!=generation and any(g['id']==generation for g in state['retired'])
 new_generation=state['current']['id'];cli(rust/'tt',['env','backup','restore',vmid,'--generation',new_generation])
 cli(rust/'tt',['env','start',name]);assert marker(vm)=='second';cli(rust/'tt',['env','stop',name])
 subprocess.run(['zfs','release','ttgo6426-test',snapshot],check=True)
 cli(rust/'tt',['env','backup','delete',vmid]);cli(rust/'tt',['env','delete',name])
 assert subprocess.run(['zfs','list','ttgo6426/runtime/clone-'+vmid],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode!=0
 print('Native ZFS: disconnected submission settled, exact retry preserved generation, held retired snapshot retained reservation without blocking refresh/restore, release and cleanup passed',flush=True)
finally:
 for process,log in reversed(processes):
  if process.poll() is None:
   process.terminate()
   try:process.wait(timeout=45)
   except subprocess.TimeoutExpired:process.kill();process.wait()
  log.close()
