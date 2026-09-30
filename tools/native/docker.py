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
agent_args=['--listen','127.0.0.1:19100','--host-id','native-linux-file','--data-dir',str(root/'native-agent'),'--image-dir',str(root/'images'),'--runtime-dir','/home/ttstack/store/run1','--container-runtime','docker','--cpu-total','2','--mem-total','2048','--disk-total','16384','--ssh-public-address','198.18.0.1','--port-start','21000','--port-end','21199']
ctl_args=['--listen','127.0.0.1:19200','--data-dir',str(root/'docker-ctl')]
os.environ['DOCKER_HOST']='unix:///run/tt-native-docker.sock'
def docker(*args):return subprocess.check_output(['docker']+list(args),text=True,stderr=subprocess.STDOUT)
try:
 if subprocess.run(['ip','link','show','tt-docker'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode!=0:subprocess.run(['ip','link','add','tt-docker','type','bridge'],check=True)
 subprocess.run(['ip','link','set','tt-docker','up'],check=True)
 (root/'docker-config.json').write_text('{}\n')
 log=open(root/'native-dockerd.log','w');daemon=subprocess.Popen(['dockerd','--config-file',str(root/'docker-config.json'),'--host','unix:///run/tt-native-docker.sock','--data-root',str(root/'docker-data'),'--exec-root','/run/tt-native-docker','--pidfile','/run/tt-native-docker.pid','--iptables=false','--ip6tables=false','--bridge=tt-docker','--exec-opt','native.cgroupdriver=cgroupfs','--storage-driver=vfs'],stdout=log,stderr=log);processes.append((daemon,log));deadline=time.monotonic()+45
 while True:
  if daemon.poll() is not None:raise RuntimeError('private Docker daemon exited')
  try:docker('info');break
  except subprocess.CalledProcessError:
   if time.monotonic()>deadline:raise RuntimeError('private Docker daemon not ready')
   time.sleep(.5)
 docker('import','--change','CMD ["/usr/local/bin/native-http"]',str(root/'downloads/docker-fixture.tar.gz'),'native-http')
 agent=start(go/'tt-agent',agent_args,19100,'/api/info','docker-go-agent');controller=start(rust/'tt-ctl',ctl_args,19200,'/api/status','docker-rust-ctl')
 for existing in call(19200,'GET','/api/envs'):
  if existing['id']=='native-docker':cli(go/'tt',['env','delete',existing['id']])
 cli(go/'tt',['host','add','127.0.0.1:19100']);cli(go/'tt',['env','create','native-docker','--image','native-http','--engine','docker','--cpu','1','--mem','128','--lifetime','0','-p','80'])
 vm=call(19200,'GET','/api/envs/native-docker')['vms'][0];vmid=vm['id'];container='tt-'+vmid;port=vm['port_map']['80']
 connection=http.client.HTTPConnection('127.0.0.1',port,timeout=5);connection.request('GET','/');response=connection.getresponse();assert response.read().strip()==b'native-http';connection.close()
 assert docker('exec',container,'sh','-c','printf retained > /tmp/tt-marker; cat /tmp/tt-marker')=='retained'
 stop(controller);stop(agent)
 changed=agent_args.copy();changed[changed.index('docker')]='podman'
 result=subprocess.run([str(go/'tt-agent')]+changed,env=dict(os.environ,TT_API_KEY=key),capture_output=True,text=True,timeout=20)
 assert result.returncode!=0 and 'runtime' in result.stderr.lower()
 assert docker('exec',container,'cat','/tmp/tt-marker')=='retained'
 agent=start(rust/'tt-agent',agent_args,19100,'/api/info','docker-rust-agent');controller=start(go/'tt-ctl',ctl_args,19200,'/api/status','docker-go-ctl')
 cli(rust/'tt',['env','stop','native-docker']);assert docker('inspect','-f','{{.State.Running}}',container).strip()=='false'
 cli(rust/'tt',['env','start','native-docker']);assert docker('exec',container,'cat','/tmp/tt-marker')=='retained'
 cli(rust/'tt',['env','delete','native-docker']);assert docker('ps','-aq','--filter','name='+container).strip()==''
 print('Private Docker: published HTTP, retained layer, mixed takeover, runtime-switch refusal, stop/start and delete passed',flush=True)
finally:
 for process,log in reversed(processes):
  if process.poll() is None:
   process.terminate()
   try:process.wait(timeout=45)
   except subprocess.TimeoutExpired:process.kill();process.wait()
  log.close()
