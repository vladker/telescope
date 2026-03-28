package codec

import (
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"telescope/internal/format"
)

type Decoder struct {
	palette format.DensePalette
	logger  func(string)
}

type DecodeLogger func(string)

func NewDecoder() *Decoder {
	return &Decoder{
		palette: format.DefaultDensePalette,
	}
}

func NewDecoderWithLogger(logger func(string)) *Decoder {
	return &Decoder{
		palette: format.DefaultDensePalette,
		logger:  logger,
	}
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

	var detectedPixelSize format.PixelSize
	var borderOffsetX, borderOffsetY int

	for ps := format.Pixel3x3; ps >= format.Pixel1x1; ps-- {
		borderPx := int(ps) * format.BorderBigPixels
		d.log("  Testing pixelSize=%d: borderPx=%d", ps, borderPx)

		if borderPx*2 >= width || borderPx*2 >= height {
			d.log("    Skipping: image too small for this pixel size")
			continue
		}

		offset, valid, matchPct := d.findBorderInImage(img, ps)
		d.log("    Border search: offset=(%d,%d), valid=%v, match=%.1f%%", offset.x, offset.y, valid, matchPct*100)
		if valid {
			detectedPixelSize = ps
			borderOffsetX = offset.x
			borderOffsetY = offset.y
			break
		}
	}

	if detectedPixelSize == 0 {
		return format.FrameInfo{}, fmt.Errorf("%w: no valid border found in image", format.ErrNoBorderFound)
	}

	d.log("Detected pixelSize: %d, borderOffset: (%d, %d)", detectedPixelSize, borderOffsetX, borderOffsetY)

	borderPx := int(detectedPixelSize) * format.BorderBigPixels
	innerW := width - 2*borderPx
	innerH := height - 2*borderPx
	bigPixelsW := innerW / int(detectedPixelSize)
	bigPixelsH := innerH / int(detectedPixelSize)

	dataCols := bigPixelsW - 2*format.BorderBigPixels
	dataRows := bigPixelsH - 2*format.BorderBigPixels
	dataBigPixels := dataCols * dataRows

	mode := format.ModeRobustValue
	metaRow := format.BorderBigPixels
	metaCol := format.BorderBigPixels
	sampleGray := d.getBigPixelGrayWithBorder(img, metaCol, metaRow, int(detectedPixelSize), borderPx, borderOffsetX, borderOffsetY)
	invertedGray := d.getBigPixelGrayWithBorder(img, metaCol, metaRow+1, int(detectedPixelSize), borderPx, borderOffsetX, borderOffsetY)
	d.log("Meta sample: row=%d, col=%d, gray=%d, invertedGray=%d", metaRow, metaCol, sampleGray, invertedGray)

	sampleDecoded := d.palette.Decode(sampleGray)
	isDenseValue := sampleDecoded <= 15

	if sampleGray > 200 && invertedGray < 50 {
		mode = format.ModeRobustValue
		d.log("Detected mode: robust (inverted row pattern)")
	} else if isDenseValue && sampleGray%17 == 0 {
		mode = format.ModeDenseValue
		d.log("Detected mode: dense (palette value 0x%02X decoded to %d)", sampleGray, sampleDecoded)
	} else {
		mode = format.ModeRobustValue
		d.log("Detected mode: robust (default)")
	}

	return format.FrameInfo{
		Width:         width,
		Height:        height,
		PixelSize:     detectedPixelSize,
		Mode:          mode,
		BigPixelsW:    bigPixelsW,
		BigPixelsH:    bigPixelsH,
		BorderW:       borderPx,
		BorderX:       borderOffsetX,
		BorderY:       borderOffsetY,
		DataBigPixels: dataBigPixels,
	}, nil
}

type point struct {
	x, y int
}

func (d *Decoder) findBorderInImage(img image.Image, pixelSize format.PixelSize) (point, bool, float64) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	borderPx := int(pixelSize) * format.BorderBigPixels

	step := int(pixelSize) * 2
	if step < 4 {
		step = 4
	}

	bestMatch := 0.0
	bestOffset := point{0, 0}
	bestValid := false

	for y := 0; y < height-borderPx*2; y += step {
		for x := 0; x < width-borderPx*2; x += step {
			valid, matchPct := d.validateBorderAtOffset(img, x, y, borderPx, int(pixelSize))
			if valid && matchPct > bestMatch {
				bestMatch = matchPct
				bestOffset = point{x, y}
				bestValid = true
			}
		}
	}

	return bestOffset, bestValid, bestMatch
}

func (d *Decoder) validateBorderAtOffset(img image.Image, offsetX, offsetY, borderPx, pixelSize int) (bool, float64) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	matchCount := 0
	totalChecks := 0

	for i := 0; i < 8*pixelSize; i += pixelSize {
		for check := 0; check < 4; check++ {
			x := offsetX + i
			y := offsetY + check*pixelSize
			if x >= width || y >= height {
				continue
			}
			bigX := i / pixelSize
			bigY := check
			gray := d.getBigPixelGrayAt(img, x, y, pixelSize)
			totalChecks++
			expected := 0
			if (bigX+bigY)%2 == 0 {
				expected = 255
			}
			if int(gray) == expected || (gray > 240 && expected == 255) || (gray < 15 && expected == 0) {
				matchCount++
			}
		}
	}

	for i := 0; i < 8*pixelSize; i += pixelSize {
		for check := 0; check < 4; check++ {
			x := offsetX + check*pixelSize
			y := offsetY + i
			if x >= width || y >= height {
				continue
			}
			bigX := check
			bigY := i / pixelSize
			gray := d.getBigPixelGrayAt(img, x, y, pixelSize)
			totalChecks++
			expected := 0
			if (bigX+bigY)%2 == 0 {
				expected = 255
			}
			if int(gray) == expected || (gray > 240 && expected == 255) || (gray < 15 && expected == 0) {
				matchCount++
			}
		}
	}

	if totalChecks == 0 {
		return false, 0
	}
	return matchCount > totalChecks*7/10, float64(matchCount) / float64(totalChecks)
}

func (d *Decoder) getBigPixelGrayAt(img image.Image, x, y, size int) uint8 {
	var sum uint32
	count := 0
	bounds := img.Bounds()
	for py := y; py < y+size && py < bounds.Max.Y; py++ {
		for px := x; px < x+size && px < bounds.Max.X; px++ {
			var gray uint8
			switch m := img.(type) {
			case *image.Gray:
				gray = m.GrayAt(px, py).Y
			case *image.RGBA:
				pixel := m.RGBAAt(px, py)
				gray = uint8((int(pixel.R) + int(pixel.G) + int(pixel.B)) / 3)
			default:
				r, g, b, _ := img.At(px, py).RGBA()
				gray = uint8((int(r) + int(g) + int(b)) / 3 >> 8)
			}
			sum += uint32(gray)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return uint8(sum / uint32(count))
}

func (d *Decoder) getBigPixelGrayAtOffset(img image.Image, bigX, bigY, size, offsetX, offsetY int) uint8 {
	startX := offsetX + bigX*size
	startY := offsetY + bigY*size
	return d.getBigPixelGrayAt(img, startX, startY, size)
}

func (d *Decoder) getBigPixelGrayWithBorder(img image.Image, bigX, bigY, size, borderPx, borderX, borderY int) uint8 {
	startX := borderX + borderPx + bigX*size
	startY := borderY + borderPx + bigY*size
	return d.getBigPixelGrayAt(img, startX, startY, size)
}

func (d *Decoder) validateBorder(img image.Image, borderPx, pixelSize int) bool {
	_, matchPct := d.validateBorderDetailed(img, borderPx, pixelSize)
	return matchPct > 0.7
}

func (d *Decoder) validateBorderDetailed(img image.Image, borderPx, pixelSize int) (bool, float64) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	matchCount := 0
	totalChecks := 0

	for offset := 0; offset < 8*pixelSize; offset += pixelSize {
		for check := 0; check < 4; check++ {
			x := offset
			y := check * pixelSize
			if x >= width || y >= height {
				continue
			}
			gray := d.getBigPixelGray(img, x/pixelSize, y/pixelSize, pixelSize, 0)
			totalChecks++
			expected := 0
			if (offset/pixelSize+check)%2 == 0 {
				expected = 255
			}
			if int(gray) == expected || (gray > 240 && expected == 255) || (gray < 15 && expected == 0) {
				matchCount++
			}
		}
	}

	if totalChecks == 0 {
		return false, 0
	}
	return matchCount > totalChecks*7/10, float64(matchCount) / float64(totalChecks)
}

func (d *Decoder) getBigPixelGray(img image.Image, bigX, bigY, size, borderPx int) uint8 {
	startX := borderPx + bigX*size
	startY := borderPx + bigY*size

	var sum uint32
	count := 0
	for y := startY; y < startY+size && y < img.Bounds().Dy(); y++ {
		for x := startX; x < startX+size && x < img.Bounds().Dx(); x++ {
			var gray uint8
			switch m := img.(type) {
			case *image.Gray:
				gray = m.GrayAt(x, y).Y
			case *image.RGBA:
				pixel := m.RGBAAt(x, y)
				gray = uint8((int(pixel.R) + int(pixel.G) + int(pixel.B)) / 3)
			default:
				r, g, b, _ := img.At(x, y).RGBA()
				gray = uint8((int(r) + int(g) + int(b)) / 3 >> 8)
			}
			sum += uint32(gray)
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return uint8(sum / uint32(count))
}

func (d *Decoder) DecodeFrame(img image.Image) ([]byte, *format.Header, error) {
	return d.DecodeFrameWithSkip(img, false)
}

func (d *Decoder) ReadHeader(img image.Image) (*format.Header, error) {
	headerData, _, err := d.readHeaderFromImage(img)
	if err != nil {
		return nil, err
	}
	return format.ParseHeader(headerData)
}

func (d *Decoder) readHeaderFromImage(img image.Image) ([]byte, format.FrameInfo, error) {
	fi, err := d.DetectFrameInfo(img)
	if err != nil {
		return nil, fi, fmt.Errorf("failed to detect frame info: %w", err)
	}

	borderPx := fi.BorderW
	ps := int(fi.PixelSize)
	borderX := fi.BorderX
	borderY := fi.BorderY

	var headerData []byte
	var headerRows int
	colsPerRow := fi.BigPixelsW - 2*format.BorderBigPixels

	if fi.Mode == format.ModeDenseValue {
		headerBigPixels := format.HeaderSize * 2
		headerRows = headerBigPixels / colsPerRow
		if headerBigPixels%colsPerRow != 0 {
			headerRows++
		}
		headerData = d.readDenseDataWithBorder(img, format.BorderBigPixels, headerRows, colsPerRow, colsPerRow, fi.BigPixelsH, ps, borderPx, borderX, borderY, format.HeaderSize)
	} else {
		for row := format.BorderBigPixels; row < format.BorderBigPixels*2; row++ {
			for col := format.BorderBigPixels; col < fi.BigPixelsW-format.BorderBigPixels; col++ {
				if len(headerData) >= format.HeaderSize {
					break
				}
				gray := d.getBigPixelGrayWithBorder(img, col, row, ps, borderPx, borderX, borderY)
				headerData = append(headerData, gray)
			}
		}
	}

	if len(headerData) < format.HeaderSize {
		return nil, fi, fmt.Errorf("%w: expected %d bytes, got %d", format.ErrInvalidHeader, format.HeaderSize, len(headerData))
	}

	return headerData, fi, nil
}

func (d *Decoder) DecodeFrameWithSkip(img image.Image, skipCRC bool) ([]byte, *format.Header, error) {
	headerData, fi, err := d.readHeaderFromImage(img)
	if err != nil {
		return nil, nil, err
	}

	d.log("Decoding frame: %dx%d, pixelSize=%d, mode=%d, bigPixels=%dx%d, dataBigPixels=%d",
		fi.Width, fi.Height, fi.PixelSize, fi.Mode, fi.BigPixelsW, fi.BigPixelsH, fi.DataBigPixels)

	d.log("Read %d bytes of header data", len(headerData))

	header, err := format.ParseHeader(headerData)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse header: %w", err)
	}

	d.log("Header parsed: frameNum=%d, totalFrames=%d, dataSize=%d, crc=0x%08x",
		header.FrameNum, header.TotalFrames, header.DataSize, header.CRC)

	borderPx := fi.BorderW
	ps := int(fi.PixelSize)
	borderX := fi.BorderX
	borderY := fi.BorderY
	colsPerRow := fi.BigPixelsW - 2*format.BorderBigPixels

	var frameData []byte
	if fi.Mode == format.ModeDenseValue {
		headerRows := format.HeaderSize * 2 / colsPerRow
		if (format.HeaderSize*2)%colsPerRow != 0 {
			headerRows++
		}
		dataStartRow := format.BorderBigPixels + headerRows
		frameDataRows := fi.BigPixelsH - format.BorderBigPixels - headerRows
		frameData = d.readDenseDataWithBorder(img, dataStartRow, frameDataRows, colsPerRow, colsPerRow, fi.BigPixelsH, ps, borderPx, borderX, borderY, int(header.DataSize))
	} else {
		dataStartRow := format.BorderBigPixels * 2
		dataEndRow := fi.BigPixelsH - format.BorderBigPixels

		for row := dataStartRow; row < dataEndRow; row++ {
			for col := format.BorderBigPixels; col < fi.BigPixelsW-format.BorderBigPixels; col++ {
				gray := d.getBigPixelGrayWithBorder(img, col, row, ps, borderPx, borderX, borderY)
				frameData = append(frameData, gray)

				if uint32(len(frameData)) >= header.DataSize {
					frameData = frameData[:header.DataSize]
					break
				}
			}
			if uint32(len(frameData)) >= header.DataSize {
				break
			}
		}
	}

	d.log("Read %d bytes of frame data", len(frameData))

	actualCRC := crc32.ChecksumIEEE(frameData)
	d.log("CRC check: expected=0x%08x, actual=0x%08x", header.CRC, actualCRC)

	if !skipCRC && actualCRC != header.CRC {
		return nil, nil, fmt.Errorf("%w: expected 0x%08x, got 0x%08x", format.ErrCRCFailed, header.CRC, actualCRC)
	}

	return frameData, header, nil
}

func (d *Decoder) readDenseData(img image.Image, startRow, rows, maxColsPerRow, dataColsPerRow, bigPixelsH, px, borderPx, maxBytes int) []byte {
	return d.readDenseDataWithOffset(img, startRow, rows, maxColsPerRow, dataColsPerRow, bigPixelsH, px, borderPx, 0, 0, maxBytes)
}

func (d *Decoder) readDenseDataWithOffset(img image.Image, startRow, rows, maxColsPerRow, dataColsPerRow, bigPixelsH, px, borderPx, borderX, borderY, maxBytes int) []byte {
	if dataColsPerRow <= 0 {
		return nil
	}

	var result []byte
	for row := startRow; row < startRow+rows && row < bigPixelsH-format.BorderBigPixels && len(result) < maxBytes; row++ {
		colsThisRow := maxColsPerRow

		bytesNeeded := maxBytes - len(result)
		nibblesNeeded := bytesNeeded * 2
		if nibblesNeeded < colsThisRow {
			colsThisRow = nibblesNeeded
		}

		col := format.BorderBigPixels
		for col < format.BorderBigPixels+colsThisRow && len(result) < maxBytes {
			highGray := d.getBigPixelGrayWithBorder(img, col, row, px, borderPx, borderX, borderY)
			highNibble := d.palette.Decode(highGray)

			col++
			if col < format.BorderBigPixels+colsThisRow && len(result) < maxBytes {
				lowGray := d.getBigPixelGrayWithBorder(img, col, row, px, borderPx, borderX, borderY)
				lowNibble := d.palette.Decode(lowGray)

				result = append(result, (highNibble<<4)|lowNibble)
			} else if len(result) < maxBytes {
				result = append(result, highNibble<<4)
			}
			col++
		}
	}
	return result
}

func (d *Decoder) readDenseDataWithBorder(img image.Image, startRow, rows, maxColsPerRow, dataColsPerRow, bigPixelsH, px, borderPx, borderX, borderY, maxBytes int) []byte {
	return d.readDenseDataWithOffset(img, startRow, rows, maxColsPerRow, dataColsPerRow, bigPixelsH, px, borderPx, borderX, borderY, maxBytes)
}

func (d *Decoder) LoadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()
	return png.Decode(f)
}

func DecodeFrames(dir string, skipCRC ...bool) (map[uint32][]byte, error) {
	return DecodeFramesWithLogger(dir, nil, skipCRC...)
}

func DecodeFramesWithLogger(dir string, logger func(string), skipCRC ...bool) (map[uint32][]byte, error) {
	if logger == nil {
		logger = func(string) {}
	}

	decoder := NewDecoderWithLogger(logger)
	logger(fmt.Sprintf("Decoding frames from directory: %s", dir))

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var frameFiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				frameFiles = append(frameFiles, filepath.Join(dir, entry.Name()))
			}
		}
	}

	logger(fmt.Sprintf("Found %d potential frame files", len(frameFiles)))
	sort.Strings(frameFiles)

	skip := len(skipCRC) > 0 && skipCRC[0]
	logger(fmt.Sprintf("CRC validation: %v", !skip))

	result := make(map[uint32][]byte)
	successCount := 0
	errorCount := 0

	for i, path := range frameFiles {
		logger(fmt.Sprintf("[%d/%d] Processing: %s", i+1, len(frameFiles), filepath.Base(path)))
		img, err := decoder.LoadImage(path)
		if err != nil {
			logger(fmt.Sprintf("  ERROR: failed to load image: %v", err))
			errorCount++
			continue
		}

		data, header, err := decoder.DecodeFrameWithSkip(img, skip)
		if err != nil {
			logger(fmt.Sprintf("  ERROR: failed to decode frame: %v", err))
			errorCount++
			continue
		}

		logger(fmt.Sprintf("  SUCCESS: frame %d, %d bytes", header.FrameNum, len(data)))
		result[header.FrameNum] = data
		successCount++
	}

	logger(fmt.Sprintf("Decode complete: %d success, %d errors", successCount, errorCount))
	return result, nil
}

func DecodeFramesFromPaths(paths []string, skipCRC ...bool) (map[uint32][]byte, error) {
	return DecodeFramesFromPathsWithLogger(paths, nil, skipCRC...)
}

func DecodeFramesFromPathsWithLogger(paths []string, logger func(string), skipCRC ...bool) (map[uint32][]byte, error) {
	if logger == nil {
		logger = func(string) {}
	}

	decoder := NewDecoderWithLogger(logger)
	logger(fmt.Sprintf("Decoding %d frames from paths", len(paths)))

	sort.Strings(paths)
	skip := len(skipCRC) > 0 && skipCRC[0]
	result := make(map[uint32][]byte)
	successCount := 0
	errorCount := 0

	for i, path := range paths {
		logger(fmt.Sprintf("[%d/%d] Processing: %s", i+1, len(paths), filepath.Base(path)))
		img, err := decoder.LoadImage(path)
		if err != nil {
			logger(fmt.Sprintf("  ERROR: failed to load: %v", err))
			errorCount++
			continue
		}

		data, header, err := decoder.DecodeFrameWithSkip(img, skip)
		if err != nil {
			logger(fmt.Sprintf("  ERROR: decode failed: %v", err))
			errorCount++
			continue
		}

		logger(fmt.Sprintf("  SUCCESS: frame %d, %d bytes", header.FrameNum, len(data)))
		result[header.FrameNum] = data
		successCount++
	}

	logger(fmt.Sprintf("Result: %d frames decoded successfully, %d errors", successCount, errorCount))
	return result, nil
}

func ReconstructFile(frames map[uint32][]byte, outputPath string) error {
	var frameNums []uint32
	for num := range frames {
		frameNums = append(frameNums, num)
	}
	sort.Slice(frameNums, func(i, j int) bool { return frameNums[i] < frameNums[j] })

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	for _, num := range frameNums {
		data := frames[num]
		if _, err := f.Write(data); err != nil {
			return fmt.Errorf("failed to write frame %d: %w", num, err)
		}
	}

	return nil
}

func ExtractFilenameFromFrames(framePaths []string) string {
	if len(framePaths) == 0 {
		return ""
	}
	decoder := NewDecoder()
	for _, path := range framePaths {
		img, err := decoder.LoadImage(path)
		if err != nil {
			continue
		}
		headerData, _, err := decoder.readHeaderFromImage(img)
		if err != nil || len(headerData) < format.HeaderSize {
			continue
		}
		header, err := format.ParseHeader(headerData)
		if err != nil {
			continue
		}
		if header.FrameNum == 0 && header.GetFilename() != "" {
			return header.GetFilename()
		}
	}
	return ""
}
