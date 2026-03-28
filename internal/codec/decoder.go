package codec

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"telescope/internal/format"
)

type Decoder struct {
	logger func(string)
}

type DecodeLogger func(string)

func NewDecoder() *Decoder {
	return &Decoder{}
}

func NewDecoderWithLogger(logger func(string)) *Decoder {
	return &Decoder{logger: logger}
}

func (d *Decoder) log(format string, args ...interface{}) {
	if d.logger != nil {
		d.logger(fmt.Sprintf(format, args...))
	}
}

func (d *Decoder) DetectFrameInfo(img image.Image) (format.FrameInfo, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	d.log("DetectFrameInfo: image is %dx%d", width, height)

	borderX, borderY := d.findBorder(img)
	if borderX == 0 && borderY == 0 {
		return format.FrameInfo{}, fmt.Errorf("%w: border not found", format.ErrNoBorderFound)
	}
	d.log("Found border at: (%d, %d)", borderX, borderY)

	startMarker, pixelSize := d.findTemplate(img, borderX, borderY)
	if startMarker.X == 0 && startMarker.Y == 0 {
		return format.FrameInfo{}, fmt.Errorf("%w: start template not found", format.ErrNoTemplateFound)
	}
	d.log("Found start template at: (%d, %d), pixelSize: %d", startMarker.X, startMarker.Y, pixelSize)

	borderPx := borderX
	templatePx := format.TemplateSize * pixelSize
	innerW := width - 2*borderPx - 2*templatePx
	innerH := height - 2*borderPx - 2*templatePx

	fi := format.FrameInfo{
		Width:       width,
		Height:      height,
		PixelSize:   pixelSize,
		BorderPx:    borderPx,
		DataCols:    innerW / pixelSize,
		DataRows:    innerH / pixelSize,
		StartMarker: startMarker,
		EndMarker:   format.Point{X: width - borderPx - templatePx, Y: height - borderPx - templatePx},
	}

	d.log("FrameInfo: DataCols=%d, DataRows=%d", fi.DataCols, fi.DataRows)
	return fi, nil
}

func (d *Decoder) findBorder(img image.Image) (int, int) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	whiteCount := 0
	borderSize := format.BorderWidth

	for x := 0; x < width; x++ {
		c := img.At(x, 0)
		if isWhite(c) {
			whiteCount++
		}
	}
	if whiteCount < width/2 {
		return 0, 0
	}

	borderX := borderSize
	borderY := borderSize

	if borderX > width/2 {
		borderX = width / 2
	}
	if borderY > height/2 {
		borderY = height / 2
	}

	return borderX, borderY
}

func (d *Decoder) findTemplate(img image.Image, borderX, borderY int) (format.Point, int) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	for ps := 1; ps <= 20; ps++ {
		templatePx := format.TemplateSize * ps
		if borderX+templatePx > width || borderY+templatePx > height {
			continue
		}

		startX := borderX
		startY := borderY

		if d.matchTemplate(img, startX, startY, ps) {
			return format.Point{X: startX, Y: startY}, ps
		}
	}

	return format.Point{}, 0
}

func (d *Decoder) matchTemplate(img image.Image, startX, startY, pixelSize int) bool {
	for row := 0; row < format.TemplateSize; row++ {
		for col := 0; col < format.TemplateSize; col++ {
			x := startX + col*pixelSize + pixelSize/2
			y := startY + row*pixelSize + pixelSize/2
			c := img.At(x, y)
			expectedWhite := (row+col)%2 == 0
			isWhiteVal := isWhite(c)
			if expectedWhite != isWhiteVal {
				return false
			}
		}
	}
	return true
}

func isWhite(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	avg := (r + g + b) / 3
	return avg > 32768
}

func (d *Decoder) DecodeImage(img image.Image, fi format.FrameInfo) ([]byte, error) {
	data, _, err := d.DecodeImageWithMeta(img, fi)
	return data, err
}

func (d *Decoder) DecodeImageWithMeta(img image.Image, fi format.FrameInfo) ([]byte, *format.MetaInfo, error) {
	d.log("Decoding image with pixelSize=%d, DataCols=%d, DataRows=%d", fi.PixelSize, fi.DataCols, fi.DataRows)

	metaData, err := d.decodeMetaData(img, fi)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode meta: %w", err)
	}

	metaInfo, err := format.ParseMeta(metaData)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse meta: %w", err)
	}

	d.log("Meta: bitDepth=%d, fileSize=%d, filename=%s, dataRows=%d, dataCols=%d",
		metaInfo.BitDepth, metaInfo.FileSize, metaInfo.FileName, metaInfo.DataRows, metaInfo.DataCols)

	data, err := d.decodeData(img, fi, metaInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode data: %w", err)
	}

	if !format.ValidateCRC(data, metaInfo.CRC32) {
		d.log("WARNING: CRC mismatch - data may be corrupted")
	}

	return data, metaInfo, nil
}

func (d *Decoder) decodeMetaData(img image.Image, fi format.FrameInfo) ([]byte, error) {
	px := fi.PixelSize
	startX := fi.StartMarker.X + format.TemplateSize*px
	startY := fi.StartMarker.Y + format.TemplateSize*px

	metaBits := format.MetaFixedBits
	bytesNeeded := (metaBits + 7) / 8

	result := make([]byte, bytesNeeded)
	bitIndex := 0

	row := 0
	col := 0

	for bitIndex < metaBits {
		x := startX + col*px
		y := startY + row*px
		c := img.At(x, y)
		bitValue := 0
		if isWhite(c) {
			bitValue = 1
		}

		byteIdx := bitIndex / 8
		bitPos := 7 - (bitIndex % 8)
		if bitValue == 1 {
			result[byteIdx] |= (1 << bitPos)
		}

		col++
		if col >= fi.DataCols {
			col = 0
			row++
		}
		bitIndex++

		if bitIndex >= metaBits {
			break
		}
	}

	return result, nil
}

func (d *Decoder) decodeData(img image.Image, fi format.FrameInfo, metaInfo *format.MetaInfo) ([]byte, error) {
	px := fi.PixelSize
	dataCols := int(metaInfo.DataCols)
	dataRows := int(metaInfo.DataRows)

	startX := fi.StartMarker.X + format.TemplateSize*px
	startY := fi.StartMarker.Y + format.TemplateSize*px + (format.MetaFixedBits+dataCols-1)/dataCols*px

	bitsPerPoint := int(metaInfo.BitDepth)
	maxValue := (1 << bitsPerPoint) - 1

	totalBits := dataRows * dataCols * bitsPerPoint
	totalBytes := (totalBits + 7) / 8

	result := make([]byte, totalBytes)

	row := 0
	col := 0
	bitBuffer := make([]bool, 0, totalBits)

	for row < dataRows {
		x := startX + col*px + px/2
		y := startY + row*px + px/2
		c := img.At(x, y)

		var value uint8
		if isWhite(c) {
			value = uint8(maxValue)
		} else {
			r, _, _, _ := c.RGBA()
			gray := uint8(r >> 8)
			value = uint8(float64(gray) / 255.0 * float64(maxValue))
		}

		for b := bitsPerPoint - 1; b >= 0; b-- {
			bitBuffer = append(bitBuffer, (value>>b)&1 == 1)
		}

		col++
		if col >= dataCols {
			col = 0
			row++
		}
	}

	for i := 0; i < len(bitBuffer); i += 8 {
		var b byte
		for j := 0; j < 8 && i+j < len(bitBuffer); j++ {
			if bitBuffer[i+j] {
				b |= (1 << (7 - j))
			}
		}
		result[i/8] = b
	}

	return result[:metaInfo.FileSize], nil
}

func (d *Decoder) LoadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()
	return png.Decode(f)
}

func DecodeFile(inputPath string, logger func(string)) ([]byte, string, error) {
	if logger == nil {
		logger = func(string) {}
	}

	decoder := NewDecoderWithLogger(logger)

	img, err := decoder.LoadImage(inputPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load image: %w", err)
	}

	fi, err := decoder.DetectFrameInfo(img)
	if err != nil {
		return nil, "", fmt.Errorf("failed to detect frame info: %w", err)
	}

	data, metaInfo, err := decoder.DecodeImageWithMeta(img, fi)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode: %w", err)
	}

	filename := metaInfo.FileName
	if filename == "" {
		filename = filepath.Base(inputPath)
		if ext := filepath.Ext(filename); ext != "" {
			filename = filename[:len(ext)]
		}
		filename = filename + "_restored"
	}

	return data, filename, nil
}

func SaveFile(data []byte, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
