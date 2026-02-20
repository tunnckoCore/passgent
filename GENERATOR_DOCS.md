# Generator Guide (`passgent gen`)

The `passgent gen` command is a highly versatile, stateless password/secrets/apikey generator. It is heavily inspired by `pwgenrust` and supports everything from simple character passwords to standard BIP39 mnemonic seeds and pronounceable passphrases.

It is perfect for piping directly into other commands or scripts, like `passgent add` and `passgent update`

---

## 1. Standard Character Generation

By default, running `passgent gen` creates a 64-character password using uppercase, lowercase, numbers, and symbols.

**Adjusting Length and Count**
*   `--length 32`: Generate a 32-character password.
*   `--count 5`: Generate 5 passwords at once (separated by newlines).

**Character Sets (Negation)**
By default, all character types are enabled. You can toggle them off using negation flags:
*   `--no-symbols`: Exclude special characters.
*   `--no-numbers`: Exclude digits.
*   `--no-upper`: Exclude uppercase letters.
*   `--no-lower`: Exclude lowercase letters.
*   `--basic`: Shorthand for `--no-upper --no-numbers --no-symbols` (generates strictly lowercase letters).

*Example: 12-character strictly lowercase string:*
```bash
passgent gen --length 12 --basic
```

**Custom Charset Override**
If you need an extremely specific pool of characters (e.g., avoiding confusing characters like `l`, `I`, `0`, `O`), use `--charset`:
```bash
passgent gen --length 12 --charset "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
```

---

## 2. Pronounceable Passwords

Pronounceable passwords alternate between consonants and vowels, making them easier to read over the phone or memorize.

*   `--pronounceable`: Enables the mode.
*   `--words <num>`: Generates multiple pronounceable chunks.
*   `--sep <char>`: The character used to join the words (default: `-`).

*Example: 4 words, 5 characters each, separated by a space:*
```bash
passgent gen --pronounceable --length 5 --words 4 --sep " " --no-symbols
# DYS1p 7Omez S5QyF Jazox

passgent gen --pronounceable --length 5 --words 4 --sep " " --no-symbols --no-numbers
# xACyF hymot VOxah PaXaN

passgent gen --pronounceable --length 4 --basic --words 5
# kuhy-nihu-pyve-nave-jiky
```

**Complexity in Pronounceable Mode:**
If `--upper`, `--lower`, `--numbers`, or `--symbols` are enabled, the generator will:
1.  Mix the casing of the letters.
2.  Have a ~20% chance to substitute a consonant or vowel with a number or symbol, maintaining the pronounceable shape while satisfying strict password complexity requirements.
*(To get pure lowercase pronounceable words, pass `--no-upper --no-numbers --no-symbols` or just `--basic` which does exactly that).*

---

## 3. Mask Patterns

If you need a strictly formatted string (like an API key or legacy system password), use `--pattern`. 

Tokens start with `?`:
*   `?u` : Random **U**ppercase letter (`A-Z`)
*   `?l` : Random **l**owercase letter (`a-z`)
*   `?d` : Random **d**igit (`0-9`)
*   `?s` : Random **s**ymbol (`!@#$%^&*...`)
*   `?x` : Random alphanumeric character (`A-Z`, `a-z`, `0-9`) - excludes symbols
*   `?a` : Random character from **a**ll pools combined

Any character without a `?` is treated as a literal. *(Note: `--pattern` ignores the `--length` flag).*

*Example: Stripe-style API key:*
```bash
passgent gen --pattern "sk_live_?a?a?a?a?a?a?a?a?a?a?a?a"
# sk_live_YPax}f_LGzAA

passgent gen --pattern "sk_live_?x?x?x?x?x?x?x?x?x?x?x?x?x?x"
# sk_live_PnWqU6SaErLxh1
```

*Example: Strict format (2 Upper, 2 Lower, 4 Digits):*
```bash
passgent gen --pattern "?u?u?l?l?d?d?d?d"
# QMou7803
```

---

## 4. BIP39 Seed Phrases & Dictionary Words

`passgent` utilizes the official BIP39 wordlist for high-entropy dictionary generation.

### Standard Wallet Seeds (`--mnemonic`)
Use this to generate strictly valid, checksum-compliant cryptocurrency recovery seeds.
*   **Must be exactly 12 or 24 words.**
*   **Bans all complexity flags** (`--upper`, `--pattern`, etc.) to ensure cryptographic validity.
```bash
passgent gen --mnemonic 24
```

### Generic Dictionary Passphrases (`--phrase`)
Use the BIP39 dictionary to create standard "diceware" style passwords.
*   `--phrase <num>`: Specify the number of words.
*   `--sep <char>`: Separator (default: `-`).
*   **Casing rules apply!** By default, phrases are all-lowercase.
    *   Add `--upper` for ALL CAPS.
    *   Add `--upper --lower` for TitleCase.

*Example: 5-word TitleCase passphrase separated by spaces:*
```bash
passgent gen --phrase 5 --upper --lower --sep " "
```

### Foreign Language Wordlists
Both `--mnemonic` and `--phrase` support foreign dictionaries via `--wordlist <lang>`.
*(Supported: english, japanese, spanish, french, italian, korean, czech, chinese_simplified, chinese_traditional).*

```bash
passgent gen --phrase 4 --wordlist japanese
```

---

## 5. UUIDs

Generate standard unique identifiers natively.

*   `--uuid <v4|v7>`: Specify the version.
    *   `v4`: Fully random identifier.
    *   `v7`: Time-ordered identifier (lexicographically sortable).
*   `--sep <char>`: Custom separator (default: `-`). Pass `""` to remove dashes entirely.

**Constraints:** 
To ensure validity, all other formatting flags (length, casing, etc.) are **banned** when using `--uuid`, with the sole exception of `--sep`.

*Example: UUID v7 without dashes:*
```bash
passgent gen --uuid v7 --sep ""
```

---

## 6. Profiles

Profiles allow you to save your favorite generator configurations into `~/.config/passgent/config.toml` so you don't have to type long flag combinations every time.

### Creating a Profile
Pass your desired flags along with `--create-profile <name>`. The CLI will generate a password and save the exact flag state to your config.

```bash
passgent gen --pronounceable --length 6 --words 4 --basic --sep "_" --create-profile "easy-read"

# Profile saved: easy-read
# fareba_hoxuta_pocysu_kimecy
```

### Using a Profile
Call it later using `-p` or `--profile`:
```bash
passgent gen -p easy-read
```

### Overriding a Profile
You can load a profile but selectively override specific flags on the fly!
```bash
# Uses all rules from 'easy-read', but changes the separator to a space
passgent gen -p easy-read --sep " "
```

### Updating a profile

To update a profile, just use it with `--profile`, override some flags, and use the `--create-profile` again. Here we are updating the `easy-read` profile.

```bash
passgent gen -p easy-read --length 4 --sep " " --create-profile "easy-read"
# Profile saved: easy-read
# fozi tody sube reli
```

---

## 7. Piping Workflows

Because `gen` strictly writes the password to `stdout`, it pairs perfectly with `passgent add` or `passgent update`.

*Generate a strict 64-char alphanumeric password and pipe it directly into a new vault entry:*
```bash
passgent gen -p dev | passgent add github/token
```

*Update an existing secret with a new 5-word phrase:*
```bash
passgent gen --phrase 5 | passgent update personal/wifi
```
