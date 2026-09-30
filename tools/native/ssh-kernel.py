import hashlib,json,pathlib,urllib.request
root=pathlib.Path('/tmp/tt-go-port-6426');url='https://s3.amazonaws.com/spec.ccfc.min/firecracker-ci/v1.12/x86_64/vmlinux-5.10.233'
config=urllib.request.urlopen(url+'.config',timeout=30).read().decode()
assert 'CONFIG_HW_RANDOM_VIRTIO=y' in config
with urllib.request.urlopen(url,timeout=60) as response:
 etag=response.headers.get('ETag','').strip('"');sha=hashlib.sha256();md5=hashlib.md5()
 target=root/'images/fc-ssh/vmlinux.new'
 with target.open('wb') as output:
  while block:=response.read(262144):output.write(block);sha.update(block);md5.update(block)
 if len(etag)==32:assert md5.hexdigest()==etag
 assert sha.hexdigest()=='3a30d91f3bf475e3950b9825d047c6839bc64249f383f6173912fe8afe89b5c3'
 target.replace(root/'images/fc-ssh/vmlinux')
 (root/'ssh-kernel.json').write_text(json.dumps(dict(url=url,sha256=sha.hexdigest(),etag=etag))+'\n')
 print('Prepared SSH kernel:',sha.hexdigest())
