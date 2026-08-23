.PHONY: all clean test vet fmt-check

SRCS = $(wildcard *.go)

BUILD = CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) go build

all: bin/shelly-bulk-update-Darwin-x86_64 bin/shelly-bulk-update-Darwin-arm64 bin/shelly-bulk-update-Linux-x86_64 bin/shelly-bulk-update-Linux-armv7 bin/shelly-bulk-update-Linux-arm64 bin/shelly-bulk-update-Windows-x86_64.exe

bin/shelly-bulk-update-Darwin-x86_64: GOOS := darwin
bin/shelly-bulk-update-Darwin-x86_64: GOARCH := amd64
bin/shelly-bulk-update-Darwin-x86_64: $(SRCS)
	$(BUILD) -o $@ .

bin/shelly-bulk-update-Darwin-arm64: GOOS := darwin
bin/shelly-bulk-update-Darwin-arm64: GOARCH := arm64
bin/shelly-bulk-update-Darwin-arm64: $(SRCS)
	$(BUILD) -o $@ .

bin/shelly-bulk-update-Linux-x86_64: GOOS := linux
bin/shelly-bulk-update-Linux-x86_64: GOARCH := amd64
bin/shelly-bulk-update-Linux-x86_64: $(SRCS)
	$(BUILD) -o $@ .

bin/shelly-bulk-update-Linux-armv7: GOOS := linux
bin/shelly-bulk-update-Linux-armv7: GOARCH := arm
bin/shelly-bulk-update-Linux-armv7: GOARM := 7
bin/shelly-bulk-update-Linux-armv7: $(SRCS)
	$(BUILD) -o $@ .

bin/shelly-bulk-update-Linux-arm64: GOOS := linux
bin/shelly-bulk-update-Linux-arm64: GOARCH := arm64
bin/shelly-bulk-update-Linux-arm64: $(SRCS)
	$(BUILD) -o $@ .

bin/shelly-bulk-update-Windows-x86_64.exe: GOOS := windows
bin/shelly-bulk-update-Windows-x86_64.exe: GOARCH := amd64
bin/shelly-bulk-update-Windows-x86_64.exe: $(SRCS)
	$(BUILD) -o $@ .

test:
	go test ./...

vet:
	go vet ./...

fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "files not gofmt'd:"; echo "$$out"; exit 1; fi

clean:
	rm -rf bin/
