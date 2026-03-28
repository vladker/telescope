package main

import (
	"image"
	"image/png"
	"os"
)

func SavePNG(img image.Image, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}
