package localed

import (
	"bufio"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/godbus/dbus/v5"

	"github.com/singularityos-lab/sinty-locale/internal/locale"
)

func privateBus(t *testing.T) string {
	cmd := exec.Command("dbus-daemon", "--session", "--nofork", "--print-address")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Skip("dbus-daemon not available")
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(line)
}

func TestService(t *testing.T) {
	addr := privateBus(t)
	server, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	sys := locale.System{
		Root:      t.TempDir(),
		Available: func() []string { return []string{"C.utf8", "en_US.utf8", "it_IT.utf8"} },
		LoadKeys:  func(string) error { return nil },
	}
	svc := New(server, sys)
	var allow atomic.Bool
	svc.isAdminHook = func(uint32) bool { return allow.Load() }
	if err := svc.Export(); err != nil {
		t.Fatal(err)
	}
	if _, err := server.RequestName(BusName, dbus.NameFlagDoNotQueue); err != nil {
		t.Fatal(err)
	}
	client, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	obj := client.Object(BusName, ObjectPath)

	if err := obj.Call(Iface+".SetLocale", 0, []string{"LANG=it_IT.UTF-8"}, false).Err; err == nil {
		t.Fatal("a non-admin caller without polkit must be refused")
	}
	allow.Store(true)
	if err := obj.Call(Iface+".SetLocale", 0, []string{"LANG=xx_XX.UTF-8"}, false).Err; err == nil {
		t.Fatal("missing locale accepted")
	}
	if err := obj.Call(Iface+".SetLocale", 0, []string{"LANG=it_IT.UTF-8", "LC_TIME=en_US.UTF-8"}, true).Err; err != nil {
		t.Fatal(err)
	}
	loc, err := obj.GetProperty(Iface + ".Locale")
	if err != nil || strings.Join(loc.Value().([]string), " ") != "LANG=it_IT.UTF-8 LC_TIME=en_US.UTF-8" {
		t.Fatalf("Locale = %v %v", loc, err)
	}
	if err := obj.Call(Iface+".SetX11Keyboard", 0, "it", "", "", "", true, false).Err; err != nil {
		t.Fatal(err)
	}
	var props map[string]dbus.Variant
	if err := obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, Iface).Store(&props); err != nil {
		t.Fatal(err)
	}
	if props["X11Layout"].Value().(string) != "it" || props["VConsoleKeymap"].Value().(string) != "it" {
		t.Fatalf("keyboard %v", props)
	}
	if err := obj.Call(Iface+".SetVConsoleKeyboard", 0, "../x", "", false, false).Err; err == nil {
		t.Fatal("path keymap accepted")
	}
}
