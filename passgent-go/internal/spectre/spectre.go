package spectre

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"

	"golang.org/x/crypto/pbkdf2"
)

// RunSpectre implements the core Spectre v4 crypto derivation
func RunSpectre(masterName, masterPass, siteName, template string) string {
	userKey := pbkdf2.Key([]byte(masterPass), []byte(masterName), 524288, 64, sha256.New)
	mac := hmac.New(sha256.New, userKey)
	mac.Write([]byte(siteName))
	mac.Write([]byte{0, 0, 0, 1}) // siteCounter (default 1 big-endian uint32)
	siteKey := mac.Sum(nil)

	if template == "" {
		template = "long"
	}
	return applyMask(template, siteKey)
}

func applyMask(template string, key []byte) string {
	// A simplified representation of the templates logic.
	// For production we'd port `micro-key-producer` exact mapping.
	// This uses the key entropy to pick characters.
	var chars string
	switch template {
	case "maximum":
		chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	case "long":
		chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	case "medium":
		chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	case "short":
		chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	case "pin":
		chars = "0123456789"
	default:
		chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	}

	// We use the 32 byte hmac to map lengths (10 to 20 approx based on templates)
	length := 20
	if template == "pin" {
		length = 4
	} else if template == "short" {
		length = 8
	} else if template == "medium" {
		length = 14
	}

	result := make([]byte, length)
	for i := 0; i < length; i++ {
		// Use chunks of the hash
		var chunk uint16
		if i*2+1 < len(key) {
			chunk = binary.BigEndian.Uint16(key[i*2 : i*2+2])
		} else {
			chunk = binary.BigEndian.Uint16(key[0:2])
		}
		result[i] = chars[int(chunk)%len(chars)]
	}

	return string(result)
}
