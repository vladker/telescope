# Telescope

File encoding system via images for VDI/screen recording transfer scenarios.

## Overview

Telescope encodes files into sequences of images that can be captured via screen recording. The encoded images contain a calibration border for auto-detection of encoding parameters, allowing reliable decoding even from compressed video.

**Use case**: Transfer files when only screen recording/video capture is available (VDI environments, restricted networks, etc.)

## Quick Start

### Encode a file
```powershell
.\telescope-encode.exe -i myfile.zip -o frames
```

### Decode frames back to file
```powershell
.\telescope-decode.exe -i frames -o restored.zip
```

### Interactive mode
```powershell
.\telescope-encode.exe
.\telescope-decode.exe
```

## Installation

### Build from source
```bash
go build -o telescope-encode.exe ./cmd/encode
go build -o telescope-decode.exe ./cmd/decode
```

### Pre-built binaries
Download from releases or use the included `telescope-encode.exe` and `telescope-decode.exe`.

## Usage

### Encoder

```powershell
telescope-encode.exe -i <input_file> -o <output_dir> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-i` | required | Input file path |
| `-o` | required | Output directory for frames |
| `-W` | 1920 | Image width (100-3840) |
| `-H` | 1080 | Image height (100-2160) |
| `-p` | 2 | Pixel size (1, 2, or 3) |
| `-m` | robust | Mode: `robust` or `dense` |
| `-f` | png | Format: `png` or `jpeg` |

**Example:**
```powershell
telescope-encode.exe -i largefile.bin -o frames -W 1920 -H 1080 -p 2 -m robust
```

### Decoder

```powershell
telescope-decode.exe -i <input_dir> -o <output_file> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-i` | required | Input: directory with frames or video file |
| `-o` | required | Output file path |
| `-unique` | true | Extract only unique frames |
| `-force` | false | Skip CRC validation |

**Example:**
```powershell
telescope-decode.exe -i frames -o restored.bin
```

## Workflow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Source    │     │   Encode    │     │   Record    │     │   Decode    │
│   File      │────▶│   Frames    │────▶│   Video     │────▶│   File      │
└─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
```

1. **Encode**: `telescope-encode.exe -i file.zip -o frames/`
2. **Record**: Capture screen with OBS Studio (lossless codec recommended)
3. **Transfer**: Copy video to target machine
4. **Decode**: `telescope-decode.exe -i video.mp4 -o file.zip`

## Image Format

```
┌──────────────────────────────────────┐
│ ▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░ │  ← Calibration border
│ ░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░ │     (4 big pixels, chessboard)
├──────────────────────────────────────┤
│ [Meta Header: 64 bytes]              │  ← File metadata + CRC
│  - Signature: "TSCOPE01"             │
│  - Frame number, total frames        │
│  - Data size, CRC32                 │
├──────────────────────────────────────┤
│ [Encoded Data...]                    │  ← File content
└──────────────────────────────────────┘
```

## Modes

### Robust (8-bit) — Default
- 256 grayscale levels per pixel
- Better tolerance to compression artifacts
- Recommended for most use cases

### Dense (4-bit)
- 16 grayscale levels (0, 17, 34, ... 255)
- Higher data density
- Use when recording quality is excellent

## Pixel Size

| Size | Description | Use Case |
|------|-------------|----------|
| 1×1 | Maximum resolution | Best for small files, high-quality recording |
| 2×2 | Balanced | **Recommended** — good balance of size and robustness |
| 3×3 | Maximum robustness | Best for low-quality recording/compression |

## Resolution Guide

| Resolution | Pixel 2×2 Capacity | Notes |
|------------|---------------------|-------|
| 640×480 | ~80 KB/frame | Quick transfer, many frames |
| 100×100 | ~1 KB/frame | For tiny files only |
| 1920×1080 | ~500 KB/frame | HD, recommended |
| 3840×2160 | ~2 MB/frame | 4K, maximum efficiency |

## Troubleshooting

### CRC validation fails
- Enable `-force` flag to skip validation
- Try higher pixel size (`-p 3`)
- Use PNG format instead of JPEG

### Too many frames
- Use larger resolution
- Use smaller pixel size (`-p 1`)

### Video file decode fails
- Extract frames using ffmpeg: `ffmpeg -i video.mp4 frame_%04d.png`
- Decode the extracted frames directory

## Technical Details

- **Language**: Go 1.21+
- **Dependencies**: Standard library only (no external dependencies)
- **File limit**: ~4GB (limited by uint32)
- **Frame format**: PNG (lossless) or JPEG (lossy)
