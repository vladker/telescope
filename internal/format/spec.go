package format

import (
	"encoding/binary"
	"hash/crc32"
	"strings"
)

const (
	Signature       = "TSCOPE01"
	HeaderSize      = 64
	Version         = uint32(1)
	BorderBigPixels = 12
	CalibrationBits = 0xFF
)

const (
	ModeDense  = "dense"
	ModeRobust = "robust"
)

type Mode uint8

const (
	ModeDenseValue  Mode = 0
	ModeRobustValue Mode = 1
)

type PixelSize uint8

const (
	Pixel1x1 PixelSize = 1
	Pixel2x2 PixelSize = 2
	Pixel3x3 PixelSize = 3
)

type Header struct {
	Signature   [8]byte
	Version     uint32
	FileSize    uint32
	FrameNum    uint32
	TotalFrames uint32
	DataSize    uint32
	CRC         uint32
	Reserved    [32]byte
}

func NewHeader(fileSize, frameNum, totalFrames, dataSize uint32) *Header {
	h := &Header{
		Version:     Version,
		FileSize:    fileSize,
		FrameNum:    frameNum,
		TotalFrames: totalFrames,
		DataSize:    dataSize,
	}
	copy(h.Signature[:], Signature)
	for i := range h.Reserved {
		h.Reserved[i] = 0
	}
	return h
}

func (h *Header) SetFilename(filename string) {
	if len(filename) > 31 {
		filename = filename[:31]
	}
	copy(h.Reserved[:], filename)
}

func (h *Header) GetFilename() string {
	filename := string(h.Reserved[:])
	if idx := strings.Index(filename, "\x00"); idx >= 0 {
		filename = filename[:idx]
	}
	return strings.TrimSpace(filename)
}

func (h *Header) Serialize() []byte {
	data := make([]byte, HeaderSize)
	copy(data[0:8], h.Signature[:])
	binary.LittleEndian.PutUint32(data[8:12], h.Version)
	binary.LittleEndian.PutUint32(data[12:16], h.FileSize)
	binary.LittleEndian.PutUint32(data[16:20], h.FrameNum)
	binary.LittleEndian.PutUint32(data[20:24], h.TotalFrames)
	binary.LittleEndian.PutUint32(data[24:28], h.DataSize)
	binary.LittleEndian.PutUint32(data[28:32], h.CRC)
	copy(data[32:64], h.Reserved[:])
	return data
}

func ParseHeader(data []byte) (*Header, error) {
	if len(data) < HeaderSize {
		return nil, ErrInvalidHeader
	}
	h := &Header{}
	copy(h.Signature[:], data[0:8])
	h.Version = binary.LittleEndian.Uint32(data[8:12])
	h.FileSize = binary.LittleEndian.Uint32(data[12:16])
	h.FrameNum = binary.LittleEndian.Uint32(data[16:20])
	h.TotalFrames = binary.LittleEndian.Uint32(data[20:24])
	h.DataSize = binary.LittleEndian.Uint32(data[24:28])
	h.CRC = binary.LittleEndian.Uint32(data[28:32])
	copy(h.Reserved[:], data[32:64])

	if string(h.Signature[:]) != Signature {
		return nil, ErrInvalidSignature
	}
	if h.Version != Version {
		return nil, ErrInvalidVersion
	}
	return h, nil
}

func (h *Header) SetCRC(data []byte) {
	h.CRC = crc32.ChecksumIEEE(data)
}

func ValidateCRC(data []byte, expected uint32) bool {
	return crc32.ChecksumIEEE(data) == expected
}

type FrameInfo struct {
	Width, Height int
	PixelSize     PixelSize
	Mode          Mode
	BigPixelsW    int
	BigPixelsH    int
	BorderW       int
	DataBigPixels int
}

func CalcFrameInfo(width, height int, pixelSize PixelSize, mode Mode) FrameInfo {
	borderPx := int(pixelSize) * BorderBigPixels
	innerW := width - 2*borderPx
	innerH := height - 2*borderPx

	if innerW < 0 || innerH < 0 {
		return FrameInfo{}
	}

	bigPixelsW := innerW / int(pixelSize)
	bigPixelsH := innerH / int(pixelSize)

	dataCols := bigPixelsW - 2*BorderBigPixels
	dataRows := bigPixelsH - 2*BorderBigPixels
	if dataCols <= 0 || dataRows <= 0 {
		return FrameInfo{
			Width:          width,
			Height:         height,
			PixelSize:      pixelSize,
			Mode:           mode,
			BigPixelsW:     bigPixelsW,
			BigPixelsH:     bigPixelsH,
			BorderW:        borderPx,
			DataBigPixels:  0,
		}
	}
	dataBigPixels := dataCols * dataRows

	return FrameInfo{
		Width:          width,
		Height:         height,
		PixelSize:      pixelSize,
		Mode:           mode,
		BigPixelsW:     bigPixelsW,
		BigPixelsH:     bigPixelsH,
		BorderW:        borderPx,
		DataBigPixels:  dataBigPixels,
	}
}

func (fi FrameInfo) BitsPerFrame() int {
	if fi.Mode == ModeDenseValue {
		return fi.DataBigPixels * 4
	}
	return fi.DataBigPixels * 8
}

func (fi FrameInfo) BytesPerFrame() int {
	return fi.BitsPerFrame() / 8
}

type DensePalette [16]uint8

func StandardDensePalette() DensePalette {
	var p DensePalette
	for i := range p {
		p[i] = uint8(i * 17)
	}
	return p
}

func (p DensePalette) Encode(value uint8) uint8 {
	nibble := value & 0x0F
	return p[nibble]
}

func (p DensePalette) Decode(pixel uint8) uint8 {
	best := uint8(0)
	bestDist := uint16(65535)
	for i, v := range p {
		d := uint16(v)
		if d > uint16(pixel) {
			d = d - uint16(pixel)
		} else {
			d = uint16(pixel) - d
		}
		if d < bestDist {
			bestDist = d
			best = uint8(i)
		}
	}
	return best
}

var DefaultDensePalette = StandardDensePalette()
