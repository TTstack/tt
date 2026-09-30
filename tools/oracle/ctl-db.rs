//! Offline controller database oracle; no networking or service startup.
#![allow(dead_code)]
#[path = "../src/db.rs"] mod db;
fn main(){let args=std::env::args().collect::<Vec<_>>();assert_eq!(args.len(),3,"ctl-db create|v4|check PATH");println!("{}",serde_json::to_string_pretty(&db::oracle_database(&args[1],&args[2]).unwrap()).unwrap());}
