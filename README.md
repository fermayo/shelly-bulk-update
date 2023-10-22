# shelly-bulk-update

Automatically updates the firmware of all your [Shelly](https://shelly.cloud/) devices at once.


## Installation

Download the latest binary for your platform following the instructions on the [Releases](https://github.com/fermayo/shelly-bulk-update/releases) page.


## Usage

Ensure you are on the same network as your Shelly devices. Then run the binary:

```bash
./shelly-bulk-update
```

It will automatically discover all your Shelly devices using mDNS and attempt to update them to the latest stable
version if possible.

Please note:
* The initial discovery can take up to 1 minute.
* While updates are in progress and devices are restarting, you might see connection errors. Sometimes it takes a few
  minutes, please be patient :-)

If any (or all) of your devices have authentication enabled, use the `-username` and `-password` flags to define your
credentials:

```bash
./shelly-bulk-update -username admin -password MyPa$$w0rd
```

To update your Shelly devices to the latest beta version, use `-stage=beta`.

If you only want to update all Shelly devices of a specific device generation, use either `-gen=1` for
[generation 1](https://shelly-api-docs.shelly.cloud/gen1/#shelly-family-overview) or `-gen=2` for
[generation 2](https://shelly-api-docs.shelly.cloud/gen2/). For example, this can be used to update all second
generation devices to the latest beta version but keep first generation devices on the stable track.
