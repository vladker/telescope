package codec

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"

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

	borderPx := format.BorderWidth
	pixelSize := 2

	borderC := img.At(0, 0)
	borderR, borderG, borderB, _ := borderC.RGBA()
	borderAvg := (borderR + borderG + borderB) / 3

	offsetY := 0
	if borderAvg < 10000 {
		for y := 0; y < height; y++ {
			c := img.At(borderPx, y)
			r, g, b, _ := c.RGBA()
			avg := (r + g + b) / 3
			if avg > 10000 {
				offsetY = y - borderPx
				break
			}
		}
	}

	templateStartY := borderPx + offsetY
	templateStartX := borderPx

	metaStartX := templateStartX + format.TemplateSize*pixelSize
	metaStartY := templateStartY + format.TemplateSize*pixelSize

	foundDataY := -1
	for y := metaStartY; y < metaStartY+50 && y < height; y++ {
		hasData := false
		for x := metaStartX; x < metaStartX+50 && x < width; x++ {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			avg := (r + g + b) / 3
			if avg > 10000 {
				hasData = true
				break
			}
		}
		if hasData {
			foundDataY = y
			break
		}
	}
	if foundDataY > metaStartY {
		templateStartY = foundDataY - format.TemplateSize*pixelSize
	}

	startMarker := format.Point{X: templateStartX, Y: templateStartY}

	templatePx := format.TemplateSize * pixelSize
	innerW := width - 2*borderPx - 2*templatePx
	innerH := height - 2*borderPx - 2*templatePx

	return format.FrameInfo{
		Width:       width,
		Height:      height,
		PixelSize:   pixelSize,
		BorderPx:    borderPx,
		DataCols:    innerW / pixelSize,
		DataRows:    innerH / pixelSize,
		StartMarker: startMarker,
		EndMarker:   format.Point{X: width - borderPx - templatePx, Y: height - borderPx - templatePx},
	}, nil
}

func (d *Decoder) IsTelescopeFrame(img image.Image) bool {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width < 50 || height < 50 {
		return false
	}

	for startX := 0; startX < 20; startX++ {
		for startY := 0; startY < 20; startY++ {
			mismatches := 0
			for row := 0; row < 9; row++ {
				for col := 0; col < 9; col++ {
					x := startX + col*2 + 1
					y := startY + row*2 + 1
					if x >= width || y >= height {
						mismatches++
						continue
					}
					c := img.At(x, y)
					r, g, b, _ := c.RGBA()
					avg := (r + g + b) / 3

					var neighborAvg uint32
					if col+1 < 9 {
						nc := img.At(x+2, y)
						nr, ng, nb, _ := nc.RGBA()
						neighborAvg = (nr + ng + nb) / 3
					} else if row+1 < 9 {
						ny := startY + (row+1)*2 + 1
						nc := img.At(x, ny)
						nr, ng, nb, _ := nc.RGBA()
						neighborAvg = (nr + ng + nb) / 3
					} else {
						neighborAvg = avg
					}

					isLighter := avg > neighborAvg
					expectedWhite := (row+col)%2 == 0
					if isLighter != expectedWhite {
						mismatches++
					}
				}
			}

			if mismatches <= 10 {
				return true
			}
		}
	}

	return false
}

func (d *Decoder) findBorder(img image.Image) (int, int) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	borderSize := format.BorderWidth
	threshold := uint32(10000)

	whiteRows := 0
	sampleX := []int{0, width / 4, width / 2, width * 3 / 4, width - 1}
	for y := 0; y < borderSize && y < height; y++ {
		whiteCount := 0
		for _, x := range sampleX {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			avg := (r + g + b) / 3
			if avg > threshold {
				whiteCount++
			}
		}
		if whiteCount >= len(sampleX)/2 {
			whiteRows++
		}
	}

	whiteCols := 0
	sampleY := []int{0, height / 4, height / 2, height * 3 / 4, height - 1}
	for x := 0; x < borderSize && x < width; x++ {
		whiteCount := 0
		for _, y := range sampleY {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			avg := (r + g + b) / 3
			if avg > threshold {
				whiteCount++
			}
		}
		if whiteCount >= len(sampleY)/2 {
			whiteCols++
		}
	}

	if whiteRows >= borderSize/2 || whiteCols >= borderSize/2 {
		return borderSize, borderSize
	}

	whiteRowsAt0 := 0
	for y := 0; y < 5 && y < height; y++ {
		whiteCount := 0
		for _, x := range sampleX {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			avg := (r + g + b) / 3
			if avg > threshold {
				whiteCount++
			}
		}
		if whiteCount >= len(sampleX)/2 {
			whiteRowsAt0++
		}
	}

	if whiteRowsAt0 >= 3 {
		return 0, 0
	}

	return -1, -1
}

func (d *Decoder) findTemplate(img image.Image, borderX, borderY int) (format.Point, int) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	d.log("findTemplate: searching from (%d,%d)", borderX, borderY)

	for startX := borderX; startX <= borderX+20; startX++ {
		for startY := borderY; startY <= borderY+20; startY++ {
			for ps := 1; ps <= 5; ps++ {
				templatePx := format.TemplateSize * ps
				if startX+templatePx > width || startY+templatePx > height {
					continue
				}

				mismatches := 0
				for row := 0; row < format.TemplateSize; row++ {
					for col := 0; col < format.TemplateSize; col++ {
						x := startX + col*ps + ps/2
						y := startY + row*ps + ps/2
						c := img.At(x, y)
						r, g, b, _ := c.RGBA()
						avg := (r + g + b) / 3

						var neighborAvg uint32
						if col+1 < format.TemplateSize {
							nx := startX + (col+1)*ps + ps/2
							nc := img.At(nx, y)
							nr, ng, nb, _ := nc.RGBA()
							neighborAvg = (nr + ng + nb) / 3
						} else if row+1 < format.TemplateSize {
							ny := startY + (row+1)*ps + ps/2
							nc := img.At(x, ny)
							nr, ng, nb, _ := nc.RGBA()
							neighborAvg = (nr + ng + nb) / 3
						} else {
							neighborAvg = avg
						}

						isLighter := avg > neighborAvg
						expectedWhite := (row+col)%2 == 0
						if isLighter != expectedWhite {
							mismatches++
						}
					}
				}

				if startX == borderX && startY == borderY {
					d.log("findTemplate: at (%d,%d) ps=%d mismatches=%d", startX, startY, ps, mismatches)
				}

				if mismatches <= 50 {
					return format.Point{X: startX, Y: startY}, ps
				}
			}
		}
	}

	return format.Point{}, 0
}

func (d *Decoder) matchTemplate(img image.Image, startX, startY, pixelSize int) bool {
	var values [9][9]uint32

	for row := 0; row < format.TemplateSize; row++ {
		for col := 0; col < format.TemplateSize; col++ {
			x := startX + col*pixelSize + pixelSize/2
			y := startY + row*pixelSize + pixelSize/2
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			values[row][col] = (r + g + b) / 3
		}
	}

	mismatches := 0
	for row := 0; row < format.TemplateSize; row++ {
		for col := 0; col < format.TemplateSize; col++ {
			expectedWhite := (row+col)%2 == 0
			currentVal := values[row][col]
			var neighborVal uint32
			if col+1 < format.TemplateSize {
				neighborVal = values[row][col+1]
			} else if row+1 < format.TemplateSize {
				neighborVal = values[row+1][col]
			} else {
				neighborVal = currentVal
			}

			isLighter := currentVal > neighborVal
			if isLighter != expectedWhite {
				mismatches++
			}
		}
	}

	return mismatches <= 20
}

func isWhite(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	avg := (r + g + b) / 3
	return avg > 8192
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
		r, g, b, _ := c.RGBA()
		avg := (r + g + b) / 3
		bitValue := 0
		if avg > 32768 {
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

	actualSize := len(result)
	if int(metaInfo.FileSize) < actualSize {
		actualSize = int(metaInfo.FileSize)
	}
	return result[:actualSize], nil
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

type FrameBlock struct {
	Index       int
	Data        []byte
	TotalBlocks int
	FileName    string
	FileSize    int
	IsFEC       bool
}

func DecodeDirectory(dirPath string, logger func(string)) ([]byte, string, error) {
	if logger == nil {
		logger = func(string) {}
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read directory: %w", err)
	}

	var pngFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".png" {
			pngFiles = append(pngFiles, filepath.Join(dirPath, entry.Name()))
		}
	}

	sort.Strings(pngFiles)

	if len(pngFiles) == 0 {
		return nil, "", fmt.Errorf("no PNG files found in directory")
	}

	logger(fmt.Sprintf("Found %d frame(s)", len(pngFiles)))

	decoder := NewDecoderWithLogger(logger)
	blocks := make([]*FrameBlock, 0, len(pngFiles))
	dataBlocks := make(map[int]*FrameBlock)
	fecBlocks := make([]*FrameBlock, 0)
	var totalBlocks int
	var fecBlocksCount int
	var fecGroupSize int
	var expectedFileName string
	var maxBlockSize int

	for _, path := range pngFiles {
		img, err := decoder.LoadImage(path)
		if err != nil {
			logger(fmt.Sprintf("Warning: failed to load %s: %v", path, err))
			continue
		}

		if !decoder.IsTelescopeFrame(img) {
			logger(fmt.Sprintf("Skipping non-telescope frame: %s", filepath.Base(path)))
			continue
		}

		fi, err := decoder.DetectFrameInfo(img)
		if err != nil {
			logger(fmt.Sprintf("Warning: failed to detect frame info for %s: %v", path, err))
			continue
		}

		data, metaInfo, err := decoder.DecodeImageWithMeta(img, fi)
		if err != nil {
			logger(fmt.Sprintf("Warning: failed to decode %s: %v", path, err))
			continue
		}

		if metaInfo.BitDepth == 0 || metaInfo.FileSize == 0 || metaInfo.DataRows > 1000 || metaInfo.DataCols > 2000 {
			continue
		}

		block := &FrameBlock{
			Index:       int(metaInfo.BlockIndex),
			Data:        data,
			TotalBlocks: int(metaInfo.TotalBlocks),
			FileName:    metaInfo.FileName,
			FileSize:    int(metaInfo.FileSize),
		}

		if int(metaInfo.BlockIndex) >= int(metaInfo.TotalBlocks)-int(metaInfo.FECBlocks) {
			block.IsFEC = true
			fecBlocks = append(fecBlocks, block)
			if fecBlocksCount == 0 {
				fecBlocksCount = int(metaInfo.FECBlocks)
				fecGroupSize = int(metaInfo.FECGroup)
			}
		} else {
			dataBlocks[block.Index] = block
			if len(data) > maxBlockSize {
				maxBlockSize = len(data)
			}
		}

		blocks = append(blocks, block)

		if totalBlocks == 0 {
			totalBlocks = block.TotalBlocks
			expectedFileName = block.FileName
		}

		logger(fmt.Sprintf("Decoded block %d/%d from %s", metaInfo.BlockIndex+1, metaInfo.TotalBlocks, filepath.Base(path)))
	}

	if len(blocks) == 0 {
		return nil, "", fmt.Errorf("no blocks successfully decoded")
	}

	dataBlocksCount := totalBlocks - fecBlocksCount

	missingCount := 0
	for i := 0; i < dataBlocksCount; i++ {
		if _, ok := dataBlocks[i]; !ok {
			missingCount++
		}
	}

	if missingCount > 0 && len(fecBlocks) > 0 {
		logger(fmt.Sprintf("Missing %d data block(s), attempting FEC recovery...", missingCount))

		recovered := recoverWithFEC(dataBlocks, fecBlocks, fecGroupSize, maxBlockSize)
		logger(fmt.Sprintf("Recovered %d block(s) using FEC", recovered))
	}

	result := make([]byte, 0, dataBlocksCount*maxBlockSize)
	missingBlocks := 0
	for i := 0; i < dataBlocksCount; i++ {
		block, ok := dataBlocks[i]
		if ok {
			result = append(result, block.Data...)
		} else {
			missingBlocks++
		}
	}

	if missingBlocks > 0 {
		logger(fmt.Sprintf("Warning: %d block(s) still missing after FEC recovery", missingBlocks))
	}

	if expectedFileName == "" {
		expectedFileName = "restored_file"
	}

	return result, expectedFileName, nil
}

func recoverWithFEC(dataBlocks map[int]*FrameBlock, fecBlocks []*FrameBlock, groupSize, maxBlockSize int) int {
	if len(fecBlocks) == 0 || groupSize == 0 {
		return 0
	}

	recovered := 0
	numGroups := (len(dataBlocks) + groupSize - 1) / groupSize

	for g := 0; g < numGroups && g < len(fecBlocks); g++ {
		fec := fecBlocks[g]
		if fec == nil || len(fec.Data) == 0 {
			continue
		}

		start := g * groupSize
		end := start + groupSize

		missing := -1
		for i := start; i < end && i < len(dataBlocks)+recovered; i++ {
			if _, ok := dataBlocks[i]; !ok {
				if missing == -1 {
					missing = i
				} else {
					break
				}
			}
		}

		if missing == -1 {
			continue
		}

		recoveredData := make([]byte, len(fec.Data))
		copy(recoveredData, fec.Data)

		for i := start; i < end; i++ {
			if block, ok := dataBlocks[i]; ok {
				for j := 0; j < len(recoveredData) && j < len(block.Data); j++ {
					recoveredData[j] ^= block.Data[j]
				}
			}
		}

		dataBlocks[missing] = &FrameBlock{
			Index:       missing,
			Data:        recoveredData,
			TotalBlocks: len(dataBlocks) + len(fecBlocks),
			IsFEC:       false,
		}
		recovered++
	}

	return recovered
}

func (d *Decoder) DetectFrameInfoFromFile(path string) (format.FrameInfo, error) {
	img, err := d.LoadImage(path)
	if err != nil {
		return format.FrameInfo{}, err
	}
	return d.DetectFrameInfo(img)
}

func (d *Decoder) DecodeImageWithMetaFromFile(path string, fi format.FrameInfo) ([]byte, *format.MetaInfo, error) {
	img, err := d.LoadImage(path)
	if err != nil {
		return nil, nil, err
	}
	return d.DecodeImageWithMeta(img, fi)
}
