# sinty-locale

A native replacement for `localectl` and `systemd-localed` on SintyOS. It shows
and sets the system language (the `LANG` and `LC_*` variables), the console
keymap and the desktop keyboard layout.

## Why it exists

SintyOS does not boot with systemd. PID 1 is `sinit` and services are driven by
`atomctl`. Without `systemd-localed` nothing answers on
`org.freedesktop.locale1`, so the language and keyboard settings cannot be read
or changed from Settings, and `localectl` refuses to run:

    System has not been booted with systemd as init system (PID 1). Can't operate.

`sinty-locale` works with the same files the rest of SintyOS already uses:

- the system locale lives in `/var/lib/singularity/locale.conf`, written by the
  first-boot setup and read by the session at login;
- the console keymap and the desktop layout live in
  `/var/lib/singularity/vconsole.conf` (`KEYMAP`, `XKBLAYOUT`, `XKBMODEL`,
  `XKBVARIANT`, `XKBOPTIONS`), applied to the console at boot by
  `atom-vconsole-setup` and to the desktop by the session;
- `/etc/locale.conf` and `/etc/vconsole.conf` are read as defaults when the
  writable copies do not exist yet, because `/etc` is part of the read-only
  image;
- a locale is accepted only if it is installed, as reported by `locale -a`.

## Usage

    sinty-locale [status]                              show the locale and keyboard
    sinty-locale list-locales                          list the installed locales
    sinty-locale set-locale LANG=... [LC_TIME=...]     set the system locale (root)
    sinty-locale set-keymap MAP [TOGGLEMAP]            set the console keymap (root)
    sinty-locale set-x11-keymap LAYOUT [MODEL [VARIANT [OPTIONS]]]
                                                       set the desktop keyboard (root)
    sinty-locale --help

`status` prints the same fields, in the same layout, as `localectl`:

    $ sinty-locale
       System Locale: LANG=it_IT.UTF-8
                    : LC_TIME=en_US.UTF-8
           VC Keymap: it
          X11 Layout: it

`set-keymap` also sets the desktop layout to the same name, and
`set-x11-keymap` sets the console keymap to the first layout, as `localectl`
does. The console keymap is loaded right away with `loadkeys` when it is
available; the saved value is applied at the next boot in any case.

## Daemon

`sinty-localed` serves the same logic over D-Bus as `org.freedesktop.locale1`,
so the Region settings and any client of the systemd API work without
`systemd-localed`. It implements `SetLocale`, `SetVConsoleKeyboard` and
`SetX11Keyboard`, and the `Locale`, `VConsoleKeymap`, `VConsoleKeymapToggle`,
`X11Layout`, `X11Model`, `X11Variant` and `X11Options` properties.

Callers are authorized through polkit with the systemd action ids
(`org.freedesktop.locale1.set-locale` and
`org.freedesktop.locale1.set-keyboard`) when polkit is running, and by
membership of `wheel` (or `sudo`, `admin`) when it is not.

Packaging files (bus policy, activation file, unit) are in `packaging/`.

## Build

    make build            # ./sinty-locale and ./sinty-localed
    make static           # stripped static binaries for the image
    make test
    go build ./...        # plain build

Requires Go >= 1.25 and Linux. The only non-stdlib dependency is
`github.com/godbus/dbus/v5`, for the daemon.
