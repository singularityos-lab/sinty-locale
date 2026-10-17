package locale

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testSystem(t *testing.T) (System, *[]string) {
	var loaded []string
	return System{
		Root:      t.TempDir(),
		Available: func() []string { return []string{"C", "C.utf8", "POSIX", "en_US.utf8", "it_IT.utf8", "de_DE.utf8"} },
		LoadKeys:  func(k string) error { loaded = append(loaded, k); return nil },
	}, &loaded
}

func TestSetLocale(t *testing.T) {
	s, _ := testSystem(t)
	if got := s.ListLocales(); !reflect.DeepEqual(got, []string{"de_DE.UTF-8", "en_US.UTF-8", "it_IT.UTF-8"}) {
		t.Fatalf("ListLocales %v", got)
	}
	if err := s.SetLocale([]string{"LANG=it_IT.UTF-8", "LC_TIME=en_US.UTF-8", "LANGUAGE=it:en"}); err != nil {
		t.Fatal(err)
	}
	if got := s.Locale(); !reflect.DeepEqual(got, []string{"LANG=it_IT.UTF-8", "LANGUAGE=it:en", "LC_TIME=en_US.UTF-8"}) {
		t.Fatalf("Locale %v", got)
	}
	for _, bad := range [][]string{{"LANG=xx_YY.UTF-8"}, {"LC_TIME=xx_YY.UTF-8"}, {"FOO=en_US.UTF-8"}, {"LANG"}, {"LANG=en_US.UTF-8", "LANGUAGE=it;rm"}} {
		if err := s.SetLocale(bad); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
	if err := s.SetLocale([]string{"LANG=C.UTF-8"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLocale(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.LocaleFile()); !os.IsNotExist(err) {
		t.Fatalf("empty locale kept: %v", err)
	}
}

func TestLocaleFallsBackToEtc(t *testing.T) {
	s, _ := testSystem(t)
	os.MkdirAll(filepath.Join(s.Root, "etc"), 0o755)
	os.WriteFile(filepath.Join(s.Root, "etc/locale.conf"), []byte("LANG=\"de_DE.UTF-8\"\n"), 0o644)
	if got := s.Locale(); !reflect.DeepEqual(got, []string{"LANG=de_DE.UTF-8"}) {
		t.Fatalf("Locale %v", got)
	}
}

func TestKeyboard(t *testing.T) {
	s, loaded := testSystem(t)
	os.MkdirAll(filepath.Dir(s.KeyboardFile()), 0o755)
	os.WriteFile(s.KeyboardFile(), []byte("KEYMAP=us\nXKBLAYOUT=us\n"), 0o644)
	if err := s.SetX11Keyboard("it,us", "pc105", "", "grp:alt_shift_toggle", true); err != nil {
		t.Fatal(err)
	}
	k := s.Keyboard()
	if k.Layout != "it,us" || k.Model != "pc105" || k.Options != "grp:alt_shift_toggle" || k.Keymap != "it" {
		t.Fatalf("x11 %+v", k)
	}
	if err := s.SetConsoleKeyboard("de", "", true); err != nil {
		t.Fatal(err)
	}
	k = s.Keyboard()
	if k.Keymap != "de" || k.Layout != "de" || k.Model != "pc105" {
		t.Fatalf("console %+v", k)
	}
	if !reflect.DeepEqual(*loaded, []string{"it", "de"}) {
		t.Fatalf("loadkeys %v", *loaded)
	}
	for _, bad := range []string{"../../etc/shadow", "/etc/x", "us;reboot", "us\nKEYMAP=x"} {
		if err := s.SetConsoleKeyboard(bad, "", false); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
