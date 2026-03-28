package codec

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"telescope/internal/format"
)

type Encoder struct {
	width     int
	height    int
	pixelSize int
	bitDepth  int
	logger    func(string)
}

type EncoderOption func(*Encoder)

func WithPixelSize(ps int) EncoderOption {
	return func(e *Encoder) {
		e.pixelSize = ps
	}
}

func WithBitDepth(bd int) EncoderOption {
	return func(e *Encoder) {
		e.bitDepth = bd
	}
}

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

func NewEncoder(width, height int, opts ...EncoderOption) *Encoder {
	e := &Encoder{
		width:     width,
		height:    height,
		pixelSize: 2,
		bitDepth:  1,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Encoder) FrameInfo() format.FrameInfo {
	return format.CalcFrameInfo(e.width, e.height, e.pixelSize)
}

func (e *Encoder) MaxBytesPerFrame() int {
	fi := e.FrameInfo()
	return (fi.DataRows * fi.DataCols * e.bitDepth) / 8
}

func (e *Encoder) EncodeFile(data []byte, filename string, totalBlocks, blockIndex uint16) (*image.Gray, error) {
	return e.EncodeFileWithFEC(data, filename, totalBlocks, blockIndex, 0, 0)
}

func (e *Encoder) EncodeFileWithFEC(data []byte, filename string, totalBlocks, blockIndex uint16, fecBlocks, fecGroup uint8) (*image.Gray, error) {
	fi := e.FrameInfo()
	e.log("EncodeFile: width=%d, height=%d, pixelSize=%d, bitDepth=%d, block=%d/%d",
		e.width, e.height, e.pixelSize, e.bitDepth, blockIndex, totalBlocks)

	if fi.DataCols < format.TemplateSize*2 || fi.DataRows < format.TemplateSize*2 {
		return nil, fmt.Errorf("%w: image too small", format.ErrImageTooSmall)
	}

	metaInfo := &format.MetaInfo{
		BitDepth:    uint8(e.bitDepth),
		FileSize:    uint32(len(data)),
		FileName:    filename,
		DataRows:    uint16(fi.DataRows),
		DataCols:    uint16(fi.DataCols),
		TotalBlocks: totalBlocks,
		BlockIndex:  blockIndex,
		FECBlocks:   fecBlocks,
		FECGroup:    fecGroup,
	}
	metaInfo.SetCRC(data)
	metaData := metaInfo.Serialize()

	img := image.NewGray(image.Rect(0, 0, e.width, e.height))
	e.DrawBorder(img)
	e.DrawTemplate(img, fi.StartMarker.X, fi.StartMarker.Y)
	e.DrawMetaData(img, metaData, fi)
	e.DrawData(img, data, metaInfo, fi)
	e.DrawTemplate(img, fi.EndMarker.X, fi.EndMarker.Y)

	return img, nil
}

func (e *Encoder) DrawBorder(img *image.Gray) {
	bounds := img.Bounds()
	borderPx := format.BorderWidth

	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			isBorder := x < borderPx || x >= bounds.Dx()-borderPx || y < borderPx || y >= bounds.Dy()-borderPx
			if isBorder {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
}

func (e *Encoder) DrawTemplate(img *image.Gray, startX, startY int) {
	px := e.pixelSize

	for row := 0; row < format.TemplateSize; row++ {
		for col := 0; col < format.TemplateSize; col++ {
			isWhite := (row+col)%2 == 0
			gray := uint8(0)
			if isWhite {
				gray = 255
			}
			e.fillPixel(img, startX+col*px, startY+row*px, int(gray))
		}
	}
}

func (e *Encoder) DrawMetaData(img *image.Gray, metaData []byte, fi format.FrameInfo) {
	px := e.pixelSize
	startX := fi.StartMarker.X + format.TemplateSize*px
	startY := fi.StartMarker.Y + format.TemplateSize*px

	bitIndex := 0
	bytesNeeded := (format.MetaFixedBits + 7) / 8
	if bytesNeeded > len(metaData) {
		bytesNeeded = len(metaData)
	}

	row := 0
	col := 0

	for byteIdx := 0; byteIdx < bytesNeeded; byteIdx++ {
		for bit := 7; bit >= 0; bit-- {
			if bitIndex >= format.MetaFixedBits {
				break
			}

			bitValue := (metaData[byteIdx] >> bit) & 1
			gray := uint8(0)
			if bitValue == 1 {
				gray = 255
			}

			px := e.pixelSize
			e.fillPixel(img, startX+col*px, startY+row*px, int(gray))

			col++
			if col >= fi.DataCols {
				col = 0
				row++
			}
			bitIndex++
		}
	}
}

func (e *Encoder) DrawData(img *image.Gray, data []byte, metaInfo *format.MetaInfo, fi format.FrameInfo) {
	px := e.pixelSize
	dataCols := int(metaInfo.DataCols)
	dataRows := int(metaInfo.DataRows)

	startX := fi.StartMarker.X + format.TemplateSize*px
	startY := fi.StartMarker.Y + format.TemplateSize*px + (format.MetaFixedBits+dataCols-1)/dataCols*px

	bitsPerPoint := e.bitDepth
	maxValue := (1 << bitsPerPoint) - 1

	bitBuffer := make([]bool, 0)
	for _, b := range data {
		for i := 7; i >= 0; i-- {
			bitBuffer = append(bitBuffer, (b>>i)&1 == 1)
		}
	}

	row := 0
	col := 0

	for bitIdx := 0; bitIdx < len(bitBuffer); bitIdx += bitsPerPoint {
		var value uint8
		for b := 0; b < bitsPerPoint && bitIdx+b < len(bitBuffer); b++ {
			if bitBuffer[bitIdx+b] {
				value |= uint8(1 << b)
			}
		}

		gray := uint8(float64(value) / float64(maxValue) * 255)
		e.fillPixel(img, startX+col*px, startY+row*px, int(gray))

		col++
		if col >= dataCols {
			col = 0
			row++
			if row >= dataRows {
				break
			}
		}
	}
}

func (e *Encoder) fillPixel(img *image.Gray, x, y, gray int) {
	px := e.pixelSize
	for dy := 0; dy < px && y+dy < e.height; dy++ {
		for dx := 0; dx < px && x+dx < e.width; dx++ {
			img.SetGray(x+dx, y+dy, color.Gray{Y: uint8(gray)})
		}
	}
}

func (e *Encoder) SaveImage(img *image.Gray, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func EncodeFile(inputPath, outputPath string, width, height, pixelSize, bitDepth int, logger func(string)) error {
	return EncodeFileMulti(inputPath, outputPath, filepath.Base(inputPath), width, height, pixelSize, bitDepth, logger)
}

func EncodeFileToDir(inputPath, outputDir string, width, height, pixelSize, bitDepth int, logger func(string)) error {
	if logger == nil {
		logger = func(string) {}
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	baseName := filepath.Base(inputPath)
	outputPathPrefix := filepath.Join(outputDir, baseName)

	return EncodeFileMulti(inputPath, outputPathPrefix, baseName, width, height, pixelSize, bitDepth, logger)
}

func EncodeFileMulti(inputPath, outputPathPrefix, filename string, width, height, pixelSize, bitDepth int, logger func(string)) error {
	if logger == nil {
		logger = func(string) {}
	}

	logger(fmt.Sprintf("Encoding: input=%s, output=%s, size=%dx%d, pixelSize=%d, bitDepth=%d",
		inputPath, outputPathPrefix, width, height, pixelSize, bitDepth))

	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	logger(fmt.Sprintf("File size: %d bytes", len(data)))

	encoder := NewEncoder(width, height, WithPixelSize(pixelSize), WithBitDepth(bitDepth), WithLogger(logger))
	maxCapacity := encoder.MaxBytesPerFrame()
	totalBlocks := uint16((len(data) + maxCapacity - 1) / maxCapacity)

	logger(fmt.Sprintf("Splitting into %d block(s) (max %d bytes each)", totalBlocks, maxCapacity))

	dataBlocks := make([][]byte, totalBlocks)
	for blockIndex := uint16(0); blockIndex < totalBlocks; blockIndex++ {
		start := int(blockIndex) * maxCapacity
		end := start + maxCapacity
		if end > len(data) {
			end = len(data)
		}
		dataBlocks[blockIndex] = data[start:end]
	}

	fecGroups := calculateFECGroups(int(totalBlocks))
	fecBlocks := generateFECBlocks(dataBlocks, fecGroups)

	totalWithFEC := totalBlocks + uint16(len(fecBlocks))
	logger(fmt.Sprintf("Generating %d FEC block(s) for recovery", len(fecBlocks)))

	for blockIndex := uint16(0); blockIndex < totalBlocks; blockIndex++ {
		img, err := encoder.EncodeFileWithFEC(dataBlocks[blockIndex], filename, totalWithFEC, blockIndex, uint8(len(fecBlocks)), uint8(fecGroups))
		if err != nil {
			return fmt.Errorf("failed to encode block %d: %w", blockIndex, err)
		}

		outputPath := fmt.Sprintf("%s_%03d.png", outputPathPrefix, blockIndex)
		if err := encoder.SaveImage(img, outputPath); err != nil {
			return fmt.Errorf("failed to save block %d: %w", blockIndex, err)
		}
		logger(fmt.Sprintf("Saved block %d/%d: %s (%d bytes)", blockIndex+1, totalBlocks, outputPath, len(dataBlocks[blockIndex])))
	}

	for i, fecData := range fecBlocks {
		fecIndex := totalBlocks + uint16(i)
		fecMeta := &format.MetaInfo{
			BitDepth:    uint8(bitDepth),
			FileSize:    uint32(len(fecData)),
			FileName:    filename,
			DataRows:    uint16(encoder.FrameInfo().DataRows),
			DataCols:    uint16(encoder.FrameInfo().DataCols),
			TotalBlocks: totalWithFEC,
			BlockIndex:  fecIndex,
			FECBlocks:   uint8(len(fecBlocks)),
			FECGroup:    uint8(fecGroups),
		}
		fecMeta.SetCRC(fecData)
		fecDataSerialized := fecMeta.Serialize()

		img := image.NewGray(image.Rect(0, 0, width, height))
		encoder.DrawBorder(img)
		encoder.DrawTemplate(img, encoder.FrameInfo().StartMarker.X, encoder.FrameInfo().StartMarker.Y)
		encoder.DrawMetaData(img, fecDataSerialized, encoder.FrameInfo())
		encoder.DrawData(img, fecData, fecMeta, encoder.FrameInfo())
		encoder.DrawTemplate(img, encoder.FrameInfo().EndMarker.X, encoder.FrameInfo().EndMarker.Y)

		outputPath := fmt.Sprintf("%s_%03d.png", outputPathPrefix, fecIndex)
		if err := encoder.SaveImage(img, outputPath); err != nil {
			return fmt.Errorf("failed to save FEC block %d: %w", i, err)
		}
		logger(fmt.Sprintf("Saved FEC block %d/%d: %s (%d bytes)", i+1, len(fecBlocks), outputPath, len(fecData)))
	}

	return nil
}

func calculateFECGroups(numBlocks int) int {
	if numBlocks <= 5 {
		return numBlocks
	}
	return 10
}

func generateFECBlocks(dataBlocks [][]byte, groupSize int) [][]byte {
	if len(dataBlocks) < 2 {
		return nil
	}

	var fecBlocks [][]byte
	maxLen := 0
	for _, block := range dataBlocks {
		if len(block) > maxLen {
			maxLen = len(block)
		}
	}

	numGroups := (len(dataBlocks) + groupSize - 1) / groupSize

	for g := 0; g < numGroups; g++ {
		start := g * groupSize
		end := start + groupSize
		if end > len(dataBlocks) {
			end = len(dataBlocks)
		}

		fec := make([]byte, maxLen)
		for i := start; i < end; i++ {
			for j := 0; j < len(dataBlocks[i]); j++ {
				fec[j] ^= dataBlocks[i][j]
			}
		}

		paddingLen := 0
		for i := start; i < end; i++ {
			if len(dataBlocks[i]) > paddingLen {
				paddingLen = len(dataBlocks[i])
			}
		}
		fecBlocks = append(fecBlocks, fec[:paddingLen])
	}

	return fecBlocks
}
