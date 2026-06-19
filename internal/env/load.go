package env

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Loaded records which .env files were applied and the resolved env name.
type Loaded struct {
	Files   []string
	EnvName string
}

// Load applies .env files to the process environment.
//
// Order: .env first, then .env.<envName> (envName from arg or AUTOQA_ENV).
// Existing process env values always win — a key already set in the
// environment is never overwritten by a file. If envName is non-empty the
// matching .env.<name> file must exist, otherwise it is an error.
func Load(cwd, envName string) (Loaded, error) {
	initial := map[string]bool{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			initial[kv[:i]] = true
		}
	}
	loaded := Loaded{}
	base := filepath.Join(cwd, ".env")
	if st, err := os.Stat(base); err == nil && !st.IsDir() {
		if err := loadFile(base, initial); err != nil {
			return loaded, err
		}
		loaded.Files = append(loaded.Files, base)
	}
	if envName == "" {
		envName = strings.TrimSpace(os.Getenv("AUTOQA_ENV"))
	}
	loaded.EnvName = envName
	if envName != "" {
		path := filepath.Join(cwd, ".env."+envName)
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			return loaded, fmt.Errorf("environment %q requires %s to exist", envName, path)
		}
		if err := loadFile(path, initial); err != nil {
			return loaded, err
		}
		loaded.Files = append(loaded.Files, path)
	}
	return loaded, nil
}

// loadFile parses a .env file (KEY=value, optional `export`, quoted values,
// # comments) and sets keys not already present in `initial` into the process
// environment.
func loadFile(path string, initial map[string]bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(line[i+1:])
		if key == "" || initial[key] {
			continue
		}
		if len(val) >= 2 {
			if (val[0] == '\'' && val[len(val)-1] == '\'') || (val[0] == '"' && val[len(val)-1] == '"') {
				val = val[1 : len(val)-1]
			}
		}
		_ = os.Setenv(key, val)
	}
	return s.Err()
}
