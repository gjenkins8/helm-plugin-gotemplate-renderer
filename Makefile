.DEFAULT: build
.PHONY: build test vet

PKG_SOURCE_FILES=$(shell go list -f '{{ $$dir := .Dir }}{{ range .GoFiles }}{{ printf "%s/%s " $$dir . }}{{ end }}' ./...)

gotemplate-renderer.wasm: $(PKG_SOURCE_FILES)
	GOOS=wasip1 GOARCH=wasm go1.24rc2 build -buildmode=c-shared -o . .
	mv helm-plugin-gotemplate-renderer gotemplate-renderer.wasm

build: gotemplate-renderer.wasm

test: gotemplate-renderer.wasm
	go test ./testdriver

vet:
	GOOS=wasip1 GOARCH=wasm go vet ./...
