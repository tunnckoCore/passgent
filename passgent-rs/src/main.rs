use age::x25519::Identity;
use clap::{Parser, Subcommand};
use hmac::{Hmac, Mac};
use pbkdf2::pbkdf2;
use rand::distr::Alphanumeric;
use rand::RngExt;
use rpassword::read_password;
use serde::{Deserialize, Serialize};
use sha2::Sha256;
use std::fs::{self};
use std::io::Write;
use std::path::PathBuf;

#[derive(Serialize, Deserialize, Default)]
struct Config {
    store_dir: String,
    recipients: Vec<String>,
}

#[derive(Parser)]
#[command(name = "passgent-rs")]
#[command(about = "Minimalist secrets manager in Rust", long_about = None)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Initialize storage in current directory
    Init,
    /// Add a new secret
    Add {
        /// Prompt for password interactively
        #[arg(short, long)]
        interactive: bool,
        /// Recipient public key
        #[arg(short, long)]
        recipient: Option<String>,
        /// Name of the secret
        name: String,
    },
    /// Generate a new secret
    Gen {
        /// Generate a passphrase instead
        #[arg(short, long)]
        passphrase: bool,
        /// Name of the secret
        name: String,
        /// Length of the password
        #[arg(default_value_t = 64)]
        length: usize,
    },
    /// Manage Spectre v4 master password
    Spectre {
        /// Master name
        #[arg(long)]
        name: Option<String>,
        /// Master password
        #[arg(long)]
        pass: Option<String>,
        /// Site to generate for
        site: Option<String>,
    },
}

// Minimal stub for Config directory access to skip external dependencies
fn config_dir() -> PathBuf {
    let mut path = dirs_next::config_dir().unwrap_or_else(|| PathBuf::from("."));
    path.push("passgent");
    path
}

fn load_config() -> Config {
    let mut path = config_dir();
    path.push("config.toml");

    if let Ok(content) = fs::read_to_string(&path) {
        toml::from_str(&content).unwrap_or_default()
    } else {
        Config::default()
    }
}

fn save_config(config: &Config) {
    let mut path = config_dir();
    fs::create_dir_all(&path).unwrap();
    path.push("config.toml");
    let content = toml::to_string(config).unwrap();
    fs::write(path, content).unwrap();
}

fn generate_password(len: usize) -> String {
    let s: String = rand::rng()
        .sample_iter(&Alphanumeric)
        .take(len)
        .map(char::from)
        .collect();
    s
}

fn run_spectre(master_name: &str, master_pass: &str, site: &str) -> String {
    let mut user_key = [0u8; 64];
    pbkdf2::<Hmac<Sha256>>(
        master_pass.as_bytes(),
        master_name.as_bytes(),
        524288,
        &mut user_key,
    );

    let mut mac = Hmac::<Sha256>::new_from_slice(&user_key).unwrap();
    mac.update(site.as_bytes());
    let result = mac.finalize().into_bytes();

    // Hex encoded truncation mapping approx template length (for scaffold context)
    let hex = hex::encode(result);
    hex[..20].to_string()
}

fn main() {
    let cli = Cli::parse();
    // In a full environment we would fetch `dirs_next` crate, mocking config flow for structure
    println!("Parsed CLI correctly. Passgent is scaffolding the Rust binaries.");

    match &cli.command {
        Commands::Init => {
            println!("Initialized.");
        }
        Commands::Add {
            interactive,
            recipient: _,
            name,
        } => {
            let secret = if *interactive {
                print!("Enter secret for {}: ", name);
                std::io::stdout().flush().unwrap();
                read_password().unwrap()
            } else {
                generate_password(64)
            };
            println!("Added {}", name);
        }
        Commands::Gen {
            passphrase: _,
            name,
            length,
        } => {
            let secret = generate_password(*length);
            println!("Generated {} length: {}", name, secret.len());
        }
        Commands::Spectre { name, pass, site } => {
            if let Some(n) = name {
                let p = pass.clone().unwrap_or_else(|| generate_password(32));
                println!("Saved master: name: {}, pass: {}", n, p);
            } else if let Some(s) = site {
                let generated = run_spectre("dummyName", "dummyPass", s);
                println!("Site password: {}", generated);
            }
        }
    }
}
