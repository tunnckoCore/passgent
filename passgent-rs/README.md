# passgent-rs

`passgent-rs` is an experimental Rust port of `passgent-go`.

The long-term goal is to bring the same directory-oriented encrypted secret store, password generation, and Spectre-style deterministic password derivation workflow to Rust. Right now, this crate is still an early scaffold rather than a finished replacement for the Go version.

## Current status

This project is **in progress**.

What exists today:
- a Clap-based CLI
- early command wiring for `init`, `add`, `gen`, and `spectre`
- random secret generation helpers
- a basic Spectre-like derivation function
- config structs and config file helpers drafted in code

What is still incomplete or missing:
- actual encrypted secret storage via `age`
- loading and saving real secrets to a `.passgent` store
- store discovery / traversal like the Go version
- recipient and identity management
- feature parity with the Go commands such as `show`, `update`, `rm`, `ls`, `search`, `otp`, `config`, and store management
- the richer generator and workflow features already present in `passgent-go`

## Relationship to `passgent-go`

If you want the working implementation, use [`../passgent-go`](../passgent-go).

The Go version is the practical, feature-complete-ish implementation in this repo and is the best reference for how the Rust port should behave once finished.

## Implemented CLI surface

At the moment, the Rust binary exposes these commands:

```bash
cargo run -- init
cargo run -- add <name>
cargo run -- gen <name> [length]
cargo run -- spectre <site>
```

These commands currently prove out CLI structure more than end-user functionality.

## Development notes

The code already includes some building blocks for the eventual port:
- `clap` for CLI parsing
- `serde` + `toml` for config handling
- `rand` for generated values
- `pbkdf2`, `hmac`, and `sha2` for derivation logic
- `age` as the intended encryption backend

However, several of those pieces are not fully integrated yet.

## Recommendation

Treat `passgent-rs` as:
- a prototype
- a design sketch for the Rust version
- a place to continue the port from `passgent-go`

Do **not** treat it as a drop-in replacement for `passgent-go` yet.
