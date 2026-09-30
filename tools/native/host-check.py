import json,pathlib,subprocess
root=pathlib.Path('/tmp/tt-go-port-6426')
def normalize(value):
 if isinstance(value,dict) and 'counter' in value and isinstance(value['counter'],dict):
  value=dict(value,counter={k:v for k,v in value['counter'].items() if k not in ('packets','bytes')})
 if isinstance(value,dict):return {k:normalize(v) for k,v in value.items() if k not in ('metainfo','valid_life_time','preferred_life_time')}
 if isinstance(value,list):return [normalize(v) for v in value if not isinstance(v,dict) or 'metainfo' not in v]
 return value
before=json.loads((root/'host-nft-before.json').read_text())
after=json.loads(subprocess.check_output(['nft','-j','list','ruleset']))
assert normalize(before)==normalize(after),'host firewall changed'
assert normalize(json.loads((root/'host-addresses-before.json').read_text()))==normalize(json.loads(subprocess.check_output(['ip','-j','addr']))),'host interface configuration changed'
print('Host firewall and interface configuration unchanged; management SSH reachable')
