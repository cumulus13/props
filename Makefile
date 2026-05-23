# props Makefile
# Author: Hadi Cahyadi <cumulus13@gmail.com>

BINARY   := props.exe
LDFLAGS  := -s -w -H windowsgui
GOOS     := windows
GOARCH   := amd64

.PHONY: all build clean install

all: build

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "$(LDFLAGS)" -o $(BINARY) .
	@echo "Built: $(BINARY)"

# Cross-compile from Linux/macOS
cross:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o props_amd64.exe .
	GOOS=windows GOARCH=386   go build -ldflags "$(LDFLAGS)" -o props_x86.exe   .
	@echo "Cross-compiled: props_amd64.exe  props_x86.exe"

clean:
	del /Q $(BINARY) 2>nul || rm -f $(BINARY) props_amd64.exe props_x86.exe

install: build
	copy $(BINARY) %USERPROFILE%\AppData\Local\Microsoft\WindowsApps\$(BINARY)
	@echo "Installed to PATH"
