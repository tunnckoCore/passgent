package generator

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"github.com/tyler-smith/go-bip39"
	"github.com/tyler-smith/go-bip39/wordlists"
)

type GeneratorOptions struct {
	Length        int
	Upper         bool
	Lower         bool
	Numbers       bool
	Symbols       bool
	Words         int
	Separator     string
	Pattern       string
	Pronounceable bool
	Mnemonic      int
	Phrase        int
	Wordlist      string
	Charset       string
	UUID          string
}

const (
	upperChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerChars  = "abcdefghijklmnopqrstuvwxyz"
	numberChars = "0123456789"
	symbolChars = "!@#$%^&*()_+-=[]{}|;':\",./<>?"
	vowels      = "aeiouy"
	consonants  = "bcdfghjklmnpqrstvwxz"
)

func setWordlist(lang string) {
	switch strings.ToLower(lang) {
	case "japanese":
		bip39.SetWordList(wordlists.Japanese)
	case "spanish":
		bip39.SetWordList(wordlists.Spanish)
	case "french":
		bip39.SetWordList(wordlists.French)
	case "italian":
		bip39.SetWordList(wordlists.Italian)
	case "korean":
		bip39.SetWordList(wordlists.Korean)
	case "czech":
		bip39.SetWordList(wordlists.Czech)
	case "chinese_simplified":
		bip39.SetWordList(wordlists.ChineseSimplified)
	case "chinese_traditional":
		bip39.SetWordList(wordlists.ChineseTraditional)
	default:
		bip39.SetWordList(wordlists.English)
	}
}

func Generate(opts GeneratorOptions) (string, error) {
	if opts.UUID != "" {
		var id string
		switch opts.UUID {
		case "v4", "true", "4", "default":
			u, _ := uuid.NewRandom()
			id = u.String()
		case "v7", "7":
			u, _ := uuid.NewV7()
			id = u.String()
		default:
			return "", fmt.Errorf("unsupported UUID type, try 'v4' or 'v7'")
		}

		// If they completely overwrite the separator from '-', respect it.
		// e.g. --sep "" to drop dashes, or --sep "_" for underscores.
		if opts.Separator != "-" {
			id = strings.ReplaceAll(id, "-", opts.Separator)
		}

		return id, nil
	}

	if opts.Mnemonic > 0 {
		if opts.Mnemonic != 12 && opts.Mnemonic != 24 {
			return "", fmt.Errorf("mnemonic must be 12 or 24")
		}
		setWordlist(opts.Wordlist)
		entropySize := 128
		if opts.Mnemonic == 24 {
			entropySize = 256
		}
		entropy, err := bip39.NewEntropy(entropySize)
		if err != nil {
			return "", err
		}

		return bip39.NewMnemonic(entropy)
	}

	if opts.Pattern != "" {
		// Example mask format: ?u (upper), ?l (lower), ?d (digit), ?s (symbol), ?a (all), else literal
		var b strings.Builder
		maskLen := len(opts.Pattern)
		for i := 0; i < maskLen; i++ {
			if opts.Pattern[i] == '?' && i+1 < maskLen {
				next := opts.Pattern[i+1]
				var pool string
				switch next {
				case 'u':
					pool = upperChars
				case 'l':
					pool = lowerChars
				case 'd':
					pool = numberChars
				case 's':
					pool = symbolChars
				case 'a':
					pool = upperChars + lowerChars + numberChars + symbolChars
				case 'x':
					pool = upperChars + lowerChars + numberChars // All excluding symbols
				default:
					b.WriteByte(opts.Pattern[i])
					continue
				}
				n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(pool))))
				b.WriteByte(pool[n.Int64()])
				i++ // skip the modifier
			} else {
				b.WriteByte(opts.Pattern[i])
			}
		}
		return b.String(), nil
	}

	if opts.Phrase > 0 {
		setWordlist(opts.Wordlist)
		list := bip39.GetWordList()
		var words []string
		for i := 0; i < opts.Phrase; i++ {
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(list))))
			word := list[n.Int64()]

			if opts.Upper && !opts.Lower {
				word = strings.ToUpper(word)
			} else if !opts.Upper && opts.Lower {
				word = strings.ToLower(word)
			} else if opts.Upper && opts.Lower && len(word) > 0 {
				word = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
			}

			words = append(words, word)
		}
		sep := opts.Separator
		if sep == "" {
			sep = "-" // Default separator for phrases if not overridden
		}
		return strings.Join(words, sep), nil
	}

	if opts.Pronounceable {
		length := opts.Length
		if length == 0 {
			length = 12
		}

		wordsCount := opts.Words
		if wordsCount <= 0 {
			wordsCount = 1
		}

		var words []string
		for w := 0; w < wordsCount; w++ {
			var b strings.Builder
			for i := 0; i < length; i++ {
				var char byte
				if i%2 == 0 {
					n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(consonants))))
					char = consonants[n.Int64()]
				} else {
					n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(vowels))))
					char = vowels[n.Int64()]
				}

				if opts.Upper && !opts.Lower {
					char = strings.ToUpper(string(char))[0]
				} else if opts.Upper && opts.Lower {
					// Mix of upper and lower
					caseRand, _ := rand.Int(rand.Reader, big.NewInt(2))
					if caseRand.Int64() == 1 {
						char = strings.ToUpper(string(char))[0]
					}
				}

				// If numbers or symbols are enabled, randomly substitute them in pronounceable mode
				// to match pwgenrust behavior, making it more secure while maintaining pronounceability shape
				if opts.Numbers || opts.Symbols {
					modRand, _ := rand.Int(rand.Reader, big.NewInt(10))
					// ~20% chance to swap a character for a number/symbol if enabled
					if modRand.Int64() < 2 {
						avail := ""
						if opts.Numbers {
							avail += numberChars
						}
						if opts.Symbols {
							avail += symbolChars
						}
						if len(avail) > 0 {
							subRand, _ := rand.Int(rand.Reader, big.NewInt(int64(len(avail))))
							char = avail[subRand.Int64()]
						}
					}
				}

				b.WriteByte(char)
			}
			words = append(words, b.String())
		}

		sep := opts.Separator
		if sep == "" {
			sep = "-" // Default separator if generating multiple words
		}

		return strings.Join(words, sep), nil
	}

	// Basic char generation
	charset := opts.Charset
	if charset == "" {
		if opts.Upper {
			charset += upperChars
		}
		if opts.Lower {
			charset += lowerChars
		}
		if opts.Numbers {
			charset += numberChars
		}
		if opts.Symbols {
			charset += symbolChars
		}
		if charset == "" {
			charset = upperChars + lowerChars + numberChars + symbolChars // Default
		}
	}

	length := opts.Length
	if length == 0 {
		length = 64
	}

	wordsCount := opts.Words
	if wordsCount <= 0 {
		wordsCount = 1
	}

	var words []string
	for w := 0; w < wordsCount; w++ {
		var b strings.Builder
		for i := 0; i < length; i++ {
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			b.WriteByte(charset[n.Int64()])
		}
		words = append(words, b.String())
	}

	sep := opts.Separator
	if sep == "" {
		sep = "-"
	}

	return strings.Join(words, sep), nil
}
