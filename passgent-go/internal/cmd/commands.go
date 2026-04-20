package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/atotto/clipboard"
	"github.com/pelletier/go-toml/v2"
	"github.com/pquerna/otp/totp"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"passgent-go/internal/config"
	"passgent-go/internal/crypto"
	"passgent-go/internal/generator"
	"passgent-go/internal/identity"
	"passgent-go/internal/spectre"
)

var (
	noGit        bool
	yesGit       bool
	interactive  bool
	ownerFile    string
	owners       []string
	presetName   string
	copyOut      bool
	showSecret   bool
	storeName    string
	idGet        bool
	idRm         bool
	storeId      string
	storeGlobal  bool
	storeDir     string
	createPreset string
	configJson   bool
	purgeStore   bool
)

func getIdentityPath() string {
	for _, s := range GlobalConfig.Stores {
		if config.ExpandHome(s.Location) == StoreDir && s.Identity != "" {
			return identity.GetIdentityPath(s.Identity, config.GetIdentitiesDir())
		}
	}
	return identity.GetIdentityPath(GlobalConfig.DefaultIdentity, config.GetIdentitiesDir())
}

func loadIdentity() (age.Identity, error) {
	return identity.LoadIdentity(getIdentityPath())
}

func getOwners() ([]string, error) {
	id, err := loadIdentity()
	if err != nil {
		return nil, err
	}

	var recs []string
	if xid, ok := id.(*age.X25519Identity); ok {
		recs = append(recs, xid.Recipient().String())
	}

	recs = append(recs, owners...)

	if ownerFile != "" {
		b, err := os.ReadFile(ownerFile)
		if err == nil {
			lines := strings.Split(string(b), "\n")
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if l != "" && !strings.HasPrefix(l, "#") {
					recs = append(recs, l)
				}
			}
		}
	}

	return recs, nil
}

func getGeneratorOptions(prof *config.Preset) generator.GeneratorOptions {
	if prof == nil {
		return generator.GeneratorOptions{}
	}
	u, l, n, s := true, true, true, true
	if prof.Upper != nil {
		u = *prof.Upper
	}
	if prof.Lower != nil {
		l = *prof.Lower
	}
	if prof.Numbers != nil {
		n = *prof.Numbers
	}
	if prof.Symbols != nil {
		s = *prof.Symbols
	}
	return generator.GeneratorOptions{
		Length: prof.Length, Upper: u, Lower: l, Numbers: n, Symbols: s, Words: prof.Words,
		Separator: prof.Separator, Pattern: prof.Pattern, Pronounceable: prof.Pronounceable, Mnemonic: prof.Mnemonic,
		Phrase: prof.Phrase, Wordlist: prof.Wordlist, Charset: prof.Charset, UUID: prof.UUID,
	}
}

func readInput(interactive bool, prompt string) (string, error) {
	if interactive {
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
		pass, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		return string(pass), err
	}
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		b, err := io.ReadAll(os.Stdin)
		return strings.TrimSuffix(string(b), "\n"), err
	}
	return "", nil
}

func copyAndClear(data string) {
	clipboard.WriteAll(data)
	timeout := GlobalConfig.ClipboardTimeout
	if timeout <= 0 {
		timeout = 45
	}
	go func() {
		time.Sleep(time.Duration(timeout) * time.Second)
		clipboard.WriteAll("")
	}()
}

func openEditor(content string) (string, error) {
	editor := os.Getenv("EDITOR")
	if !GlobalConfig.UseEditor || editor == "" {
		for _, e := range []string{"nano", "vim", "vi"} {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		editor = "vi" // last resort
	}

	tmp, err := os.CreateTemp("", "passgent-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(content); err != nil {
		return "", err
	}
	tmp.Close()

	cmd := exec.Command("sh", "-c", editor+" "+tmp.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	b, err := os.ReadFile(tmp.Name())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func getGitCommit(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func syncStoreMetadata(storeDir string) {
	if GlobalConfig == nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	gitRoot := filepath.Dir(storeDir)
	gitDir := filepath.Join(gitRoot, ".git")
	_, gitDirExists := os.Stat(gitDir)

	for name, s := range GlobalConfig.Stores {
		if config.ExpandHome(s.Location) == storeDir {
			s.UpdatedAt = now

			if gitDirExists == nil {
				if !s.UseGit {
					s.UseGit = true
					exec.Command("git", "-C", gitRoot, "init").Run()
					files, _ := os.ReadDir(storeDir)
					for _, f := range files {
						if !f.IsDir() && strings.HasSuffix(f.Name(), ".age") {
							exec.Command("git", "-C", gitRoot, "add", filepath.Join("store", f.Name())).Run()
						}
					}
					exec.Command("git", "-C", gitRoot, "commit", "-m", "Initial sync").Run()
				}
				s.Commit = getGitCommit(gitRoot)
			} else {
				if s.UseGit {
					s.UseGit = false
					s.Commit = ""
				} else {
					s.Commit = ""
				}
			}
			GlobalConfig.Stores[name] = s
			config.SaveConfig(GlobalConfig)
			break
		}
	}
}

func runSetup() error {
	cfgPath := config.GetConfigPath()
	if _, err := os.Stat(cfgPath); err == nil {
		return fmt.Errorf("configuration already exists at %s", config.CollapseHome(cfgPath))
	}

	t := true
	f := false
	cfg := &config.Config{
		DefaultIdentity:  "main",
		UseEditor:        false,
		ClipboardTimeout: 45,
		Presets: map[string]*config.Preset{
			"default": {
				Length: 64, Upper: &t, Lower: &t, Numbers: &t, Symbols: &t,
			},
			"easy-read": {
				Phrase: 4, Separator: "_", Pronounceable: true, Upper: &f, Lower: &t, Numbers: &f, Symbols: &f,
			},
		},
		Stores: make(map[string]config.StoreConfig),
	}

	globalStorePath := config.GetDefaultGlobalStoreDir()
	os.MkdirAll(globalStorePath, 0700)

	now := time.Now().UTC().Format(time.RFC3339)
	cfg.Stores["global"] = config.StoreConfig{
		Location:  globalStorePath,
		Identity:  "main",
		UseGit:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := config.SaveConfig(cfg)
	if err != nil {
		return err
	}
	GlobalConfig = cfg

	fmt.Printf("Initialized passgent in %s\n", config.CollapseHome(config.GetConfigDir()))
	fmt.Printf("Global store initialized at %s\n", config.CollapseHome(globalStorePath))
	fmt.Println("Next: run 'passgent id' to create your 'main' identity.")
	return nil
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Initialize passgent configuration and global store",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetup()
	},
}

var storeNewCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Create a new store",
	RunE: func(cmd *cobra.Command, args []string) error {
		if GlobalConfig == nil {
			if storeGlobal {
				return runSetup()
			}
			return fmt.Errorf("configuration not found. run 'passgent setup' first")
		}

		name := ""
		in, _ := readInput(false, "")
		if in != "" {
			name = strings.TrimSpace(in)
		} else if len(args) > 0 {
			name = args[0]
		}

		dir := storeDir
		if storeGlobal {
			if name != "" && name != "global" {
				return fmt.Errorf("cannot provide a custom name when using --global")
			}
			name = "global"
			if dir == "" {
				dir = config.GetDefaultGlobalStoreDir()
			}
		}

		if name == "" {
			name, _ = generator.Generate(generator.GeneratorOptions{Phrase: 4, Separator: "-"})
		}

		if _, ok := GlobalConfig.Stores[name]; ok && !storeGlobal {
			return fmt.Errorf("store %q already exists in config", name)
		}

		if dir == "" {
			dir = "."
		}

		storePath := dir
		if !storeGlobal && !strings.Contains(dir, ".passgent/store") {
			storePath = filepath.Join(dir, ".passgent", "store")
		}
		absPath, err := filepath.Abs(config.ExpandHome(storePath))
		if err == nil {
			storePath = absPath
		}

		os.MkdirAll(storePath, 0700)

		idName := storeId
		if idName == "" {
			idName = GlobalConfig.DefaultIdentity
		}

		idSource := identity.GetIdentityPath(idName, config.GetIdentitiesDir())
		if _, err := os.Stat(idSource); os.IsNotExist(err) {
			return fmt.Errorf("identity %q not found, create it first with 'passgent id %s'", idName, idName)
		}

		gitEnabled := false
		if cmd.Flags().Changed("git") {
			gitEnabled = true
		} else if cmd.Flags().Changed("no-git") {
			gitEnabled = false
		}

		gitRoot := filepath.Dir(storePath)
		if gitEnabled {
			exec.Command("git", "-C", gitRoot, "init").Run()
		}

		now := time.Now().UTC().Format(time.RFC3339)
		GlobalConfig.Stores[name] = config.StoreConfig{
			Location:  storePath,
			Identity:  idName,
			UseGit:    gitEnabled,
			CreatedAt: now,
			UpdatedAt: now,
			Commit:    getGitCommit(gitRoot),
		}
		config.SaveConfig(GlobalConfig)

		fmt.Printf("Initialized store %q in %s (using identity %q, git: %v)\n", name, config.CollapseHome(storePath), idName, gitEnabled)
		return nil
	},
}

var idCmd = &cobra.Command{
	Use:   "id [name] [passphrase]",
	Short: "Create or show identity",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := GlobalConfig.DefaultIdentity
		if len(args) > 0 {
			name = args[0]
		}
		dir := config.GetIdentitiesDir()
		path := identity.GetIdentityPath(name, dir)

		if idGet {
			pass := ""
			if len(args) > 1 {
				pass = args[1]
			}
			content, err := identity.GetIdentityContent(path, pass)
			if err != nil {
				return err
			}
			fmt.Print(content)
			return nil
		}

		if idRm {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return fmt.Errorf("identity %q does not exist", name)
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			fmt.Printf("Removed identity %q\n", name)
			return nil
		}

		if _, err := os.Stat(path); err == nil {
			fmt.Printf("Identity %q exists at %s\n", name, config.CollapseHome(path))
			return nil
		}
		var pass string
		if len(args) > 1 {
			pass = args[1]
		} else if interactive {
			fmt.Fprintf(os.Stderr, "Enter passphrase: ")
			p, _ := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			pass = string(p)
		}
		recip, err := identity.GenerateIdentity(name, dir, pass)
		if err != nil {
			return err
		}
		fmt.Printf("Created identity %q\nPublic key: %s\n", name, recip)
		return nil
	},
}

var storeCmd = &cobra.Command{
	Use:   "store",
	Short: "Manage stores",
}

var storeLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all stores",
	Run: func(cmd *cobra.Command, args []string) {
		var names []string
		for name := range GlobalConfig.Stores {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			s := GlobalConfig.Stores[name]
			fmt.Printf("%s: %s (id: %s, git: %v, updated: %s)\n", name, config.CollapseHome(s.Location), s.Identity, s.UseGit, s.UpdatedAt)
		}
	},
}

var storeRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a store from config",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		s, ok := GlobalConfig.Stores[name]
		if !ok {
			return fmt.Errorf("store %q not found", name)
		}

		if purgeStore {
			path := config.ExpandHome(s.Location)
			purgePath := path
			if strings.HasSuffix(path, ".passgent/store") {
				purgePath = filepath.Dir(path)
			}
			if err := os.RemoveAll(purgePath); err != nil {
				return fmt.Errorf("failed to purge store files at %s: %v", purgePath, err)
			}
			fmt.Printf("Purged store files at %s\n", config.CollapseHome(purgePath))
		}

		delete(GlobalConfig.Stores, name)
		if err := config.SaveConfig(GlobalConfig); err != nil {
			return err
		}

		fmt.Printf("Removed store %q from configuration\n", name)
		return nil
	},
}

var addCmd = &cobra.Command{
	Use:   "add <name> [secret]",
	Short: "Add a new secret",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		outPath := filepath.Join(StoreDir, name+".age")
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("secret %q already exists, use 'update' to change it", name)
		}

		var secret string
		if len(args) > 1 {
			secret = args[1]
		} else {
			in, _ := readInput(interactive, "Secret")
			if in != "" {
				secret = in
			} else {
				prof := GlobalConfig.Presets[presetName]
				if prof == nil {
					prof = GlobalConfig.Presets["default"]
				}
				opts := getGeneratorOptions(prof)
				secret, _ = generator.Generate(opts)
			}
		}

		if copyOut {
			copyAndClear(secret)
		}

		recs, err := getOwners()
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(outPath), 0700)
		err = crypto.Encrypt([]byte(secret), recs, outPath)
		if err != nil {
			return err
		}

		syncStoreMetadata(StoreDir)
		gitDir := filepath.Join(filepath.Dir(StoreDir), ".git")
		if _, err := os.Stat(gitDir); err == nil {
			gitRoot := filepath.Dir(StoreDir)
			relPath, _ := filepath.Rel(gitRoot, outPath)
			exec.Command("git", "-C", gitRoot, "add", relPath).Run()
			exec.Command("git", "-C", gitRoot, "commit", "-m", "Add "+name).Run()
			syncStoreMetadata(StoreDir)
		}

		currentStoreName := "unknown"
		for n, s := range GlobalConfig.Stores {
			if config.ExpandHome(s.Location) == StoreDir {
				currentStoreName = n
				break
			}
		}

		if showSecret {
			fmt.Println(secret)
		} else {
			fmt.Printf("Saved %q to store %q (%s)\n", name, currentStoreName, config.CollapseHome(outPath))
		}
		return nil
	},
}

var genLength, genWords, genCount, genMnemonic, genPhrase int
var genUpper, genLower, genNumbers, genSymbols, genPronounceable bool
var genNoUpper, genNoLower, genNoNumbers, genNoSymbols, genBasic bool
var genSeparator, genSep, genPattern, genWordlist, genCharset, genUUID string

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate password(s)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("sep") {
			genSeparator = genSep
		}
		prof := GlobalConfig.Presets[presetName]
		if prof == nil {
			prof = GlobalConfig.Presets["default"]
		}
		if cmd.Flags().Changed("preset") && prof != nil {
			if !cmd.Flags().Changed("length") && prof.Length != 0 {
				genLength = prof.Length
			}
			if !cmd.Flags().Changed("separator") && !cmd.Flags().Changed("sep") && prof.Separator != "" {
				genSeparator = prof.Separator
			}
			if !cmd.Flags().Changed("words") && prof.Words != 0 {
				genWords = prof.Words
			}
			if !cmd.Flags().Changed("upper") && prof.Upper != nil {
				genUpper = *prof.Upper
			}
			if !cmd.Flags().Changed("lower") && prof.Lower != nil {
				genLower = *prof.Lower
			}
			if !cmd.Flags().Changed("numbers") && prof.Numbers != nil {
				genNumbers = *prof.Numbers
			}
			if !cmd.Flags().Changed("symbols") && prof.Symbols != nil {
				genSymbols = *prof.Symbols
			}
			if !cmd.Flags().Changed("pronounceable") && prof.Pronounceable {
				genPronounceable = prof.Pronounceable
			}
			if !cmd.Flags().Changed("pattern") && prof.Pattern != "" {
				genPattern = prof.Pattern
			}
			if !cmd.Flags().Changed("mnemonic") && prof.Mnemonic != 0 {
				genMnemonic = prof.Mnemonic
			}
			if !cmd.Flags().Changed("phrase") && prof.Phrase != 0 {
				genPhrase = prof.Phrase
			}
			if !cmd.Flags().Changed("wordlist") && prof.Wordlist != "" {
				genWordlist = prof.Wordlist
			}
			if !cmd.Flags().Changed("charset") && prof.Charset != "" {
				genCharset = prof.Charset
			}
			if !cmd.Flags().Changed("uuid") && prof.UUID != "" {
				genUUID = prof.UUID
			}
		}
		if genNoUpper || genBasic {
			genUpper = false
		}
		if genNoLower {
			genLower = false
		}
		if genNoNumbers || genBasic {
			genNumbers = false
		}
		if genNoSymbols || genBasic {
			genSymbols = false
		}
		opts := generator.GeneratorOptions{
			Length: genLength, Upper: genUpper, Lower: genLower, Numbers: genNumbers, Symbols: genSymbols, Words: genWords,
			Separator: genSeparator, Pattern: genPattern, Pronounceable: genPronounceable, Mnemonic: genMnemonic,
			Phrase: genPhrase, Wordlist: genWordlist, Charset: genCharset, UUID: genUUID,
		}
		if createPreset != "" {
			u, l, n, s := genUpper, genLower, genNumbers, genSymbols
			GlobalConfig.Presets[createPreset] = &config.Preset{
				Length: genLength, Words: genWords, Separator: genSeparator, Pattern: genPattern, Pronounceable: genPronounceable,
				Mnemonic: genMnemonic, Phrase: genPhrase, Wordlist: genWordlist, Charset: genCharset, Upper: &u, Lower: &l, Numbers: &n, Symbols: &s, UUID: genUUID,
			}
			config.SaveConfig(GlobalConfig)
		}
		c := genCount
		if c <= 0 {
			c = 1
		}
		for i := 0; i < c; i++ {
			s, _ := generator.Generate(opts)
			fmt.Println(s)
		}
		return nil
	},
}

var spectreName, spectrePass string
var spectreCmd = &cobra.Command{
	Use:   "spectre [site]",
	Short: "Spectre V4 derivation",
	RunE: func(cmd *cobra.Command, args []string) error {
		if spectreName != "" {
			outPath := filepath.Join(StoreDir, "spectre.age")
			recs, _ := getOwners()
			return crypto.Encrypt([]byte(fmt.Sprintf("name: %s\npass: %s\n", spectreName, spectrePass)), recs, outPath)
		}
		if len(args) == 0 {
			return fmt.Errorf("site name required")
		}
		site := args[0]
		id, _ := loadIdentity()
		b, _ := crypto.Decrypt(filepath.Join(StoreDir, "spectre.age"), id)
		content := string(b)
		var mName, mPass string
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "name: ") {
				mName = strings.TrimPrefix(line, "name: ")
			}
			if strings.HasPrefix(line, "pass: ") {
				mPass = strings.TrimPrefix(line, "pass: ")
			}
		}
		s := spectre.RunSpectre(mName, mPass, site, presetName)
		if copyOut {
			copyAndClear(s)
		} else {
			fmt.Println(s)
		}
		return nil
	},
}

var showCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Decrypt and show a secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := loadIdentity()
		if err != nil {
			return err
		}
		b, err := crypto.Decrypt(filepath.Join(StoreDir, args[0]+".age"), id)
		if err != nil {
			return err
		}
		if copyOut {
			copyAndClear(string(b))
		} else {
			s := string(b)
			fmt.Print(s)
			if !strings.HasSuffix(s, "\n") {
				fmt.Println()
			}
		}
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <name> [secret]",
	Short: "Update an existing secret",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		outPath := filepath.Join(StoreDir, name+".age")
		if _, err := os.Stat(outPath); os.IsNotExist(err) {
			return fmt.Errorf("secret %q does not exist, use 'add' to create it", name)
		}

		var secret string
		if len(args) > 1 {
			secret = args[1]
		} else if interactive {
			in, _ := readInput(true, "Secret")
			secret = in
		} else if cmd.Flags().Changed("preset") {
			prof := GlobalConfig.Presets[presetName]
			if prof == nil {
				prof = GlobalConfig.Presets["default"]
			}
			opts := getGeneratorOptions(prof)
			secret, _ = generator.Generate(opts)
		} else {
			in, _ := readInput(false, "")
			if in != "" {
				secret = in
			} else {
				id, err := loadIdentity()
				if err != nil {
					return err
				}
				b, err := crypto.Decrypt(outPath, id)
				if err != nil {
					return err
				}
				secret, err = openEditor(string(b))
				if err != nil {
					return err
				}
			}
		}

		if secret == "" {
			return fmt.Errorf("empty secret, update aborted")
		}

		if copyOut {
			copyAndClear(secret)
		}

		recs, _ := getOwners()
		crypto.Encrypt([]byte(secret), recs, outPath)

		syncStoreMetadata(StoreDir)
		gitDir := filepath.Join(filepath.Dir(StoreDir), ".git")
		if _, err := os.Stat(gitDir); err == nil {
			gitRoot := filepath.Dir(StoreDir)
			relPath, _ := filepath.Rel(gitRoot, outPath)
			exec.Command("git", "-C", gitRoot, "add", relPath).Run()
			exec.Command("git", "-C", gitRoot, "commit", "-m", "Update "+name).Run()
			syncStoreMetadata(StoreDir)
		}

		currentStoreName := "unknown"
		for n, s := range GlobalConfig.Stores {
			if config.ExpandHome(s.Location) == StoreDir {
				currentStoreName = n
				break
			}
		}

		if showSecret {
			fmt.Println(secret)
		} else {
			fmt.Printf("Updated %q in store %q (%s)\n", name, currentStoreName, config.CollapseHome(outPath))
		}
		return nil
	},
}

var otpCmd = &cobra.Command{
	Use:   "otp <name>",
	Short: "Generate OTP code from a secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := loadIdentity()
		b, _ := crypto.Decrypt(filepath.Join(StoreDir, args[0]+".age"), id)
		secret := strings.TrimSpace(string(b))
		code, _ := totp.GenerateCode(secret, time.Now())
		if copyOut {
			copyAndClear(code)
		} else {
			fmt.Println(code)
		}
		return nil
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outPath := filepath.Join(StoreDir, args[0]+".age")
		if _, err := os.Stat(outPath); os.IsNotExist(err) {
			return fmt.Errorf("secret %q not found", args[0])
		}
		os.Remove(outPath)

		syncStoreMetadata(StoreDir)
		gitDir := filepath.Join(filepath.Dir(StoreDir), ".git")
		if _, err := os.Stat(gitDir); err == nil {
			gitRoot := filepath.Dir(StoreDir)
			exec.Command("git", "-C", gitRoot, "add", "-u").Run()
			exec.Command("git", "-C", gitRoot, "commit", "-m", "Remove "+args[0]).Run()
			syncStoreMetadata(StoreDir)
		}

		currentStoreName := "unknown"
		for n, s := range GlobalConfig.Stores {
			if config.ExpandHome(s.Location) == StoreDir {
				currentStoreName = n
				break
			}
		}

		fmt.Printf("Removed %q from store %q (%s)\n", args[0], currentStoreName, config.CollapseHome(outPath))
		return nil
	},
}

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List secrets",
	Run: func(cmd *cobra.Command, args []string) {
		stores := make(map[string]string)
		if storeName != "" {
			if s, ok := GlobalConfig.Stores[storeName]; ok {
				stores[storeName] = config.ExpandHome(s.Location)
			}
		} else {
			for n, s := range GlobalConfig.Stores {
				stores[n] = config.ExpandHome(s.Location)
			}
			found := false
			for _, loc := range stores {
				if loc == StoreDir {
					found = true
					break
				}
			}
			if !found && StoreDir != "" {
				stores["current"] = StoreDir
			}
		}

		var storeNames []string
		for n := range stores {
			storeNames = append(storeNames, n)
		}
		sort.Strings(storeNames)

		for _, sn := range storeNames {
			loc := stores[sn]
			var secrets []string
			filepath.Walk(loc, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".age") && info.Name() != "spectre.age" {
					rel, _ := filepath.Rel(loc, path)
					secrets = append(secrets, strings.TrimSuffix(rel, ".age"))
				}
				return nil
			})
			if len(secrets) > 0 || sn == storeName {
				fmt.Printf("%s (%d entries):\n", sn, len(secrets))
				sort.Strings(secrets)
				for _, s := range secrets {
					fmt.Printf("  - %s\n", s)
				}
			}
		}
	},
}

var searchCmd = &cobra.Command{
	Use:   "search [term]",
	Short: "Search and show a secret using fzf",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("fzf"); err != nil {
			return fmt.Errorf("fzf not found, install it first (e.g., brew install fzf)")
		}

		var allSecrets []string
		stores := make(map[string]string)
		for n, s := range GlobalConfig.Stores {
			stores[n] = config.ExpandHome(s.Location)
		}

		for sn, loc := range stores {
			filepath.Walk(loc, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".age") && info.Name() != "spectre.age" {
					rel, _ := filepath.Rel(loc, path)
					allSecrets = append(allSecrets, sn+":"+strings.TrimSuffix(rel, ".age"))
				}
				return nil
			})
		}

		if len(allSecrets) == 0 {
			return fmt.Errorf("no secrets found")
		}

		fzfArgs := []string{"--prompt=Select secret: "}
		if len(args) > 0 && args[0] != "" {
			fzfArgs = append(fzfArgs, "-f", args[0])
		}
		fzf := exec.Command("fzf", fzfArgs...)
		fzf.Stdin = strings.NewReader(strings.Join(allSecrets, "\n"))
		fzf.Stderr = os.Stderr
		out, err := fzf.Output()
		if err != nil {
			return nil
		}

		choice := strings.TrimSpace(string(out))
		parts := strings.SplitN(choice, ":", 2)
		if len(parts) != 2 {
			return nil
		}

		sName, secretName := parts[0], parts[1]
		storePath := stores[sName]

		id, err := identity.LoadIdentity(identity.GetIdentityPath(GlobalConfig.DefaultIdentity, config.GetIdentitiesDir()))
		if err != nil {
			return err
		}

		b, err := crypto.Decrypt(filepath.Join(storePath, secretName+".age"), id)
		if err != nil {
			return err
		}

		if copyOut {
			copyAndClear(string(b))
		} else {
			fmt.Print(string(b))
			if !strings.HasSuffix(string(b), "\n") {
				fmt.Println()
			}
		}
		return nil
	},
}

var configCmd = &cobra.Command{
	Use:     "config",
	Aliases: []string{"cfg"},
	Short:   "Show or manage config",
	Run: func(cmd *cobra.Command, args []string) {
		if configJson {
			b, _ := json.MarshalIndent(GlobalConfig, "", "  ")
			fmt.Println(string(b))
		} else {
			b, _ := toml.Marshal(GlobalConfig)
			fmt.Println(string(b))
		}
	},
}

func isValidConfigKey(keyPath string) bool {
	parts := strings.Split(keyPath, ".")
	switch len(parts) {
	case 1:
		valid := []string{"default_identity", "use_editor", "clipboard_timeout", "disable_auto_traverse"}
		for _, v := range valid {
			if v == parts[0] {
				return true
			}
		}
	case 2:
		return parts[0] == "stores" || parts[0] == "presets"
	case 3:
		if parts[0] == "stores" {
			valid := []string{"location", "identity", "use_git", "created_at", "updated_at", "commit"}
			for _, v := range valid {
				if v == parts[2] {
					return true
				}
			}
		}
		if parts[0] == "presets" {
			valid := []string{"length", "upper", "lower", "numbers", "symbols", "words", "separator", "count", "pattern", "pronounceable", "mnemonic", "phrase", "wordlist", "charset", "uuid"}
			for _, v := range valid {
				if v == parts[2] {
					return true
				}
			}
		}
	}
	return false
}

func getNested(m map[string]interface{}, keyPath string) (interface{}, error) {
	parts := strings.Split(keyPath, ".")
	var curr interface{} = m
	for _, p := range parts {
		if nextMap, ok := curr.(map[string]interface{}); ok {
			if val, ok := nextMap[p]; ok {
				curr = val
			} else {
				return nil, fmt.Errorf("key %q not found", keyPath)
			}
		} else {
			return nil, fmt.Errorf("key %q not found", keyPath)
		}
	}
	return curr, nil
}

func setNested(m map[string]interface{}, keyPath string, value interface{}) error {
	if !isValidConfigKey(keyPath) {
		return fmt.Errorf("unknown configuration key: %q", keyPath)
	}
	parts := strings.Split(keyPath, ".")
	var curr interface{} = m
	for i, p := range parts {
		if i == len(parts)-1 {
			if nextMap, ok := curr.(map[string]interface{}); ok {
				nextMap[p] = value
				if strings.HasPrefix(keyPath, "stores.") {
					storeName := parts[1]
					if stores, ok := m["stores"].(map[string]interface{}); ok {
						if s, ok := stores[storeName].(map[string]interface{}); ok {
							s["updated_at"] = time.Now().UTC().Format(time.RFC3339)
						}
					}
				}
				return nil
			}
			return fmt.Errorf("failed to set %q", keyPath)
		}

		if nextMap, ok := curr.(map[string]interface{}); ok {
			if nextVal, ok := nextMap[p]; ok {
				curr = nextVal
			} else {
				newMap := make(map[string]interface{})
				nextMap[p] = newMap
				curr = newMap
			}
		} else {
			return fmt.Errorf("failed to set %q", keyPath)
		}
	}
	return nil
}

func deleteNested(m map[string]interface{}, keyPath string) error {
	if !isValidConfigKey(keyPath) {
		return fmt.Errorf("unknown configuration key: %q", keyPath)
	}
	parts := strings.Split(keyPath, ".")
	var curr interface{} = m
	for i, p := range parts {
		if i == len(parts)-1 {
			if nextMap, ok := curr.(map[string]interface{}); ok {
				delete(nextMap, p)
				if strings.HasPrefix(keyPath, "stores.") && len(parts) > 2 {
					storeName := parts[1]
					if stores, ok := m["stores"].(map[string]interface{}); ok {
						if s, ok := stores[storeName].(map[string]interface{}); ok {
							s["updated_at"] = time.Now().UTC().Format(time.RFC3339)
						}
					}
				}
				return nil
			}
			return fmt.Errorf("failed to delete %q", keyPath)
		}

		if nextMap, ok := curr.(map[string]interface{}); ok {
			if nextVal, ok := nextMap[p]; ok {
				curr = nextVal
			} else {
				return fmt.Errorf("key %q not found", keyPath)
			}
		} else {
			return fmt.Errorf("failed to delete %q", keyPath)
		}
	}
	return nil
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value (supports nested keys like stores.global)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, _ := toml.Marshal(GlobalConfig)
		var m map[string]interface{}
		toml.Unmarshal(b, &m)

		val, err := getNested(m, args[0])
		if err != nil {
			return err
		}

		if configJson {
			bj, _ := json.MarshalIndent(val, "", "  ")
			fmt.Println(string(bj))
		} else {
			bt, err := toml.Marshal(val)
			if err == nil {
				fmt.Print(string(bt))
			} else {
				fmt.Println(val)
			}
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, _ := toml.Marshal(GlobalConfig)
		var m map[string]interface{}
		toml.Unmarshal(b, &m)

		key, valStr := args[0], args[1]
		var val interface{} = valStr
		if valStr == "true" {
			val = true
		} else if valStr == "false" {
			val = false
		} else if n, err := strconv.Atoi(valStr); err == nil {
			val = n
		}

		if err := setNested(m, key, val); err != nil {
			return err
		}

		nb, _ := toml.Marshal(m)
		GlobalConfig.Presets = make(map[string]*config.Preset)
		GlobalConfig.Stores = make(map[string]config.StoreConfig)
		if err := toml.Unmarshal(nb, GlobalConfig); err != nil {
			return err
		}

		if err := config.SaveConfig(GlobalConfig); err != nil {
			return err
		}
		fmt.Printf("Set %s = %v\n", key, val)
		syncStoreMetadata(StoreDir) // Sync after potentially changing use_git
		return nil
	},
}

var configRmCmd = &cobra.Command{
	Use:   "rm <key>",
	Short: "Delete a config value (supports nested keys)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		b, _ := toml.Marshal(GlobalConfig)
		var m map[string]interface{}
		toml.Unmarshal(b, &m)

		if err := deleteNested(m, args[0]); err != nil {
			return err
		}

		nb, _ := toml.Marshal(m)
		GlobalConfig.Presets = make(map[string]*config.Preset)
		GlobalConfig.Stores = make(map[string]config.StoreConfig)
		if err := toml.Unmarshal(nb, GlobalConfig); err != nil {
			return err
		}

		if err := config.SaveConfig(GlobalConfig); err != nil {
			return err
		}
		fmt.Printf("Deleted key %q\n", args[0])
		return nil
	},
}

func init() {
	storeCmd.AddCommand(storeNewCmd)
	storeCmd.AddCommand(storeLsCmd)
	storeCmd.AddCommand(storeRmCmd)

	storeNewCmd.Flags().BoolVar(&noGit, "no-git", false, "Disable git")
	storeNewCmd.Flags().BoolVar(&yesGit, "git", false, "Enable git")
	storeNewCmd.Flags().BoolVarP(&storeGlobal, "global", "g", false, "Initialize global store")
	storeNewCmd.Flags().StringVar(&storeId, "id", "", "Identity to use (defaults to config default_identity)")
	storeNewCmd.Flags().StringVarP(&storeDir, "dir", "d", "", "Directory to initialize (defaults to cwd)")

	storeRmCmd.Flags().BoolVar(&purgeStore, "purge", false, "Remove store files from disk")

	idCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Protect with passphrase")
	idCmd.Flags().BoolVar(&idGet, "get", false, "Print identity contents")
	idCmd.Flags().BoolVar(&idRm, "rm", false, "Remove identity")

	addCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Prompt for input")
	addCmd.Flags().StringVarP(&ownerFile, "owner-file", "O", "", "Owners file")
	addCmd.Flags().StringSliceVarP(&owners, "owner", "o", nil, "Owners")
	addCmd.Flags().StringVarP(&presetName, "preset", "p", "default", "Preset name")
	addCmd.Flags().BoolVarP(&copyOut, "copy", "c", false, "Copy to clipboard")
	addCmd.Flags().BoolVarP(&showSecret, "show", "s", false, "Show generated secret")

	genCmd.Flags().IntVar(&genLength, "length", 64, "Length")
	genCmd.Flags().BoolVar(&genUpper, "upper", true, "Upper")
	genCmd.Flags().BoolVar(&genLower, "lower", true, "Lower")
	genCmd.Flags().BoolVar(&genNumbers, "numbers", true, "Numbers")
	genCmd.Flags().BoolVar(&genSymbols, "symbols", true, "Symbols")

	genCmd.Flags().BoolVar(&genNoUpper, "no-upper", false, "Exclude upper")
	genCmd.Flags().BoolVar(&genNoLower, "no-lower", false, "Exclude lower")
	genCmd.Flags().BoolVar(&genNoNumbers, "no-numbers", false, "Exclude numbers")
	genCmd.Flags().BoolVar(&genNoSymbols, "no-symbols", false, "Exclude symbols")
	genCmd.Flags().BoolVar(&genBasic, "basic", false, "Basic alphanumeric (no upper, numbers, symbols)")
	genCmd.Flags().IntVar(&genWords, "words", 0, "Words")
	genCmd.Flags().StringVar(&genSeparator, "separator", "-", "Separator")
	genCmd.Flags().StringVar(&genSep, "sep", "", "Alias for separator")
	genCmd.Flags().IntVar(&genCount, "count", 1, "Count")
	genCmd.Flags().StringVar(&genPattern, "pattern", "", "Pattern")
	genCmd.Flags().BoolVar(&genPronounceable, "pronounceable", false, "Pronounceable")
	genCmd.Flags().IntVar(&genMnemonic, "mnemonic", 0, "BIP39 Mnemonic 12|24")
	genCmd.Flags().IntVar(&genPhrase, "phrase", 0, "BIP39 Phrase words")
	genCmd.Flags().StringVar(&genWordlist, "wordlist", "english", "Wordlist lang")
	genCmd.Flags().StringVar(&genCharset, "charset", "", "Charset")
	genCmd.Flags().StringVar(&genUUID, "uuid", "", "UUID v4|v7")
	genCmd.Flags().StringVar(&createPreset, "create-preset", "", "Create preset")
	genCmd.Flags().StringVarP(&presetName, "preset", "p", "default", "Preset")

	spectreCmd.Flags().StringVar(&spectreName, "name", "", "Master name")
	spectreCmd.Flags().StringVar(&spectrePass, "pass", "", "Master pass")
	spectreCmd.Flags().StringVarP(&presetName, "preset", "p", "default", "Preset")
	spectreCmd.Flags().BoolVarP(&copyOut, "copy", "c", false, "Copy to clipboard")

	showCmd.Flags().BoolVarP(&copyOut, "copy", "c", false, "Copy")
	otpCmd.Flags().BoolVarP(&copyOut, "copy", "c", false, "Copy")

	updateCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive")
	updateCmd.Flags().StringVarP(&presetName, "preset", "p", "default", "Preset")
	updateCmd.Flags().BoolVarP(&copyOut, "copy", "c", false, "Copy to clipboard")
	updateCmd.Flags().BoolVarP(&showSecret, "show", "s", false, "Show generated secret")

	configGetCmd.Flags().BoolVar(&configJson, "json", false, "Output in JSON format")
	configSetCmd.Flags().BoolVar(&configJson, "json", false, "Output in JSON format")
	configRmCmd.Flags().BoolVar(&configJson, "json", false, "Output in JSON format")
	configCmd.PersistentFlags().BoolVar(&configJson, "json", false, "Output in JSON format")
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configRmCmd)

	searchCmd.Flags().BoolVarP(&copyOut, "copy", "c", false, "Copy to clipboard")
}
