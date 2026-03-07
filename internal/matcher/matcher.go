package matcher

import (
	"log/slog"
	"path/filepath"
	"strings"

	"gp-takeout-resolver/internal/metadata"
)

// MatchResult represents the outcome of matching a JSON sidecar to a media file.
type MatchResult struct {
	JSONPath  string
	MediaPath string
	Meta      *metadata.Metadata
	FolderDir string
}

// MatchReport holds all matching results for a directory.
type MatchReport struct {
	Matched     []MatchResult
	OrphanJSON  []string // JSON files with no matching media
	OrphanMedia []string // Media files with no matching JSON
}

// MatchDirectory matches JSON sidecar files to media files within a single directory.
func MatchDirectory(dir string, jsonFiles, mediaFiles []string, logger *slog.Logger) MatchReport {
	// Build case-insensitive media lookup: lowercase filename → original filename
	mediaMap := make(map[string]string, len(mediaFiles))
	matchedMedia := make(map[string]bool, len(mediaFiles))
	for _, f := range mediaFiles {
		mediaMap[strings.ToLower(f)] = f
	}

	var report MatchReport

	for _, jsonFile := range jsonFiles {
		parsed := ParseJSONFilename(jsonFile)
		jsonPath := filepath.Join(dir, jsonFile)

		// Parse the JSON metadata
		meta, err := metadata.Parse(jsonPath)
		if err != nil {
			logger.Warn("failed to parse JSON", "file", jsonFile, "error", err)
			report.OrphanJSON = append(report.OrphanJSON, jsonPath)
			continue
		}

		// Strategy 1: Use parsed filename from JSON filename
		targetName := parsed.TargetMediaFilename()
		mediaFile, found := lookupMedia(mediaMap, targetName)

		// Strategy 2: If not found and it's a duplicate, try base name
		if !found && parsed.DupNumber >= 0 {
			baseName := parsed.BaseMediaFilename()
			mediaFile, found = lookupMedia(mediaMap, baseName)
		}

		// Strategy 3: Fall back to title field from JSON content
		if !found && meta.Title != "" {
			mediaFile, found = lookupMedia(mediaMap, meta.Title)

			// If title didn't work and we have a dup number, try title with dup number
			if !found && parsed.DupNumber >= 0 {
				ext := filepath.Ext(meta.Title)
				base := strings.TrimSuffix(meta.Title, ext)
				titleWithDup := base + "(" + itoa(parsed.DupNumber) + ")" + ext
				mediaFile, found = lookupMedia(mediaMap, titleWithDup)
			}
		}

		// Strategy 4: For bare JSON files, try the JSON base name as a media filename
		if !found && parsed.IsBareJSON {
			nameWithoutJSON := strings.TrimSuffix(jsonFile, ".json")
			mediaFile, found = lookupMedia(mediaMap, nameWithoutJSON)
		}

		// Strategy 5: Prefix-based matching for truncated filenames.
		// Google truncates both JSON and media filenames independently.
		// Use the title field as a bridge: find a media file whose name is a prefix of the title.
		if !found && meta.Title != "" {
			titleLower := strings.ToLower(meta.Title)
			for mediaLower, mediaOrig := range mediaMap {
				// Media filename (without ext) should be a prefix of the title
				mediaNoExt := strings.TrimSuffix(mediaLower, strings.ToLower(filepath.Ext(mediaLower)))
				titleNoExt := strings.TrimSuffix(titleLower, strings.ToLower(filepath.Ext(titleLower)))
				if len(mediaNoExt) >= 10 && strings.HasPrefix(titleNoExt, mediaNoExt) {
					mediaFile = mediaOrig
					found = true
					break
				}
			}
		}

		if !found {
			logger.Debug("no media match for JSON", "json", jsonFile, "target", targetName, "title", meta.Title)
			report.OrphanJSON = append(report.OrphanJSON, jsonPath)
			continue
		}

		mediaPath := filepath.Join(dir, mediaFile)
		matchedMedia[strings.ToLower(mediaFile)] = true

		report.Matched = append(report.Matched, MatchResult{
			JSONPath:  jsonPath,
			MediaPath: mediaPath,
			Meta:      meta,
			FolderDir: dir,
		})
	}

	// Find orphan media files (no JSON matched)
	for _, f := range mediaFiles {
		if !matchedMedia[strings.ToLower(f)] {
			report.OrphanMedia = append(report.OrphanMedia, filepath.Join(dir, f))
		}
	}

	return report
}

// lookupMedia does a case-insensitive lookup in the media map.
func lookupMedia(mediaMap map[string]string, target string) (string, bool) {
	f, ok := mediaMap[strings.ToLower(target)]
	return f, ok
}
