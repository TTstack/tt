"""Private task RPC inside bounded mount/network/PID/cgroup namespaces."""
import json,os,pathlib,signal,socketserver,subprocess
root=pathlib.Path('/tmp/tt-go-port-6426');path=root/'lab.sock'
if path.exists():raise RuntimeError('refusing to replace an existing lab socket')
# Daemonized engines can outlive their service parent during recovery drills.
# Namespace PID 1 must reap adopted children; the RPC worker owns and waits for
# its direct commands separately, so their exit statuses cannot be stolen.
if os.getpid()==1:
 worker=os.fork()
 if worker:
  stopping=False
  def terminate(signum,_frame):
   global stopping
   stopping=True
   try:os.kill(worker,signum)
   except ProcessLookupError:pass
  signal.signal(signal.SIGTERM,terminate)
  signal.signal(signal.SIGINT,terminate)
  while True:
   try:child,status=os.wait()
   except InterruptedError:continue
   if child==worker:raise SystemExit(0 if stopping else os.waitstatus_to_exitcode(status))
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
