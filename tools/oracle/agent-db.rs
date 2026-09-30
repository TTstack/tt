//! Offline database interchange harness. No Runtime constructor or host tools.
#![allow(dead_code)]
#[path = "../src/runtime.rs"]
mod runtime;
#[path = "../src/ssh_ingress.rs"]
mod ssh_ingress;
fn main() {
    let args = std::env::args().collect::<Vec<_>>();
    assert_eq!(args.len(), 3, "agent-db create|v5|check PATH");
    let result = runtime::oracle_database(&args[1], &args[2]).unwrap();
    println!("{}", serde_json::to_string_pretty(&result).unwrap());
}
