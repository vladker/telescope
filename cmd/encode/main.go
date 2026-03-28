package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"telescope/internal/codec"
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

	output := readLine("Output directory [frames]: ")
	if output == "" {
		output = "frames"
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

	pixelSize := readInt("Pixel size (1-10) [2]: ", 2)
	if pixelSize < 1 {
		pixelSize = 1
	}
	if pixelSize > 10 {
		pixelSize = 10
	}

	bitDepth := readChoice("Bit depth:", []string{"1-bit (black/white)", "2-bit (4 levels)", "4-bit (16 levels)", "8-bit (256 levels)"}, 0)
	bitDepths := []int{1, 2, 4, 8}
	bd := bitDepths[bitDepth]

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║           Configuration                 ║")
	fmt.Println("╠════════════════════════════════════════╣")
	fmt.Printf("║  Input:      %-26s║\n", truncate(input, 26))
	fmt.Printf("║  Output:     %-26s║\n", truncate(output, 26))
	fmt.Printf("║  Resolution: %-26s║\n", fmt.Sprintf("%dx%d", width, height))
	fmt.Printf("║  Pixel:      %-26s║\n", fmt.Sprintf("%dx%d", pixelSize, pixelSize))
	fmt.Printf("║  Bit depth:  %-26s║\n", fmt.Sprintf("%d-bit", bd))
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	confirm := readLine("Start encoding? (Y/n): ")
	if strings.ToLower(confirm) == "n" {
		fmt.Println("Cancelled.")
		os.Exit(0)
	}

	if err := codec.EncodeFileToDir(input, output, width, height, pixelSize, bd, func(msg string) {
		fmt.Println("[LOG]", msg)
	}); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nSuccessfully encoded to: %s\n", output)
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
	output := flag.String("o", "frames", "Output directory for frames")
	width := flag.Int("W", 1920, "Image width in pixels")
	height := flag.Int("H", 1080, "Image height in pixels")
	pixelSize := flag.Int("p", 2, "Pixel size (number of image pixels per data point)")
	bitDepth := flag.Int("b", 1, "Bit depth (1, 2, 4, or 8)")

	flag.Parse()

	if len(os.Args) == 1 || *interactive {
		interactiveMode()
		return
	}

	if *input == "" {
		fmt.Println("Error: -i flag is required (or use -interactive)")
		flag.Usage()
		os.Exit(1)
	}

	if *pixelSize < 1 {
		fmt.Printf("Error: invalid pixel size %d (must be >= 1)\n", *pixelSize)
		os.Exit(1)
	}

	if *bitDepth != 1 && *bitDepth != 2 && *bitDepth != 4 && *bitDepth != 8 {
		fmt.Printf("Error: invalid bit depth %d (must be 1, 2, 4, or 8)\n", *bitDepth)
		os.Exit(1)
	}

	outputDir := *output

	fmt.Printf("Encoding configuration:\n")
	fmt.Printf("  Input: %s\n", *input)
	fmt.Printf("  Output: %s\n", outputDir)
	fmt.Printf("  Resolution: %dx%d\n", *width, *height)
	fmt.Printf("  Pixel size: %dx%d\n", *pixelSize, *pixelSize)
	fmt.Printf("  Bit depth: %d\n", *bitDepth)
	fmt.Println()

	if err := codec.EncodeFileToDir(*input, outputDir, *width, *height, *pixelSize, *bitDepth, func(msg string) {
		fmt.Println("[LOG]", msg)
	}); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully encoded to: %s\n", outputDir)
}
