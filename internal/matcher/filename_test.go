package matcher

import (
	"testing"
)

func TestParseJSONFilename(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantMedia  string
		wantExt    string
		wantDup    int
		wantBare   bool
		wantTarget string
	}{
		{
			name:       "standard supplemental-metadata",
			input:      "IMG_7378.JPG.supplemental-metadata.json",
			wantMedia:  "IMG_7378",
			wantExt:    ".JPG",
			wantDup:    -1,
			wantTarget: "IMG_7378.JPG",
		},
		{
			name:       "standard with duplicate number",
			input:      "IMG_0055.PNG.supplemental-metadata(1).json",
			wantMedia:  "IMG_0055",
			wantExt:    ".PNG",
			wantDup:    1,
			wantTarget: "IMG_0055(1).PNG",
		},
		{
			name:       "truncated supplemental-metad",
			input:      "2020-12-04-15-01-17-685.jpg.supplemental-metad.json",
			wantMedia:  "2020-12-04-15-01-17-685",
			wantExt:    ".jpg",
			wantDup:    -1,
			wantTarget: "2020-12-04-15-01-17-685.jpg",
		},
		{
			name:       "truncated suppl",
			input:      "146A6807-3E2F-4846-8651-CBA72786F130.jpg.suppl.json",
			wantMedia:  "146A6807-3E2F-4846-8651-CBA72786F130",
			wantExt:    ".jpg",
			wantDup:    -1,
			wantTarget: "146A6807-3E2F-4846-8651-CBA72786F130.jpg",
		},
		{
			name:       "truncated suppl with dup",
			input:      "146A6807-3E2F-4846-8651-CBA72786F130.jpg.suppl(1).json",
			wantMedia:  "146A6807-3E2F-4846-8651-CBA72786F130",
			wantExt:    ".jpg",
			wantDup:    1,
			wantTarget: "146A6807-3E2F-4846-8651-CBA72786F130(1).jpg",
		},
		{
			name:       "truncated supplemental-metad with dup",
			input:      "IMG_0175.HEIC.supplemental-metad(1).json",
			wantMedia:  "IMG_0175",
			wantExt:    ".HEIC",
			wantDup:    1,
			wantTarget: "IMG_0175(1).HEIC",
		},
		{
			name:       "bare JSON (no supplemental suffix)",
			input:      "71039584209__1ABD4D4A-AE7D-42F7-BB82-A8A4B5EC3.json",
			wantMedia:  "71039584209__1ABD4D4A-AE7D-42F7-BB82-A8A4B5EC3",
			wantExt:    "",
			wantDup:    -1,
			wantBare:   true,
			wantTarget: "71039584209__1ABD4D4A-AE7D-42F7-BB82-A8A4B5EC3",
		},
		{
			name:       "HEIC standard",
			input:      "IMG_0240.HEIC.supplemental-metadata.json",
			wantMedia:  "IMG_0240",
			wantExt:    ".HEIC",
			wantDup:    -1,
			wantTarget: "IMG_0240.HEIC",
		},
		{
			name:       "MP4 video",
			input:      "IMG_4591.MOV.supplemental-metadata.json",
			wantMedia:  "IMG_4591",
			wantExt:    ".MOV",
			wantDup:    -1,
			wantTarget: "IMG_4591.MOV",
		},
		{
			name:       "MP4 video with dup",
			input:      "IMG_4591.MOV.supplemental-metadata(2).json",
			wantMedia:  "IMG_4591",
			wantExt:    ".MOV",
			wantDup:    2,
			wantTarget: "IMG_4591(2).MOV",
		},
		{
			name:       "lowercase jpeg",
			input:      "lp_image.jpeg.supplemental-metadata.json",
			wantMedia:  "lp_image",
			wantExt:    ".jpeg",
			wantDup:    -1,
			wantTarget: "lp_image.jpeg",
		},
		{
			name:       "heic with long truncation supplemental-me",
			input:      "some-very-long-filename-here.heic.supplemental-me.json",
			wantMedia:  "some-very-long-filename-here",
			wantExt:    ".heic",
			wantDup:    -1,
			wantTarget: "some-very-long-filename-here.heic",
		},
		{
			name:       "Original suffix file",
			input:      "IMG_0176_Original.HEIC.supplemental-metadata.json",
			wantMedia:  "IMG_0176_Original",
			wantExt:    ".HEIC",
			wantDup:    -1,
			wantTarget: "IMG_0176_Original.HEIC",
		},
		{
			name:       "duplicate number 10",
			input:      "lp_image.heic.supplemental-metadata(10).json",
			wantMedia:  "lp_image",
			wantExt:    ".heic",
			wantDup:    10,
			wantTarget: "lp_image(10).heic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseJSONFilename(tt.input)

			if result.MediaName != tt.wantMedia {
				t.Errorf("MediaName = %q, want %q", result.MediaName, tt.wantMedia)
			}
			if result.MediaExt != tt.wantExt {
				t.Errorf("MediaExt = %q, want %q", result.MediaExt, tt.wantExt)
			}
			if result.DupNumber != tt.wantDup {
				t.Errorf("DupNumber = %d, want %d", result.DupNumber, tt.wantDup)
			}
			if result.IsBareJSON != tt.wantBare {
				t.Errorf("IsBareJSON = %v, want %v", result.IsBareJSON, tt.wantBare)
			}
			if got := result.TargetMediaFilename(); got != tt.wantTarget {
				t.Errorf("TargetMediaFilename() = %q, want %q", got, tt.wantTarget)
			}
		})
	}
}

func TestIsMediaFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"photo.jpg", true},
		{"photo.JPG", true},
		{"photo.HEIC", true},
		{"photo.heic", true},
		{"video.mp4", true},
		{"video.MOV", true},
		{"video.avi", true},
		{"photo.png", true},
		{"photo.webp", true},
		{"photo.avif", true},
		{"photo.gif", true},
		{"file.json", false},
		{"file.txt", false},
		{"file.xml", false},
		{"photo.fullsizerender", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := IsMediaFile(tt.filename); got != tt.want {
				t.Errorf("IsMediaFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestIsJSONFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"file.json", true},
		{"file.JSON", true},
		{"file.Json", true},
		{"file.jpg", false},
		{"file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := IsJSONFile(tt.filename); got != tt.want {
				t.Errorf("IsJSONFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}
