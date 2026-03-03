.PHONY: build install clean

BINARY = recipe-lint
INSTALL_DIR = $(HOME)/.wk/plugins/recipe-lint

build:
	go build -o $(BINARY) .

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/
	cp plugin.toml $(INSTALL_DIR)/

clean:
	rm -f $(BINARY)
