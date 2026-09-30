"""Run the pinned CLI against synthetic loopback responses; never deploy guests."""
import copy,http.server,json,os,pathlib,subprocess,sys,threading
if len(sys.argv)!=3:raise SystemExit('usage: capture-cli-output.py PINNED_TT_BINARY WIRE_FIXTURE')
wire=json.loads(pathlib.Path(sys.argv[2]).read_text())
def full(kind):return copy.deepcopy(next(c['output'] for c in wire['cases'] if c['kind']==kind and c['name']=='full' and c['ok']))
host,vm,env=full('Host'),full('Vm'),full('Env')
vm['port_map']={'22':20022,'80':20080,'443':20443}
vm['host_id']=host['id'];env['vm_ids']=[vm['id']];env['expires_at']=0
matrix=wire['matrix'];backup=full('BackupView');backup['vm']=vm
requests=[];empty=False
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):self.handle_api()
 def do_POST(self):self.handle_api()
 def do_PUT(self):self.handle_api()
 def do_DELETE(self):self.handle_api()
 def handle_api(self):
  raw=self.rfile.read(int(self.headers.get('Content-Length','0')));body=json.loads(raw) if raw else None
  requests.append(dict(method=self.command,path=self.path,body=body,idempotency_key=self.headers.get('Idempotency-Key'),if_match=self.headers.get('If-Match')))
  data=None;include=True;status=200
  if self.path=='/api/status':data=full('FleetStatus')
  elif self.path=='/api/capabilities':data=matrix
  elif self.path=='/api/hosts':data=([] if empty else [host]) if self.command=='GET' else host
  elif self.path.endswith('/detach'):data=[vm]
  elif self.path.startswith('/api/hosts/'):
   if self.command=='DELETE':include=False
   else:data=host
  elif self.path=='/api/envs':
   if self.command=='GET':data=[] if empty else [env]
   else:
    created=copy.deepcopy(env);created.update(id=body['id'],owner=body['owner'],state='active');guest=copy.deepcopy(vm);guest['state']='running';data=dict(env=created,vms=[guest],warnings=[]);status=202
  elif self.path.startswith('/api/envs/'):
   if self.command!='GET':include=False
   else:data=dict(env=env,vms=[vm],warnings=[])
  elif self.path.endswith('/resources'):
   data=copy.deepcopy(vm);data.update(cpu=body['cpu'],mem=body['mem'],disk=body['disk']);data['options']['requested_disk']=body['disk']
  elif '/backup' in self.path:data=backup
  elif self.path=='/api/images':data=[] if empty else [dict(name='image-fixture',host_id=host['id'])]
  else:raise RuntimeError(self.path)
  response={'ok':True}
  if include:response['data']=data
  payload=json.dumps(response,separators=(',',':')).encode();self.send_response(status);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(payload)));self.end_headers();self.wfile.write(payload)
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
cases=[]
commands=[['status'],['capabilities'],['capabilities','--json'],['host','list'],['host','show',host['id']],['host','add','192.0.2.1:9100'],['host','remove',host['id']],['host','detach',host['id']],['env','list'],['env','show',env['id']],['env','delete',env['id']],['env','stop',env['id']],['env','start',env['id']],['env','resize',vm['id'],'--cpu','2','--mem','512','--disk','4096'],['env','backup','show',vm['id']],['image','list'],['image','recipes'],['env','create','new','--image','image-fixture','--owner','fixture','--cpu','1','--mem','256','--disk','2048','--lifetime','0']]
for action in ['create','restore','delete']:
 command=['env','backup',action,vm['id'],'--operation-id','00000000-0000-4000-8000-000000000002','--expected-revision',vm['backup']['revision']]
 if action=='restore':command+=['--generation','00000000-0000-4000-8000-000000000003']
 commands.append(command)
environment=dict(os.environ);environment.pop('TT_API_KEY',None)
try:
 for command in commands+[['host','list'],['env','list'],['image','list']]:
  empty=len(cases)>=len(commands);requests.clear()
  result=subprocess.run([sys.argv[1],'--server',f'127.0.0.1:{server.server_port}']+command,env=environment,capture_output=True,text=True,timeout=20)
  if result.returncode!=0:raise RuntimeError(str(command)+result.stderr)
  cases.append(dict(args=command,empty=empty,stdout=result.stdout,stderr=result.stderr,requests=copy.deepcopy(requests)))
finally:server.shutdown();server.server_close();thread.join()
print(json.dumps(dict(revision=wire['revision'],host=host,vm=vm,env=env,matrix=matrix,backup=backup,status=full('FleetStatus'),cases=cases),ensure_ascii=False,indent=2))
