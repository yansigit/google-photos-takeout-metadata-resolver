package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gp-takeout-resolver/internal/config"
	"gp-takeout-resolver/internal/matcher"
	"gp-takeout-resolver/internal/pipeline"
	"gp-takeout-resolver/internal/scanner"
	"gp-takeout-resolver/internal/writer"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Parse()
	if err != nil {
		return err
	}

	// Setup logger
	logLevel := slog.LevelInfo
	if cfg.Verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	// Check exiftool availability (skip in dry-run mode)
	if !cfg.DryRun {
		ver, err := writer.CheckExifTool(cfg.ExifToolPath)
		if err != nil {
			return fmt.Errorf("exiftool check failed: %w\n\nPlease install exiftool:\n  Arch: sudo pacman -S perl-image-exiftool\n  Ubuntu: sudo apt install libimage-exiftool-perl\n  macOS: brew install exiftool", err)
		}
		logger.Info("exiftool found", "version", ver)
	} else {
		logger.Info("dry-run mode enabled, skipping exiftool check")
	}

	// Setup cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("received interrupt, shutting down gracefully...")
		cancel()
	}()

	startTime := time.Now()

	// Phase 1: Scan input directory
	logger.Info("scanning input directory", "path", cfg.InputDir)
	scanOpts := scanner.ScanOptions{
		SkipTrash:   cfg.SkipTrash,
		SkipArchive: cfg.SkipArchive,
	}
	folders, err := scanner.ScanInput(cfg.InputDir, scanOpts)
	if err != nil {
		return fmt.Errorf("scanning input: %w", err)
	}

	totalMedia := 0
	totalJSON := 0
	for _, f := range folders {
		totalMedia += len(f.MediaFiles)
		totalJSON += len(f.JSONFiles)
	}
	logger.Info("scan complete",
		"folders", len(folders),
		"media_files", totalMedia,
		"json_files", totalJSON,
	)

	if totalJSON == 0 {
		return fmt.Errorf("no JSON metadata files found in %s", cfg.InputDir)
	}

	// Phase 2: Match JSON to media files
	logger.Info("matching JSON metadata to media files")
	var reports []matcher.MatchReport
	for _, folder := range folders {
		report := matcher.MatchDirectory(folder.Dir, folder.JSONFiles, folder.MediaFiles, logger)
		reports = append(reports, report)
	}

	totalMatched := 0
	totalOrphanJSON := 0
	totalOrphanMedia := 0
	for _, r := range reports {
		totalMatched += len(r.Matched)
		totalOrphanJSON += len(r.OrphanJSON)
		totalOrphanMedia += len(r.OrphanMedia)
	}
	logger.Info("matching complete",
		"matched", totalMatched,
		"orphan_json", totalOrphanJSON,
		"orphan_media", totalOrphanMedia,
	)

	// Phase 3: Process matched pairs through pipeline
	logger.Info("processing files", "workers", cfg.Workers, "output", cfg.OutputDir)
	p := pipeline.NewPipeline(
		cfg.Workers,
		cfg.OutputDir,
		cfg.InputDir,
		cfg.ExifToolPath,
		cfg.DryRun,
		cfg.CopyOrphans,
		logger,
	)

	stats := p.Run(ctx, reports)
	stats.Print(time.Since(startTime))

	return nil
}
