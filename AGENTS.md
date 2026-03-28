# Telescope - Agent Guidelines

## Project Overview

Telescope is a Go file encoding system that transfers files via images for VDI/screen recording. Files are encoded into PNG/JPEG images with a calibration border, then decoded from recorded video/screenshots.

## Build Commands

### Standard Go Commands
```bash
# Build binaries
go build -o telescope-encode.exe ./cmd/encode
go build -o telescope-decode.exe ./cmd/decode

# Run all tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Run a single test
go test ./internal/codec/... -run TestEncodeDecodeRoundTrip/Large_data_multi-frame -v

# Run specific test file
go test ./internal/codec/... -run TestEncodeDecodeRoundTrip -v

# Tidy dependencies
go mod tidy

# Vet code
go vet ./...
```

### Quick Build Script
```bash
go build -o telescope-encode-new.exe ./cmd/encode && \
go build -o telescope-decode-new.exe ./cmd/decode && \
mv telescope-encode-new.exe telescope-encode.exe && \
mv telescope-decode-new.exe telescope-decode.exe
```

## Code Style Guidelines

### General
- Use `gofmt` for formatting (standard Go style)
- No line length limit (follow Go idioms)
- Use `pkg/errors` or standard errors with wrapping

### Imports
```go
import (
    "encoding/binary"
    "fmt"
    "hash/crc32"
    "image"
    "image/png"
    "os"
    "path/filepath"

    "telescope/internal/format"
)
```
- Standard library first, then third-party, then internal
- Group by blank line
- Use short aliases only when necessary

### Naming Conventions
- **Types**: PascalCase (`FrameInfo`, `PixelSize`, `DensePalette`)
- **Constants**: PascalCase for exported, camelCase for unexported
- **Variables/Functions**: camelCase for unexported, PascalCase for exported
- **Files**: lowercase with underscores (`encoder.go`, `encoder_test.go`)
- **Test files**: `*_test.go` suffix

### Types
```go
// Enumerations via const + type
type Mode uint8

const (
    ModeDenseValue  Mode = 0
    ModeRobustValue Mode = 1
)

// Structs with clear field names
type Header struct {
    Signature   [8]byte
    Version     uint32
    FileSize    uint32
    FrameNum    uint32
}

// Option functions for constructors
type EncoderOption func(*Encoder)

func WithPixelSize(ps PixelSize) EncoderOption {
    return func(e *Encoder) {
        e.pixelSize = ps
    }
}
```

### Error Handling
```go
// Wrap errors with context
return nil, fmt.Errorf("failed to detect frame info: %w", err)

// Use sentinel errors for expected conditions
var ErrNoBorderFound = errors.New("no border found")

// Check errors explicitly
if err != nil {
    return fmt.Errorf("encoding failed: %w", err)
}
```

### Logging
- Use logger functions passed as options
- Structured logging via `func(string)` callback
- Log levels via prefixes: `[LOG]`, `[WARN]`, `[ERROR]`

```go
type EncodeLogger func(string)

func WithLogger(logger func(string)) EncoderOption {
    return func(e *Encoder) {
        e.logger = logger
    }
}

func (e *Encoder) log(format string, args ...interface{}) {
    if e.logger != nil {
        e.logger(fmt.Sprintf(format, args...))
    }
}
```

### Project Structure
```
telescope/
├── cmd/
│   ├── encode/main.go    # Encoder CLI entry point
│   └── decode/main.go    # Decoder CLI entry point
├── internal/
│   ├── codec/           # Core encoding/decoding logic
│   │   ├── encoder.go
│   │   ├── decoder.go
│   │   └── encoder_test.go
│   ├── format/          # Data structures and constants
│   │   ├── spec.go
│   │   └── errors.go
│   └── detector/        # Frame detection/scanning
│       └── scanner.go
└── go.mod
```

### FrameInfo Calculation (Critical)
When modifying encoder/decoder, ensure FrameInfo calculations match:

```go
func CalcFrameInfo(width, height int, pixelSize PixelSize, mode Mode) FrameInfo {
    borderPx := int(pixelSize) * BorderBigPixels
    innerW := width - 2*borderPx
    innerH := height - 2*borderPx

    bigPixelsW := innerW / int(pixelSize)
    bigPixelsH := innerH / int(pixelSize)

    // Data area excludes all borders
    dataCols := bigPixelsW - 2*BorderBigPixels
    dataRows := bigPixelsH - 2*BorderBigPixels
    dataBigPixels := dataCols * dataRows

    return FrameInfo{
        DataBigPixels: dataBigPixels,
        // ...
    }
}
```

### Testing Guidelines
```go
func TestEncodeDecodeRoundTrip(t *testing.T) {
    tests := []struct {
        name      string
        width     int
        height    int
        pixelSize format.PixelSize
        mode      format.Mode
        data      []byte
    }{
        {"100x100 1x1 robust", 100, 100, format.Pixel1x1, format.ModeRobustValue, []byte("test")},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            encoder := NewEncoder(tt.width, tt.height, WithPixelSize(tt.pixelSize), WithMode(tt.mode))
            img, err := encoder.EncodeChunk(tt.data, 0, 1)
            // ...
        })
    }
}
```

### Key Constants
```go
const (
    Signature       = "TSCOPE01"  // 8-byte magic signature
    HeaderSize      = 64          // Fixed header size in bytes
    Version         = uint32(1)   // Protocol version
    BorderBigPixels = 4           // Border width in big pixels
    CalibrationBits = 0xFF       // White calibration value
)
```

### Image Processing Notes
- Big pixel = group of `pixelSize × pixelSize` image pixels
- Border is `BorderBigPixels` big pixels wide on each side
- Data encoding: Robust = 1 byte per big pixel, Dense = 4 bits per big pixel
- Use `image.Gray` for grayscale encoding
