import json,socket,sys
s=socket.socket(socket.AF_UNIX);s.connect('/tmp/tt-go-port-6426/lab.sock')
s.sendall(json.dumps(dict(argv=sys.argv[1:],timeout=1800)).encode()+b'\n')
data=b''
while chunk:=s.recv(65536):data+=chunk
result=json.loads(data);sys.stdout.write(result['stdout']);sys.stderr.write(result['stderr']);raise SystemExit(result['code'])
