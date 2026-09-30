// Generate deployment text from the pinned CLI without executing host commands.
#[path = "../src/deploy.rs"]
mod deploy;
#[path = "../src/image_builder.rs"]
mod image_builder;
fn main() {
    println!("{}", serde_json::to_string_pretty(&deploy::fixtures()).unwrap());
}
