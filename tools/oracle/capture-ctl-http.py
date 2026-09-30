"""Capture the pinned controller against private empty state on loopback."""
import http.client,json,os,pathlib,re,socket,subprocess,sys,tempfile,time
if len(sys.argv)!=2:raise SystemExit('usage: capture-ctl-http.py PINNED_TT_CTL_BINARY')
cases=[]
def add(method,path,body=None,headers=None):cases.append(dict(method=method,path=path,input=body,headers=headers or {}))
for method,path in [('GET','/'),('HEAD','/'),('POST','/'),('GET','/missing'),('GET','/api/hosts'),('OPTIONS','/api/hosts'),('GET','/api/hosts/absent'),('DELETE','/api/hosts/absent'),('POST','/api/hosts/absent/detach'),('GET','/api/envs'),('GET','/api/envs/absent'),('DELETE','/api/envs/absent'),('POST','/api/envs/absent/start'),('POST','/api/envs/absent/stop'),('GET','/api/vms/absent'),('GET','/api/images'),('GET','/api/status'),('GET','/api/envs/'),('GET','/api//envs'),('GET','/api/envs/%ff'),('GET','/api/vms/absent/backup'),('PUT','/api/vms/absent/backup')]:add(method,path)
for path,bodies in [('/api/hosts',['{}','{"addr":"localhost"}','{"addr":"localhost:80"}','{"addr":"user@localhost:9100"}','{"addr":"localhost:65536"}','{"addr":"localhost:9100/path"}']),('/api/envs',['{}','null','{"id":"test","vms":[]}','{"id":"test","vms":[{"image":"ubuntu"}]}','{"id":"test","vms":[{"image":"ubuntu","cpu":0}]}','{"id":"bad/name","vms":[{"image":"ubuntu"}]}','{"id":"test","vms":[{"image":"ubuntu","ports":[0]}]}']),('/api/vms/absent/resources',['{}','{"cpu":1,"mem":128,"disk":16}','{"cpu":0,"mem":128,"disk":16}']),('/api/vms/absent/backup/restore',['{}','{"generation":"bad"}'])]:
 for body in bodies:add('POST',path,body,{'Content-Type':'application/json'})
output=[]
for key in [None,'fixture-key']:
 with tempfile.TemporaryDirectory(prefix='tt-ctl-http-') as root:
  reserve=socket.socket();reserve.bind(('127.0.0.1',0));port=reserve.getsockname()[1];reserve.close()
  env=dict(os.environ);env.pop('TT_API_KEY',None)
  if key is not None:env['TT_API_KEY']=key
  p=subprocess.Popen([sys.argv[1],'--listen',f'127.0.0.1:{port}','--data-dir',root],env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
  try:
   deadline=time.monotonic()+20
   while True:
    if p.poll() is not None:raise RuntimeError(p.stderr.read())
    try:
     probe=socket.create_connection(('127.0.0.1',port),timeout=.1);probe.close();break
    except OSError:
     if time.monotonic()>deadline:raise RuntimeError('controller readiness timeout')
     time.sleep(.05)
   selected=cases if key is None else [dict(method=method,path=path,input=None,headers=headers) for method,path in [('GET','/'),('POST','/'),('GET','/missing'),('GET','/api/hosts'),('OPTIONS','/api/hosts'),('HEAD','/api/envs')] for headers in [{},{'Authorization':'Bearer fixture-key'}]]
   for case in selected:
    c=http.client.HTTPConnection('127.0.0.1',port,timeout=10);c.request(case['method'],case['path'],case['input'],case['headers']);response=c.getresponse();body=response.read().decode('utf8');normalized=False
    if body and response.getheader('content-type')=='application/json':
     value=json.loads(body)
     if isinstance(value,dict) and isinstance(value.get('error'),str) and '\x1b[' in value['error']:
      plain=re.sub(r'\x1b\[[0-9;]*m','',value['error']);value['error']=plain.split('ERROR: ',1)[1].split('\n├──file:',1)[0];body=json.dumps(value,separators=(',',':'),ensure_ascii=False);normalized=True
    html=response.getheader('content-type')=='text/html; charset=utf-8' and case['method']!='HEAD'
    if html:body='<PINNED_DASHBOARD>'
    output.append(dict(case,api_key=key,status=response.status,content_type=response.getheader('content-type'),allow=response.getheader('allow'),csp=response.getheader('content-security-policy'),frame=response.getheader('x-frame-options'),body=body,normalized_ruc=normalized));c.close()
  finally:
   if p.poll() is None:p.terminate()
   p.communicate(timeout=20)
print(json.dumps(dict(revision='1ad83a9feed5fac820d74b5e7ef752e50a552740',cases=output),ensure_ascii=False,indent=2))
