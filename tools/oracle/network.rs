//! Pure networking fixtures; host commands are impossible in this harness.
use ruc::*;
use serde_json::json;
use sha2::{Digest, Sha256};
use std::process::Command;
use ttcore::command::CommandExt;
use ttcore::net::*;

// The original isolation source is compiled verbatim in this example. The
// generator only appends an export for its private, pure rules function.
#[path = "../src/net/isolation.rs"]
mod isolation;
mod platform {
    pub fn nft(_: &str) -> ruc::Result<()> {
        panic!("fixture generation must never invoke a host firewall")
    }
}
fn main() {
    let identifiers = ["", "vm1", "vm2", "83b7e95-test-vm", "long-id-abcdefghijklmnopqrstuvwxyz", "虚拟机"]
        .into_iter().map(|id| json!({"id":id,"tap":tap_name(id)})).collect::<Vec<_>>();
    let ips = [0,1,252,253,254,999].into_iter().map(|index|json!({"index":index,"ip":vm_ip(index)})).collect::<Vec<_>>();
    let rules = isolation::fixture_rules("vm1", "10.10.0.2").unwrap();
    let lock = std::fs::read("Cargo.lock").unwrap();
    let hash = Sha256::digest(lock).iter().map(|b|format!("{b:02x}")).collect::<String>();
    println!("{}",serde_json::to_string_pretty(&json!({
        "revision":"1ad83a9feed5fac820d74b5e7ef752e50a552740",
        "cargo_lock_sha256":hash,"identifiers":identifiers,"ips":ips,"isolation_rules":rules
    })).unwrap());
}
