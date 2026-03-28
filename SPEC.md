# Telescope — File Encoding via Images

## Overview

Telescope encodes files into sequences of images (or video frames) and decodes them back. Designed for transferring files through VDI/screen recording when direct network access is unavailable.

## Modes

### Dense Mode (4-bit)
- **Bits per pixel**: 4 (16 grayscale levels)
- **Redundancy**: CRC32 per frame only
- **Use case**: Maximum data density, clean recording conditions

### Robust Mode (8-bit with duplication)
- **Bits per pixel**: 8 (256 grayscale levels)
- **Redundancy**: 
  - CRC32 per frame
  - Row duplication with inversion
- **Use case**: Better tolerance to compression artifacts, noise

## Image Format Specification

### Layout (top to bottom)
```
┌──────────────────────────────────────┐
│ Calibration border (8 big pixels)    │ ← Always 8×8 big-pixels, chessboard
├──────────────────────────────────────┤
│ Meta header (8 big pixels × 8 rows)  │ ← 64 bytes fixed format
├──────────────────────────────────────┤
│ Data area                            │ ← Variable, depends on image size
└──────────────────────────────────────┘
```

### Calibration Border
- 8 "big pixels" wide on each side
- Chessboard pattern: alternating 0x00 and 0xFF for big pixels
- **Purpose**: Auto-detect big-pixel size (1×1, 2×2, 3×3)

### Meta Header (64 bytes, fixed)
```
Offset  Size  Field
0       8     Signature: "TSCOPE01"
8       4     Version (uint32, LE) = 1
12      4     Total file size (uint32, LE)
16      4     Frame number (uint32, LE), 0-indexed
20      4     Total frames (uint32, LE)
24      4     Data size in this frame (uint32, LE)
28      4     CRC32 of frame data (IEEE 802.3)
32      32    Reserved (zeros)
```

### Big Pixel Structure
- **1×1**: 1 image pixel = 1 data pixel
- **2×2**: 2×2 image pixels = 1 data pixel
- **3×3**: 3×3 image pixels = 1 data pixel

Big pixel value encoding:
- **Dense (4-bit)**: Lower 4 bits of 8-bit grayscale
  - Valid values: 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF
- **Robust (8-bit)**: Full 8-bit grayscale

### Robust Mode: Row Duplication
Each row is encoded twice:
1. Original row
2. Inverted row (255 - value)

This allows detection and correction of single-row errors.

## Encoder CLI

```bash
telescope encode [flags]
```

### Flags
| Flag | Default | Description |
|------|---------|-------------|
| `-i, --input` | required | Input file path |
| `-o, --output` | required | Output directory for frames |
| `-W, --width` | 1920 | Image width in pixels |
| `-H, --height` | 1080 | Image height in pixels |
| `-p, --pixel` | 2 | Big pixel size (1, 2, or 3) |
| `-m, --mode` | robust | Encoding mode: dense or robust |
| `-f, --format` | png | Output format: png or jpeg |
| `-q, --quality` | 95 | JPEG quality (1-100) |

### Output
- `frame_0000.png`, `frame_0001.png`, ...
- `manifest.json` — metadata for decoder

## Decoder CLI

```bash
telescope decode [flags]
```

### Flags
| Flag | Default | Description |
|------|---------|-------------|
| `-i, --input` | required | Input: directory with images or video file |
| `-o, --output` | required | Output file path |
| `--video` | false | Input is video file (requires ffmpeg) |
| `--fps` | 1 | FPS for video frame extraction |
| `--unique` | true | Extract only unique frames (hash-based) |
| `--force` | false | Skip CRC validation |

### Workflow for Video
1. User records screen with OBS (lossless codec preferred)
2. Decoder uses ffmpeg to extract frames
3. Frames are hashed to find unique ones
4. Each frame is scanned for calibration border
5. Data is extracted and reassembled
6. File is validated and saved

## Detection Algorithm

1. **Find border**: Scan for chessboard pattern in border region
2. **Determine pixel size**: Count alternating pixels to calculate big-pixel size
3. **Extract meta**: Read 64-byte header
4. **Validate**: Check signature, CRC, frame sequence
5. **Reassemble**: Sort frames, concat data, restore file

## File Format Support

- Any binary file
- Maximum file size: ~4GB (limited by uint32 addressing)
- Large files are split across multiple frames

## Technical Details

- Language: Go 1.21+
- Dependencies: 
  - `image` (stdlib) — image processing
  - `image/png`, `image/jpeg` (stdlib) — format encoding
  - `github.com/veandco/go-sdl2/sdl` — optional preview (future)
- No external dependencies for core functionality

## Data Flow

### Encoding
```
Input File → Chunking → Per-frame:
  [Calibration][Meta Header][Data + Padding]
  → Render to Image → Save
```

### Decoding
```
Images/Frames → Border Detection → Pixel Size Detection
  → Meta Parsing → CRC Validation → Data Extraction
  → Concatenation → Output File
```
