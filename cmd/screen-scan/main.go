// Программа screen-scan выполняет попиксельное сканирование экрана
// по нажатию двух Shift одновременно и сохраняет результаты в JSON
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"telescope/internal/screenscan"
)

const (
	// Коды клавиш
	VK_LSHIFT = 0xA0
	VK_RSHIFT = 0xA1
	VK_LCTRL  = 0xA2
	VK_RCTRL  = 0xA3

	// MB_OK для MessageBeep
	MB_OK = 0x00000000
)

var (
	mu            sync.Mutex
	scanning      bool
	lastComboTime time.Time
	scanDir       string // Папка сессии
	scanCounter   int    // Счётчик сканирований в сессии

	// Загружаем функции из user32.dll
	user32   = syscall.NewLazyDLL("user32.dll")
	getKeyStateProc = user32.NewProc("GetKeyState")
	messageBeepProc = user32.NewProc("MessageBeep")
)

func main() {
	fmt.Println("Screen Scanner - попиксельное сканирование экрана")
	fmt.Println("Нажмите Ctrl+Shift для сканирования")
	fmt.Println("Нажмите Ctrl+C для выхода")
	fmt.Println()

	fmt.Println("Горячая клавиша (Ctrl+Shift) активирована. Ожидание нажатия...")

	// Основной цикл опроса клавиш
	pollShiftKeys()
}

// getKeyState получает состояние клавиши
func getKeyState(vk int) int16 {
	ret, _, _ := getKeyStateProc.Call(uintptr(vk))
	return int16(ret)
}

// pollShiftKeys опрашивает состояние клавиш Ctrl+Shift
func pollShiftKeys() {
	for {
		time.Sleep(50 * time.Millisecond)

		// Проверяем состояние Ctrl и Shift
		leftShift := (getKeyState(VK_LSHIFT) & 0x80) != 0
		rightShift := (getKeyState(VK_RSHIFT) & 0x80) != 0
		leftCtrl := (getKeyState(VK_LCTRL) & 0x80) != 0
		rightCtrl := (getKeyState(VK_RCTRL) & 0x80) != 0

		// Любая комбинация Ctrl+Shift
		shiftPressed := leftShift || rightShift
		ctrlPressed := leftCtrl || rightCtrl
		comboPressed := shiftPressed && ctrlPressed

		mu.Lock()
		if comboPressed && !scanning {
			now := time.Now()

			// Проверяем, что комбинация не была нажата ранее (защита от повторов)
			if now.Sub(lastComboTime) > 500*time.Millisecond {
				lastComboTime = now

				fmt.Println("\n[!] Запуск сканирования экрана...")
				mu.Unlock()

				if err := performScan(); err != nil {
					fmt.Fprintf(os.Stderr, "Ошибка сканирования: %v\n", err)
				} else {
					fmt.Println("[✓] Сканирование завершено успешно!")
				}

				mu.Lock()
			}
		}
		mu.Unlock()
	}
}

// performScan выполняет сканирование экрана и сохранение результатов
func performScan() error {
	// Захват экрана
	fmt.Println("Захват изображения экрана...")
	startTime := time.Now()

	capture, err := screenscan.CaptureScreen()
	if err != nil {
		return fmt.Errorf("ошибка захвата экрана: %w", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("Захват выполнен за %v\n", elapsed)
	fmt.Printf("Размеры экрана: %d x %d\n", capture.Width, capture.Height)
	fmt.Printf("Всего пикселей: %d\n", len(capture.Pixels))

	mu.Lock()
	
	// Создаём папку сессии при первом сканировании
	if scanDir == "" {
		timestamp := time.Now().Format("020106_15_04_05")
		scanDir = fmt.Sprintf("scanned_%s", timestamp)
		if err := os.MkdirAll(scanDir, 0755); err != nil {
			mu.Unlock()
			return fmt.Errorf("ошибка создания директории: %w", err)
		}
		fmt.Printf("Создана папка сессии: %s\n", scanDir)
	}

	// Формируем имя файла: scan_0000.json, scan_0001.json, ...
	scanFile := fmt.Sprintf("scan_%04d.json", scanCounter)
	scanCounter++
	
	mu.Unlock()

	// Сохраняем в JSON
	jsonPath := filepath.Join(scanDir, scanFile)

	fmt.Println("Сериализация данных в JSON...")
	jsonData, err := json.MarshalIndent(capture, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка сериализации JSON: %w", err)
	}

	fmt.Printf("Запись файла: %s\n", jsonPath)
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return fmt.Errorf("ошибка записи файла: %w", err)
	}

	fmt.Printf("Данные сохранены в: %s\n", jsonPath)
	fmt.Printf("Размер файла: %d байт\n", len(jsonData))

	// Звуковой сигнал завершения
	playCompletionSound()

	return nil
}

// playCompletionSound воспроизводит звуковой сигнал завершения
func playCompletionSound() {
	fmt.Println("Воспроизведение звукового сигнала...")

	// Используем MessageBeep для системного звука
	messageBeepProc.Call(uintptr(MB_OK))
}
