"""Bounded native VM/reflink acceptance inside the private Linux lab."""
import http.client,json,os,pathlib,subprocess,time,uuid
assert b'/tmp/tt-go-port-6426/lab-server.py\x00' in pathlib.Path('/proc/1/cmdline').read_bytes(), 'run only inside the owned lab'
root=pathlib.Path('/tmp/tt-go-port-6426');go=root/'bin';rust=root/'rust-target/debug';key=uuid.uuid4().hex;processes=[]
def call(port,method,path,body=None):
 connection=http.client.HTTPConnection('127.0.0.1',port,timeout=240);headers={'Authorization':'Bearer '+key}
 if body is not None:headers['Content-Type']='application/json';body=json.dumps(body)
 connection.request(method,path,body,headers);response=connection.getresponse();data=response.read();connection.close();result=json.loads(data)
 if not (200<=response.status<300 and result['ok']):raise RuntimeError((method,path,response.status,result))
 return result.get('data')
def start(binary,args,port,ready,label,address="127.0.0.1"):
 log=open(root/(label+'.log'),'w');process=subprocess.Popen([str(binary)]+args,env=dict(os.environ,TT_API_KEY=key,GOMAXPROCS='2',DOCKER_HOST='unix:///run/tt-native-docker.sock'),stdout=log,stderr=log);processes.append((process,log));deadline=time.monotonic()+40
 while True:
  if process.poll() is not None:raise RuntimeError(label+' exited; inspect its log')
  try:
   connection=http.client.HTTPConnection(address,port,timeout=2);connection.request('GET',ready,headers={'Authorization':'Bearer '+key});response=connection.getresponse();response.read();connection.close()
   if response.status==200:return process
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
def run(*args):subprocess.run(args,check=True,stdout=subprocess.DEVNULL)
for directory in ['agent2-run','agent2-data','mixed-ctl']:(root/directory).mkdir(exist_ok=True)
if 'tt-agent2' not in [line.split()[0] for line in subprocess.check_output(['ip','netns','list'],text=True).splitlines()]:
 run('ip','netns','add','tt-agent2')
 run('ip','link','add','tt-agent2-link','type','veth','peer','name','tt-agent2-nic')
 run('ip','link','set','tt-agent2-nic','netns','tt-agent2')
 run('ip','addr','add','198.19.0.1/24','dev','tt-agent2-link');run('ip','link','set','tt-agent2-link','up')
 run('ip','netns','exec','tt-agent2','ip','link','set','lo','up')
 run('ip','netns','exec','tt-agent2','ip','addr','add','198.19.0.2/24','dev','tt-agent2-nic');run('ip','netns','exec','tt-agent2','ip','link','set','tt-agent2-nic','up')
 run('ip','netns','exec','tt-agent2','ip','route','add','default','via','198.19.0.1')
run('ip','netns','exec','tt-peer','ip','route','replace','default','via','198.18.0.1')
common=['--image-dir',str(root/'images'),'--cpu-total','2','--mem-total','2048','--disk-total','16384','--ssh-public-address','198.18.0.1']
args1=['--listen','127.0.0.1:19100','--host-id','native-linux-file','--data-dir',str(root/'native-agent'),'--runtime-dir','/home/ttstack/store/run1','--port-start','21000','--port-end','21199']+common
args2=['--listen','198.19.0.2:19101','--host-id','native-linux-second','--data-dir',str(root/'agent2-data'),'--runtime-dir','/home/ttstack/store/run2','--port-start','22000','--port-end','22199','--ssh-ingress-netns','/proc/1/ns/net','--ssh-ingress-target','198.19.0.2']+common
pathlib.Path('/home/ttstack/store/run2').mkdir(exist_ok=True)
ctl_args=['--listen','127.0.0.1:19200','--data-dir',str(root/'mixed-ctl')]
def second(binary,label):
 script='mount --make-rprivate /; mount --bind /tmp/tt-go-port-6426/agent2-run /home/ttstack/run; exec ip netns exec tt-agent2 "$@"'
 return start('/usr/bin/unshare',['--mount','/bin/bash','-c',script,'agent2',str(binary)]+args2,19101,'/api/info',label,address='198.19.0.2')
def http_server(prefix,address,port,label):
 log=open(root/(label+'.log'),'w');process=subprocess.Popen(prefix+['python3','-m','http.server',str(port),'--bind',address,'--directory',str(root/'http-fixture')],stdout=log,stderr=log);processes.append((process,log))
try:
 http_server(['ip','netns','exec','tt-peer'],'198.18.0.2',18080,'mixed-peer')
 http_server([],'0.0.0.0',18081,'mixed-host1')
 http_server(['ip','netns','exec','tt-agent2'],'0.0.0.0',18081,'mixed-host2')
 agent1=start(go/'tt-agent',args1,19100,'/api/info','mixed-go-agent1');agent2=second(rust/'tt-agent','mixed-rust-agent2');controller=start(go/'tt-ctl',ctl_args,19200,'/api/status','mixed-go-ctl')
 for existing in call(19200,'GET','/api/envs'):
  if existing['id'] in ['native-mixed','native-denied']:cli(go/'tt',['env','delete',existing['id']])
 cli(go/'tt',['host','add','127.0.0.1:19100']);cli(go/'tt',['host','add','198.19.0.2:19101'])
 cli(go/'tt',['env','create','native-mixed','--image','alpine-cloud','--engine','qemu','--dup','2','--cpu','2','--mem','512','--disk','4096','--lifetime','0','--ssh-key',str(root/'guest-key.pub'),'--ssh-user','tester','--isolated-network'])
 vms=call(19200,'GET','/api/envs/native-mixed')['vms'];assert len(vms)==2 and len({v['host_id'] for v in vms})==2
 for vm in vms:
  assert ssh(vm,'printf '+vm['id']+' > /home/tester/mixed-marker; cat /home/tester/mixed-marker',external=True).stdout==vm['id']
  assert ssh(vm,'wget -qO- -T 3 http://198.18.0.2:18080/',external=True).stdout.strip()=='tt-native-egress'
  assert ssh(vm,'wget -qO- -T 2 http://10.10.0.1:18081/',external=True,allow_failure=True).returncode!=0
 print('Mixed environment spans two agents; outer SSH ingress, allowed egress and blocked host access passed',flush=True)
 stop(controller);stop(agent1);stop(agent2)
 agent1=start(rust/'tt-agent',args1,19100,'/api/info','mixed-rust-agent1');agent2=second(go/'tt-agent','mixed-go-agent2');controller=start(rust/'tt-ctl',ctl_args,19200,'/api/status','mixed-rust-ctl')
 for vm in vms:assert ssh(vm,'cat /home/tester/mixed-marker',external=True).stdout==vm['id']
 cli(rust/'tt',['env','stop','native-mixed']);cli(rust/'tt',['env','start','native-mixed'])
 for vm in vms:assert ssh(vm,'cat /home/tester/mixed-marker',external=True).stdout==vm['id']
 cli(rust/'tt',['env','delete','native-mixed'])
 rules=subprocess.check_output(['nft','list','table','ip','tt-ssh'],text=True)
 assert all(vm['id'] not in rules for vm in vms)
 cli(rust/'tt',['env','create','native-denied','--image','alpine-cloud','--engine','qemu','--cpu','1','--mem','512','--disk','4096','--lifetime','0','--ssh-key',str(root/'guest-key.pub'),'--ssh-user','tester','--deny-outgoing','--isolated-network'])
 vm=call(19200,'GET','/api/envs/native-denied')['vms'][0]
 assert ssh(vm,'echo ingress-works',external=True).stdout.strip()=='ingress-works'
 assert ssh(vm,'wget -qO- -T 2 http://198.18.0.2:18080/',external=True,allow_failure=True).returncode!=0
 cli(rust/'tt',['env','delete','native-denied'])
 print('Mixed Go/Rust live takeover and stop/start/delete, retained markers, owned ingress cleanup and deny-outgoing enforcement passed',flush=True)
finally:
 for process,log in reversed(processes):
  if process.poll() is None:
   process.terminate()
   try:process.wait(timeout=45)
   except subprocess.TimeoutExpired:process.kill();process.wait()
  log.close()
