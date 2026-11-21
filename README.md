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
- Background firmware hashing and checksum approval tied to the exact image
- Editable serial/interface dropdowns with adapter discovery
- Input validation and fixed settings throughout each conversion
- macOS direct-connect setup helpers using administrator prompts
- Windows direct-connect setup helpers using `netsh` and bundled TFTP serving
- Optional DHCP helper isolated to the selected Ethernet interface, with startup
  acknowledgement and automatic shutdown after each run
- Serial/APBoot conversion engine
- AP manufacturing info detection from console output
- Country-code inventory programming
- Dual-partition flashing and verification of each distinct partition
- Full TFTP download/checksum verification before AP changes
- Factory reset and first-boot verification
- Fallback IP decoding and WebUI probing
- CSV-compatible conversion and inventory logging
- Bounded, batched live-log rendering with complete transcripts on disk
- Visible warnings for transcript or inventory recording failures
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

The optional Windows DHCP helper requires Windows 10 version 1803 or newer
for interface isolation using [Winsock interface filtering](https://learn.microsoft.com/en-us/windows/win32/winsock/ipproto-ip-socket-options).
It fails to start if the operating system cannot apply the interface restriction.

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
5. Compare the displayed SHA256 with the official checksum and confirm it.
   Changing or reselecting firmware clears approval. The bytes are checked again
   before station setup and after downloading the staged image through TFTP.
6. Click `Convert AP`.
7. Power-cycle the AP when prompted.
8. Watch progress, serial logs, CSV inventory, and final WebUI result.

Use **Refresh adapters** after connecting a USB serial adapter or Ethernet
adapter. The dropdowns remain editable for devices that discovery cannot find.
Settings are locked during conversion, and closing the window is blocked until
the run finishes. Each run uses one snapshot of the settings.

Station checks reject invalid country-code formatting, invalid IPv4 addresses,
noncontiguous masks, identical AP/server addresses, and mismatched subnets before
changing station settings. Country formatting validation does not verify the
AP's regulatory domain; select the appropriate country for your hardware.

The optional DHCP helper serves one device on the selected interface, ignores
requests intended for another DHCP server/address, and stops after the run. A
failed helper startup stops preparation before the AP is modified. The TFTP
check verifies a complete local download (through `127.0.0.1` on macOS to
avoid routing through the Mac's own Ethernet alias); APBoot ping and the actual transfer
still verify the AP-facing connection.

`SUCCESS` requires matching versions in partitions 0 and 1, the target boot
version, a startup-ready/login marker, and a short subsequent observation with
no panic or serial error. `UNVERIFIED` means firmware was flashed but later
startup confirmation was unavailable. Neither status replaces checking radio
operation and the WebUI. A recording warning means the conversion outcome is
known but the CSV or transcript could not be fully saved; preserve the displayed
serial and result before continuing.

## Logs And CSV Files

Runtime files are written under `logs/` in the directory from which you launch
the app. For example, launching from your home directory writes to `~/logs`.
The exact session-log path appears above the live activity log. Logs remain
local; the app does not upload error reports automatically.

Files include:

- `app-session-YYYYMMDD-HHMMSS-<unique-id>.log`: startup, station setup,
  displayed errors, and live conversion output, starting before serial opens
- `auto-convert-YYYYMMDD-HHMMSS-<unique-id>.log`
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
conversion and session log tails. Setup failures display the session-log path;
DHCP startup failures also include the newly written helper error output.

## License

MIT. See [LICENSE](LICENSE).

## Validation

Run `go test -race ./...` and `go vet ./...` before shipping changes. Tests include
simulated serial failures, boot/partition safeguards, changed firmware, DHCP
request filtering and lifecycle acknowledgement, and loopback TFTP transfers.
The TFTP tests need permission to bind an ephemeral UDP socket on localhost.
They do not contact or flash an AP. Real macOS/Windows adapter setup, firewall
behavior, and AP conversion still require a hardware smoke test.
