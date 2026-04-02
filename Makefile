INSTALL_DIR := $(HOME)/.local/bin
BINARY      := gl1tch-mud

.PHONY: build install clean

build:
	go build -o $(BINARY) .

install: build
	install -m 0755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	rm $(BINARY)

clean:
	rm -f $(BINARY)
