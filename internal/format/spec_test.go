package format

import (
	"testing"
)

func TestMetaInfoSerializeParse(t *testing.T) {
	meta := &MetaInfo{
		BitDepth:    1,
		FileSize:    12345,
		FileName:    "test.txt",
		CRC32:       0xDEADBEEF,
		DataRows:    100,
		DataCols:    50,
		TotalBlocks: 3,
		BlockIndex:  1,
	}

	data := meta.Serialize()
	if len(data) != MetaFixedBytes {
		t.Errorf("expected serialized length %d, got %d", MetaFixedBytes, len(data))
	}

	parsed, err := ParseMeta(data)
	if err != nil {
		t.Fatalf("failed to parse meta: %v", err)
	}

	if parsed.BitDepth != meta.BitDepth {
		t.Errorf("BitDepth: expected %d, got %d", meta.BitDepth, parsed.BitDepth)
	}
	if parsed.FileSize != meta.FileSize {
		t.Errorf("FileSize: expected %d, got %d", meta.FileSize, parsed.FileSize)
	}
	if parsed.FileName != meta.FileName {
		t.Errorf("FileName: expected %s, got %s", meta.FileName, parsed.FileName)
	}
	if parsed.CRC32 != meta.CRC32 {
		t.Errorf("CRC32: expected %x, got %x", meta.CRC32, parsed.CRC32)
	}
	if parsed.DataRows != meta.DataRows {
		t.Errorf("DataRows: expected %d, got %d", meta.DataRows, parsed.DataRows)
	}
	if parsed.DataCols != meta.DataCols {
		t.Errorf("DataCols: expected %d, got %d", meta.DataCols, parsed.DataCols)
	}
	if parsed.TotalBlocks != meta.TotalBlocks {
		t.Errorf("TotalBlocks: expected %d, got %d", meta.TotalBlocks, parsed.TotalBlocks)
	}
	if parsed.BlockIndex != meta.BlockIndex {
		t.Errorf("BlockIndex: expected %d, got %d", meta.BlockIndex, parsed.BlockIndex)
	}
}

func TestMetaFixedBytes(t *testing.T) {
	expected := MetaBitDepth + MetaFileSize + MetaFileNameLen + MetaCRC32 + MetaDataRows + MetaDataCols + MetaTotalBlocks + MetaBlockIndex + MetaSeparator + MetaFileNameMax
	if MetaFixedBytes != expected {
		t.Errorf("MetaFixedBytes: expected %d, got %d (components sum: %d)", expected, MetaFixedBytes, expected)
	}
}
