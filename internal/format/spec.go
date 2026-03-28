package format

import (
	"encoding/binary"
	"hash/crc32"
)

const (
	BorderWidth      = 12
	TemplateSize     = 9
	SeparatorPattern = 0xFF
	SeparatorBits    = 8
	MaxBitDepth      = 8
)

const (
	MetaBitDepth    = 1
	MetaFileSize    = 4
	MetaFileNameLen = 1
	MetaCRC32       = 4
	MetaDataRows    = 2
	MetaDataCols    = 2
	MetaTotalBlocks = 2
	MetaBlockIndex  = 2
	MetaSeparator   = 1
	MetaFileNameMax = 32

	MetaMinBytes   = MetaBitDepth + MetaFileSize + MetaFileNameLen + MetaCRC32 + MetaDataRows + MetaDataCols + MetaTotalBlocks + MetaBlockIndex + MetaSeparator
	MetaFixedBytes = MetaMinBytes + MetaFileNameMax
	MetaFixedBits  = MetaFixedBytes * 8
)

type MetaInfo struct {
	BitDepth    uint8
	FileSize    uint32
	FileName    string
	CRC32       uint32
	DataRows    uint16
	DataCols    uint16
	TotalBlocks uint16
	BlockIndex  uint16
}

func (m *MetaInfo) Serialize() []byte {
	nameBytes := []byte(m.FileName)
	if len(nameBytes) > MetaFileNameMax {
		nameBytes = nameBytes[:MetaFileNameMax]
	}

	nameLen := len(nameBytes)
	paddedLen := MetaFileNameMax
	padding := paddedLen - nameLen

	data := make([]byte, 0, MetaFixedBytes)

	data = append(data, m.BitDepth)
	data = binary.LittleEndian.AppendUint32(data, m.FileSize)
	data = append(data, uint8(nameLen))
	data = append(data, nameBytes...)
	for i := 0; i < padding; i++ {
		data = append(data, 0)
	}
	data = binary.LittleEndian.AppendUint32(data, m.CRC32)
	data = binary.LittleEndian.AppendUint16(data, m.DataRows)
	data = binary.LittleEndian.AppendUint16(data, m.DataCols)
	data = binary.LittleEndian.AppendUint16(data, m.TotalBlocks)
	data = binary.LittleEndian.AppendUint16(data, m.BlockIndex)
	data = append(data, SeparatorPattern)

	return data
}

func ParseMeta(data []byte) (*MetaInfo, error) {
	if len(data) < MetaFixedBytes {
		return nil, ErrInvalidHeader
	}

	m := &MetaInfo{}
	offset := 0

	m.BitDepth = data[offset]
	offset += MetaBitDepth

	m.FileSize = binary.LittleEndian.Uint32(data[offset : offset+MetaFileSize])
	offset += MetaFileSize

	nameLen := int(data[offset])
	offset += MetaFileNameLen

	if nameLen > MetaFileNameMax {
		nameLen = MetaFileNameMax
	}

	m.FileName = string(data[offset : offset+nameLen])
	offset += MetaFileNameMax

	m.CRC32 = binary.LittleEndian.Uint32(data[offset : offset+MetaCRC32])
	offset += MetaCRC32

	m.DataRows = binary.LittleEndian.Uint16(data[offset : offset+MetaDataRows])
	offset += MetaDataRows

	m.DataCols = binary.LittleEndian.Uint16(data[offset : offset+MetaDataCols])
	offset += MetaDataCols

	m.TotalBlocks = binary.LittleEndian.Uint16(data[offset : offset+MetaTotalBlocks])
	offset += MetaTotalBlocks

	m.BlockIndex = binary.LittleEndian.Uint16(data[offset : offset+MetaBlockIndex])
	offset += MetaBlockIndex

	if data[offset] != SeparatorPattern {
		return nil, ErrInvalidHeader
	}

	return m, nil
}

func (m *MetaInfo) SetCRC(data []byte) {
	m.CRC32 = crc32.ChecksumIEEE(data)
}

func ValidateCRC(data []byte, expected uint32) bool {
	return crc32.ChecksumIEEE(data) == expected
}

type FrameInfo struct {
	Width       int
	Height      int
	PixelSize   int
	BorderPx    int
	DataCols    int
	DataRows    int
	StartMarker Point
	EndMarker   Point
}

type Point struct {
	X, Y int
}

func CalcFrameInfo(width, height int, pixelSize int) FrameInfo {
	borderPx := BorderWidth
	templatePx := TemplateSize * pixelSize

	innerW := width - 2*borderPx - 2*templatePx
	innerH := height - 2*borderPx - 2*templatePx

	if innerW < 0 || innerH < 0 {
		return FrameInfo{}
	}

	cols := innerW / pixelSize
	rows := innerH / pixelSize

	if cols <= 0 || rows <= 0 {
		return FrameInfo{}
	}

	return FrameInfo{
		Width:       width,
		Height:      height,
		PixelSize:   pixelSize,
		BorderPx:    borderPx,
		DataCols:    cols,
		DataRows:    rows,
		StartMarker: Point{X: borderPx, Y: borderPx},
		EndMarker:   Point{X: width - borderPx - templatePx, Y: height - borderPx - templatePx},
	}
}
