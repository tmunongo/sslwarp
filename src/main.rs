use client::client::Client;
use config::config::load;
// use tokio;
use clap::Parser;
use dotenv;

#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Args {
    // Mode to run sslwarp in
    #[arg(short, long)]
    mode: String,

    // Location of the config file
    #[arg(short, long, default_value_t = String::from(""))]
    config: String,
}

mod client;
mod server;
mod config;

#[tokio::main]
async fn main() {
    dotenv::dotenv().ok();

    let args = Args::parse();

    if args.mode == "server" {
        run_server().await
    } else {
        run_client(args.config).await
    }
}

async fn run_server() -> Result<(), Box<dyn std::error::Error>> {
    println!("SSLWarp is listening on port :8000");

    let server = Server::new();

    server.run().await
}

async fn run_client(path: String) {
    println!("SSLWarp is running in client mode");
    
    let config = load(path);

    match config {
        Ok(config) => Client::new(config).await,
        Err(_error) => println!("Failed to start client with config error")
    }
}