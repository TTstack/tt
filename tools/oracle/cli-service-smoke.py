import contextlib,http.client,json,os,pathlib,socket,subprocess,sys,tempfile,time,uuid
if len(sys.argv)!=2:raise SystemExit('usage: cli-service-smoke.py PREPARED_NATIVE_TEST_ROOT')
root=pathlib.Path(sys.argv[1]);key=uuid.uuid4().hex
reserve=socket.socket();reserve.bind(('127.0.0.1',9100));reserve.close()
def call(port,method,path,body=None,authorized=True):
 c=http.client.HTTPConnection('127.0.0.1',port,timeout=5);headers={'Authorization':'Bearer '+key} if authorized else {}
 if body is not None:headers['Content-Type']='application/json';body=json.dumps(body)
 c.request(method,path,body,headers);response=c.getresponse();data=response.read();status=response.status;ctype=response.getheader('content-type');c.close();return status,json.loads(data) if ctype=='application/json' and data else data
@contextlib.contextmanager
def service(binary,args,port,ready):
 p=subprocess.Popen([str(binary)]+args,env=dict(os.environ,TT_API_KEY=key,GOMAXPROCS='1'),stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
 try:
  deadline=time.monotonic()+30
  while True:
   if p.poll() is not None:raise RuntimeError(p.stderr.read())
   try:
    status,body=call(port,'GET',ready)
    if status==200:break
   except (OSError,http.client.HTTPException):pass
   if time.monotonic()>deadline:raise RuntimeError('service did not become ready')
   time.sleep(.1)
  yield p
 finally:
  if p.poll() is None:p.terminate()
  try:out,err=p.communicate(timeout=30)
  except subprocess.TimeoutExpired:p.kill();out,err=p.communicate();raise RuntimeError('service did not stop')
  if p.returncode!=0:raise RuntimeError('service failed: '+err)
with tempfile.TemporaryDirectory(prefix='controller-service-smoke-',dir=root) as directory:
 base=pathlib.Path(directory)
 for name in ['agent','ctl','images','runtime']:(base/name).mkdir(mode=0o700)
 subprocess.run([str(root/'rust-target/debug/examples/agent-db'),'create',str(base/'agent/agent.db')],check=True,stdout=subprocess.DEVNULL)
 subprocess.run([str(root/'rust-target/debug/examples/ctl-db'),'create',str(base/'ctl/ctl.db')],check=True,stdout=subprocess.DEVNULL)
 retained=None
 agent_args=['--listen','127.0.0.1:9100','--data-dir',str(base/'agent'),'--image-dir',str(base/'images'),'--runtime-dir',str(base/'runtime'),'--cpu-total','1','--mem-total','128','--disk-total','1024']
 for agent,label in [(root/'bin/tt-agent','Go agent'),(root/'rust-target/debug/tt-agent','Rust agent')]:
  with service(agent,agent_args,9100,'/api/info'):
   for controller,ctl_label in [(root/'bin/tt-ctl','Go controller'),(root/'rust-target/debug/tt-ctl','Rust controller')]:
    reserve=socket.socket();reserve.bind(('127.0.0.1',0));port=reserve.getsockname()[1];reserve.close()
    args=['--listen',f'127.0.0.1:{port}','--data-dir',str(base/'ctl')]
    with service(controller,args,port,'/api/status'):
     deadline=time.monotonic()+20
     while True:
      status,body=call(port,'GET','/api/status')
      if status==200 and body['data']['hosts_online']==1 and body['data']['mem_total']==128:break
      if time.monotonic()>deadline:raise RuntimeError('controller did not refresh agent')
      time.sleep(.1)
     assert body['data']['total_vms']==5
     status,body=call(port,'GET','/api/envs/oracle-env');assert status==200 and len(body['data']['vms'])==5
     records={v['id']:(v['host_id'],v['disk'],v['backup']['revision']) for v in body['data']['vms']}
     if retained is None:retained=records
     else:assert retained==records
     status,body=call(port,'GET','/api/vms/qemu/backup');assert status==200 and not body['data']['enabled']
     status,body=call(port,'POST','/api/envs',{'id':'unavailable','vms':[{'image':'base','engine':'qemu','cpu':1,'mem':64,'disk':128}]});assert status==422
     status,body=call(port,'GET','/api/envs/unavailable');assert status==404
     status,body=call(port,'GET','/api/status',authorized=False);assert status==401
     status,body=call(port,'GET','/',authorized=False);assert status==200 and b'TTstack' in body
     for command in [['status'],['host','list'],['host','show','oracle-host'],['env','list'],['env','backup','show','qemu'],['image','list'],['capabilities','--json']]:
      outputs=[]
      for cli in [root/'bin/tt',root/'rust-target/debug/tt']:
       result=subprocess.run([str(cli),'--server',f'127.0.0.1:{port}']+command,env=dict(os.environ,TT_API_KEY=key,GOMAXPROCS='1'),capture_output=True,text=True,timeout=20)
       assert result.returncode==0,(command,result.stderr)
       outputs.append(result.stdout)
      assert outputs[0]==outputs[1],('CLI output mismatch',command,outputs)
     print(ctl_label+' -> '+label+': both CLIs, authenticated refresh, retained VM/backup state and capability refusal passed',flush=True)
print('Controller/agent processes and private state cleaned',flush=True)
