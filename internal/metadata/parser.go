package metadata

import (
	"encoding/json"
	"fmt"
	"os"
)

func Parse(path string) (*Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing JSON %s: %w", path, err)
	}

	return &meta, nil
}
