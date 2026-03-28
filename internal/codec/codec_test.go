package codec

import (
	"os"
	"path/filepath"
	"testing"

	"telescope/internal/format"
)

func TestMultiFrameEncodeDecode(t *testing.T) {
	width := 1920
	height := 1080
	pixelSize := 2
	bitDepth := 1

	encoder := NewEncoder(width, height, WithPixelSize(pixelSize), WithBitDepth(bitDepth))
	maxBytes := encoder.MaxBytesPerFrame()
	t.Logf("Max bytes per frame: %d", maxBytes)

	testData := make([]byte, maxBytes*3+1000)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	img1, err := encoder.EncodeFile(testData[:maxBytes], "test.bin", 3, 0)
	if err != nil {
		t.Fatalf("failed to encode block 0: %v", err)
	}

	img2, err := encoder.EncodeFile(testData[maxBytes:maxBytes*2], "test.bin", 3, 1)
	if err != nil {
		t.Fatalf("failed to encode block 1: %v", err)
	}

	img3, err := encoder.EncodeFile(testData[maxBytes*2:maxBytes*2+1000], "test.bin", 3, 2)
	if err != nil {
		t.Fatalf("failed to encode block 2: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "telescope-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := encoder.SaveImage(img1, filepath.Join(tmpDir, "test_000.png")); err != nil {
		t.Fatalf("failed to save img1: %v", err)
	}
	if err := encoder.SaveImage(img2, filepath.Join(tmpDir, "test_001.png")); err != nil {
		t.Fatalf("failed to save img2: %v", err)
	}
	if err := encoder.SaveImage(img3, filepath.Join(tmpDir, "test_002.png")); err != nil {
		t.Fatalf("failed to save img3: %v", err)
	}

	decoder := NewDecoder()
	fi, err := decoder.DetectFrameInfo(img1)
	if err != nil {
		t.Fatalf("failed to detect frame info: %v", err)
	}

	data1, meta1, err := decoder.DecodeImageWithMeta(img1, fi)
	if err != nil {
		t.Fatalf("failed to decode img1: %v", err)
	}
	if meta1.TotalBlocks != 3 {
		t.Errorf("expected TotalBlocks=3, got %d", meta1.TotalBlocks)
	}
	if meta1.BlockIndex != 0 {
		t.Errorf("expected BlockIndex=0, got %d", meta1.BlockIndex)
	}

	data2, meta2, err := decoder.DecodeImageWithMeta(img2, fi)
	if err != nil {
		t.Fatalf("failed to decode img2: %v", err)
	}
	if meta2.BlockIndex != 1 {
		t.Errorf("expected BlockIndex=1, got %d", meta2.BlockIndex)
	}

	data3, meta3, err := decoder.DecodeImageWithMeta(img3, fi)
	if err != nil {
		t.Fatalf("failed to decode img3: %v", err)
	}
	if meta3.BlockIndex != 2 {
		t.Errorf("expected BlockIndex=2, got %d", meta3.BlockIndex)
	}

	result := append(data1, data2...)
	result = append(result, data3...)

	expectedLen := maxBytes + maxBytes + 1000
	if len(result) != expectedLen {
		t.Errorf("expected %d bytes, got %d", expectedLen, len(result))
	}

	for i, b := range result {
		if b != testData[i] {
			t.Errorf("byte mismatch at index %d: expected %02x, got %02x", i, testData[i], b)
			break
		}
	}
}

func TestDecodeDirectory(t *testing.T) {
	width := 1920
	height := 1080
	pixelSize := 2
	bitDepth := 1

	encoder := NewEncoder(width, height, WithPixelSize(pixelSize), WithBitDepth(bitDepth))
	maxBytes := encoder.MaxBytesPerFrame()

	testData := make([]byte, maxBytes*2+500)
	for i := range testData {
		testData[i] = byte((i * 7) % 256)
	}

	tmpDir, err := os.MkdirTemp("", "telescope-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	totalBlocks := 3
	for i := 0; i < totalBlocks; i++ {
		start := i * maxBytes
		end := start + maxBytes
		if end > len(testData) {
			end = len(testData)
		}
		chunk := testData[start:end]

		img, err := encoder.EncodeFile(chunk, "testfile.bin", uint16(totalBlocks), uint16(i))
		if err != nil {
			t.Fatalf("failed to encode block %d: %v", i, err)
		}

		if err := encoder.SaveImage(img, filepath.Join(tmpDir, "testfile_000.png")); err != nil {
			t.Fatalf("failed to save block %d: %v", i, err)
		}
	}

	decoder := NewDecoder()
	fi, err := decoder.DetectFrameInfoFromFile(filepath.Join(tmpDir, "testfile_000.png"))
	if err != nil {
		t.Fatalf("failed to detect frame info: %v", err)
	}

	data, meta, err := decoder.DecodeImageWithMetaFromFile(filepath.Join(tmpDir, "testfile_000.png"), fi)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if meta.TotalBlocks != uint16(totalBlocks) {
		t.Errorf("expected TotalBlocks=%d, got %d", totalBlocks, meta.TotalBlocks)
	}

	_ = data
}

func TestCalcFrameInfo(t *testing.T) {
	tests := []struct {
		width     int
		height    int
		pixelSize int
		wantCols  int
		wantRows  int
	}{
		{1920, 1080, 2, 930, 510},
		{800, 600, 2, 370, 270},
		{100, 100, 2, 20, 20},
	}

	for _, tt := range tests {
		fi := format.CalcFrameInfo(tt.width, tt.height, tt.pixelSize)
		if fi.DataCols != tt.wantCols || fi.DataRows != tt.wantRows {
			t.Errorf("CalcFrameInfo(%d, %d, %d): got cols=%d, rows=%d, want cols=%d, rows=%d",
				tt.width, tt.height, tt.pixelSize, fi.DataCols, fi.DataRows, tt.wantCols, tt.wantRows)
		}
	}
}
