use std::{collections::HashMap, iter::Map, sync::Arc};

use serde::{Deserialize, Serialize};
use tokio::{io::{AsyncReadExt, AsyncWriteExt}, net::{TcpListener, TcpStream}, sync::Mutex};
use uuid::Uuid;

#[derive(Serialize, Deserialize, Debug)]
struct Server {
    tunnels: Arc<Mutex<HashMap<String, TcpStream>>>,
}

#[derive(Serialize, Deserialize, Debug)]
struct ReceivedRequest {
    api_key: String,
    subdomain: String,
    local_addr: String,
    message: String
}

impl Server {
    fn new() -> Self {
        Server {
            tunnels: Arc::new(Mutex::new(HashMap::new()))
        }
    }

    async fn run(&self) -> Result<(), Box<dyn std::error::Error>> {
        let listener = TcpListener::bind("0.0.0.0:3000").await?;
        println!("Server running on port 3000");

        loop {
            let (stream, _) = listener.accept().await?;
            let tunnels = Arc::clone(&self.tunnels);

            tokio::spawn(async move {
                if let Err(e) = handle_connection(stream, tunnels).await {
                    eprintln!("Error handling connection: {}", e);
                }
            })
        }
    }
}

async fn handle_connection(mut stream: TcpStream, tunnels: Arc<Mutex<HashMap<String, TcpStream>>>) -> Result<(), Box<dyn std::error::Error>> {
    let mut buffer = [0; 1024];
    let n = stream.read(&mut buffer).await?;
    let json_request = String::from_utf8_lossy(&buffer[..n]);

    let client_request: ReceivedRequest = merde_json::from_str(&json_request)?;
    println!("Received request: {:?}", client_request);

    if client_request.message == "TUNNEL_REQUEST" {
        println!("Establishing");

        establish_tunnel(stream, tunnels).await?;
    } else {
        handle_client_request(stream, tunnels, &client_request.message).await?;
    }

    Ok(())
}

async fn establish_tunnel(mut stream: TcpStream, tunnels: Arc<Mutex<HashMap<String, TcpStream>>>) -> Result<(), Box<dyn std::error::Error>> {
    let tunnel_id = Uuid::new_v4().to_string();

    tunnels.lock().await.insert(tunnel_id.clone(), stream.try_clone().await?);
    println!("Connection established with ID: {}", tunnel_id);

    let json = merde_json::to_string(&tunnel_id)?;

    stream.write_all(json.as_bytes()).await?;
    stream.flush().await?;

    loop {
        let mut buffer = [0; 1024];

        match stream.read(&mut buffer).await {
            Ok(0) | Err(_) => {
                println!("Tunnel {} closed", tunnel_id);
                tunnels.lock().await.remove(&tunnel_id);
                Ok()
            }
            Ok(_) => continue,
        }
    }
}

async fn handle_client_request(mut client_stream: TcpStream, tunnels: Arc<Mutex<HashMap<String, TcpStream>>>, msg: &str) -> Result<(), Box<dyn std::error::Error>> {
    let tunnel_id = msg;

    if Uuid::parse_str(tunnel_id).is_err() {
        client_stream.write_all(b"Provided an invalid tunnel ID\n").await?;
        return Ok(());
    }

    let mut tunnels = tunnels.lock().await;
    let tunnel_stream = match tunnels.get_mut(tunnel_id) {
        Some(stream) => stream.try_clone().await?,
        None => {
            client_stream.write_all(b"Tunnel not found\n").await?;
            Ok(())
        }
    };

    drop(tunnels);

    handle_full_duplex_communication(client_stream, tunnel_stream).await
}

async fn handle_full_duplex_communication(mut client_stream: TcpStream, mut tunnel_stream: TcpStream) -> Result<(), Box<dyn std::error::Error>> {
    let (mut client_read, mut client_write) = client_stream.split();
    let (mut tunnel_read, mut tunnel_write) = tunnel_stream.split();

    tokio::try_join!(client_to_tunnel, tunnel_to_client)?;

    println!("Client request handled!");

    Ok(())
}