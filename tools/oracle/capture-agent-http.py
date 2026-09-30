import subprocess,tempfile,pathlib,http.client,json,re,sys
if len(sys.argv) != 2: raise SystemExit('usage: capture-agent-http.py PINNED_AGENT_HTTP_ORACLE')
oracle=sys.argv[1]
valid={'vm_id':'guest','env_id':'env','image':'alpine','engine':'qemu','cpu':1,'mem':128,'disk':128,'ports':[],'deny_outgoing':False}
enc=lambda v:json.dumps(v,separators=(',',':'),ensure_ascii=False)
cases=[]
def add(label,method='POST',path='/api/vms',body=None,ctype='application/json',headers=None,repeat=None):
 h={} if headers is None else dict(headers)
 if ctype is not None:h['Content-Type']=ctype
 cases.append(dict(label=label,method=method,path=path,input=body,headers=h,repeat=repeat))
for method,path in [('GET','/missing'),('GET','/api/vms/'),('GET','/api//vms'),('GET','/api/%76ms'),('GET','/api/vms/..'),('GET','/api/vms/a%2Fb'),('GET','/api/vms/%ff'),('OPTIONS','/api/vms'),('PUT','/api/vms'),('HEAD','/api/vms'),('GET','/api/images'),('GET','/api/vms?x=1'),('GET','/api/vms/absent'),('DELETE','/api/vms/absent'),('POST','/api/vms/absent/stop'),('POST','/api/vms/absent/start'),('PUT','/api/vms/absent/backup')]:
 add(method+' '+path,method,path,ctype=None)
for value in ['',' ','{','[','{}','[]','null','true','12','"value"','{} {}','{"vm_id":','{"vm_id":"x",}','{"vm_id":"\\uD800"}','{\n"vm_id": "x",\n"env_id": false\n}']:
 add('raw '+repr(value),body=value)
for field in list(valid):
 copy=dict(valid);del copy[field];add('missing '+field,body=enc(copy))
 for value in [None,True,'wrong',-1,1.5,4294967296,{},[],{'qemu':None}]:
  copy=dict(valid);copy[field]=value;add(field+' '+repr(value),body=enc(copy))
for value in ['application/json','application/json; charset=utf-8','application/problem+json','text/json','application/JSON','application/json garbage',None]:add('content-type '+str(value),body=enc(valid),ctype=value)
for tail in ['x',' {}','\ntrue',' ']:add('trailing '+repr(tail),body=enc(valid)+tail)
add('duplicate',body=enc(valid)[:-1]+',"cpu":2}')
add('unknown',body=enc(dict(valid,future={'ignored':'field'})))
add('size-limit',body=None,repeat={'text':' ','count':2097153})
add('resource empty',path='/api/vms/absent/resources',body='{}')
add('restore unknown',path='/api/vms/absent/backup/restore',body='{"generation":"x","extra":true}')
for text in ['{"vm_id":false','{"cpu":"wrong"','{"engine":null','{"unknown":','[false','["guest",false','["guest","env","alpine","qemu",1,128,128,[],false]']:
 add('partial '+repr(text),body=text)
for field,value in [('ports',[None]),('ports',[65536]),('ports',["wrong"]),('ssh',{}),('ssh',{'user':'root','unknown':True}),('guest_config',{'a':None}),('guest_config',{'a':True}),('engine',{'qemu':True}),('engine',{'qemu':None,'extra':None})]:
 copy=dict(valid);copy[field]=value;add('nested '+field+' '+repr(value),body=enc(copy))
for depth in [127,128,129,256]:add('ignored nesting '+str(depth),body=enc(valid)[:-1]+',"ignored":'+'['*depth+'0'+']'*depth+'}')
result=[]
for key in [None,'fixture-key']:
 with tempfile.TemporaryDirectory(prefix='tt-http-oracle-') as root:
  p=subprocess.Popen([oracle,str(pathlib.Path(root)/'agent.db')]+([] if key is None else [key]),stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
  try:
   address=p.stdout.readline().strip()
   selected=cases if key is None else [dict(label='auth '+m+' '+path,method=m,path=path,input='{}',headers=h,repeat=None) for m,path in [('GET','/missing'),('OPTIONS','/api/vms'),('POST','/api/vms'),('HEAD','/api/vms')] for h in [{},{'Authorization':'Bearer wrong'},{'Authorization':'bEaReR fixture-key'},{'Authorization':'Bearer  fixture-key'}]]
   for case in selected:
    c=http.client.HTTPConnection(address,timeout=10)
    body=case['input'] if case['repeat'] is None else case['repeat']['text']*case['repeat']['count']
    c.request(case['method'],case['path'],body=body,headers=case['headers']);r=c.getresponse();response=r.read().decode('utf8')
    normalized=False
    if response and r.getheader('content-type')=='application/json':
     data=json.loads(response)
     if isinstance(data,dict) and isinstance(data.get('error'),str) and '\x1b[' in data['error']:
      plain=re.sub(r'\x1b\[[0-9;]*m','',data['error'])
      data['error']=plain.split('ERROR: ',1)[1].split('\n├──file:',1)[0];response=enc(data);normalized=True
    result.append(dict(case,api_key=key,status=r.status,content_type=r.getheader('content-type'),allow=r.getheader('allow'),body=response,normalized_ruc=normalized));c.close()
  finally:p.terminate();p.communicate(timeout=10)
print(json.dumps(dict(revision='1ad83a9feed5fac820d74b5e7ef752e50a552740',cases=result),ensure_ascii=False,indent=2))
