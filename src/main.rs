use tokio;
use clap::Parser;
use dotenv;
use crate::config::config::config::load;
use crate::server::server::server::new;

#[derive(Parser, Debug)]
#[command(version, about, long_about = None)]
struct Args {
    // Mode to run sslwarp in
    #[arg(short, long)]
    mode: String,

    // Location of the config file
    #[arg(short, long, default_value_t = String::from(""))]
    config: String,
}

mod server;
mod config;

#[tokio::main]
fn main() {
    dotenv::dotenv().ok();

    let args = Args::parse();

    if args.mode == "server" {
        run_server().await
    } else {
        run_client(args.config).await
    }
}

async fn run_server() {
    println!("SSLWarp is listening on port :8000");

    let _server = new();
}

async fn run_client(path: String) {
    println!("SSLWarp is running in client mode");
    
    let config = load(path);

    match config {
        Ok(config) => crate::client::new(config).await,
        Err(error) => println!("Failed to start client with config error: {}", error)
    }
}