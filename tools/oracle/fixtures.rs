//! Emit synthetic wire fixtures using the pinned ttcore implementation.
use serde::{de::DeserializeOwned, Serialize};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;
use ttcore::{api::*, backup, capability, model::*, ssh};

const REVISION: &str = "1ad83a9feed5fac820d74b5e7ef752e50a552740";
const TOKEN: &str = "00000000-0000-4000-8000-000000000001";

fn normalize(v: &mut Value) {
    match v {
        Value::Object(fields) => {
            // Vm's serde(default) creates a random backup revision. Assert UUID
            // validity here and in Go, then normalize only this volatile value.
            if let Some(revision) = fields.get_mut("revision") {
                if let Some(text) = revision.as_str() {
                    uuid::Uuid::parse_str(text).expect("backup revision UUID");
                    *revision = json!(TOKEN);
                }
            }
            for value in fields.values_mut() { normalize(value); }
        }
        Value::Array(values) => for value in values { normalize(value); },
        _ => (),
    }
}
fn record<T: DeserializeOwned + Serialize>(cases: &mut Vec<Value>, kind: &str, name: &str, input: String) {
    let result = serde_json::from_str::<T>(&input);
    let (ok, output) = match result {
        Ok(value) => { let mut v=serde_json::to_value(value).unwrap(); normalize(&mut v); (true, v) },
        Err(_) => (false, Value::Null),
    };
    cases.push(json!({"kind":kind,"name":name,"input":input,"ok":ok,"output":output}));
}
// Declaration order from the pinned Rust wire structs, including skipped fields.
fn field_order(kind: &str) -> &[&str] { match kind {
"Resource" => &["cpu_total","cpu_used","mem_total","mem_used","disk_total","disk_used","vm_count"],
"Host" => &["capability_report","error","image_sizes","capabilities","id","addr","resource","state","engines","images","storage","registered_at"],
"Vm" => &["backup","ssh","pending_resources","id","env_id","host_id","image","engine","cpu","mem","disk","ip","port_map","options","error","state","created_at"],
"VmResources" => &["cpu","mem","disk"],
"VmOptions" => &["ssh","ports","ssh_keys","deny_outgoing","requested_disk","isolated_network","guest_config_digest"],
"Env" => &["id","owner","vm_ids","created_at","expires_at","error","state"],
"CreateVmReq" => &["vm_id","env_id","image","engine","cpu","mem","disk","ports","deny_outgoing","isolated_network","guest_config","ssh_keys","ssh"],
"CreateVmResp" => &["vm"],
"AgentInfo" => &["capability_report","vms","warnings","image_sizes","capabilities","host_id","resource","engines","storage","images"],
"VmSpec" => &["image","engine","cpu","mem","disk","ports","deny_outgoing","isolated_network","guest_config","ssh_keys","ssh"],
"CreateEnvReq" => &["id","owner","vms","lifetime","ssh_keys"],
"EnvDetail" => &["env","vms","warnings"],
"RegisterHostReq" => &["addr"],
"ImageInfo" => &["name","host_id"],
"FleetStatus" => &["hosts","hosts_online","total_vms","total_envs","cpu_total","cpu_used","mem_total","mem_used","disk_total","disk_used"],
"RespUnit" | "RespU64" => &["ok","data","error"],
"BackupRequest" => &["operation_id","expected_revision","action","generation"],
"BackupGeneration" => &["id","backend","root_bytes","created_at","identity","dependencies","restore_staging"],
"BackupPending" => &["request","artifact","disk_identity","error"],
"BackupReceipt" => &["request","completed_at","error"],
"BackupState" => &["revision","sequence","current","pending","retired","last_result","cleanup_error","cleanup_retry_at","cleanup_failures","forwarding","forwarding_reserved_mib","forwarding_settled_sequence"],
"BackupView" => &["vm","enabled","supported","unsupported_reason"],
"RestoreBody" => &["generation"],
"Definition" => &["tag","meaning"],
"DesignSupport" => &["tag","supported","limit"],
"Combination" => &["engine","storage","platform","experimental","features"],
"Matrix" => &["version","definitions","create_fields","combinations"],
"CapabilityStatus" => &["tag","enabled","reason","evidence"],
"EngineReport" => &["engine","features"],
"CapabilityReport" => &["version","storage","engines","backup_admission"],
"SshOptions" => &["user","sudo"],
"SshInfo" => &["observation_error","user","sudo","host","port","host_key","ready","checked_at","initialized"],
_ => &[],
}}
fn seed<T: DeserializeOwned + Serialize>(cases: &mut Vec<Value>,kind: &str, value: Value) {
    record::<T>(cases,kind,"full",value.to_string());
    record::<T>(cases,kind,"null","null".into());
    record::<T>(cases,kind,"empty-object","{}".into());
    record::<T>(cases,kind,"empty-array","[]".into());
    record::<T>(cases,kind,"trailing-value",format!("{} true",value));
    if let Value::Object(fields)=value {
        let ordered=field_order(kind).iter().map(|key|fields.get(*key).cloned().unwrap_or_else(||match *key {
            "evidence"|"warnings"|"ssh_keys" => json!([]), "guest_config" => json!({}), _ => Value::Null,
        })).collect::<Vec<_>>();
        for count in 0..=ordered.len(){
            record::<T>(cases,kind,&format!("sequence-{count}"),serde_json::to_string(&ordered[..count]).unwrap());
        }
        let mut extra=ordered;extra.push(json!("extra"));
        record::<T>(cases,kind,"sequence-extra",serde_json::to_string(&extra).unwrap());
        for key in fields.keys() {
            let mut absent=fields.clone(); absent.remove(key);
            record::<T>(cases,kind,&format!("missing-{key}"),Value::Object(absent).to_string());
            let mut null=fields.clone(); null.insert(key.clone(),Value::Null);
            record::<T>(cases,kind,&format!("null-{key}"),Value::Object(null).to_string());
            for replacement in [json!(-1),json!(4294967296u64),json!(1.5),json!(true),json!([])] {
                if fields[key].is_number() {
                    let mut invalid=fields.clone();invalid.insert(key.clone(),replacement.clone());
                    record::<T>(cases,kind,&format!("number-{key}-{replacement}"),Value::Object(invalid).to_string());
                }
            }
            let mut changed=fields.clone();let old=changed.remove(key).unwrap();changed.insert(key.to_uppercase(),old);
            record::<T>(cases,kind,&format!("case-{key}"),Value::Object(changed).to_string());
            let base=Value::Object(fields.clone()).to_string();
            record::<T>(cases,kind,&format!("duplicate-{key}"),format!("{},{}:{}}}",&base[..base.len()-1],json!(key),fields[key]));
        }
        let mut future=fields.clone();future.insert("future_extension".into(),json!(true));
        record::<T>(cases,kind,"unknown-field",Value::Object(future).to_string());
        let base=Value::Object(fields).to_string();
        record::<T>(cases,kind,"unknown-invalid-surrogate",format!("{},\"future_extension\":\"\\ud800\"}}",&base[..base.len()-1]));
    }
}
fn enum_forms<T: DeserializeOwned + Serialize>(cases: &mut Vec<Value>, kind: &str, variant: &str) {
    for (name, input) in [
        ("unit-object", format!("{{\"{variant}\":null}}")),
        ("unit-object-value", format!("{{\"{variant}\":true}}")),
        ("unit-object-extra", format!("{{\"{variant}\":null,\"extra\":null}}")),
        ("unit-object-duplicate", format!("{{\"{variant}\":null,\"{variant}\":null}}")),
        ("unit-object-unknown", "{\"unknown\":null}".into()),
    ] { record::<T>(cases, kind, name, input); }
}
fn main() {
    let mut cases=vec![];
    let mut state=backup::State::default();state.revision=TOKEN.into();
    let backup_state=serde_json::to_value(&state).unwrap();
    let resource=json!({"cpu_total":8,"cpu_used":0,"mem_total":8192,"mem_used":0,"disk_total":100000,"disk_used":0,"vm_count":0});
    let ssh_options=json!({"user":"root","sudo":false});
    let ssh_info=json!({"user":"root","sudo":false,"host":"127.0.0.1","port":20000,"host_key":"ssh-ed25519 AAAA synthetic","ready":false,"checked_at":0,"initialized":false,"observation_error":null});
    let options=json!({"ports":[],"ssh_keys":[],"deny_outgoing":false,"requested_disk":0,"isolated_network":false,"ssh":ssh_options,"guest_config_digest":null});
    let vm=json!({"backup":backup_state,"ssh":ssh_info,"pending_resources":null,"id":"vm-fixture","env_id":"env-fixture","host_id":"host-fixture","image":"image-fixture","engine":"qemu","cpu":1,"mem":256,"disk":2048,"ip":"10.10.0.2","port_map":{"22":20000},"options":options,"error":null,"state":"stopped","created_at":18446744073709551615u64});
    let env=json!({"id":"env-fixture","owner":"fixture","vm_ids":["vm-fixture"],"created_at":1,"expires_at":0,"error":null,"state":"stopped"});
    let report=capability::Report::from_probes(Storage::File,&[Engine::Qemu,Engine::Firecracker],&capability::legacy::ALL.iter().map(|s|s.to_string()).collect::<Vec<_>>(),true,None);
    let host=json!({"capability_report":report,"error":null,"image_sizes":{},"capabilities":[],"id":"host-fixture","addr":"127.0.0.1:9100","resource":resource,"state":"online","engines":["qemu"],"images":[],"storage":"file","registered_at":0});
    let spec=json!({"image":"image-fixture","engine":"qemu","cpu":null,"mem":null,"disk":null,"ports":[],"deny_outgoing":false,"isolated_network":false,"guest_config":{},"ssh_keys":[],"ssh":null});
    let request=json!({"operation_id":TOKEN,"expected_revision":TOKEN,"action":"create","generation":null});
    let generation=json!({"id":TOKEN,"backend":"reflink","root_bytes":18446744073709551615u64,"created_at":1,"identity":"synthetic:inode","dependencies":"synthetic","restore_staging":false});
    let status=json!({"tag":"future.custom.tag","enabled":false,"reason":null,"evidence":[]});
    macro_rules! wire {($t:ty,$kind:expr,$value:expr)=>{seed::<$t>(&mut cases,$kind,$value)};}
    wire!(Resource,"Resource",resource.clone());
    wire!(Host,"Host",host);
    wire!(Vm,"Vm",vm.clone());
    wire!(VmResources,"VmResources",json!({"cpu":1,"mem":256,"disk":2048}));
    wire!(VmOptions,"VmOptions",options);
    for key in ["+22","022","65536","-1","x","22.0"] {
        let mut keyed=vm.clone();keyed["port_map"]=json!({key:20000});
        record::<Vm>(&mut cases,"Vm",&format!("port-key-{key}"),keyed.to_string());
    }
    let raw=vm.to_string().replace("\"22\":20000",r#""\u0032\u0032":20000"#);
    record::<Vm>(&mut cases,"Vm","escaped-port-key",raw);
    wire!(Env,"Env",env.clone());
    wire!(ssh::SshOptions,"SshOptions",ssh_options);
    wire!(ssh::SshInfo,"SshInfo",ssh_info);
    wire!(backup::Request,"BackupRequest",request.clone());
    wire!(backup::Generation,"BackupGeneration",generation.clone());
    wire!(backup::Pending,"BackupPending",json!({"request":request,"artifact":generation,"disk_identity":"synthetic","error":null}));
    wire!(backup::Receipt,"BackupReceipt",json!({"request":request,"completed_at":2,"error":null}));
    wire!(backup::State,"BackupState",backup_state);
    wire!(backup::View,"BackupView",json!({"vm":vm,"enabled":true,"supported":true,"unsupported_reason":null}));
    wire!(backup::RestoreBody,"RestoreBody",json!({"generation":TOKEN}));
    wire!(capability::Status,"CapabilityStatus",status.clone());
    wire!(capability::EngineReport,"EngineReport",json!({"engine":"qemu","features":[status]}));
    wire!(capability::Report,"CapabilityReport",serde_json::to_value(&report).unwrap());
    let matrix=capability::matrix();
    wire!(capability::Matrix,"Matrix",serde_json::to_value(&matrix).unwrap());
    wire!(capability::Definition,"Definition",serde_json::to_value(&matrix.definitions[0]).unwrap());
    wire!(capability::DesignSupport,"DesignSupport",serde_json::to_value(&matrix.combinations[0].features[0]).unwrap());
    wire!(capability::Combination,"Combination",serde_json::to_value(&matrix.combinations[0]).unwrap());
    wire!(CreateVmReq,"CreateVmReq",json!({"vm_id":"vm-fixture","env_id":"env-fixture","image":"image-fixture","engine":"qemu","cpu":1,"mem":256,"disk":2048,"ports":[],"deny_outgoing":false,"isolated_network":false,"guest_config":{},"ssh_keys":[],"ssh":null}));
    wire!(CreateVmResp,"CreateVmResp",json!({"vm":vm}));
    wire!(AgentInfo,"AgentInfo",json!({"capability_report":report,"vms":[],"warnings":[],"image_sizes":{},"capabilities":[],"host_id":"host-fixture","resource":resource,"engines":["qemu"],"storage":"file","images":[]}));
    wire!(VmSpec,"VmSpec",spec.clone());
    wire!(CreateEnvReq,"CreateEnvReq",json!({"id":"env-fixture","owner":"fixture","vms":[spec],"lifetime":null,"ssh_keys":[]}));
    wire!(EnvDetail,"EnvDetail",json!({"env":env,"vms":[vm],"warnings":[]}));
    wire!(RegisterHostReq,"RegisterHostReq",json!({"addr":"127.0.0.1:9100"}));
    wire!(ImageInfo,"ImageInfo",json!({"name":"image-fixture","host_id":"host-fixture"}));
    wire!(FleetStatus,"FleetStatus",json!({"hosts":0,"hosts_online":0,"total_vms":0,"total_envs":0,"cpu_total":0,"cpu_used":0,"mem_total":0,"mem_used":0,"disk_total":0,"disk_used":0}));
    wire!(ApiResp<u64>,"RespU64",json!({"ok":true,"data":18446744073709551615u64,"error":null}));
    wire!(ApiResp<()>,"RespUnit",json!({"ok":true,"data":null,"error":null}));
    for value in ["qemu","firecracker","bhyve","docker","jail","kvm","fc","podman","QEMU",""] {wire!(Engine,"Engine",json!(value));}
    for value in ["file","zvol","File","unknown"] {wire!(Storage,"Storage",json!(value));}
    for value in ["running","stopped","paused","creating","failed","deleting","unknown"] {wire!(VmState,"VmState",json!(value));}
    for value in ["active","stopped","creating","deleting","failed","unknown"] {wire!(EnvState,"EnvState",json!(value));}
    for value in ["online","offline","unknown"] {wire!(HostState,"HostState",json!(value));}
    for value in ["zvol","reflink","unknown"] {wire!(backup::Backend,"BackupBackend",json!(value));}
    for value in ["create","restore","delete","unknown"] {wire!(backup::Action,"BackupAction",json!(value));}
    enum_forms::<Engine>(&mut cases,"Engine","qemu");
    enum_forms::<Storage>(&mut cases,"Storage","file");
    enum_forms::<VmState>(&mut cases,"VmState","stopped");
    enum_forms::<EnvState>(&mut cases,"EnvState","active");
    enum_forms::<HostState>(&mut cases,"HostState","online");
    enum_forms::<backup::Backend>(&mut cases,"BackupBackend","reflink");
    enum_forms::<backup::Action>(&mut cases,"BackupAction","create");
    wire!(ttcore::guest_config::GuestConfig,"GuestConfig",json!({"a":"<>&\u{2028}\u{2029}é\n\t\u{0}"}));
    let digests=[BTreeMap::new(),BTreeMap::from([("a".into(),"<>&\u{2028}\u{2029}é\n\t\u{0}".into()),("z".into(),"\\\"".into())])].into_iter().map(|files|json!({"files":files,"digest":ttcore::guest_config::digest(&files),"canonical":serde_json::to_string(&files).unwrap()})).collect::<Vec<_>>();
    let lock=std::fs::read("Cargo.lock").expect("run from archived Rust workspace root");
    let hash=Sha256::digest(lock).iter().map(|b|format!("{b:02x}")).collect::<String>();
    let output=json!({"revision":REVISION,"cargo_lock_sha256":hash,"cases":cases,"digests":digests,"matrix":matrix,"report":report});
    println!("{}",serde_json::to_string_pretty(&output).unwrap());
}
