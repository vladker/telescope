# Telescope

File encoding via images for VDI/screen recording transfer scenarios.

## Building

```bash
go mod tidy
go build -o telescope-encode.exe ./cmd/encode
go build -o telescope-decode.exe ./cmd/decode
```

## Quick Start (Interactive Mode)

```bash
# Run without arguments for interactive mode
telescope-encode.exe
telescope-decode.exe
```

## CLI Usage

### Encoding

```bash
telescope-encode.exe -i file.zip -o frames/ -W 1920 -H 1080 -p 2 -m robust
```

| Flag | Default | Description |
|------|---------|-------------|
| `-i` | required | Input file path |
| `-o` | required | Output directory |
| `-W` | 1920 | Image width |
| `-H` | 1080 | Image height |
| `-p` | 2 | Big pixel size (1, 2, 3) |
| `-m` | robust | Mode: dense or robust |
| `-f` | png | Format: png or jpeg |
| `-I` | false | Interactive mode |

### Decoding

```bash
telescope-decode.exe -i frames/ -o restored.zip
```

| Flag | Default | Description |
|------|---------|-------------|
| `-i` | required | Input directory or video file |
| `-o` | required | Output file path |
| `-video` | false | Input is video file |
| `-fps` | 1.0 | FPS for video extraction |
| `-unique` | true | Extract only unique frames |
| `-force` | false | Skip CRC validation |
| `-I` | false | Interactive mode |

## Interactive Mode

Run either tool without flags or with `-I`:

```bash
telescope-encode.exe -I
# Follow prompts for file, resolution, pixel size, etc.

telescope-decode.exe -I
# Follow prompts for input, output, CRC validation, etc.
```

## Modes

### Dense (4-bit)
- 16 grayscale levels per pixel
- Maximum data density
- Minimal redundancy

### Robust (8-bit)
- 256 grayscale levels
- Row duplication with inversion
- Better tolerance to compression artifacts

## Image Format

```
┌──────────────────────────────────────┐
│ ▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░ │  ← Calibration border
├──────────────────────────────────────┤
│ [Meta: 64 bytes]                    │  ← Header with CRC
├──────────────────────────────────────┤
│ [Data...]                            │
└──────────────────────────────────────┘
```

## Workflow

1. **Encode**: `telescope-encode.exe -I` → generates frames in directory
2. **Record**: OBS Studio captures screen to video
3. **Transfer**: Transfer video to target machine
4. **Decode**: `telescope-decode.exe -I` → reconstructs file
