use serde::{Deserialize, Serialize};

use crate::config::config::Config;

#[derive(Debug, Deserialize, Serialize)]
pub struct Client {
    config: Config,
    tunnel_id: String
}

#[derive(Debug, Deserialize, Serialize)]
pub struct TunnelRequest {
    api_key: String,
    subdomain: String,
    local_add: String,
    message: String,
}

impl Client {
    pub fn new(config: Config) -> Self {
        Client {
            config,
            tunnel_id: "".to_string()
        }
    }

    pub fn run() -> Result<(), Box<dyn std::error::Error>>{
        todo!()
    }
}