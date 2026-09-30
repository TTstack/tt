
// Appended only to the archived reference for offline interoperability checks.
pub fn oracle_database(action: &str, path: &str) -> Result<serde_json::Value> {
    use serde_json::json;
    let db = Connection::open(path).c(d!())?;
    match action {
        "create" | "v5" => {
            init_db(&db)?;
            db.execute("INSERT INTO _meta VALUES ('host_id','oracle-host'),('container_runtime','podman'),('network_config','{\"ingress\":null,\"port_start\":20000,\"port_end\":65535}')", []).c(d!())?;
            for engine in [Engine::Qemu, Engine::Firecracker, Engine::Docker, Engine::Bhyve, Engine::Jail] {
                let vm: Vm = serde_json::from_value(json!({
                    "id":engine.to_string(),"env_id":"oracle-env","host_id":"oracle-host",
                    "image":"oracle-image","engine":engine,"cpu":2,"mem":256,"disk":128,
                    "ip":"10.10.0.2","port_map":{"22":20022},"state":"stopped","created_at":123,
                    "options":{"ports":[22],"ssh_keys":[],"deny_outgoing":false,"requested_disk":128,"isolated_network":false}
                })).c(d!())?;
                save_vm(&db,&vm)?;
                if action == "v5" {
                    let mut value = serde_json::to_value(&vm).c(d!())?;
                    value.as_object_mut().unwrap().remove("backup");
                    value["future_opaque_field"] = json!({"integer":18446744073709551615u64,"text":"<&>\u{2028}"});
                    db.execute("UPDATE vms SET data=?1 WHERE id=?2",rusqlite::params![value.to_string(),vm.id]).c(d!())?;
                }
            }
            if action == "v5" {
                db.execute("UPDATE _meta SET value='5' WHERE key='schema_version'",[]).c(d!())?;
            }
        }
        "check" => { init_db(&db)?; }
        _ => panic!("unknown fixture action"),
    }
    let vms = load_all_vms(&db)?;
    let host: String = db.query_row("SELECT value FROM _meta WHERE key='host_id'",[],|r|r.get(0)).c(d!())?;
    Ok(json!({"revision":"1ad83a9feed5fac820d74b5e7ef752e50a552740","host_id":host,"vms":vms,"schema_version":get_schema_version(&db)?}))
}

pub fn oracle_http_runtime(path: &str) -> Result<Runtime> {
    let db = Connection::open(path).c(d!())?;
    init_db(&db)?;
    Ok(Runtime {
        probed_capabilities:vec![],backup_enabled:false,
        backup_unsupported:Some("backup unsupported: backend has not been qualified".into()),
        backup_running:HashSet::new(),backup_locks:std::path::Path::new(path).parent().unwrap().into(),
        ssh_ingress:None,host_id:"oracle-host".into(),db,engines:vec![],
        store:storage::create_store(Storage::File),storage:Storage::File,
        image_dir:"/unused".into(),runtime_dir:"/unused".into(),network_ready:false,
        container_runtime:None,port_range:20000..=65535,
        resource:Resource{cpu_total:8,mem_total:4096,disk_total:16384,..Default::default()},
        engine_factory:engine::create_engine,
    })
}
