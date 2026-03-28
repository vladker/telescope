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
	"telescope/internal/detector"
)

var reader = bufio.NewReader(os.Stdin)

func readLine(prompt string) string {
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func readBool(prompt string, defaultVal bool) bool {
	defaultStr := "N"
	if defaultVal {
		defaultStr = "Y"
	}
	input := readLine(prompt + " (" + defaultStr + "/n): ")
	if input == "" {
		return defaultVal
	}
	return strings.ToLower(input) != "n"
}

func readFloat(prompt string, defaultVal float64) float64 {
	input := readLine(prompt)
	if input == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(input, 64)
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
	fmt.Println("║     Telescope Decoder - Interactive     ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	input := readLine("Input (directory with frames or video file): ")
	if input == "" {
		fmt.Println("Error: input required")
		os.Exit(1)
	}

	info, err := os.Stat(input)
	if os.IsNotExist(err) {
		fmt.Printf("Error: '%s' not found\n", input)
		os.Exit(1)
	}

	isVideo := !info.IsDir()
	if !isVideo {
		ext := strings.ToLower(filepath.Ext(input))
		isVideo = ext == ".mp4" || ext == ".avi" || ext == ".mkv" || ext == ".mov"
		if isVideo {
			fmt.Println("Video file detected.")
		} else {
			fmt.Println("Directory detected.")
		}
	}

	output := readLine("Output file path: ")
	if output == "" {
		fmt.Println("Error: output file required")
		os.Exit(1)
	}

	var fps float64 = 1.0
	var unique bool = true
	var force bool = false

	if isVideo {
		fps = readFloat("FPS for extraction (0.5-30) [1.0]: ", 1.0)
		if fps < 0.5 {
			fps = 0.5
		}
		if fps > 30 {
			fps = 30
		}
	}

	unique = readBool("Extract unique frames only", true)
	force = !readBool("Validate CRC checksums", true)

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║           Configuration                 ║")
	fmt.Println("╠════════════════════════════════════════╣")
	fmt.Printf("║  Input:      %-26s║\n", truncate(input, 26))
	fmt.Printf("║  Output:     %-26s║\n", truncate(output, 26))
	fmt.Printf("║  Source:     %-26s║\n", func() string {
		if isVideo {
			return fmt.Sprintf("Video @ %.1f fps", fps)
		}
		return "Directory"
	}())
	fmt.Printf("║  Unique:     %-26s║\n", strconv.FormatBool(unique))
	fmt.Printf("║  CRC Check:  %-26s║\n", strconv.FormatBool(!force))
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	confirm := readLine("Start decoding? (Y/n): ")
	if strings.ToLower(confirm) == "n" {
		fmt.Println("Cancelled.")
		os.Exit(0)
	}

	var tempDir string
	if isVideo {
		fmt.Println("Extracting frames from video...")
		tempDir = filepath.Join(os.TempDir(), "telescope-frames")
		os.MkdirAll(tempDir, 0755)
		defer os.RemoveAll(tempDir)

		fmt.Printf("Extracting frames at %.1f FPS...\n", fps)
		if err := detector.ExtractFramesFromVideo(input, tempDir, fps); err != nil {
			fmt.Printf("Error extracting frames: %v\n", err)
			os.Exit(1)
		}
		input = tempDir
	}

	scanner := detector.NewScanner(unique)
	framePaths, err := scanner.ScanDirectory(input)
	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
		fmt.Println("\nPress Enter to exit...")
		reader.ReadString('\n')
		os.Exit(1)
	}

	if len(framePaths) == 0 {
		fmt.Println("Error: no frames found")
		fmt.Println("\nPress Enter to exit...")
		reader.ReadString('\n')
		os.Exit(1)
	}

	fmt.Printf("Found %d frames\n", len(framePaths))
	fmt.Println("Decoding frames...")

	frames, err := codec.DecodeFramesFromPathsWithLogger(framePaths, func(msg string) {
		fmt.Println("[LOG]", msg)
	}, force)
	if err != nil {
		fmt.Printf("Error decoding frames: %v\n", err)
		fmt.Println("\nPress Enter to exit...")
		reader.ReadString('\n')
		os.Exit(1)
	}

	if len(frames) == 0 {
		fmt.Println("Error: no valid frames decoded")
		fmt.Println("\nPress Enter to exit...")
		reader.ReadString('\n')
		os.Exit(1)
	}

	fmt.Printf("Successfully decoded %d frames\n", len(frames))
	fmt.Printf("Reconstructing file: %s\n", output)

	if err := codec.ReconstructFile(frames, output); err != nil {
		fmt.Printf("Error reconstructing file: %v\n", err)
		fmt.Println("\nPress Enter to exit...")
		reader.ReadString('\n')
		os.Exit(1)
	}

	info, _ = os.Stat(output)
	fmt.Printf("\nOutput file size: %d bytes\n", info.Size())
	fmt.Println("\nPress Enter to exit...")
	reader.ReadString('\n')
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

	input := flag.String("i", "", "Input directory with frames or video file (required)")
	output := flag.String("o", "", "Output file path (required)")
	isVideo := flag.Bool("video", false, "Input is a video file (requires ffmpeg)")
	fps := flag.Float64("fps", 1.0, "FPS for video frame extraction")
	unique := flag.Bool("unique", true, "Extract only unique frames")
	force := flag.Bool("force", false, "Skip CRC validation")

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

	var tempDir string
	var err error

	if *isVideo {
		fmt.Println("Video mode detected, extracting frames...")
		tempDir = filepath.Join(os.TempDir(), "telescope-frames")
		os.MkdirAll(tempDir, 0755)
		defer os.RemoveAll(tempDir)

		fmt.Printf("Extracting frames from %s at %.1f FPS...\n", *input, *fps)
		if err := detector.ExtractFramesFromVideo(*input, tempDir, *fps); err != nil {
			fmt.Printf("Error extracting frames: %v\n", err)
			os.Exit(1)
		}
		*input = tempDir
	}

	scanner := detector.NewScanner(*unique)
	framePaths, err := scanner.ScanDirectory(*input)
	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
		fmt.Println("\nPress Enter to exit...")
		reader.ReadString('\n')
		os.Exit(1)
	}

	if len(framePaths) == 0 {
		fmt.Println("Error: no frames found")
		fmt.Println("\nPress Enter to exit...")
		reader.ReadString('\n')
		os.Exit(1)
	}

	fmt.Printf("Found %d frames\n", len(framePaths))
	fmt.Println("Decoding frames...")

	frames, err := codec.DecodeFramesFromPathsWithLogger(framePaths, func(msg string) {
		fmt.Println("[LOG]", msg)
	}, *force)
	if err != nil {
		fmt.Printf("Error decoding frames: %v\n", err)
		fmt.Println("\nPress Enter to exit...")
		reader.ReadString('\n')
		os.Exit(1)
	}

	if len(frames) == 0 {
		fmt.Println("Error: no valid frames decoded")
		fmt.Println("\nPress Enter to exit...")
		reader.ReadString('\n')
		os.Exit(1)
	}

	fmt.Printf("Successfully decoded %d frames\n", len(frames))
	fmt.Printf("Reconstructing file: %s\n", *output)

	if err := codec.ReconstructFile(frames, *output); err != nil {
		fmt.Printf("Error reconstructing file: %v\n", err)
		fmt.Println("\nPress Enter to exit...")
		reader.ReadString('\n')
		os.Exit(1)
	}

	info, _ := os.Stat(*output)
	fmt.Printf("Output file size: %d bytes\n", info.Size())
	fmt.Println("\nPress Enter to exit...")
	reader.ReadString('\n')
}
