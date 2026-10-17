package localed

import (
	"errors"
	"os/user"
	"strconv"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"

	"github.com/singularityos-lab/sinty-locale/internal/locale"
)

const (
	BusName    = "org.freedesktop.locale1"
	ObjectPath = dbus.ObjectPath("/org/freedesktop/locale1")
	Iface      = "org.freedesktop.locale1"
	propsIface = "org.freedesktop.DBus.Properties"

	actionLocale   = "org.freedesktop.locale1.set-locale"
	actionKeyboard = "org.freedesktop.locale1.set-keyboard"
	polkitName     = "org.freedesktop.PolicyKit1"
)

type Backend interface {
	Locale() []string
	SetLocale(assignments []string) error
	Keyboard() locale.Keyboard
	SetConsoleKeyboard(keymap, toggle string, convert bool) error
	SetX11Keyboard(layout, model, variant, options string, convert bool) error
}

type Service struct {
	conn        *dbus.Conn
	backend     Backend
	mu          sync.Mutex
	adminGroups []string
	isAdminHook func(uint32) bool
}

func New(conn *dbus.Conn, backend Backend) *Service {
	return &Service{conn: conn, backend: backend, adminGroups: []string{"wheel", "sudo", "admin"}}
}

func in(name, typ string) introspect.Arg {
	return introspect.Arg{Name: name, Type: typ, Direction: "in"}
}
func out(name, typ string) introspect.Arg {
	return introspect.Arg{Name: name, Type: typ, Direction: "out"}
}

var introspection = introspect.Interface{
	Name: Iface,
	Methods: []introspect.Method{
		{Name: "SetLocale", Args: []introspect.Arg{in("locale", "as"), in("interactive", "b")}},
		{Name: "SetVConsoleKeyboard", Args: []introspect.Arg{in("keymap", "s"), in("keymap_toggle", "s"), in("convert", "b"), in("interactive", "b")}},
		{Name: "SetX11Keyboard", Args: []introspect.Arg{in("layout", "s"), in("model", "s"), in("variant", "s"), in("options", "s"), in("convert", "b"), in("interactive", "b")}},
	},
	Properties: []introspect.Property{
		{Name: "Locale", Type: "as", Access: "read"},
		{Name: "VConsoleKeymap", Type: "s", Access: "read"},
		{Name: "VConsoleKeymapToggle", Type: "s", Access: "read"},
		{Name: "X11Layout", Type: "s", Access: "read"},
		{Name: "X11Model", Type: "s", Access: "read"},
		{Name: "X11Variant", Type: "s", Access: "read"},
		{Name: "X11Options", Type: "s", Access: "read"},
	},
}

var propsIntrospection = introspect.Interface{
	Name: propsIface,
	Methods: []introspect.Method{
		{Name: "Get", Args: []introspect.Arg{in("interface_name", "s"), in("property_name", "s"), out("value", "v")}},
		{Name: "GetAll", Args: []introspect.Arg{in("interface_name", "s"), out("properties", "a{sv}")}},
		{Name: "Set", Args: []introspect.Arg{in("interface_name", "s"), in("property_name", "s"), in("value", "v")}},
	},
	Signals: []introspect.Signal{
		{Name: "PropertiesChanged", Args: []introspect.Arg{{Name: "interface_name", Type: "s"}, {Name: "changed_properties", Type: "a{sv}"}, {Name: "invalidated_properties", Type: "as"}}},
	},
}

func (s *Service) Export() error {
	if err := s.conn.ExportMethodTable(map[string]interface{}{
		"SetLocale":           s.SetLocale,
		"SetVConsoleKeyboard": s.SetVConsoleKeyboard,
		"SetX11Keyboard":      s.SetX11Keyboard,
	}, ObjectPath, Iface); err != nil {
		return err
	}
	if err := s.conn.ExportMethodTable(map[string]interface{}{
		"Get":    s.get,
		"GetAll": s.getAll,
		"Set":    s.set,
	}, ObjectPath, propsIface); err != nil {
		return err
	}
	node := &introspect.Node{Name: string(ObjectPath), Interfaces: []introspect.Interface{introspect.IntrospectData, propsIntrospection, introspection}}
	return s.conn.Export(introspect.NewIntrospectable(node), ObjectPath, "org.freedesktop.DBus.Introspectable")
}

func (s *Service) props() map[string]dbus.Variant {
	k := s.backend.Keyboard()
	return map[string]dbus.Variant{
		"Locale":               dbus.MakeVariant(s.backend.Locale()),
		"VConsoleKeymap":       dbus.MakeVariant(k.Keymap),
		"VConsoleKeymapToggle": dbus.MakeVariant(k.KeymapToggle),
		"X11Layout":            dbus.MakeVariant(k.Layout),
		"X11Model":             dbus.MakeVariant(k.Model),
		"X11Variant":           dbus.MakeVariant(k.Variant),
		"X11Options":           dbus.MakeVariant(k.Options),
	}
}

func (s *Service) get(iface, name string) (dbus.Variant, *dbus.Error) {
	all, err := s.getAll(iface)
	if err != nil {
		return dbus.Variant{}, err
	}
	v, ok := all[name]
	if !ok {
		return dbus.Variant{}, dbus.NewError("org.freedesktop.DBus.Error.UnknownProperty", []interface{}{name})
	}
	return v, nil
}

func (s *Service) getAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != Iface {
		return nil, dbus.NewError("org.freedesktop.DBus.Error.UnknownInterface", []interface{}{iface})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.props(), nil
}

func (s *Service) set(iface, name string, v dbus.Variant) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.PropertyReadOnly", []interface{}{name})
}

func (s *Service) changed(names ...string) {
	all := s.props()
	out := map[string]dbus.Variant{}
	for _, n := range names {
		out[n] = all[n]
	}
	s.conn.Emit(ObjectPath, propsIface+".PropertiesChanged", Iface, out, []string{})
}

func failed(err error) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.Failed", []interface{}{err.Error()})
}

func invalid(err error) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.InvalidArgs", []interface{}{err.Error()})
}

var errDenied = errors.New("not authorized")

func (s *Service) authorize(sender dbus.Sender, action string, interactive bool) *dbus.Error {
	var uid uint32
	if err := s.conn.BusObject().Call("org.freedesktop.DBus.GetConnectionUnixUser", 0, string(sender)).Store(&uid); err != nil {
		return failed(err)
	}
	if uid == 0 {
		return nil
	}
	if s.hasPolkit() {
		subject := struct {
			Kind    string
			Details map[string]dbus.Variant
		}{"system-bus-name", map[string]dbus.Variant{"name": dbus.MakeVariant(string(sender))}}
		var res struct {
			Authorized bool
			Challenge  bool
			Details    map[string]string
		}
		flags := uint32(0)
		if interactive {
			flags = 1
		}
		obj := s.conn.Object(polkitName, "/org/freedesktop/PolicyKit1/Authority")
		if err := obj.Call("org.freedesktop.PolicyKit1.Authority.CheckAuthorization", 0, subject, action, map[string]string{}, flags, "").Store(&res); err == nil {
			if res.Authorized {
				return nil
			}
			return dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []interface{}{errDenied.Error()})
		}
	}
	if s.isAdmin(uid) {
		return nil
	}
	return dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []interface{}{errDenied.Error()})
}

func (s *Service) hasPolkit() bool {
	var owned bool
	if err := s.conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, polkitName).Store(&owned); err == nil && owned {
		return true
	}
	var names []string
	if err := s.conn.BusObject().Call("org.freedesktop.DBus.ListActivatableNames", 0).Store(&names); err != nil {
		return false
	}
	for _, n := range names {
		if n == polkitName {
			return true
		}
	}
	return false
}

func (s *Service) isAdmin(uid uint32) bool {
	if s.isAdminHook != nil {
		return s.isAdminHook(uid)
	}
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return false
	}
	ids, err := u.GroupIds()
	if err != nil {
		return false
	}
	for _, id := range ids {
		g, err := user.LookupGroupId(id)
		if err != nil {
			continue
		}
		for _, want := range s.adminGroups {
			if g.Name == want {
				return true
			}
		}
	}
	return false
}

func (s *Service) SetLocale(assignments []string, interactive bool, sender dbus.Sender) *dbus.Error {
	if err := s.authorize(sender, actionLocale, interactive); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.backend.SetLocale(assignments); err != nil {
		return invalid(err)
	}
	s.changed("Locale")
	return nil
}

func (s *Service) SetVConsoleKeyboard(keymap, toggle string, convert, interactive bool, sender dbus.Sender) *dbus.Error {
	if err := s.authorize(sender, actionKeyboard, interactive); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.backend.SetConsoleKeyboard(keymap, toggle, convert); err != nil {
		return invalid(err)
	}
	s.changed("VConsoleKeymap", "VConsoleKeymapToggle", "X11Layout", "X11Variant")
	return nil
}

func (s *Service) SetX11Keyboard(layout, model, variant, options string, convert, interactive bool, sender dbus.Sender) *dbus.Error {
	if err := s.authorize(sender, actionKeyboard, interactive); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.backend.SetX11Keyboard(layout, model, variant, options, convert); err != nil {
		return invalid(err)
	}
	s.changed("X11Layout", "X11Model", "X11Variant", "X11Options", "VConsoleKeymap")
	return nil
}
