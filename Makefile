.PHONY: all

all: bin/shelly-bulk-upgrade-Darwin-x86_64 bin/shelly-bulk-upgrade-Linux-x86_64 bin/shelly-bulk-upgrade-Windows-x86_64

bin/shelly-bulk-upgrade-Darwin-x86_64: main.go
	GOOS=darwin GOARCH=amd64 go build -o $@ .

bin/shelly-bulk-upgrade-Linux-x86_64: main.go
	GOOS=linux GOARCH=amd64 go build -o $@ .

bin/shelly-bulk-upgrade-Windows-x86_64: main.go
	GOOS=windows GOARCH=amd64 go build -o $@ .
