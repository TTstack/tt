// Only parse the pinned configuration and inspect an in-memory connection.
// Never initialize an agent, an engine, networking, or persistent state.
#[path = "../src/config.rs"]
mod config;
use clap::Parser;
fn main() {
    let cfg = config::Config::parse();
    let db = rusqlite::Connection::open_in_memory().unwrap();
    let busy: u32 = db.query_row("PRAGMA busy_timeout", [], |row| row.get(0)).unwrap();
    println!("{}", serde_json::json!({
        "cpu_total": cfg.cpu_total, "mem_total": cfg.mem_total,
        "disk_total": cfg.disk_total, "port_start": cfg.port_start,
        "port_end": cfg.port_end, "busy_timeout_ms": busy,
        "prepared_marker_accepts_vertical_tab": b"1\x0b".trim_ascii() == b"1"
    }));
}
