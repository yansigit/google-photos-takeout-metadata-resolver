package pipeline

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"gp-takeout-resolver/internal/matcher"
	"gp-takeout-resolver/internal/report"
	"gp-takeout-resolver/internal/writer"
)

// Pipeline orchestrates parallel processing of matched media files.
type Pipeline struct {
	workers    int
	outputDir  string
	inputRoot  string
	exiftool   string
	dryRun     bool
	copyOrphan bool
	logger     *slog.Logger
}

// NewPipeline creates a new processing pipeline.
func NewPipeline(workers int, outputDir, inputRoot, exiftool string, dryRun, copyOrphan bool, logger *slog.Logger) *Pipeline {
	return &Pipeline{
		workers:    workers,
		outputDir:  outputDir,
		inputRoot:  inputRoot,
		exiftool:   exiftool,
		dryRun:     dryRun,
		copyOrphan: copyOrphan,
		logger:     logger,
	}
}

// Run processes all match reports through the worker pool.
func (p *Pipeline) Run(ctx context.Context, reports []matcher.MatchReport) *report.Stats {
	stats := &report.Stats{}

	// Collect all matched items and orphans
	var allMatched []matcher.MatchResult
	var allOrphanMedia []string

	for _, r := range reports {
		allMatched = append(allMatched, r.Matched...)
		allOrphanMedia = append(allOrphanMedia, r.OrphanMedia...)
		atomic.AddInt64(&stats.OrphanJSON, int64(len(r.OrphanJSON)))
	}

	atomic.StoreInt64(&stats.TotalMedia, int64(len(allMatched)+len(allOrphanMedia)))
	atomic.StoreInt64(&stats.TotalJSON, int64(len(allMatched))+stats.OrphanJSON)
	atomic.StoreInt64(&stats.OrphanMedia, int64(len(allOrphanMedia)))

	// Process matched pairs through worker pool
	matchCh := make(chan matcher.MatchResult, p.workers*2)
	var wg sync.WaitGroup

	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			var et *writer.ExifToolBatch
			var err error
			if !p.dryRun {
				et, err = writer.NewExifToolBatch(p.exiftool)
				if err != nil {
					p.logger.Error("failed to start exiftool", "worker", workerID, "error", err)
					// Drain channel to avoid deadlock
					for range matchCh {
						atomic.AddInt64(&stats.Failed, 1)
					}
					return
				}
				defer et.Close()
			}

			w := writer.NewWriter(p.outputDir, p.inputRoot, et, p.dryRun, p.logger)

			for match := range matchCh {
				if ctx.Err() != nil {
					atomic.AddInt64(&stats.Failed, 1)
					continue
				}

				result := w.Process(match)
				if result.Success {
					atomic.AddInt64(&stats.Processed, 1)
				} else {
					atomic.AddInt64(&stats.Failed, 1)
					if result.Error != nil {
						p.logger.Warn("processing failed",
							"file", result.MediaPath,
							"error", result.Error,
						)
					}
				}
			}
		}(i)
	}

	// Feed matched pairs to workers
	for _, m := range allMatched {
		if ctx.Err() != nil {
			break
		}
		matchCh <- m
	}
	close(matchCh)
	wg.Wait()

	// Handle orphan media
	if p.copyOrphan && len(allOrphanMedia) > 0 {
		p.processOrphans(ctx, allOrphanMedia, stats)
	}

	return stats
}

func (p *Pipeline) processOrphans(ctx context.Context, orphans []string, stats *report.Stats) {
	orphanCh := make(chan string, p.workers*2)
	var wg sync.WaitGroup

	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var et *writer.ExifToolBatch
			// Orphans don't need exiftool
			w := writer.NewWriter(p.outputDir, p.inputRoot, et, p.dryRun, p.logger)

			for mediaPath := range orphanCh {
				if ctx.Err() != nil {
					continue
				}
				result := w.CopyOrphan(mediaPath)
				if result.Success {
					atomic.AddInt64(&stats.OrphansCopied, 1)
				}
			}
		}()
	}

	for _, path := range orphans {
		if ctx.Err() != nil {
			break
		}
		orphanCh <- path
	}
	close(orphanCh)
	wg.Wait()
}
