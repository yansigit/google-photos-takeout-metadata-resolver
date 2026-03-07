package config

import (
	"flag"
	"fmt"
	"os"
	"runtime"
)

// Config holds the application configuration.
type Config struct {
	InputDir     string
	OutputDir    string
	Workers      int
	DryRun       bool
	Verbose      bool
	SkipTrash    bool
	SkipArchive  bool
	ExifToolPath string
	CopyOrphans  bool
}

// Parse parses command-line flags and validates the configuration.
func Parse() (*Config, error) {
	cfg := &Config{}

	flag.StringVar(&cfg.InputDir, "i", "", "Input directory (Takeout root or 'Google 포토' dir) [required]")
	flag.StringVar(&cfg.InputDir, "input", "", "Input directory (Takeout root or 'Google 포토' dir) [required]")
	flag.StringVar(&cfg.OutputDir, "o", "", "Output directory for processed files [required]")
	flag.StringVar(&cfg.OutputDir, "output", "", "Output directory for processed files [required]")
	flag.IntVar(&cfg.Workers, "w", runtime.NumCPU(), "Number of parallel workers")
	flag.IntVar(&cfg.Workers, "workers", runtime.NumCPU(), "Number of parallel workers")
	flag.BoolVar(&cfg.DryRun, "n", false, "Dry run - show what would be done without writing")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Dry run - show what would be done without writing")
	flag.BoolVar(&cfg.Verbose, "v", false, "Verbose logging")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&cfg.SkipTrash, "skip-trash", false, "Skip trash folder (휴지통)")
	flag.BoolVar(&cfg.SkipArchive, "skip-archive", false, "Skip archive folder (보관함)")
	flag.StringVar(&cfg.ExifToolPath, "exiftool", "exiftool", "Path to exiftool binary")
	flag.BoolVar(&cfg.CopyOrphans, "copy-orphans", false, "Copy orphan media files (no JSON) to output unchanged")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gp-takeout-resolver [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Google Photos Takeout metadata resolver.\n")
		fmt.Fprintf(os.Stderr, "Reads JSON metadata files from Google Takeout export and writes\n")
		fmt.Fprintf(os.Stderr, "EXIF/XMP metadata into the corresponding photo/video files.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.InputDir == "" {
		return fmt.Errorf("input directory is required (-i)")
	}
	if c.OutputDir == "" {
		return fmt.Errorf("output directory is required (-o)")
	}

	// Check input dir exists
	info, err := os.Stat(c.InputDir)
	if err != nil {
		return fmt.Errorf("input directory %q: %w", c.InputDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("input path %q is not a directory", c.InputDir)
	}

	// Ensure output dir can be created
	if err := os.MkdirAll(c.OutputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory %q: %w", c.OutputDir, err)
	}

	if c.Workers < 1 {
		c.Workers = 1
	}

	return nil
}
