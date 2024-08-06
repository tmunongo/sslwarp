use std::fs;

pub fn load(path: String) -> Result {
    let file = fs::read_to_string(path).expect("Config file is required to run in client mode.");

    // use match to handle error is read_to_string returns an Option

    // wrangle the yaml
}