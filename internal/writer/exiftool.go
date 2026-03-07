package writer

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"gp-takeout-resolver/internal/metadata"
)

// ExifToolBatch manages a persistent exiftool process using -stay_open mode.
type ExifToolBatch struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex
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

	// Discard stderr to avoid blocking
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, fmt.Errorf("starting exiftool: %w", err)
	}

	return &ExifToolBatch{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
	}, nil
}

// WriteMetadata writes EXIF metadata to the file at mediaPath.
// This method is NOT concurrency-safe — each goroutine should own its own ExifToolBatch.
func (e *ExifToolBatch) WriteMetadata(mediaPath string, meta *metadata.Metadata) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	args := buildExifArgs(meta, mediaPath)

	// Write each argument on its own line, terminated by -execute
	for _, arg := range args {
		if _, err := fmt.Fprintln(e.stdin, arg); err != nil {
			return fmt.Errorf("writing to exiftool stdin: %w", err)
		}
	}
	if _, err := fmt.Fprintln(e.stdin, "-execute"); err != nil {
		return fmt.Errorf("writing -execute to exiftool: %w", err)
	}

	// Read output until we see "{ready}" marker
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

	// Check for errors in output
	result := output.String()
	if strings.Contains(result, "Error") || strings.Contains(result, "error") {
		// Filter out non-critical warnings
		if strings.Contains(result, "0 image files updated") {
			return fmt.Errorf("exiftool failed for %s: %s", mediaPath, strings.TrimSpace(result))
		}
	}

	return nil
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

// buildExifArgs constructs the exiftool arguments for writing metadata.
func buildExifArgs(meta *metadata.Metadata, mediaPath string) []string {
	var args []string

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

// CheckExifTool verifies that exiftool is available and returns its version.
func CheckExifTool(exiftoolPath string) (string, error) {
	out, err := exec.Command(exiftoolPath, "-ver").Output()
	if err != nil {
		return "", fmt.Errorf("exiftool not found or not executable at %q: %w", exiftoolPath, err)
	}
	return strings.TrimSpace(string(out)), nil
}
