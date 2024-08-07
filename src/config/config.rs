use serde::Deserialize;

#[derive(Deserialize, Debug)]
struct Config {
    api_key: String,
    tunnels: Vec<TunnelConfig>
}

#[derive(Deserialize, Debug)]
struct TunnelConfig {
    name: String,
    addr: int,
    proto: String,
    domain: String,
}

// pub mod config {
    use std::fs;

    pub fn load(path: String) -> Result<T, E> {
        let file = fs::read_to_string(path).expect("Config file is required to run in client mode.");

        // use match to handle error is read_to_string returns an Option
        let config: Config = toml::from_str(&file)?;

        println!("config: {}", config);

        Ok(config)

        // wrangle the yaml
    }
// }
