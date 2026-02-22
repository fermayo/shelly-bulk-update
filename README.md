# shelly-bulk-update

Automatically updates the firmware of all your [Shelly](https://shelly.cloud/) devices at once.

Supports Gen1, Gen2, and Gen3 devices. Progress is shown in a live terminal UI that updates in real time as devices are discovered and updated.


## Installation

Download the latest binary for your platform following the instructions on the [Releases](https://github.com/fermayo/shelly-bulk-update/releases) page.


## Usage

Ensure you are on the same network as your Shelly devices. Then run the binary:

```bash
./shelly-bulk-update
```

It will automatically discover all your Shelly devices using mDNS and attempt to update them to the latest stable version if possible.

Please note:
* The initial discovery can take up to 1 minute.
* While updates are in progress and devices are restarting, you might see connection errors. Sometimes it takes a few minutes, please be patient :-)

### Authentication

If any (or all) of your devices have authentication enabled, use the `-password` flag:

```bash
./shelly-bulk-update -password MyPa$$w0rd
```

For Gen1 devices you can also specify a username (default: `admin`):

```bash
./shelly-bulk-update -username admin -password MyPa$$w0rd
```

### Firmware channel

To update to the latest beta firmware instead of stable, use `-stage=beta`:

```bash
./shelly-bulk-update -stage=beta
```

### Device generation filter

To target only a specific device generation, use the `-gen` flag:

| Flag | Targets |
|------|---------|
| `-gen=1` | [Gen1](https://shelly-api-docs.shelly.cloud/gen1/#shelly-family-overview) devices only |
| `-gen=2` | [Gen2](https://shelly-api-docs.shelly.cloud/gen2/) devices only |
| `-gen=3` | Gen3 devices only |
| *(omitted)* | All generations |

For example, to update all Gen2 devices to the latest beta while keeping Gen1 devices on stable:

```bash
./shelly-bulk-update -gen=2 -stage=beta
```
