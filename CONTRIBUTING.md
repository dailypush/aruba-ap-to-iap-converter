# Contributing

Thanks for helping improve Aruba AP to IAP Converter.

## Development

```bash
go test ./...
go run ./cmd/iap325-converter
```

Please keep changes focused and avoid committing runtime logs, firmware images,
or generated debug dumps.

## Pull Requests

Good pull requests usually include:

- A short description of the workflow or bug being improved
- Tests for parser, firmware, inventory, or conversion logic where practical
- Notes about any manual AP conversion testing performed
- Screenshots for GUI layout changes when useful

## Firmware And Hardware

Do not attach firmware images to issues or pull requests. Firmware must remain
user-supplied and should be verified against official Aruba checksums.
