# Aruba AP to IAP Converter

Aruba AP to IAP Converter is a Go/Fyne desktop app for converting 256 MB
Aruba AP-325 Campus access points to Aruba Instant AP-325 using a direct-connect
serial, APBoot, and TFTP workflow.

The app is intended for a single conversion station: one computer, one serial
adapter, one direct Ethernet connection, and one AP at a time.

Subtitle: AP-325 Campus-to-Instant conversion station.

## Features

- Native desktop GUI built with Fyne
- Firmware filename/platform validation for Aruba Instant Hercules 6.x images
- SHA256 display and operator checksum confirmation
- macOS direct-connect setup helpers using administrator prompts
- Windows direct-connect setup helpers using `netsh` and bundled TFTP serving
- Optional direct-connect DHCP helper
- Serial/APBoot conversion engine
- AP manufacturing info detection from console output
- Country-code inventory programming
- Dual-partition flashing and `osinfo` verification
- Factory reset and first-boot verification
- Fallback IP decoding and WebUI probing
- CSV-compatible conversion and inventory logging
- Debug dump generation for troubleshooting

## Important Safety Notes

- Firmware is not bundled. You must provide your own Aruba Instant Hercules
  image and verify it against official Aruba checksums.
- The app rejects ArubaOS 8 Hercules images because converted 256 MB AP-325
  units require compatible Instant 6.x firmware.
- Use this only on hardware you own or are authorized to service.
- Conversion can change device inventory fields and firmware partitions.

Aruba and related product names are trademarks of their respective owners. This
project is not affiliated with, endorsed by, or sponsored by Aruba or HPE.

## Requirements

- Go 1.22 or newer
- macOS or Windows for automated station setup in the current release
- Serial adapter connected to the AP console
- Direct Ethernet cable to the AP
- TFTP-capable firmware staging:
  - macOS: `/private/tftpboot/aruba`
  - Windows: local `tftpboot/aruba` folder served by the bundled helper
- An Aruba Instant Hercules 6.x firmware image, for example:
  `ArubaInstant_Hercules_6.5.4.3_61959`

Linux platform interfaces exist as stubs for future support.

## Windows Notes

- Run the app as Administrator when using automatic setup. Windows needs
  elevated rights to add the direct-connect IP address and bind UDP/69 for
  TFTP. The optional DHCP helper also needs elevated rights for UDP/67.
- Defaults are `COM3` for serial and `Ethernet` for the direct-connect
  interface. Change these in the GUI if your adapter uses a different COM port
  or Windows network-interface name.
- The app stages firmware under `tftpboot/aruba` beside the executable/project
  and starts a read-only bundled TFTP helper for APBoot.

## Build And Run

```bash
git clone https://github.com/dailypush/aruba-ap-to-iap-converter.git
cd aruba-ap-to-iap-converter
go test ./...
go run ./cmd/iap325-converter
```

Build a local binary:

```bash
go build -o ./dist/aruba-ap-to-iap-converter ./cmd/iap325-converter
```

Build with version metadata:

```bash
./scripts/build-release.sh 0.1.0
```

The built binary will be written to `dist/`.

## Publishing

GitHub Actions publishes release artifacts when a version tag is pushed.

Create a release candidate:

```bash
git tag v0.1.0-rc.1
git push origin v0.1.0-rc.1
```

Create a stable release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release workflow builds zipped macOS Apple Silicon, macOS Intel, and Windows
binaries, stamps version metadata into the About dialog, and attaches the
packages to the GitHub Release. Tags containing `-rc.`, `-alpha.`, or `-beta.`
are published as GitHub prereleases.

## Basic Workflow

1. Connect serial console to the AP.
2. Connect the computer directly to the AP Ethernet port.
3. Launch the app.
4. Select a valid Aruba Instant Hercules 6.x firmware image.
5. Confirm the SHA256 checksum.
6. Click `Convert AP`.
7. Power-cycle the AP when prompted.
8. Watch progress, serial logs, CSV inventory, and final WebUI result.

## Logs And CSV Files

Runtime files are written under `logs/`:

- `auto-convert-YYYYMMDD-HHMMSS.log`
- `conversion-results.csv`
- `ap-inventory.csv`
- `debug-dump-YYYYMMDD-HHMMSS.txt`
- `dhcp-helper.log`
- `tftp-helper.log`

These files are ignored by git.

## Debugging

Use `Dump Debug Info` in the GUI or `Tools -> Dump Debug Info` to generate a
single debug report. The report includes current UI settings, interface status,
serial-port ownership, TFTP listener checks, ARP table, CSV tails, and recent
conversion log tails.

## License

MIT. See [LICENSE](LICENSE).
