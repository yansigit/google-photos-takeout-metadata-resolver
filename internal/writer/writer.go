package writer

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gp-takeout-resolver/internal/matcher"
)

// Writer handles copying media files and writing metadata.
type Writer struct {
	outputDir    string
	inputRootDir string
	exiftool     *ExifToolBatch
	dryRun       bool
	logger       *slog.Logger
}

// NewWriter creates a new Writer instance.
func NewWriter(outputDir, inputRootDir string, exiftool *ExifToolBatch, dryRun bool, logger *slog.Logger) *Writer {
	return &Writer{
		outputDir:    outputDir,
		inputRootDir: inputRootDir,
		exiftool:     exiftool,
		dryRun:       dryRun,
		logger:       logger,
	}
}

// ProcessResult holds the outcome of processing a single matched pair.
type ProcessResult struct {
	MediaPath  string
	OutputPath string
	Success    bool
	Error      error
}

// Process copies a media file to the output directory and writes metadata.
func (w *Writer) Process(match matcher.MatchResult) ProcessResult {
	// Compute relative path for preserving directory structure
	relDir, err := filepath.Rel(w.inputRootDir, match.FolderDir)
	if err != nil {
		relDir = filepath.Base(match.FolderDir)
	}

	mediaFilename := filepath.Base(match.MediaPath)
	outputDir := filepath.Join(w.outputDir, relDir)
	outputPath := filepath.Join(outputDir, mediaFilename)

	if w.dryRun {
		w.logger.Info("dry-run: would process",
			"media", match.MediaPath,
			"output", outputPath,
			"title", match.Meta.Title,
		)
		return ProcessResult{MediaPath: match.MediaPath, OutputPath: outputPath, Success: true}
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ProcessResult{
			MediaPath: match.MediaPath,
			Error:     fmt.Errorf("creating output dir %s: %w", outputDir, err),
		}
	}

	// Copy media file
	if err := copyFile(match.MediaPath, outputPath); err != nil {
		return ProcessResult{
			MediaPath: match.MediaPath,
			Error:     fmt.Errorf("copying %s: %w", match.MediaPath, err),
		}
	}

	// Write metadata via exiftool
	if err := w.exiftool.WriteMetadata(outputPath, match.Meta); err != nil {
		w.logger.Warn("exiftool write failed, file still copied",
			"file", outputPath,
			"error", err,
		)
		// Non-fatal: file is copied but metadata may not be written
	}

	// Set file modification time
	if takenTime, err := match.Meta.PhotoTakenTime.Time(); err == nil {
		os.Chtimes(outputPath, takenTime, takenTime)
	}

	return ProcessResult{MediaPath: match.MediaPath, OutputPath: outputPath, Success: true}
}

// CopyOrphan copies a media file without metadata to the output directory.
func (w *Writer) CopyOrphan(mediaPath string) ProcessResult {
	relPath, err := filepath.Rel(w.inputRootDir, mediaPath)
	if err != nil {
		relPath = filepath.Base(mediaPath)
	}

	outputPath := filepath.Join(w.outputDir, relPath)

	if w.dryRun {
		w.logger.Info("dry-run: would copy orphan", "media", mediaPath, "output", outputPath)
		return ProcessResult{MediaPath: mediaPath, OutputPath: outputPath, Success: true}
	}

	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ProcessResult{
			MediaPath: mediaPath,
			Error:     fmt.Errorf("creating output dir %s: %w", outputDir, err),
		}
	}

	if err := copyFile(mediaPath, outputPath); err != nil {
		return ProcessResult{
			MediaPath: mediaPath,
			Error:     fmt.Errorf("copying orphan %s: %w", mediaPath, err),
		}
	}

	return ProcessResult{MediaPath: mediaPath, OutputPath: outputPath, Success: true}
}

func copyFile(src, dst string) error {
	// Skip if source and destination are the same
	if src == dst {
		return nil
	}

	// Avoid overwriting if already exists
	if _, err := os.Stat(dst); err == nil {
		// Destination exists — append a suffix
		ext := filepath.Ext(dst)
		base := strings.TrimSuffix(dst, ext)
		for i := 1; ; i++ {
			candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				dst = candidate
				break
			}
		}
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}
