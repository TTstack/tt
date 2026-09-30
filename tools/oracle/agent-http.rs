//! HTTP differential oracle with empty, unqualified inventory. No host setup.
#![allow(dead_code)]
#[path = "../src/runtime.rs"] mod runtime;
#[path = "../src/ssh_ingress.rs"] mod ssh_ingress;
#[path = "../src/handler.rs"] mod handler;
#[path = "../src/auth.rs"] mod auth;
use axum::{Router,routing::{get,post}};
#[tokio::main(flavor="current_thread")]
async fn main() {
    let args=std::env::args().collect::<Vec<_>>();
    let rt=runtime::oracle_http_runtime(&args[1]).unwrap();
    let state=std::sync::Arc::new(handler::AgentShared {
        backup_settings:rt.backup_settings(),info:rt.agent_info().unwrap(),
        runtime:std::sync::Arc::new(tokio::sync::Mutex::new(rt)),db_path:args[1].clone(),
        image_dir:"/unused".into(),images:Default::default()
    });
    let app=Router::new()
        .route("/api/info",get(handler::get_info))
        .route("/api/images",get(handler::list_images))
        .route("/api/vms",get(handler::list_vms).post(handler::create_vm))
        .route("/api/vms/{id}",get(handler::get_vm).delete(handler::destroy_vm))
        .route("/api/vms/{id}/stop",post(handler::stop_vm))
        .route("/api/vms/{id}/start",post(handler::start_vm))
        .route("/api/vms/{id}/resources",post(handler::resize_vm))
        .route("/api/vms/{id}/backup",get(handler::get_backup).put(handler::create_backup).delete(handler::delete_backup))
        .route("/api/vms/{id}/backup/restore",post(handler::restore_backup))
        .with_state(state);
    let app=if let Some(key)=args.get(2){app.layer(axum::middleware::from_fn(auth::make_auth_layer(key.clone())))}else{app};
    let listener=tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    println!("{}",listener.local_addr().unwrap());
    axum::serve(listener,app).await.unwrap();
}
