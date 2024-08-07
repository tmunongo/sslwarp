#[derive(Debug)]
struct Server {
    listener: tokio::TcpListener,
}

pub mod server {
    pub fn new() {
        println!("Creating new server")
    }

    pub fn run() {
        println!("Running server")
    }
}
