# Telescope — Technical Specification

## Overview

Telescope encodes binary files into sequences of PNG/JPEG images for transfer via screen recording or VDI. Each image contains:
- A calibration border for auto-detection
- A 64-byte metadata header
- Encoded file data

## Constants

```go
const (
    Signature       = "TSCOPE01"  // 8-byte magic signature
    HeaderSize      = 64          // Fixed header size
    Version         = 1           // Protocol version
    BorderBigPixels = 4           // Border width in big pixels
    CalibrationBits = 0xFF        // White calibration value
)
```

## Image Layout

```
┌──────────────────────────────────────┐
│ ▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░ │  BorderBigPixels = 4
│ ░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░ │
│ ░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░ │
│ ░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░░▓▓░ │
├──────────────────────────────────────┤
│ [Meta Header: 64 bytes]              │  Rows 4-5 in inner area
├──────────────────────────────────────┤
│ [Encoded Data...]                    │  Remaining rows
└──────────────────────────────────────┘
```

## Calibration Border

- **Width**: `BorderBigPixels` big pixels on each side
- **Pattern**: Chessboard (alternating 0xFF and 0x00 per big pixel)
- **Purpose**: Auto-detect pixel size and validate frame

### Border Detection Algorithm

1. Iterate through possible pixel sizes (3, 2, 1) in descending order
2. Calculate expected border width in pixels: `borderPx = pixelSize * BorderBigPixels`
3. Check inner dimensions: must be ≥ 16 big pixels each way
4. Validate chessboard pattern at 4 corners and edges
5. Require ≥70% pattern match

## Pixel Size

| Size | Description | Formula |
|------|-------------|---------|
| 1×1 | Full resolution | 1 image pixel = 1 data byte |
| 2×2 | Half resolution | 2×2 image pixels = 1 data byte |
| 3×3 | Third resolution | 3×3 image pixels = 1 data byte |

## Big Pixel Structure

Big pixels are averaged for value extraction:
```go
func getBigPixelGray(img, bigX, bigY, size, borderPx) uint8 {
    // Average all pixels in the big pixel region
    sum := 0
    for y := borderPx+bigY*size; y < borderPx+(bigY+1)*size; y++ {
        for x := borderPx+bigX*size; x < borderPx+(bigX+1)*size; x++ {
            sum += pixelGray(x, y)
        }
    }
    return uint8(sum / (size * size))
}
```

## Modes

### Robust Mode (8-bit)

- **Data encoding**: Full 8-bit grayscale (0-255)
- **CRC**: IEEE 802.3 CRC32 per frame
- **Redundancy**: None (uses all rows for data)

### Dense Mode (4-bit)

- **Data encoding**: 4-bit nibbles mapped to palette
- **Palette**: `value = nibble * 17` (0, 17, 34, 51, ..., 255)
- **Encoding**: Two pixels per byte (high nibble, low nibble)
- **CRC**: IEEE 802.3 CRC32 per frame

## Metadata Header (64 bytes)

```
Offset  Size  Type     Field
─────────────────────────────────
0       8     [8]byte Signature ("TSCOPE01")
8       4     uint32   Version (= 1)
12      4     uint32   FileSize (total original file size)
16      4     uint32   FrameNum (0-indexed)
20      4     uint32   TotalFrames
24      4     uint32   DataSize (bytes in this frame)
28      4     uint32   CRC32 (IEEE, of frame data only)
32      32    [32]byte Reserved
```

All multi-byte values are little-endian.

## FrameInfo Calculation

```go
func CalcFrameInfo(width, height int, pixelSize PixelSize, mode Mode) FrameInfo {
    borderPx := pixelSize * BorderBigPixels
    innerW := width - 2*borderPx
    innerH := height - 2*borderPx

    bigPixelsW := innerW / pixelSize
    bigPixelsH := innerH / pixelSize

    // Data area: exclude border on all sides
    dataCols := bigPixelsW - 2*BorderBigPixels
    dataRows := bigPixelsH - 2*BorderBigPixels
    dataBigPixels := dataCols * dataRows

    return FrameInfo{
        Width:         width,
        Height:        height,
        PixelSize:     pixelSize,
        Mode:          mode,
        BigPixelsW:    bigPixelsW,
        BigPixelsH:    bigPixelsH,
        BorderW:       borderPx,
        DataBigPixels: dataBigPixels,
    }
}
```

## Encoding Process

1. Read file into buffer
2. Calculate `dataPerFrame` from FrameInfo
3. For each frame:
   - Create image with specified dimensions
   - Draw calibration border
   - Draw metadata header in rows 4-5 of inner area
   - Draw data starting from row 8 of inner area
   - Set CRC32 of data in header
4. Save as PNG or JPEG

## Decoding Process

1. Load image file(s)
2. Detect pixel size from border
3. Read metadata header (rows 4-5)
4. Parse header, validate signature and CRC
5. Read frame data based on `DataSize` field
6. Concatenate frames in order
7. Write to output file

## Data Capacity Examples

| Resolution | Pixel | Mode | DataBigPixels | Bytes/Frame |
|------------|-------|------|--------------|-------------|
| 100×100 | 2×2 | Robust | 1156 | 1156 |
| 200×200 | 2×2 | Robust | 4624 | 4624 |
| 1920×1080 | 2×2 | Robust | 494,656 | 494 KB |
| 3840×2160 | 2×2 | Robust | ~2 MB | ~2 MB |

## Mode Detection

Decoder auto-detects mode from meta header sample:
1. Read pixel at (row=4, col=4)
2. Check if value suggests Dense palette (multiple of 17)
3. Default to Robust mode

## Error Handling

- **No border found**: Return `ErrNoBorderFound`
- **Invalid signature**: Return `ErrInvalidSignature`
- **Invalid version**: Return `ErrInvalidVersion`
- **Invalid header size**: Return `ErrInvalidHeader`
- **CRC mismatch**: Return `ErrCRCFailed` (can be skipped with `-force`)

## File Structure

### Encoder Output
```
output_dir/
├── frame_0000.png
├── frame_0001.png
├── frame_0002.png
└── manifest.json
```

### manifest.json
```json
{
    "version": 1,
    "total_size": 1234567,
    "total_frames": 3,
    "width": 1920,
    "height": 1080,
    "pixel_size": 2,
    "mode": "robust"
}
```

## Limitations

- Maximum file size: ~4GB (limited by uint32 FileSize field)
- Minimum image size: 20×20 pixels (for 2×2 pixel size)
- All frames must use same encoding parameters
- JPEG compression may cause decode errors (use PNG for reliability)

## Dependencies

- Go 1.21+
- Standard library only (`image`, `image/png`, `image/jpeg`, `hash/crc32`)
