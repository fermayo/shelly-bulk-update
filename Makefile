.PHONY: all

all: bin/shelly-bulk-update-Darwin-x86_64 bin/shelly-bulk-update-Linux-x86_64 bin/shelly-bulk-update-Windows-x86_64.exe

bin/shelly-bulk-update-Darwin-x86_64: main.go
	GOOS=darwin GOARCH=amd64 go build -o $@ .

bin/shelly-bulk-update-Linux-x86_64: main.go
	GOOS=linux GOARCH=amd64 go build -o $@ .

bin/shelly-bulk-update-Windows-x86_64.exe: main.go
	GOOS=windows GOARCH=amd64 go build -o $@ .
