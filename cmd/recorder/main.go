package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/kbinani/screenshot"
)

func main() {
	outputDir := flag.String("output", "capture", "output directory for unique frames")
	interval := flag.Int("interval", 100, "check interval in milliseconds")
	display := flag.Int("display", 0, "display index (0 for primary)")
	minDiff := flag.Int("min-diff", 1, "minimum pixel difference to consider frame changed (0-255)")
	width := flag.Int("width", 0, "capture width (0 = full screen)")
	height := flag.Int("height", 0, "capture height (0 = full screen)")
	flag.Parse()

	if *interval < 10 {
		fmt.Println("interval must be at least 10ms")
		os.Exit(1)
	}

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	fullBounds := screenshot.GetDisplayBounds(*display)
	if *width == 0 {
		*width = fullBounds.Dx()
	}
	if *height == 0 {
		*height = fullBounds.Dy()
	}

	captureBounds := image.Rect(fullBounds.Min.X, fullBounds.Min.Y, fullBounds.Min.X+*width, fullBounds.Min.Y+*height)

	runtime.LockOSThread()

	done := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		close(done)
	}()

	fmt.Printf("Starting recorder...\n")
	fmt.Printf("  Output: %s\n", *outputDir)
	fmt.Printf("  Interval: %dms\n", *interval)
	fmt.Printf("  Display: %d\n", *display)
	fmt.Printf("  Capture: %dx%d (from %dx%d)\n", *width, *height, fullBounds.Dx(), fullBounds.Dy())
	fmt.Printf("  Min diff: %d\n", *minDiff)
	fmt.Printf("\nPress Ctrl+C to stop.\n\n")

	frames, err := NewDedupChecker(*minDiff)
	if err != nil {
		log.Fatalf("failed to create dedup checker: %v", err)
	}
	defer frames.Close()

	counter := 0
	uniqueCounter := 0
	lastSave := time.Now()

	for {
		select {
		case <-done:
			fmt.Printf("\n\nStopped. Saved %d unique frames out of %d total.\n", uniqueCounter, counter)
			return
		default:
		}

		counter++

		img, err := screenshot.CaptureRect(captureBounds)
		if err != nil {
			log.Printf("capture error: %v", err)
			time.Sleep(time.Duration(*interval) * time.Millisecond)
			continue
		}

		hash, err := frames.Add(img)
		if err != nil {
			if err == ErrDuplicate {
				time.Sleep(time.Duration(*interval) * time.Millisecond)
				continue
			}
			log.Printf("dedup error: %v", err)
			time.Sleep(time.Duration(*interval) * time.Millisecond)
			continue
		}

		filename := fmt.Sprintf("%s/%09d_%08x.png", *outputDir, uniqueCounter, hash)
		if err := SavePNG(img, filename); err != nil {
			log.Printf("save error: %v", err)
			time.Sleep(time.Duration(*interval) * time.Millisecond)
			continue
		}

		uniqueCounter++
		since := time.Since(lastSave)
		fmt.Printf("\rSaved frame %d (hash: %08x) - %d unique / %d total (%.1fs since last save)   ",
			uniqueCounter, hash, uniqueCounter, counter, since.Seconds())
		lastSave = time.Now()

		time.Sleep(time.Duration(*interval) * time.Millisecond)
	}
}
