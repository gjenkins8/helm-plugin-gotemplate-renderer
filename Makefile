.DEFAULT: build
.PHONY: build test vet

PKG_SOURCE_FILES=$(shell go list -f '{{ $$dir := .Dir }}{{ range .GoFiles }}{{ printf "%s/%s " $$dir . }}{{ end }}' ./...)

gotemplate-renderer.wasm: $(PKG_SOURCE_FILES)
	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o . .
	mv helm-plugin-gotemplate-renderer gotemplate-renderer.wasm

build: gotemplate-renderer.wasm

TEST_FLAGS?= # -cpuprofile cpu.prof -memprofile mem.prof # -bench

test: gotemplate-renderer.wasm
	go test $(TEST_FLAGS) ./testdriver/

vet:
	GOOS=wasip1 GOARCH=wasm go vet ./...
