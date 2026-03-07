package metadata

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

type Metadata struct {
	Title              string             `json:"title"`
	Description        string             `json:"description"`
	ImageViews         string             `json:"imageViews"`
	CreationTime       Timestamp          `json:"creationTime"`
	PhotoTakenTime     Timestamp          `json:"photoTakenTime"`
	GeoData            GeoData            `json:"geoData"`
	GeoDataExif        *GeoData           `json:"geoDataExif,omitempty"`
	URL                string             `json:"url"`
	GooglePhotosOrigin GooglePhotosOrigin `json:"googlePhotosOrigin"`
}

type Timestamp struct {
	TimestampStr string `json:"timestamp"`
	Formatted    string `json:"formatted"`
}

func (t Timestamp) Time() (time.Time, error) {
	sec, err := strconv.ParseInt(t.TimestampStr, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing timestamp %q: %w", t.TimestampStr, err)
	}
	return time.Unix(sec, 0), nil
}

func (t Timestamp) ExifString() (string, error) {
	tm, err := t.Time()
	if err != nil {
		return "", err
	}
	return tm.UTC().Format("2006:01:02 15:04:05"), nil
}

type GeoData struct {
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Altitude      float64 `json:"altitude"`
	LatitudeSpan  float64 `json:"latitudeSpan,omitempty"`
	LongitudeSpan float64 `json:"longitudeSpan,omitempty"`
}

func (g GeoData) HasLocation() bool {
	return g.Latitude != 0.0 || g.Longitude != 0.0
}

func (g GeoData) HasAltitude() bool {
	return g.Altitude != 0.0
}

func (g GeoData) LatRef() string {
	if g.Latitude >= 0 {
		return "N"
	}
	return "S"
}

func (g GeoData) LonRef() string {
	if g.Longitude >= 0 {
		return "E"
	}
	return "W"
}

func (g GeoData) AbsLat() float64 {
	return math.Abs(g.Latitude)
}

func (g GeoData) AbsLon() float64 {
	return math.Abs(g.Longitude)
}

type GooglePhotosOrigin struct {
	MobileUpload *MobileUpload `json:"mobileUpload,omitempty"`
	DriveSync    *struct{}     `json:"driveSync,omitempty"`
	WebUpload    *WebUpload    `json:"webUpload,omitempty"`
}

type MobileUpload struct {
	DeviceType string `json:"deviceType"`
}

type WebUpload struct {
	ComputerUpload *struct{} `json:"computerUpload,omitempty"`
}

// BestGeoData returns geoDataExif if present and has location, otherwise geoData.
func (m *Metadata) BestGeoData() GeoData {
	if m.GeoDataExif != nil && m.GeoDataExif.HasLocation() {
		return *m.GeoDataExif
	}
	return m.GeoData
}
