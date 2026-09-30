
// Appended only to the archived oracle build.
pub fn oracle_database(action:&str,path:&str)->Result<serde_json::Value>{
    use serde_json::json;
    let db=Db::open(path)?;
    match action{
        "create"|"v4"=>{
            let host:Host=serde_json::from_value(json!({"id":"oracle-host","addr":"127.0.0.1:9100","resource":{"cpu_total":8,"cpu_used":0,"mem_total":8192,"mem_used":0,"disk_total":16384,"disk_used":0,"vm_count":0},"state":"online","engines":["qemu","firecracker","bhyve","docker","jail"],"images":["base"],"storage":"file","registered_at":123})).c(d!())?;
            db.put_host(&host)?;
            let mut vms=vec![];
            for engine in [Engine::Qemu,Engine::Firecracker,Engine::Docker,Engine::Bhyve,Engine::Jail]{
                let vm:Vm=serde_json::from_value(json!({"id":engine.to_string(),"env_id":"oracle-env","host_id":"oracle-host","image":"base","engine":engine,"cpu":1,"mem":128,"disk":16,"ip":"10.10.0.2","port_map":{},"state":"stopped","created_at":123})).c(d!())?;
                vms.push(vm);
            }
            let env=Env{id:"oracle-env".into(),owner:"fixture".into(),vm_ids:vms.iter().map(|v|v.id.clone()).collect(),created_at:123,expires_at:0,error:None,state:EnvState::Stopped};
            db.put_environment(&env,&vms)?;
            if action=="v4"{
                for vm in &vms{
                    let mut value=serde_json::to_value(vm).c(d!())?;value.as_object_mut().unwrap().remove("backup");value["future_opaque_field"]=json!({"integer":18446744073709551615u64});
                    db.conn.execute("UPDATE vms SET data=?1 WHERE id=?2",rusqlite::params![value.to_string(),vm.id]).c(d!())?;
                }
                Db::set_schema_version(&db.conn,4)?;
            }
        }
        "check"=>{},
        _=>panic!("unknown fixture action"),
    }
    Ok(json!({"revision":"1ad83a9feed5fac820d74b5e7ef752e50a552740","schema_version":Db::get_schema_version(&db.conn)?,"hosts":db.list_hosts()?,"envs":db.list_envs()?,"vms":db.vms_by_env("oracle-env")?}))
}
