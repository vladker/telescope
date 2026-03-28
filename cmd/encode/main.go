package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"telescope/internal/codec"
	"telescope/internal/format"
)

var reader = bufio.NewReader(os.Stdin)

func readLine(prompt string) string {
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func readInt(prompt string, defaultVal int) int {
	input := readLine(prompt)
	if input == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(input)
	if err != nil {
		return defaultVal
	}
	return val
}

func readChoice(prompt string, options []string, defaultIdx int) int {
	for i, opt := range options {
		if i == defaultIdx {
			fmt.Printf("  [%d] %s (default)\n", i+1, opt)
		} else {
			fmt.Printf("  [%d] %s\n", i+1, opt)
		}
	}
	input := readLine(prompt + " (1-" + fmt.Sprint(len(options)) + ") ")
	if input == "" {
		return defaultIdx
	}
	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(options) {
		return defaultIdx
	}
	return idx - 1
}

func interactiveMode() {
	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║     Telescope Encoder - Interactive    ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	input := readLine("Input file path: ")
	if input == "" {
		fmt.Println("Error: input file required")
		os.Exit(1)
	}
	if _, err := os.Stat(input); os.IsNotExist(err) {
		fmt.Printf("Error: file '%s' not found\n", input)
		os.Exit(1)
	}

	output := readLine("Output directory: ")
	if output == "" {
		output = filepath.Base(input) + "_frames"
		fmt.Printf("Using: %s\n", output)
	}

	width := readInt("Width (100-3840) [1920]: ", 1920)
	if width < 100 {
		width = 100
	}
	if width > 3840 {
		width = 3840
	}

	height := readInt("Height (100-2160) [1080]: ", 1080)
	if height < 100 {
		height = 100
	}
	if height > 2160 {
		height = 2160
	}

	pixelIdx := readChoice("Pixel size:", []string{"1x1 - Maximum density", "2x2 - Balanced (recommended)", "3x3 - Best for recording"}, 1)
	pixelSizes := []int{1, 2, 3}
	pixelSize := pixelSizes[pixelIdx]

	modeIdx := readChoice("Encoding mode:", []string{"Robust (8-bit + redundancy)", "Dense (4-bit, max density)"}, 0)
	modes := []string{"robust", "dense"}
	modeStr := modes[modeIdx]

	formatIdx := readChoice("Output format:", []string{"PNG (lossless, larger)", "JPEG (lossy, smaller)"}, 0)
	formats := []string{"png", "jpeg"}
	outFormat := formats[formatIdx]

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║           Configuration                 ║")
	fmt.Println("╠════════════════════════════════════════╣")
	fmt.Printf("║  Input:      %-26s║\n", truncate(input, 26))
	fmt.Printf("║  Output:     %-26s║\n", truncate(output, 26))
	fmt.Printf("║  Resolution: %-26s║\n", fmt.Sprintf("%dx%d", width, height))
	fmt.Printf("║  Pixel:      %-26s║\n", fmt.Sprintf("%dx%d", pixelSize, pixelSize))
	fmt.Printf("║  Mode:       %-26s║\n", modeStr)
	fmt.Printf("║  Format:     %-26s║\n", outFormat)
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	confirm := readLine("Start encoding? (Y/n): ")
	if strings.ToLower(confirm) == "n" {
		fmt.Println("Cancelled.")
		os.Exit(0)
	}

	ps := format.PixelSize(pixelSize)
	mode := format.ModeRobustValue
	if modeStr == "dense" {
		mode = format.ModeDenseValue
	}

	frames, err := codec.EncodeFile(input, output, width, height, ps, mode, outFormat, func(msg string) {
		fmt.Println("[LOG]", msg)
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nSuccessfully encoded to %d frames\n", frames)
	fmt.Printf("Output directory: %s\n", output)
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func main() {
	interactive := flag.Bool("I", false, "Interactive mode")
	flag.BoolVar(interactive, "interactive", false, "Interactive mode")

	input := flag.String("i", "", "Input file path (required)")
	output := flag.String("o", "", "Output directory for frames (required)")
	width := flag.Int("W", 1920, "Image width in pixels")
	height := flag.Int("H", 1080, "Image height in pixels")
	pixelSize := flag.Int("p", 2, "Big pixel size (1, 2, or 3)")
	modeStr := flag.String("m", "robust", "Encoding mode: dense or robust")
	formatStr := flag.String("f", "png", "Output format: png or jpeg")

	flag.Parse()

	if len(os.Args) == 1 || *interactive {
		interactiveMode()
		return
	}

	if *input == "" || *output == "" {
		fmt.Println("Error: -i and -o flags are required (or use -interactive)")
		flag.Usage()
		os.Exit(1)
	}

	ps := format.PixelSize(*pixelSize)
	if ps != format.Pixel1x1 && ps != format.Pixel2x2 && ps != format.Pixel3x3 {
		fmt.Printf("Error: invalid pixel size %d (must be 1, 2, or 3)\n", *pixelSize)
		os.Exit(1)
	}

	mode := format.ModeRobustValue
	if *modeStr == "dense" {
		mode = format.ModeDenseValue
	} else if *modeStr != "robust" {
		fmt.Printf("Error: invalid mode %s (must be 'dense' or 'robust')\n", *modeStr)
		os.Exit(1)
	}

	outFormat := *formatStr
	if outFormat != "png" && outFormat != "jpeg" && outFormat != "jpg" {
		fmt.Printf("Error: invalid format %s (must be 'png' or 'jpeg')\n", *formatStr)
		os.Exit(1)
	}
	if outFormat == "jpg" {
		outFormat = "jpeg"
	}

	fmt.Printf("Encoding configuration:\n")
	fmt.Printf("  Input: %s\n", *input)
	fmt.Printf("  Output: %s\n", *output)
	fmt.Printf("  Resolution: %dx%d\n", *width, *height)
	fmt.Printf("  Pixel size: %dx%d\n", *pixelSize, *pixelSize)
	fmt.Printf("  Mode: %s\n", *modeStr)
	fmt.Printf("  Format: %s\n", outFormat)
	fmt.Println()

	frames, err := codec.EncodeFile(*input, *output, *width, *height, ps, mode, outFormat, func(msg string) {
		fmt.Println("[LOG]", msg)
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully encoded to %d frames\n", frames)
}
