package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"

	"gp-takeout-resolver/internal/matcher"
)

// FolderScan holds the scan results for a single directory.
type FolderScan struct {
	Dir        string
	MediaFiles []string // filenames only (not full paths)
	JSONFiles  []string // filenames only (not full paths)
}

// ScanOptions controls which directories to process.
type ScanOptions struct {
	SkipTrash   bool
	SkipArchive bool
}

// knownSkipFiles are metadata files that should be ignored.
var knownSkipFiles = map[string]bool{
	"메타데이터.json":  true,
	"metadata.json": true,
}

// trashFolderNames are folder names for the trash directory (multi-language).
var trashFolderNames = map[string]bool{
	"휴지통":   true,
	"Trash": true,
	"Bin":   true,
}

// archiveFolderNames are folder names for the archive directory (multi-language).
var archiveFolderNames = map[string]bool{
	"보관함":     true,
	"Archive": true,
}

// ScanInput scans the input directory for Google Photos takeout subdirectories.
// It expects the input to be either the "Google 포토" directory or a Takeout root.
func ScanInput(inputDir string, opts ScanOptions) ([]FolderScan, error) {
	// Check if inputDir itself contains media/JSON files (flat structure)
	// or if it has subdirectories (year-based structure)
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, fmt.Errorf("reading input directory %s: %w", inputDir, err)
	}

	// Look for "Google 포토" or "Google Photos" subdirectory
	googlePhotosDir := ""
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rawName := e.Name()
		name := norm.NFC.String(rawName)
		if strings.HasPrefix(name, "Google") && (strings.Contains(name, "포토") || strings.Contains(name, "Photos") || strings.Contains(name, "Fotos")) {
			googlePhotosDir = filepath.Join(inputDir, rawName)
			break
		}
	}

	rootDir := inputDir
	if googlePhotosDir != "" {
		rootDir = googlePhotosDir
	}

	return scanDirectory(rootDir, opts)
}

func scanDirectory(rootDir string, opts ScanOptions) ([]FolderScan, error) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", rootDir, err)
	}

	var scans []FolderScan

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		rawDirName := e.Name()
		dirName := norm.NFC.String(rawDirName)

		// Skip trash/archive if configured
		if opts.SkipTrash && trashFolderNames[dirName] {
			continue
		}
		if opts.SkipArchive && archiveFolderNames[dirName] {
			continue
		}

		dirPath := filepath.Join(rootDir, rawDirName)
		scan, err := scanSingleDir(dirPath)
		if err != nil {
			return nil, fmt.Errorf("scanning %s: %w", dirPath, err)
		}

		if len(scan.MediaFiles) > 0 || len(scan.JSONFiles) > 0 {
			scans = append(scans, scan)
		}
	}

	// Also scan the root directory itself for any files directly in it
	rootScan, err := scanSingleDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("scanning root %s: %w", rootDir, err)
	}
	if len(rootScan.MediaFiles) > 0 || len(rootScan.JSONFiles) > 0 {
		scans = append(scans, rootScan)
	}

	return scans, nil
}

func scanSingleDir(dir string) (FolderScan, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return FolderScan{}, fmt.Errorf("reading %s: %w", dir, err)
	}

	scan := FolderScan{Dir: dir}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		rawName := e.Name()
		name := norm.NFC.String(rawName)

		// Skip known non-per-file metadata
		if knownSkipFiles[name] {
			continue
		}

		if matcher.IsJSONFile(name) {
			scan.JSONFiles = append(scan.JSONFiles, rawName)
		} else if matcher.IsMediaFile(name) {
			scan.MediaFiles = append(scan.MediaFiles, rawName)
		} else if filepath.Ext(name) == "" {
			// Files without extensions — treat as potential media (e.g. QuickTime movies)
			scan.MediaFiles = append(scan.MediaFiles, rawName)
		}
	}

	return scan, nil
}
