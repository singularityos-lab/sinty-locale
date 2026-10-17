package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/singularityos-lab/sinty-locale/internal/locale"
)

var sys = locale.System{}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return cmdStatus()
	}
	switch args[0] {
	case "-h", "--help", "help":
		usage(os.Stdout)
		return 0
	case "status":
		return cmdStatus()
	case "list-locales":
		for _, l := range sys.ListLocales() {
			fmt.Println(l)
		}
		return 0
	case "set-locale":
		return cmdSetLocale(args[1:])
	case "set-keymap":
		return cmdSetKeymap(args[1:])
	case "set-x11-keymap":
		return cmdSetX11Keymap(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "sinty-locale: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func cmdStatus() int {
	loc := sys.Locale()
	if len(loc) == 0 {
		fmt.Printf("%16s: %s\n", "System Locale", "n/a")
	}
	for i, l := range loc {
		label := ""
		if i == 0 {
			label = "System Locale"
		}
		fmt.Printf("%16s: %s\n", label, l)
	}
	k := sys.Keyboard()
	show := func(label, v string) {
		if v == "" {
			v = "n/a"
		}
		fmt.Printf("%16s: %s\n", label, v)
	}
	show("VC Keymap", k.Keymap)
	show("X11 Layout", k.Layout)
	if k.Model != "" {
		show("X11 Model", k.Model)
	}
	if k.Variant != "" {
		show("X11 Variant", k.Variant)
	}
	if k.Options != "" {
		show("X11 Options", k.Options)
	}
	return 0
}

func cmdSetLocale(args []string) int {
	if len(args) == 0 {
		return fail("set-locale: expected LANG=... or a locale name")
	}
	if !isRoot() {
		return fail("set-locale: must be run as root")
	}
	var assignments []string
	for _, a := range args {
		if !strings.Contains(a, "=") {
			a = "LANG=" + a
		}
		assignments = append(assignments, a)
	}
	if err := sys.SetLocale(assignments); err != nil {
		return fail("set-locale: %v", err)
	}
	return 0
}

func cmdSetKeymap(args []string) int {
	if len(args) < 1 || len(args) > 2 {
		return fail("set-keymap: expected MAP [TOGGLEMAP]")
	}
	if !isRoot() {
		return fail("set-keymap: must be run as root")
	}
	toggle := ""
	if len(args) == 2 {
		toggle = args[1]
	}
	if err := sys.SetConsoleKeyboard(args[0], toggle, true); err != nil {
		return fail("set-keymap: %v", err)
	}
	return 0
}

func cmdSetX11Keymap(args []string) int {
	if len(args) < 1 || len(args) > 4 {
		return fail("set-x11-keymap: expected LAYOUT [MODEL [VARIANT [OPTIONS]]]")
	}
	if !isRoot() {
		return fail("set-x11-keymap: must be run as root")
	}
	f := append(args, "", "", "")
	if err := sys.SetX11Keyboard(f[0], f[1], f[2], f[3], true); err != nil {
		return fail("set-x11-keymap: %v", err)
	}
	return 0
}

func isRoot() bool { return os.Geteuid() == 0 }

func fail(format string, a ...any) int {
	fmt.Fprintf(os.Stderr, "sinty-locale: "+format+"\n", a...)
	return 1
}

func usage(w *os.File) {
	fmt.Fprint(w, `sinty-locale - query and set the system language and keyboard on SintyOS

Usage:
  sinty-locale [status]                              show the locale and keyboard
  sinty-locale list-locales                          list the installed locales
  sinty-locale set-locale LANG=... [LC_TIME=...]     set the system locale (root)
  sinty-locale set-keymap MAP [TOGGLEMAP]            set the console keymap (root)
  sinty-locale set-x11-keymap LAYOUT [MODEL [VARIANT [OPTIONS]]]
                                                     set the desktop keyboard (root)
  sinty-locale --help

set-keymap also sets the desktop layout to the same name, and set-x11-keymap
sets the console keymap to the first layout, as localectl does. A locale that
is not installed is refused.
`)
}
