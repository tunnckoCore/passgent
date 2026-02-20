package crypto

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
)

func Encrypt(data []byte, owners []string, outPath string) error {
	var recs []age.Recipient
	for _, r := range owners {
		r = strings.TrimSpace(r)
		if r == "" || strings.HasPrefix(r, "#") {
			continue
		}

		if strings.HasPrefix(r, "github:") {
			user := strings.TrimPrefix(r, "github:")
			resp, err := http.Get(fmt.Sprintf("https://github.com/%s.keys", user))
			if err == nil {
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				parsed, err := age.ParseRecipients(strings.NewReader(string(b)))
				if err == nil {
					recs = append(recs, parsed...)
				}
			}
			continue
		}

		parsed, err := age.ParseRecipients(strings.NewReader(r))
		if err == nil {
			recs = append(recs, parsed...)
		}
	}

	if len(recs) == 0 {
		return fmt.Errorf("no owners provided")
	}

	os.MkdirAll(filepath.Dir(outPath), 0700)
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w, err := age.Encrypt(f, recs...)
	if err != nil {
		return err
	}

	if _, err := w.Write(data); err != nil {
		return err
	}
	return w.Close()
}

func Decrypt(path string, identity age.Identity) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	r, err := age.Decrypt(bytes.NewReader(b), identity)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
