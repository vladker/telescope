package main

import (
	"fmt"
	"hash/crc32"
	"image"
	"sync"
)

var ErrDuplicate = fmt.Errorf("duplicate frame")

type DedupChecker struct {
	seen    map[uint32]struct{}
	mu      sync.RWMutex
	minDiff int
}

func NewDedupChecker(minDiff int) (*DedupChecker, error) {
	return &DedupChecker{
		seen:    make(map[uint32]struct{}),
		minDiff: minDiff,
	}, nil
}

func (d *DedupChecker) Add(img image.Image) (uint32, error) {
	hash := d.computeHash(img)

	d.mu.RLock()
	_, exists := d.seen[hash]
	d.mu.RUnlock()

	if exists {
		return hash, ErrDuplicate
	}

	d.mu.Lock()
	d.seen[hash] = struct{}{}
	d.mu.Unlock()

	return hash, nil
}

func (d *DedupChecker) computeHash(img image.Image) uint32 {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width <= 0 || height <= 0 {
		return 0
	}

	hash := crc32.NewIEEE()

	y0 := bounds.Min.Y
	y1 := bounds.Max.Y
	x0 := bounds.Min.X
	x1 := bounds.Max.X

	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a < 32768 {
				hash.Write([]byte{0, 0, 0, 0})
				continue
			}
			hash.Write([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8)})
		}
	}

	return hash.Sum32()
}

func (d *DedupChecker) Close() error {
	return nil
}

func (d *DedupChecker) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.seen)
}
