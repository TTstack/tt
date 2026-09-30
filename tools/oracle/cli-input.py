"""Exercise parser boundaries without credentials, host mutation or deployment."""
import http.server,json,os,subprocess,sys,threading
if len(sys.argv)!=2:raise SystemExit('usage: cli-input.py TT_BINARY')
requests=[]
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  requests.append(self.path)
  payload=json.dumps({'ok':True,'data':{'total_hosts':0,'hosts_online':0,'total_envs':0,'total_vms':0,'cpu_total':0,'cpu_used':0,'mem_total':0,'mem_used':0,'disk_total':0,'disk_used':0}} if self.path=='/api/status' else {'ok':True,'data':[]}).encode()
  self.send_response(200);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(payload)));self.end_headers();self.wfile.write(payload)
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
environment=dict(os.environ);environment.pop('TT_API_KEY',None)
results=[]
try:
 for args in [['--','status'],['env','--','list'],['env','create','fixture','--image','base','--owner','-','--dup','0'],['env','delete','--','--help']]:
  requests.clear()
  result=subprocess.run([sys.argv[1],'--server',f'127.0.0.1:{server.server_port}']+args,env=environment,capture_output=True,text=True,timeout=5)
  results.append({'args':args,'exit':result.returncode,'requests':requests.copy()})
finally:server.shutdown();server.server_close();thread.join()
print(json.dumps(results,indent=2))
