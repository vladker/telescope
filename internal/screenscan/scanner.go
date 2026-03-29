// Package screenscan предоставляет функции для попиксельного сканирования экрана
package screenscan

import (
	"fmt"
	"image/color"
	"time"

	"github.com/kbinani/screenshot"
)

// Pixel представляет значение одного пикселя экрана
type Pixel struct {
	X int `json:"x"`
	Y int `json:"y"`
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
	A uint8 `json:"a"`
}

// ScreenCapture содержит результаты захвата экрана
type ScreenCapture struct {
	Timestamp string  `json:"timestamp"`
	Width     int     `json:"screen_width"`
	Height    int     `json:"screen_height"`
	Pixels    []Pixel `json:"pixels"`
}

// CaptureScreen выполняет попиксельный захват всего экрана
// Возвращает массив пикселей с точными значениями цвета без сжатия
func CaptureScreen() (*ScreenCapture, error) {
	// Получаем все мониторы
	bounds := screenshot.GetDisplayBounds(0)
	width := bounds.Dx()
	height := bounds.Dy()

	// Делаем скриншот с нативным разрешением экрана
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, fmt.Errorf("ошибка захвата экрана: %w", err)
	}

	// Преобразуем в массив пикселей
	pixels := make([]Pixel, 0, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.At(x, y)
			r, g, b, a := c.RGBA()
			pixel := Pixel{
				X: x,
				Y: y,
				R: uint8(r >> 8), // RGBA() возвращает 16-битные значения
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			}
			pixels = append(pixels, pixel)
		}
	}

	return &ScreenCapture{
		Timestamp: time.Now().Format("020106_15_04_05"),
		Width:     width,
		Height:    height,
		Pixels:    pixels,
	}, nil
}

// GetScreenSize возвращает текущие размеры основного экрана
func GetScreenSize() (width, height int, err error) {
	bounds := screenshot.GetDisplayBounds(0)
	if bounds.Empty() {
		return 0, 0, fmt.Errorf("не удалось получить размеры экрана")
	}
	return bounds.Dx(), bounds.Dy(), nil
}

// GetPixelColor получает цвет конкретного пикселя на экране
func GetPixelColor(x, y int) (color.RGBA, error) {
	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("ошибка захвата экрана: %w", err)
	}

	if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return color.RGBA{}, fmt.Errorf("координаты вне диапазона экрана")
	}

	c := img.At(x, y)
	r, g, b, a := c.RGBA()
	return color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}, nil
}
