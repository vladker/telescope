package codec

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"telescope/internal/format"
)

type Encoder struct {
	width      int
	height     int
	pixelSize  format.PixelSize
	mode       format.Mode
	palette    format.DensePalette
	logger     func(string)
}

type EncoderOption func(*Encoder)

func WithPixelSize(ps format.PixelSize) EncoderOption {
	return func(e *Encoder) {
		e.pixelSize = ps
	}
}

func WithMode(m format.Mode) EncoderOption {
	return func(e *Encoder) {
		e.mode = m
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
		pixelSize: format.Pixel2x2,
		mode:      format.ModeRobustValue,
		palette:   format.DefaultDensePalette,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Encoder) FrameInfo() format.FrameInfo {
	return format.CalcFrameInfo(e.width, e.height, e.pixelSize, e.mode)
}

func (e *Encoder) EncodeChunk(data []byte, frameNum, totalFrames uint32, filename string) (*image.Gray, error) {
	fi := e.FrameInfo()
	e.log("EncodeChunk: width=%d, height=%d, pixelSize=%d, dataBigPixels=%d, dataLen=%d",
		e.width, e.height, e.pixelSize, fi.DataBigPixels, len(data))

	if fi.DataBigPixels <= 0 {
		return nil, fmt.Errorf("%w: image too small for encoding (dataBigPixels=%d)", format.ErrImageTooSmall, fi.DataBigPixels)
	}

	dataLen := len(data)
	if dataLen > fi.DataBigPixels {
		e.log("WARNING: data (%d bytes) exceeds capacity (%d bytes), truncating", dataLen, fi.DataBigPixels)
		dataLen = fi.DataBigPixels
	}

	img := image.NewGray(image.Rect(0, 0, e.width, e.height))
	e.drawBorder(img)
	e.drawMetaHeader(img, data[:dataLen], frameNum, totalFrames, filename, uint32(len(data)))
	e.drawData(img, data[:dataLen], frameNum, totalFrames)

	return img, nil
}

func (e *Encoder) drawBorder(img *image.Gray) {
	bounds := img.Bounds()
	px := int(e.pixelSize)
	fi := e.FrameInfo()

	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			bigY := y / px
			bigX := x / px
			isBorder := bigX < format.BorderBigPixels || bigX >= fi.BigPixelsW-format.BorderBigPixels ||
				bigY < format.BorderBigPixels || bigY >= fi.BigPixelsH-format.BorderBigPixels

			if isBorder {
				isWhite := (bigX+bigY)%2 == 0
				c := uint8(0)
				if isWhite {
					c = format.CalibrationBits
				}
				img.SetGray(x, y, color.Gray{Y: c})
			}
		}
	}
}

func (e *Encoder) drawMetaHeader(img *image.Gray, data []byte, frameNum, totalFrames uint32, filename string, fileSize uint32) {
	fi := e.FrameInfo()
	px := int(e.pixelSize)

	header := format.NewHeader(fileSize, frameNum, totalFrames, uint32(len(data)))
	header.SetCRC(data)
	if frameNum == 0 {
		header.SetFilename(filename)
	}
	headerData := header.Serialize()

	if e.mode == format.ModeDenseValue {
		e.drawDenseData(img, headerData, format.BorderBigPixels, fi.BigPixelsW, fi.BigPixelsH, px)
	} else {
		dataOffset := 0
		for row := format.BorderBigPixels; row < format.BorderBigPixels*2 && row < fi.BigPixelsH; row++ {
			for col := format.BorderBigPixels; col < fi.BigPixelsW-format.BorderBigPixels && dataOffset < len(headerData); col++ {
				gray := headerData[dataOffset]
				e.fillBigPixel(img, col, row, gray, px)
				dataOffset++
			}
		}
	}
}

func (e *Encoder) drawData(img *image.Gray, data []byte, frameNum, totalFrames uint32) {
	fi := e.FrameInfo()
	px := int(e.pixelSize)

	metaRows := format.BorderBigPixels
	if e.mode == format.ModeDenseValue {
		metaRows = (format.HeaderSize * 2) / (fi.BigPixelsW - 2*format.BorderBigPixels)
		if (format.HeaderSize*2)%(fi.BigPixelsW-2*format.BorderBigPixels) != 0 {
			metaRows++
		}
	}

	if e.mode == format.ModeDenseValue {
		e.drawDenseData(img, data, metaRows, fi.BigPixelsW, fi.BigPixelsH, px)
	} else {
		metaEndRow := format.BorderBigPixels * 2
		dataCols := fi.BigPixelsW - 2*format.BorderBigPixels
		rowsNeeded := (len(data) + dataCols - 1) / dataCols
		dataEndRow := metaEndRow + rowsNeeded
		if dataEndRow > fi.BigPixelsH-format.BorderBigPixels {
			dataEndRow = fi.BigPixelsH - format.BorderBigPixels
		}
		dataOffset := 0
		for row := metaEndRow; row < dataEndRow && dataOffset < len(data); row++ {
			for col := format.BorderBigPixels; col < fi.BigPixelsW-format.BorderBigPixels && dataOffset < len(data); col++ {
				gray := data[dataOffset]
				e.fillBigPixel(img, col, row, gray, px)
				dataOffset++
			}
		}
	}
}

func (e *Encoder) drawDenseData(img *image.Gray, data []byte, startRow, bigPixelsW, bigPixelsH, px int) (rowsUsed int) {
	colsPerRow := bigPixelsW - 2*format.BorderBigPixels
	if colsPerRow <= 0 {
		return 0
	}

	dataOffset := 0
	for row := startRow; row < bigPixelsH-format.BorderBigPixels && dataOffset < len(data); row++ {
		rowsUsed++
		for col := format.BorderBigPixels; col < bigPixelsW-format.BorderBigPixels && dataOffset < len(data); col++ {
			byteVal := data[dataOffset]
			highNibble := (byteVal >> 4) & 0x0F
			lowNibble := byteVal & 0x0F

			e.fillBigPixel(img, col, row, e.palette[highNibble], px)
			dataOffset++

			if dataOffset < len(data) && col+1 < bigPixelsW-format.BorderBigPixels {
				byteVal = data[dataOffset]
				highNibble = (byteVal >> 4) & 0x0F
				lowNibble = byteVal & 0x0F
				e.fillBigPixel(img, col+1, row, e.palette[lowNibble], px)
				dataOffset++
			}
		}
	}
	return rowsUsed
}

func (e *Encoder) fillBigPixel(img *image.Gray, bigX, bigY int, gray uint8, size int) {
	fi := e.FrameInfo()
	borderPx := fi.BorderW
	startX := borderPx + bigX*size
	startY := borderPx + bigY*size

	for dy := 0; dy < size && startY+dy < e.height; dy++ {
		for dx := 0; dx < size && startX+dx < e.width; dx++ {
			img.SetGray(startX+dx, startY+dy, color.Gray{Y: gray})
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

func (e *Encoder) SaveImageJPEG(img *image.Gray, path string, quality int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	rgba := image.NewRGBA(img.Bounds())
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			gray := img.GrayAt(x, y).Y
			rgba.Set(x, y, color.Gray{Y: gray})
		}
	}

	// Use standard library's jpeg encoder
	return jpeg.Encode(f, rgba, &jpeg.Options{Quality: quality})
}

type EncodeLogger func(string)

func EncodeFile(inputPath, outputDir string, width, height int, pixelSize format.PixelSize, mode format.Mode, format_ string, logger EncodeLogger) (int, error) {
	if logger == nil {
		logger = func(string) {}
	}

	logger(fmt.Sprintf("Starting encode: input=%s, output=%s, width=%d, height=%d, pixelSize=%d, mode=%d, format=%s",
		inputPath, outputDir, width, height, pixelSize, mode, format_))

	file, err := os.Open(inputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat input file: %w", err)
	}
	totalSize := uint32(info.Size())
	logger(fmt.Sprintf("File size: %d bytes", totalSize))

	encoder := NewEncoder(width, height, WithPixelSize(pixelSize), WithMode(mode))
	fi := encoder.FrameInfo()
	logger(fmt.Sprintf("FrameInfo: bigPixelsW=%d, bigPixelsH=%d, dataBigPixels=%d, borderW=%d",
		fi.BigPixelsW, fi.BigPixelsH, fi.DataBigPixels, fi.BorderW))

	if fi.DataBigPixels <= 0 {
		return 0, fmt.Errorf("%w: calculated dataBigPixels=%d is too small", format.ErrImageTooSmall, fi.DataBigPixels)
	}

	dataPerFrame := uint32(fi.DataBigPixels)
	if mode == format.ModeDenseValue {
		dataPerFrame = uint32(fi.DataBigPixels) / 2
	}
	logger(fmt.Sprintf("Data per frame: %d bytes", dataPerFrame))

	totalFrames := (totalSize + dataPerFrame - 1) / dataPerFrame
	if totalFrames == 0 {
		totalFrames = 1
	}
	logger(fmt.Sprintf("Total frames needed: %d", totalFrames))

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create output directory: %w", err)
	}

	buf := make([]byte, dataPerFrame)
	frameNum := uint32(0)

	for {
		n, err := file.Read(buf)
		if n == 0 && err == io.EOF {
			break
		}
		if err != nil && err != io.EOF {
			return int(frameNum), fmt.Errorf("failed to read file: %w", err)
		}

		chunk := buf[:n]
		logger(fmt.Sprintf("Encoding frame %d/%d with %d bytes", frameNum+1, totalFrames, n))

		img, err := encoder.EncodeChunk(chunk, frameNum, totalFrames, filepath.Base(inputPath))
		if err != nil {
			return int(frameNum), fmt.Errorf("failed to encode chunk: %w", err)
		}

		frameFilename := fmt.Sprintf("frame_%04d.%s", frameNum, format_)
		framePath := filepath.Join(outputDir, frameFilename)

		if format_ == "png" {
			if err := encoder.SaveImage(img, framePath); err != nil {
				return int(frameNum) + 1, fmt.Errorf("failed to save PNG: %w", err)
			}
		} else {
			if err := encoder.SaveImageJPEG(img, framePath, 95); err != nil {
				return int(frameNum) + 1, fmt.Errorf("failed to save JPEG: %w", err)
			}
		}

		logger(fmt.Sprintf("Saved frame %d to %s", frameNum, frameFilename))
		frameNum++
	}

	logger(fmt.Sprintf("Encoding complete: %d frames saved to %s", frameNum, outputDir))
	return int(frameNum), nil
}
