package locale

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const StateDir = "/var/lib/singularity"

var Keys = []string{
	"LANG", "LANGUAGE", "LC_CTYPE", "LC_NUMERIC", "LC_TIME", "LC_COLLATE", "LC_MONETARY",
	"LC_MESSAGES", "LC_PAPER", "LC_NAME", "LC_ADDRESS", "LC_TELEPHONE", "LC_MEASUREMENT",
	"LC_IDENTIFICATION",
}

var keyboardKeys = []string{"KEYMAP", "KEYMAP_TOGGLE", "XKBLAYOUT", "XKBMODEL", "XKBVARIANT", "XKBOPTIONS"}

type Keyboard struct {
	Keymap, KeymapToggle, Layout, Model, Variant, Options string
}

type System struct {
	Root      string
	Available func() []string
	LoadKeys  func(keymap string) error
}

func (s System) path(p string) string {
	if s.Root == "" {
		return p
	}
	return filepath.Join(s.Root, p)
}

func (s System) LocaleFile() string   { return s.path(filepath.Join(StateDir, "locale.conf")) }
func (s System) KeyboardFile() string { return s.path(filepath.Join(StateDir, "vconsole.conf")) }

func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}

func parseEnvFile(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = unquote(strings.TrimSpace(v))
	}
	return out
}

func (s System) read(state, fallback string) map[string]string {
	if _, err := os.Stat(state); err == nil {
		return parseEnvFile(state)
	}
	return parseEnvFile(s.path(fallback))
}

func writeEnvFile(path string, values map[string]string, order []string) error {
	var b strings.Builder
	for _, k := range order {
		if v := values[k]; v != "" {
			b.WriteString(k + "=" + v + "\n")
		}
	}
	return writeFileAtomic(path, []byte(b.String()), 0o644)
}

func (s System) Locale() []string {
	values := s.read(s.LocaleFile(), "/etc/locale.conf")
	out := []string{}
	for _, k := range Keys {
		if v := values[k]; v != "" {
			out = append(out, k+"="+v)
		}
	}
	return out
}

func normalize(name string) string {
	lang, rest, _ := strings.Cut(name, ".")
	if rest == "" {
		return lang
	}
	codeset, mod, _ := strings.Cut(rest, "@")
	codeset = strings.ToLower(strings.ReplaceAll(codeset, "-", ""))
	if mod != "" {
		return lang + "." + codeset + "@" + mod
	}
	return lang + "." + codeset
}

func SystemLocales() []string {
	out, err := exec.Command("locale", "-a").Output()
	if err != nil {
		return nil
	}
	var list []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			list = append(list, l)
		}
	}
	return list
}

func (s System) available() []string {
	if s.Available != nil {
		return s.Available()
	}
	return SystemLocales()
}

func (s System) ListLocales() []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range s.available() {
		if l == "C" || l == "POSIX" || strings.HasPrefix(l, "C.") {
			continue
		}
		lang, rest, _ := strings.Cut(l, ".")
		name := l
		if strings.EqualFold(strings.ReplaceAll(rest, "-", ""), "utf8") {
			name = lang + ".UTF-8"
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func (s System) validLocale(name string) error {
	if name == "C" || name == "POSIX" || normalize(name) == "C.utf8" {
		return nil
	}
	want := normalize(name)
	for _, l := range s.available() {
		if normalize(l) == want {
			return nil
		}
	}
	return fmt.Errorf("locale %q is not installed", name)
}

func validLanguage(v string) error {
	for _, c := range v {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == ':' || c == '@') {
			return fmt.Errorf("LANGUAGE contains %q", c)
		}
	}
	return nil
}

func (s System) SetLocale(assignments []string) error {
	values := map[string]string{}
	for _, a := range assignments {
		k, v, ok := strings.Cut(a, "=")
		if !ok {
			return fmt.Errorf("%q is not KEY=value", a)
		}
		known := false
		for _, key := range Keys {
			if key == k {
				known = true
			}
		}
		if !known {
			return fmt.Errorf("unknown locale variable %s", k)
		}
		if v == "" {
			continue
		}
		if k == "LANGUAGE" {
			if err := validLanguage(v); err != nil {
				return err
			}
		} else if err := s.validLocale(v); err != nil {
			return err
		}
		values[k] = v
	}
	if len(values) > 0 && values["LANG"] == "" {
		return errors.New("LANG must be set")
	}
	if len(values) == 0 {
		err := os.Remove(s.LocaleFile())
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return writeEnvFile(s.LocaleFile(), values, Keys)
}

func (s System) Keyboard() Keyboard {
	v := s.read(s.KeyboardFile(), "/etc/vconsole.conf")
	return Keyboard{v["KEYMAP"], v["KEYMAP_TOGGLE"], v["XKBLAYOUT"], v["XKBMODEL"], v["XKBVARIANT"], v["XKBOPTIONS"]}
}

func validKeyboardField(name, v string) error {
	for _, c := range v {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_-,:+()./", c)) {
			return fmt.Errorf("%s contains %q", name, c)
		}
	}
	if strings.Contains(v, "..") || strings.HasPrefix(v, "/") {
		return fmt.Errorf("%s is not a name", name)
	}
	return nil
}

func (k Keyboard) validate() error {
	for name, v := range map[string]string{"keymap": k.Keymap, "toggle keymap": k.KeymapToggle, "layout": k.Layout, "model": k.Model, "variant": k.Variant, "options": k.Options} {
		if err := validKeyboardField(name, v); err != nil {
			return err
		}
	}
	return nil
}

func (s System) setKeyboard(k Keyboard) error {
	if err := k.validate(); err != nil {
		return err
	}
	values := map[string]string{
		"KEYMAP": k.Keymap, "KEYMAP_TOGGLE": k.KeymapToggle, "XKBLAYOUT": k.Layout,
		"XKBMODEL": k.Model, "XKBVARIANT": k.Variant, "XKBOPTIONS": k.Options,
	}
	return writeEnvFile(s.KeyboardFile(), values, keyboardKeys)
}

func (s System) loadKeys(keymap string) error {
	if s.LoadKeys != nil {
		return s.LoadKeys(keymap)
	}
	if _, err := exec.LookPath("loadkeys"); err != nil {
		return nil
	}
	return exec.Command("loadkeys", keymap).Run()
}

func (s System) SetConsoleKeyboard(keymap, toggle string, convert bool) error {
	k := s.Keyboard()
	k.Keymap, k.KeymapToggle = keymap, toggle
	if convert {
		k.Layout, k.Variant = keymap, ""
	}
	if err := s.setKeyboard(k); err != nil {
		return err
	}
	if keymap != "" {
		s.loadKeys(keymap)
	}
	return nil
}

func (s System) SetX11Keyboard(layout, model, variant, options string, convert bool) error {
	k := s.Keyboard()
	k.Layout, k.Model, k.Variant, k.Options = layout, model, variant, options
	if convert {
		first, _, _ := strings.Cut(layout, ",")
		k.Keymap = first
	}
	if err := s.setKeyboard(k); err != nil {
		return err
	}
	if convert && k.Keymap != "" {
		s.loadKeys(k.Keymap)
	}
	return nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".sinty-locale-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
