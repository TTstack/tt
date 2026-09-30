"""Private task RPC inside bounded mount/network/PID/cgroup namespaces."""
import json,os,pathlib,signal,socketserver,subprocess
root=pathlib.Path('/tmp/tt-go-port-6426');path=root/'lab.sock'
if path.exists():raise RuntimeError('refusing to replace an existing lab socket')
class Handler(socketserver.StreamRequestHandler):
 def handle(self):
  request=json.loads(self.rfile.readline(524288))
  environment=dict(os.environ,PATH=str(root/'tools')+':'+str(root/'bin')+':'+os.environ['PATH'],GOMAXPROCS='2')
  environment.update(request.get('env',{}))
  result=subprocess.run(request['argv'],cwd=root,env=environment,capture_output=True,text=True,timeout=request.get('timeout',1800))
  self.wfile.write(json.dumps(dict(code=result.returncode,stdout=result.stdout,stderr=result.stderr)).encode()+b'\n')
class Server(socketserver.ThreadingUnixStreamServer):daemon_threads=True
with Server(str(path),Handler) as server:
 path.chmod(0o600)
 (root/'lab-ready').write_text('ready\n')
 try:server.serve_forever()
 finally:path.unlink(missing_ok=True)
