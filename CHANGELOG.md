# Changelog

## Unreleased

- Log setup failures and displayed errors from startup in a visible session log;
  include session logs and underlying helper failures in diagnostics.
- Launch the macOS DHCP helper without `nohup`, which could fail to detach from
  the terminal-free AppleScript administrator session.

- Use loopback for the macOS local TFTP check to avoid rejected routes to the
  station's own Ethernet alias; retain the selected server IP for APBoot.

- Bind checksum approval to the validated image and verify the staged TFTP bytes.
- Validate network/country/serial inputs and snapshot settings before conversion.
- Discover serial and Ethernet adapters; hash firmware off the UI thread and
  bound/batch live-log updates.
- Isolate DHCP to the selected adapter, filter requests, and acknowledge helper
  startup and shutdown for each run.
- Verify both distinct OS partitions and monitor later boot failures; report
  unconfirmed startup separately from success.
- Preserve APBoot prompts arriving with manufacturing output in the same read.
- Report recording failures and use unique transcript filenames.
- Stream Windows firmware staging and restrict bundled TFTP paths/concurrency.
- Add regression tests for conversion, staging, DHCP, serial, and TFTP failures.

## 0.1.0-dev

- Initial Go/Fyne GUI application
- macOS direct-connect station setup
- Serial/APBoot conversion engine
- Optional direct-connect DHCP helper
- CSV inventory and conversion results
- Debug dump and About dialogs
- Version metadata support
