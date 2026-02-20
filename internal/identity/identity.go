package identity

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"filippo.io/age"
	"golang.org/x/term"
)

func GetIdentityPath(name string, dir string) string {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path + ".age"); err == nil {
		return path + ".age"
	}
	return path
}

func GenerateIdentity(name string, dir string, passphrase string) (string, error) {
	os.MkdirAll(dir, 0700)
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, name)
	content := []byte(fmt.Sprintf("# created: %s\n# public key: %s\n%s\n",
		time.Now().UTC().Format(time.RFC3339),
		id.Recipient().String(),
		id.String()))

	if passphrase != "" {
		path += ".age"
		recipient, _ := age.NewScryptRecipient(passphrase)
		var buf bytes.Buffer
		w, _ := age.Encrypt(&buf, recipient)
		w.Write(content)
		w.Close()
		err = os.WriteFile(path, buf.Bytes(), 0600)
	} else {
		err = os.WriteFile(path, content, 0600)
	}

	if err != nil {
		return "", err
	}

	return id.Recipient().String(), nil
}

func LoadIdentity(path string) (age.Identity, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Try parsing as plain
	ids, err := age.ParseIdentities(bytes.NewReader(b))
	if err == nil && len(ids) > 0 {
		return ids[0], nil
	}

	// Try decrypting with passphrase
	fmt.Fprintf(os.Stderr, "Identity file %s is encrypted. Enter passphrase: ", filepath.Base(path))
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}

	identity, err := age.NewScryptIdentity(string(pass))
	if err != nil {
		return nil, err
	}

	r, err := age.Decrypt(bytes.NewReader(b), identity)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt identity: %v", err)
	}

	decrypted, _ := io.ReadAll(r)
	ids, err = age.ParseIdentities(bytes.NewReader(decrypted))
	if err != nil || len(ids) == 0 {
		return nil, fmt.Errorf("decrypted content is not a valid age identity")
	}

	return ids[0], nil
}

// GetIdentityContent returns the decrypted/plain content of the identity file
func GetIdentityContent(path string, passphrase string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Try parsing as plain
	ids, err := age.ParseIdentities(bytes.NewReader(b))
	if err == nil && len(ids) > 0 {
		return string(b), nil
	}

	// Try decrypting with passphrase
	pass := passphrase
	if pass == "" {
		fmt.Fprintf(os.Stderr, "Identity file %s is encrypted. Enter passphrase: ", filepath.Base(path))
		p, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		pass = string(p)
	}

	identity, err := age.NewScryptIdentity(pass)
	if err != nil {
		return "", err
	}

	r, err := age.Decrypt(bytes.NewReader(b), identity)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt identity: %v", err)
	}

	decrypted, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}
