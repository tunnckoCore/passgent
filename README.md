# passgent-go

`passgent` is a minimalist, secure, and highly versatile secrets manager and password generator written in Go. It leverages [age](https://github.com/FiloSottile/age) for modern encryption and follows a "vault-per-directory" philosophy inspired by the logic of Git and Pass.

## Key Features

- **Age-based Cryptography**: Built on `filippo.io/age`. It supports standard X25519 identities and is designed with Post-Quantum (PQ) readiness in mind.
- **Dynamic Store Resolution**: Unlike traditional managers with one global vault, `passgent` traverses upwards from your current directory to find a `.passgent` folder (similar to how `.git` works). It falls back to a global store if none is found.
- **State-of-the-Art Generator**: A powerhouse `gen` command that handles everything from high-entropy character strings and BIP39 mnemonics to pronounceable passphrases and strict pattern masks.
- **Native Spectre (MasterPassword) Support**: A pure Go implementation of the Spectre v4 algorithm for deterministic, site-specific password derivation without storing them.
- **Git Integration**: Optional automatic Git initialization and commits for every change in your secret store, making synchronization across devices trivial.
- **Clipboard Safety**: Integrated clipboard support with automatic clearing.
- **UNIX Philosophy**: Designed to be piped. Generate a secret and pipe it directly into `add` or `update`.
- **2FA Secrets:** Supports HOTP/TOTP through `oauthtool`

## Installation

```bash
# Clone the repository
git clone https://github.com/your-username/agent-secrets-manager.git
cd agent-secrets-manager/passgent-go

# Build the binary
go build -o passgent main.go

# Install to your path
mv passgent /usr/local/bin/
```

## Core Concepts

### Store Resolution
`passgent` determines the active store using the following algorithm:
1. Search `$PWD` for a `.passgent` directory.
2. Traverse up the directory tree until `.passgent` is found.
3. If not found, use the `store_dir` defined in `~/.config/passgent/config.toml` (defaults to `~/.config/passgent/store`).
4. You can disable the traversal with `disable_auto_traverse = true`, then you can use the `--store` global flag

### Configuration
Global settings, stores, and generator presets are stored in `~/.config/passgent/config.toml`. You can define reusable presets for different use cases (e.g., "work-policy", "memorable", "api-key").

## Command Overview

### Vault Management
- `passgent setup`: Performs initial global configuration and initial global store
- `passgent id [name]`: Generates a named identity, defaults to `main`
- `passgent store new [dir]`: Initializes a new store at dir with a `.passgent` directory and optional Git repo. Optionally you can define which named identity would be the controlled of the store.
- `passgent add <name> [secret]`: Adds a secret. Supports interactive prompts (`-i`), specific recipients (`-r`), and generator presets (`-p`).
- `passgent show <name>`: Decrypts and prints a secret or copies it to the clipboard (`-c`).
- `passgent update <name>`: Updates an existing entry. Uses a secure editor fallback chain (nano -> vim -> vi -> $EDITOR) or interactive input.
- `passgent rm <name>`: Securely removes an entry.
- `passgent ls` / `passgent search`: Browse and find secrets across all stores or with `--store apikeys`  for specific store
- `passgent search [term] -c`: Fuzzy Search with `fzf` and copy to clipboard when selected. 

### Generation & Derivation
- `passgent gen`: The stateless generator. 
    - `--mnemonic 24`: Generate BIP39 seeds.
    - `--phrase 5`: Generate diceware-style phrases.
    - `--pattern "sk_?a?a?a"`: Generate based on a mask.
    - `--pronounceable`: Generate easier-to-read passwords.
    - `--numbers, --no-numbers`: Include/exclude numbers
    - `--symbols, --no-symbols`: Include/exclude symbols
    - `--upper`: Force to uppercase alphabet (sometimes you may need bot `--upper --no-lower`)
    - `--lower`: Force to lowercase alphabets
    - `--charset`: A charset to use for generation.
    - `--create-profile <name>`: Save current flags as a reusable profile.
- `passgent spectre --name "User" --pass "Master"` Use to initialize your master credentials.
- `passgent spectre <site>`: Derives a password using the Spectre algorithm based on you identity private key.
- `passgent otp <name>`: Generates a TOTP code if the secret `name` contains an `otpauth://` URI or a `totp:` field.

## Usage Examples

**Initialize a new project-specific vault:**
```bash
cd my-project
passgent store new .
```

**Generate a 32-character alphanumeric password and save it:**
```bash
passgent gen --length 32 --no-symbols | passgent add production/db-password
```

**Use a saved profile to update a secret:**
```bash
passgent update production/db-password -p high-entropy
```

**Copy (or show) a TOTP code to clipboard:**
```bash
passgent otp personal/github -c

# or output directly
passgent otp personal/github
# 123456
```

**Generate 10 random UUID v7s:**
```bash
passgent gen --uuid v7 --count 10
```

## Technical Details

- **Language**: Go 1.25+
- **Encryption**: age (X25519)
- **Config Format**: TOML
- **CLI Framework**: Cobra
- **Dependencies**: 
    - `filippo.io/age` for crypto.
    - `github.com/atotto/clipboard` for cross-platform clipboard.
    - `github.com/tyler-smith/go-bip39` for mnemonic support.

## Security Considerations
`passgent` stores your private identity in `~/.config/passgent/identity.txt` by default. Ensure this file is backed up securely and has strict permissions (`600`). When using the editor for `update`, `passgent` attempts to use memory-backed temporary files where possible to avoid leaking decrypted secrets to disk.

## License
[MIT/Apache-2.0]
