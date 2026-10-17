BIN := sinty-locale
DAEMON := sinty-localed
PREFIX ?= /usr

.PHONY: all build static test vet install clean

all: build

build:
	go build -o $(BIN) ./cmd/sinty-locale
	go build -o $(DAEMON) ./cmd/sinty-localed

static:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $(BIN) ./cmd/sinty-locale
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $(DAEMON) ./cmd/sinty-localed

test:
	go test ./...

vet:
	go vet ./...

install: static
	install -Dm755 $(BIN) $(DESTDIR)$(PREFIX)/bin/$(BIN)
	install -Dm755 $(DAEMON) $(DESTDIR)$(PREFIX)/libexec/$(DAEMON)
	install -Dm644 packaging/org.freedesktop.locale1.conf $(DESTDIR)$(PREFIX)/share/dbus-1/system.d/org.freedesktop.locale1.conf
	install -Dm644 packaging/org.freedesktop.locale1.service $(DESTDIR)$(PREFIX)/share/dbus-1/system-services/org.freedesktop.locale1.service
	install -Dm644 packaging/sinty-localed.service $(DESTDIR)$(PREFIX)/lib/systemd/system/sinty-localed.service

clean:
	rm -f $(BIN) $(DAEMON)
