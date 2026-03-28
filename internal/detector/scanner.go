package detector

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Scanner struct {
	uniqueMode bool
}

func NewScanner(uniqueMode bool) *Scanner {
	return &Scanner{
		uniqueMode: uniqueMode,
	}
}

func (s *Scanner) ScanDirectory(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var frameFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			frameFiles = append(frameFiles, filepath.Join(dir, entry.Name()))
		}
	}

	sort.Strings(frameFiles)

	if s.uniqueMode {
		return s.extractUniqueFrames(frameFiles)
	}

	return frameFiles, nil
}

func (s *Scanner) extractUniqueFrames(paths []string) ([]string, error) {
	hashes := make(map[string]string)
	unique := make(map[string]bool)

	fmt.Printf("[SCANNER] Processing %d frames for uniqueness...\n", len(paths))
	for i, path := range paths {
		if i > 0 && i%5 == 0 {
			fmt.Printf("[SCANNER] Processed %d/%d frames...\n", i, len(paths))
		}
		img, err := s.loadImage(path)
		if err != nil {
			fmt.Printf("[SCANNER] Warning: failed to load %s: %v\n", path, err)
			continue
		}

		hash := s.hashImage(img)
		if _, ok := hashes[hash]; ok {
			continue
		}
		hashes[hash] = path
		unique[path] = true
	}

	fmt.Printf("[SCANNER] Found %d unique frames\n", len(unique))
	os.Stdout.Sync()

	var result []string
	for _, path := range paths {
		if unique[path] {
			result = append(result, path)
		}
	}

	return result, nil
}

func (s *Scanner) loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func (s *Scanner) hashImage(img image.Image) string {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	sampleStep := 10
	var hashData []byte

	for y := 0; y < height; y += sampleStep {
		for x := 0; x < width; x += sampleStep {
			r, g, b, _ := img.At(x, y).RGBA()
			gray := (uint8(r>>8) + uint8(g>>8) + uint8(b>>8)) / 3
			hashData = append(hashData, gray)
		}
	}

	hash := sha256.Sum256(hashData)
	return fmt.Sprintf("%x", hash)
}

func ExtractFramesFromVideo(videoPath, outputDir string, fps float64) error {
	os.MkdirAll(outputDir, 0755)

	cmd := exec.Command("ffmpeg", "-i", videoPath, "-vf", fmt.Sprintf("fps=%f", fps), filepath.Join(outputDir, "frame_%04d.png"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
