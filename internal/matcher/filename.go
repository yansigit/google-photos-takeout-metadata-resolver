package matcher

import (
	"path/filepath"
	"regexp"
	"strings"
)

// supplementalRe matches the supplemental-metadata suffix (possibly truncated) and optional duplicate number.
// Examples matched:
//   .supplemental-metadata       → suffix="supplemental-metadata", dup=""
//   .supplemental-metadata(1)    → suffix="supplemental-metadata", dup="1"
//   .supplemental-metad          → suffix="supplemental-metad", dup=""
//   .supplemental-metad(1)       → suffix="supplemental-metad", dup="1"
//   .suppl                       → suffix="suppl", dup=""
//   .suppl(1)                    → suffix="suppl", dup="1"
var supplementalRe = regexp.MustCompile(`\.(sup[a-z-]*)(?:\((\d+)\))?$`)

// ParseResult holds the parsed components of a JSON sidecar filename.
type ParseResult struct {
	// MediaName is the base media filename without extension (e.g. "IMG_0055")
	MediaName string
	// MediaExt is the media file extension including dot (e.g. ".JPG")
	MediaExt string
	// DupNumber is the duplicate number from (N) suffix, or -1 if none
	DupNumber int
	// IsBareJSON is true when the JSON has no supplemental-metadata suffix
	IsBareJSON bool
}

// TargetMediaFilename returns the expected media filename.
// For duplicates, the (N) is inserted before the extension: IMG_0055(1).JPG
func (p ParseResult) TargetMediaFilename() string {
	if p.DupNumber >= 0 {
		return p.MediaName + "(" + itoa(p.DupNumber) + ")" + p.MediaExt
	}
	return p.MediaName + p.MediaExt
}

// BaseMediaFilename returns the media filename without any duplicate suffix.
func (p ParseResult) BaseMediaFilename() string {
	return p.MediaName + p.MediaExt
}

func itoa(n int) string {
	buf := [20]byte{}
	pos := len(buf)
	if n == 0 {
		return "0"
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

// ParseJSONFilename parses a JSON sidecar filename to extract the target media filename.
// The input should be just the filename (not the full path).
//
// Patterns handled:
//   IMG_0055.PNG.supplemental-metadata.json        → IMG_0055.PNG, dup=-1
//   IMG_0055.PNG.supplemental-metadata(1).json      → IMG_0055.PNG, dup=1
//   IMG_0055.PNG.supplemental-metad.json            → IMG_0055.PNG, dup=-1
//   IMG_0055.PNG.suppl.json                         → IMG_0055.PNG, dup=-1
//   IMG_0055.PNG.suppl(1).json                      → IMG_0055.PNG, dup=1
//   somefile.json                                   → somefile (bare JSON, no supplemental suffix)
func ParseJSONFilename(jsonFilename string) ParseResult {
	// Strip .json extension
	name := strings.TrimSuffix(jsonFilename, ".json")
	if name == jsonFilename {
		// Not a .json file
		return ParseResult{MediaName: jsonFilename, DupNumber: -1, IsBareJSON: true}
	}

	// Try to match supplemental suffix
	loc := supplementalRe.FindStringSubmatchIndex(name)
	if loc == nil {
		// No supplemental suffix — bare JSON (e.g. "somefile.json")
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		return ParseResult{
			MediaName:  base,
			MediaExt:   ext,
			DupNumber:  -1,
			IsBareJSON: true,
		}
	}

	// Extract the media filename part (everything before the supplemental suffix)
	mediaFullName := name[:loc[0]]

	// Extract duplicate number if present
	dupNum := -1
	if loc[4] != -1 && loc[5] != -1 {
		dupStr := name[loc[4]:loc[5]]
		n := 0
		for _, c := range dupStr {
			n = n*10 + int(c-'0')
		}
		dupNum = n
	}

	ext := filepath.Ext(mediaFullName)
	base := strings.TrimSuffix(mediaFullName, ext)

	return ParseResult{
		MediaName:  base,
		MediaExt:   ext,
		DupNumber:  dupNum,
		IsBareJSON: false,
	}
}

// IsMediaFile checks if the given filename has a known media extension.
func IsMediaFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	_, ok := mediaExtensions[ext]
	return ok
}

// IsJSONFile checks if the given filename ends with .json.
func IsJSONFile(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".json")
}

var mediaExtensions = map[string]bool{
	// Photos
	".jpg":            true,
	".jpeg":           true,
	".heic":           true,
	".heif":           true,
	".png":            true,
	".gif":            true,
	".webp":           true,
	".avif":           true,
	".bmp":            true,
	".tiff":           true,
	".tif":            true,
	".ico":            true,
	".raw":            true,
	".cr2":            true,
	".nef":            true,
	".dng":            true,
	".arw":            true,
	".svg":            true,
	".fullsizerender": true,
	// Videos
	".mp4":  true,
	".mov":  true,
	".avi":  true,
	".wmv":  true,
	".flv":  true,
	".mkv":  true,
	".webm": true,
	".m4v":  true,
	".3gp":  true,
	".mpg":  true,
	".mpeg": true,
	".mts":  true,
}
