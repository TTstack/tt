//! Pure engine command/config fixtures, with extracted pinned Rust constructors.
include!("engine-fragments.rs");
use serde_json::json;
use sha2::{Digest, Sha256};
fn argv(command: Command) -> Vec<String> {
    std::iter::once(command.get_program()).chain(command.get_args())
        .map(|s|s.to_str().unwrap().to_owned()).collect()
}
fn main() {
    let mut vm: Vm = serde_json::from_value(json!({
        "id":"guest","env_id":"env","host_id":"host","image":"alpine",
        "engine":"qemu","cpu":2,"mem":256,"disk":128,"ip":"10.10.1.2",
        "port_map":{"80":20080,"22":20022},"state":"stopped","created_at":0
    })).unwrap();
    // QEMU may inspect this path, but the harness never creates host resources.
    assert!(!Path::new(&Qemu.seed_path(&vm)).exists(), "fixture seed already exists");
    let qemu = argv(Qemu.build_cmd(&vm,"/retained/disk","raw").unwrap());
    let network = Qemu::network_config(&vm).unwrap();
    let docker = argv(Docker::fixture(&vm,"podman"));
    let bhyve = argv(bhyve_command(&vm,Path::new("/retained/disk")).unwrap());
    let jail = Jail::fixture(&vm,"/retained/root").unwrap();
    vm.engine = ttcore::model::Engine::Firecracker;
    let bare = configuration(&vm,"02:54:0a:0a:01:02");
    vm.options.ssh = Some(Default::default());
    let configured = configuration(&vm,"02:54:0a:0a:01:02");
    let lock = std::fs::read("Cargo.lock").unwrap();
    let hash = Sha256::digest(lock).iter().map(|b|format!("{b:02x}")).collect::<String>();
    println!("{}",serde_json::to_string_pretty(&json!({
        "revision":"1ad83a9feed5fac820d74b5e7ef752e50a552740",
        "cargo_lock_sha256":hash,"qemu":qemu,"qemu_network":network,
        "docker":docker,"bhyve":bhyve,"jail":jail,"jailer":argv(jailer_command(&vm)),
        "firecracker":bare,"firecracker_configured":configured,
        "firecracker_bytes":serde_json::to_string(&configured).unwrap()
    })).unwrap());
}
