package format

import "errors"

var (
	ErrInvalidHeader    = errors.New("invalid header: too short")
	ErrInvalidSignature = errors.New("invalid signature")
	ErrInvalidVersion   = errors.New("invalid version")
	ErrCRCFailed        = errors.New("CRC validation failed")
	ErrNoBorderFound    = errors.New("calibration border not found")
	ErrInvalidPixelSize = errors.New("invalid pixel size")
	ErrInvalidMode      = errors.New("invalid mode")
	ErrImageTooSmall    = errors.New("image too small for encoding")
	ErrNoFramesFound    = errors.New("no frames found")
)
