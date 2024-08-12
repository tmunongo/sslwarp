use serde::Deserialize;

#[derive(Deserialize, Debug)]
pub struct Config {
    pub api_key: String,
    pub tunnels: Vec<TunnelConfig>
}

#[derive(Deserialize, Debug)]
pub struct TunnelConfig {
    pub name: String,
    pub addr: u16,
    pub proto: String,
    pub domain: String,
}

use std::fs;

pub fn load(path: String) -> Result<Config, Box<dyn std::error::Error>> {
    let file = fs::read_to_string(path).expect("Config file is required to run in client mode.");

    // use match to handle error is read_to_string returns an Option
    let config: Config = toml::from_str(&file).unwrap();

    println!("key: {}", config.api_key);
    // println!("tunnels: {:?}", config.tunnels);

    Ok(config)
}