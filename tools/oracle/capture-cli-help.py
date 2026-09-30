"""Capture clap help without loading CLI credentials or running host actions."""
import os,pathlib,subprocess,sys
if len(sys.argv)!=3:raise SystemExit('usage: capture-cli-help.py PINNED_TT_BINARY OUTPUT_DIRECTORY')
binary,directory=sys.argv[1],pathlib.Path(sys.argv[2]);directory.mkdir(parents=True,exist_ok=True)
paths=['','config','capabilities','status','host','host add','host list','host show','host remove','host detach','env','env backup','env backup show','env backup create','env backup restore','env backup delete','env create','env list','env show','env delete','env stop','env start','env resize','image','image list','image recipes','image create','deploy','deploy agent','deploy ctl','deploy all','deploy dist']
env=dict(os.environ);env.pop('TT_API_KEY',None)
for path in paths:
 result=subprocess.run([binary]+path.split()+['--help'],env=env,capture_output=True,check=True)
 (directory/((path.replace(' ','-') or 'root')+'.txt')).write_bytes(result.stdout)
for parent in ['', 'host', 'env', 'env backup', 'image', 'deploy']:
 result=subprocess.run([binary]+parent.split()+['help','help'],env=env,capture_output=True,check=True)
 path=(parent+' help').strip()
 (directory/(path.replace(' ','-')+'.txt')).write_bytes(result.stdout)
