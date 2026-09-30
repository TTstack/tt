"""Observe controller shutdown/disconnect against a synthetic loopback agent.

Run on an authorized native test host with a freshly built pinned controller.
No engine, firewall, deployment or real guest operation is performed.
"""
import http.client,http.server,json,os,pathlib,socket,sqlite3,subprocess,sys,tempfile,threading,time,uuid
if len(sys.argv)!=2:raise SystemExit('usage: controller-lifecycle.py TT_CTL_BINARY')
key='lifecycle-fixture-key'
def port():
 with socket.socket() as sock:sock.bind(('127.0.0.1',0));return sock.getsockname()[1]
def call(number,method,path,body=None):
 conn=http.client.HTTPConnection('127.0.0.1',number,timeout=5);headers={'Authorization':'Bearer '+key}
 if body is not None:headers['Content-Type']='application/json';body=json.dumps(body)
 conn.request(method,path,body,headers);response=conn.getresponse();data=json.loads(response.read());conn.close()
 if not (200<=response.status<300 and data['ok']):raise RuntimeError((response.status,data))
 return data.get('data')
class Peer(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def reply(self,data=None,status=200):
  payload=json.dumps({'ok':True,'data':data}).encode();self.send_response(status);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(payload)));self.end_headers()
  try:self.wfile.write(payload)
  except (BrokenPipeError,ConnectionResetError):pass
 def do_GET(self):
  if self.path=='/api/info':self.reply({'host_id':'fixture','resource':{'cpu_total':8,'cpu_used':0,'mem_total':8192,'mem_used':0,'disk_total':32768,'disk_used':0,'vm_count':len(self.server.vms)},'storage':'file','engines':['qemu'],'images':['base'],'image_sizes':{'base':16},'capabilities':[]})
  elif self.path=='/api/vms':self.reply(self.server.vms)
  else:raise RuntimeError(self.path)
 def do_POST(self):
  raw=self.rfile.read(int(self.headers.get('Content-Length','0')))
  if self.path=='/api/vms':
   request=json.loads(raw);self.server.entered.set()
   if self.server.block:self.server.release.wait(15)
   vm={'id':request['vm_id'],'env_id':request['env_id'],'host_id':'fixture','image':'base','engine':'qemu','cpu':1,'mem':128,'disk':1024,'ip':'10.10.0.2','port_map':{},'state':'running','created_at':1,'backup':{'revision':str(uuid.uuid4()),'sequence':0,'retired':[],'cleanup_retry_at':0,'cleanup_failures':0,'forwarding_reserved_mib':0}}
   self.server.vms.append(vm);self.reply({'vm':vm},201)
  elif self.path.endswith('/stop'):
   self.server.entered.set();self.server.release.wait(10)
   for vm in self.server.vms:vm['state']='stopped'
   self.reply()
  else:raise RuntimeError(self.path)
def snapshot(directory):
 with sqlite3.connect('file:'+str(pathlib.Path(directory)/'ctl.db')+'?mode=ro',uri=True) as db:
  env=json.loads(db.execute("select data from envs where id='audit-env'").fetchone()[0])
  vms=[json.loads(row[0]) for row in db.execute('select data from vms')]
 return {'env':env['state'],'vms':[v['state'] for v in vms]}
results={}
for mode in ['shutdown','disconnect']:
 with tempfile.TemporaryDirectory(prefix='tt-controller-lifecycle-') as directory:
  peer=http.server.ThreadingHTTPServer(('127.0.0.1',0),Peer);peer.vms=[];peer.entered=threading.Event();peer.release=threading.Event();peer.block=mode=='shutdown'
  thread=threading.Thread(target=peer.serve_forever,daemon=True);thread.start();number=port()
  process=subprocess.Popen([sys.argv[1],'--listen',f'127.0.0.1:{number}','--data-dir',directory],env=dict(os.environ,TT_API_KEY=key,GOMAXPROCS='2'),stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
  try:
   deadline=time.monotonic()+15
   while True:
    if process.poll() is not None:raise RuntimeError('controller exited at startup')
    try:call(number,'GET','/api/status');break
    except OSError:
     if time.monotonic()>deadline:raise
     time.sleep(.02)
   call(number,'POST','/api/hosts',{'addr':f'127.0.0.1:{peer.server_port}'})
   call(number,'POST','/api/envs',{'id':'audit-env','lifetime':0,'vms':[{'image':'base','engine':'qemu','cpu':1,'mem':128,'disk':1024}]})
   if not peer.entered.wait(5):raise RuntimeError('agent mutation not entered')
   if mode=='shutdown':
    process.terminate()
    try:process.wait(timeout=1);early=True
    except subprocess.TimeoutExpired:early=False
    before=snapshot(directory);peer.release.set();process.wait(timeout=10)
    results[mode]={'exited_before_agent_reply':early,'before_release':before,'after_exit':snapshot(directory),'exit_code':process.returncode}
   else:
    deadline=time.monotonic()+10
    while call(number,'GET','/api/envs/audit-env')['env']['state']!='active':
     if time.monotonic()>deadline:raise RuntimeError('creation not completed')
     time.sleep(.02)
    peer.entered.clear()
    client=socket.create_connection(('127.0.0.1',number));client.sendall(f'POST /api/envs/audit-env/stop HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {key}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n'.encode());
    if not peer.entered.wait(5):raise RuntimeError('stop not forwarded')
    client.shutdown(socket.SHUT_WR);client.close();time.sleep(.1)
    peer.release.set();time.sleep(.4)
    results[mode]=snapshot(directory)
  finally:
   peer.release.set()
   if process.poll() is None:process.terminate()
   try:process.wait(timeout=10)
   except subprocess.TimeoutExpired:process.kill();process.wait()
   peer.shutdown();peer.server_close();thread.join()
print(json.dumps(results,indent=2))
