use clap::Parser;
use dotenv::dotenv;

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

#[tokio::main]
async fn main() {
    dotenv().ok();

    let args = Args::parse();

    if args.mode == "server" {
        run_server()
    } else {
        run_client()
    }
}

fn run_server() {
    println!("SSLWarp is listening on port :8000");

    server = crate::server::new()


}

fn run_client() {
    println!("SSLWarp is running in client mode");
    
    let config = crate::config.load();

    match config {
        Ok(config) => crate::client::new(config),
        Err(error) => println!("Failed to start client with config error: {}", error)
    }
}