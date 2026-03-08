package writer

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"gp-takeout-resolver/internal/metadata"
)

// videoExtensions are file extensions for video formats.
var videoExtensions = map[string]bool{
	".mov": true, ".mp4": true, ".avi": true, ".mkv": true,
	".m4v": true, ".3gp": true, ".wmv": true, ".mpg": true,
	".mpeg": true, ".mts": true, ".m2ts": true,
}

// ExifToolBatch manages a persistent exiftool process using -stay_open mode.
type ExifToolBatch struct {
	cmd          *exec.Cmd
	exiftoolPath string
	stdin        io.WriteCloser
	stdout       *bufio.Scanner
	stderr       *bufio.Scanner
	mu           sync.Mutex
}

// NewExifToolBatch starts a persistent exiftool process.
func NewExifToolBatch(exiftoolPath string) (*ExifToolBatch, error) {
	cmd := exec.Command(exiftoolPath, "-stay_open", "True", "-@", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("starting exiftool: %w", err)
	}

	return &ExifToolBatch{
		cmd:          cmd,
		exiftoolPath: exiftoolPath,
		stdin:        stdin,
		stdout:       bufio.NewScanner(stdout),
		stderr:       bufio.NewScanner(stderr),
	}, nil
}

// WriteMetadata writes EXIF metadata to the file at mediaPath.
// This method is NOT concurrency-safe — each goroutine should own its own ExifToolBatch.
func (e *ExifToolBatch) WriteMetadata(mediaPath string, meta *metadata.Metadata) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	args := buildExifArgs(meta, mediaPath)
	if err := e.execBatch(args); err == nil {
		return nil
	}

	// Retry with -F flag to fix corrupt metadata (e.g. truncated EXIF IFD)
	if err := e.execBatch(addFixFlag(args)); err == nil {
		return nil
	}

	// For video files, retry with video-compatible tags only
	// (skip EXIF-only tags like ImageDescription).
	ext := strings.ToLower(filepath.Ext(mediaPath))
	if videoExtensions[ext] {
		videoArgs := buildVideoExifArgs(meta, mediaPath)
		if err := e.execBatch(videoArgs); err == nil {
			return nil
		}
		if err := e.execBatch(addFixFlag(videoArgs)); err == nil {
			return nil
		}
	}

	// Last resort: try setting only FileModifyDate (filesystem-level, works on corrupt files)
	if takenTime, err := meta.PhotoTakenTime.ExifString(); err == nil && takenTime != "1970:01:01 00:00:00" {
		minArgs := []string{"-m", "-F", "-overwrite_original", "-FileModifyDate=" + takenTime, mediaPath}
		if err := e.execBatch(minArgs); err == nil {
			return nil
		}
	}

	// All batch attempts failed. Run a one-off command without -m to get the real error.
	return e.diagnoseFailed(args)
}

// execBatch sends arguments to the persistent exiftool process and checks the result.
func (e *ExifToolBatch) execBatch(args []string) error {
	for _, arg := range args {
		if _, err := fmt.Fprintln(e.stdin, arg); err != nil {
			return fmt.Errorf("writing to exiftool stdin: %w", err)
		}
	}
	// Add stderr sync marker so we can read stderr reliably
	if _, err := fmt.Fprintln(e.stdin, "-echo2"); err != nil {
		return fmt.Errorf("writing to exiftool stdin: %w", err)
	}
	if _, err := fmt.Fprintln(e.stdin, "{stderr_ready}"); err != nil {
		return fmt.Errorf("writing to exiftool stdin: %w", err)
	}
	if _, err := fmt.Fprintln(e.stdin, "-execute"); err != nil {
		return fmt.Errorf("writing -execute to exiftool: %w", err)
	}

	// Read stdout until we see "{ready}" marker
	var output strings.Builder
	for e.stdout.Scan() {
		line := e.stdout.Text()
		if strings.Contains(line, "{ready}") {
			break
		}
		output.WriteString(line)
		output.WriteByte('\n')
	}

	if err := e.stdout.Err(); err != nil {
		return fmt.Errorf("reading exiftool output: %w", err)
	}

	// Drain stderr until sync marker
	for e.stderr.Scan() {
		if strings.Contains(e.stderr.Text(), "{stderr_ready}") {
			break
		}
	}

	if strings.Contains(output.String(), "0 image files updated") {
		return fmt.Errorf("0 image files updated")
	}

	return nil
}

// diagnoseFailed runs a one-off exiftool command without -m to get the detailed error.
func (e *ExifToolBatch) diagnoseFailed(args []string) error {
	// Build args without -m so error details are not suppressed
	var diagArgs []string
	for _, a := range args {
		if a != "-m" {
			diagArgs = append(diagArgs, a)
		}
	}

	cmd := exec.Command(e.exiftoolPath, diagArgs...)
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	cmd.Run() // ignore exit code — we want the error output

	// Combine stdout and stderr for the full error picture
	mediaPath := args[len(args)-1]
	detail := strings.TrimSpace(stdout.String())
	if errMsg := strings.TrimSpace(stderr.String()); errMsg != "" {
		detail = fmt.Sprintf("%s (%s)", detail, errMsg)
	}
	if detail == "" {
		detail = "unknown error"
	}
	return fmt.Errorf("exiftool failed for %s: %s", mediaPath, detail)
}

// Close shuts down the persistent exiftool process.
func (e *ExifToolBatch) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Send -stay_open False to gracefully terminate
	fmt.Fprintln(e.stdin, "-stay_open")
	fmt.Fprintln(e.stdin, "False")
	e.stdin.Close()
	return e.cmd.Wait()
}

// addFixFlag inserts -F after -m to enable fixing corrupt metadata structures.
func addFixFlag(args []string) []string {
	result := make([]string, 0, len(args)+1)
	for _, a := range args {
		result = append(result, a)
		if a == "-m" {
			result = append(result, "-F")
		}
	}
	return result
}

// buildExifArgs constructs the exiftool arguments for writing metadata.
func buildExifArgs(meta *metadata.Metadata, mediaPath string) []string {
	var args []string

	args = append(args, "-m")
	args = append(args, "-overwrite_original")

	// Timestamps
	if takenTime, err := meta.PhotoTakenTime.ExifString(); err == nil && takenTime != "1970:01:01 00:00:00" {
		args = append(args, "-DateTimeOriginal="+takenTime)
		args = append(args, "-FileModifyDate="+takenTime)
	}

	if createTime, err := meta.CreationTime.ExifString(); err == nil && createTime != "1970:01:01 00:00:00" {
		args = append(args, "-CreateDate="+createTime)
	}

	// GPS coordinates — prefer geoDataExif over geoData
	geo := meta.BestGeoData()
	if geo.HasLocation() {
		args = append(args, fmt.Sprintf("-GPSLatitude=%.10f", geo.AbsLat()))
		args = append(args, "-GPSLatitudeRef="+geo.LatRef())
		args = append(args, fmt.Sprintf("-GPSLongitude=%.10f", geo.AbsLon()))
		args = append(args, "-GPSLongitudeRef="+geo.LonRef())

		if geo.HasAltitude() {
			alt := geo.Altitude
			ref := "0" // above sea level
			if alt < 0 {
				ref = "1" // below sea level
				alt = -alt
			}
			args = append(args, fmt.Sprintf("-GPSAltitude=%.2f", alt))
			args = append(args, "-GPSAltitudeRef="+ref)
		}
	}

	// Description
	if meta.Description != "" {
		args = append(args, "-ImageDescription="+meta.Description)
		args = append(args, "-Description="+meta.Description)
	}

	args = append(args, mediaPath)

	return args
}

// buildVideoExifArgs constructs exiftool arguments optimized for video files.
// Uses QuickTime-compatible tags and skips EXIF-only tags like ImageDescription.
func buildVideoExifArgs(meta *metadata.Metadata, mediaPath string) []string {
	var args []string

	args = append(args, "-m")
	args = append(args, "-overwrite_original")

	// Timestamps — use QuickTime-native tags
	if takenTime, err := meta.PhotoTakenTime.ExifString(); err == nil && takenTime != "1970:01:01 00:00:00" {
		args = append(args, "-QuickTime:CreateDate="+takenTime)
		args = append(args, "-QuickTime:ModifyDate="+takenTime)
		args = append(args, "-FileModifyDate="+takenTime)
	}

	if createTime, err := meta.CreationTime.ExifString(); err == nil && createTime != "1970:01:01 00:00:00" {
		args = append(args, "-QuickTime:CreateDate="+createTime)
	}

	// GPS coordinates
	geo := meta.BestGeoData()
	if geo.HasLocation() {
		args = append(args, fmt.Sprintf("-GPSLatitude=%.10f", geo.AbsLat()))
		args = append(args, "-GPSLatitudeRef="+geo.LatRef())
		args = append(args, fmt.Sprintf("-GPSLongitude=%.10f", geo.AbsLon()))
		args = append(args, "-GPSLongitudeRef="+geo.LonRef())

		if geo.HasAltitude() {
			alt := geo.Altitude
			ref := "0"
			if alt < 0 {
				ref = "1"
				alt = -alt
			}
			args = append(args, fmt.Sprintf("-GPSAltitude=%.2f", alt))
			args = append(args, "-GPSAltitudeRef="+ref)
		}
	}

	// Description — use XMP only (no EXIF ImageDescription for video)
	if meta.Description != "" {
		args = append(args, "-XMP:Description="+meta.Description)
	}

	args = append(args, mediaPath)

	return args
}

// CheckExifTool verifies that exiftool is available and returns its version.
func CheckExifTool(exiftoolPath string) (string, error) {
	out, err := exec.Command(exiftoolPath, "-ver").Output()
	if err != nil {
		return "", fmt.Errorf("exiftool not found or not executable at %q: %w", exiftoolPath, err)
	}
	return strings.TrimSpace(string(out)), nil
}
