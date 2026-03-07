# Google Photos Takeout Metadata Resolver

A Go CLI tool that merges separated JSON metadata from Google Photos Takeout exports back into the actual photo/video files as EXIF/XMP tags.

Google Takeout exports media files with their metadata (timestamps, GPS coordinates, descriptions) stripped out and stored in separate `.json` sidecar files. This tool reads those JSON files, matches them to their media counterparts, and writes the metadata into the files using `exiftool`.

## Features

- Writes `DateTimeOriginal`, `CreateDate`, GPS coordinates, and descriptions into files
- Sets file modification time to the original photo taken time
- Handles all Google Takeout naming quirks: truncated filenames, `(N)` duplicates, case mismatches
- Supports all common formats: JPEG, HEIC, PNG, AVIF, WEBP, MP4, MOV, AVI, and more
- Parallel processing with goroutine worker pool (defaults to all CPU cores)
- Outputs to a separate directory — never modifies the original export
- 99.99% match rate on real-world Takeout data (~20K+ files)

## Quick Start (Docker)

No dependencies needed on the host — exiftool is bundled in the image.

```bash
# Build the image
docker build -t gp-takeout-resolver .

# Dry run (preview without writing)
docker run --rm \
  -v /path/to/Takeout:/data/input:ro \
  -v /path/to/output:/data/output \
  gp-takeout-resolver \
  -i /data/input -o /data/output -n

# Full run
docker run --rm \
  -v /path/to/Takeout:/data/input:ro \
  -v /path/to/output:/data/output \
  gp-takeout-resolver \
  -i /data/input -o /data/output
```

## Running Locally (without Docker)

### Prerequisites

- **Go 1.22+** — [https://go.dev/dl/](https://go.dev/dl/)
- **exiftool** — [https://exiftool.org/](https://exiftool.org/)

Install exiftool for your platform:

```bash
# Arch Linux
sudo pacman -S perl-image-exiftool

# Ubuntu / Debian
sudo apt install libimage-exiftool-perl

# Fedora / RHEL
sudo dnf install perl-Image-ExifTool

# macOS
brew install exiftool

# Windows (Chocolatey)
choco install exiftool
```

### Build & Run

```bash
# Clone and build
git clone <repo-url> && cd gp-takeout-resolver
go build -o gp-takeout-resolver .

# Dry run first (recommended — previews matching without copying/writing)
./gp-takeout-resolver -i ./Takeout -o ./Outputs -n -v

# Full run
./gp-takeout-resolver -i ./Takeout -o ./Outputs

# Skip trash and archive, use 8 workers
./gp-takeout-resolver -i ./Takeout -o ./Outputs -skip-trash -skip-archive -w 8

# Also copy files that have no metadata (orphans)
./gp-takeout-resolver -i ./Takeout -o ./Outputs -copy-orphans
```

### Run without building

```bash
go run . -i ./Takeout -o ./Outputs -n
```

## CLI Flags

```
  -i, -input string      Input directory (Takeout root or Google Photos dir) [required]
  -o, -output string     Output directory for processed files [required]
  -w, -workers int       Number of parallel workers (default: NumCPU)
  -n, -dry-run           Preview what would be done without writing
  -v, -verbose           Verbose logging
      -skip-trash        Skip trash folder
      -skip-archive      Skip archive folder
      -exiftool string   Path to exiftool binary (default: "exiftool")
      -copy-orphans      Copy media files without metadata to output unchanged
```

## How It Works

1. **Scan** — Walks the Takeout directory, separating JSON metadata files from media files
2. **Match** — Links each JSON to its media file using a 5-strategy algorithm:
   - Exact filename extraction from JSON sidecar name
   - Duplicate `(N)` number resolution
   - Title field fallback from JSON content
   - Bare JSON filename matching
   - Prefix-based matching for truncated filenames
3. **Write** — Copies media to the output directory, writes EXIF metadata via `exiftool` in batch mode, and sets file modification timestamps

## License

MIT
