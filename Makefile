PLUGIN_NAME := selective-router
OUT_DIR := bin
EXT := so

.PHONY: build test clean

build:
	mkdir -p $(OUT_DIR)
	go build -buildvcs=false -buildmode=c-shared -o $(OUT_DIR)/$(PLUGIN_NAME).$(EXT) ./cmd/selective-model-router

test:
	go test ./...

clean:
	rm -rf $(OUT_DIR)
