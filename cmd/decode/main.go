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
	fmt.Println("║     Telescope Decoder - Interactive    ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	input := readLine("Input (image file or video): ")
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
			fmt.Println("Image file detected.")
		}
	}

	output := readLine("Output file [decoded]: ")
	if output == "" {
		output = "decoded"
	}

	var fps float64 = 1.0

	if isVideo {
		fps = readFloat("FPS for extraction (0.5-30) [1.0]: ", 1.0)
		if fps < 0.5 {
			fps = 0.5
		}
		if fps > 30 {
			fps = 30
		}
	}

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
		return "Image"
	}())
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
			fmt.Println("Install ffmpeg: winget install ffmpeg")
			os.Exit(1)
		}
		input = tempDir
	}

	entries, err := os.ReadDir(input)
	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
		os.Exit(1)
	}

	var framePaths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			framePaths = append(framePaths, filepath.Join(input, entry.Name()))
		}
	}

	if len(framePaths) == 0 {
		fmt.Println("Error: no image frames found")
		os.Exit(1)
	}

	fmt.Printf("Found %d frame(s)\n", len(framePaths))

	logger := func(msg string) {
		fmt.Println("[LOG]", msg)
	}

	data, filename, err := codec.DecodeDirectory(input, logger)
	if err != nil {
		fmt.Printf("Error decoding directory: %v\n", err)
		os.Exit(1)
	}

	outputPath := output
	if output == "decoded" {
		os.MkdirAll("decoded", 0755)
		outputPath = filepath.Join("decoded", filename)
	} else if filepath.Ext(output) == "" {
		os.MkdirAll(output, 0755)
		outputPath = filepath.Join(output, filename)
	}

	if err := codec.SaveFile(data, outputPath); err != nil {
		fmt.Printf("Error saving %s: %v\n", outputPath, err)
		os.Exit(1)
	}

	fmt.Printf("Saved: %s (%d bytes)\n", outputPath, len(data))
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

	input := flag.String("i", "", "Input image file or directory with frames")
	output := flag.String("o", "decoded", "Output file or directory")
	isVideo := flag.Bool("video", false, "Input is a video file")
	flag.BoolVar(isVideo, "v", false, "Input is a video file")
	fps := flag.Float64("fps", 1.0, "FPS for video frame extraction")

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

	info, err := os.Stat(*input)
	if os.IsNotExist(err) {
		fmt.Printf("Error: '%s' not found\n", *input)
		os.Exit(1)
	}

	videoMode := *isVideo
	if !videoMode && !info.IsDir() {
		ext := strings.ToLower(filepath.Ext(*input))
		videoExts := map[string]bool{".mp4": true, ".avi": true, ".mkv": true, ".mov": true, ".webm": true, ".wmv": true}
		if videoExts[ext] {
			videoMode = true
		}
	}

	var tempDir string

	if videoMode {
		fmt.Println("Video file detected, extracting frames...")
		tempDir = filepath.Join(os.TempDir(), "telescope-frames")
		os.MkdirAll(tempDir, 0755)
		defer os.RemoveAll(tempDir)

		fmt.Printf("Extracting frames from %s at %.1f FPS...\n", *input, *fps)
		if err := detector.ExtractFramesFromVideo(*input, tempDir, *fps); err != nil {
			fmt.Printf("Error: ffmpeg not found or failed\n")
			fmt.Printf("Install ffmpeg: winget install ffmpeg\n")
			os.Exit(1)
		}
		*input = tempDir
	}

	entries, err := os.ReadDir(*input)
	if err == nil && len(entries) > 0 {
		var framePaths []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				framePaths = append(framePaths, filepath.Join(*input, entry.Name()))
			}
		}

		if len(framePaths) > 0 {
			fmt.Printf("Found %d frame(s)\n", len(framePaths))

			logger := func(msg string) {
				fmt.Println("[LOG]", msg)
			}

			data, filename, err := codec.DecodeDirectory(*input, logger)
			if err != nil {
				fmt.Printf("Error decoding directory: %v\n", err)
				os.Exit(1)
			}

			outputPath := *output
			if info, err := os.Stat(*output); err == nil && info.IsDir() {
				outputPath = filepath.Join(*output, filename)
			} else if *output == "decoded" || filepath.Ext(*output) == "" {
				os.MkdirAll(*output, 0755)
				outputPath = filepath.Join(*output, filename)
			}

			if err := codec.SaveFile(data, outputPath); err != nil {
				fmt.Printf("Error saving %s: %v\n", outputPath, err)
				os.Exit(1)
			}

			fmt.Printf("Saved: %s (%d bytes)\n", outputPath, len(data))
			return
		}
	}

	fmt.Printf("Decoding: %s\n", filepath.Base(*input))

	data, filename, err := codec.DecodeFile(*input, func(msg string) {
		fmt.Println("[LOG]", msg)
	})
	if err != nil {
		fmt.Printf("Error decoding: %v\n", err)
		os.Exit(1)
	}

	outputPath := *output
	if *output == "decoded" || filepath.Ext(*output) == "" {
		if filepath.Ext(*output) == "" && !strings.HasPrefix(*output, ".") {
			os.MkdirAll(*output, 0755)
			outputPath = filepath.Join(*output, filename)
		} else {
			outputPath = filename
		}
	}

	if err := codec.SaveFile(data, outputPath); err != nil {
		fmt.Printf("Error saving: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved: %s (%d bytes)\n", outputPath, len(data))
}
