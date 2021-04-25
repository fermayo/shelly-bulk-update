# shelly-bulk-update

Automatically updates the firmware of all your Shelly devices at once.


## Installation

Download the binary for your platform:

### macOS

```bash
curl -sSL https://github.com/fermayo/shelly-bulk-update/releases/download/v1.0/shelly-bulk-update-Darwin-x86_64 -o shelly-bulk-update; chmod +x shelly-bulk-update
```

### Linux

```bash
curl -sSL https://github.com/fermayo/shelly-bulk-update/releases/download/v1.0/shelly-bulk-update-Linux-x86_64 -o shelly-bulk-update; chmod +x shelly-bulk-update
```

### Windows

[Click here](https://github.com/fermayo/shelly-bulk-update/releases/download/v1.0/shelly-bulk-update-Windows-x86_64.exe) to download the binary

## Usage

Ensure you are on the same network as your Shelly devices. Then run the binary:

```bash
./shelly-bulk-update
```

It will automatically discover all your Shelly devices using mDNS and attempt to update them if possible.
While updates are in progress and devices are restarting, you might see connection errors. Sometimes it takes a few minutes, please be patient :-)


## TODO

* Better UI
* Support authentication
