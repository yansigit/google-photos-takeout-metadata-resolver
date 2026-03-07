package report

import (
	"fmt"
	"time"
)

// Stats holds aggregate processing statistics.
type Stats struct {
	TotalMedia   int64
	TotalJSON    int64
	Processed    int64
	Failed       int64
	OrphanJSON   int64
	OrphanMedia  int64
	OrphansCopied int64
}

// Print prints a human-readable summary to stdout.
func (s *Stats) Print(elapsed time.Duration) {
	fmt.Println()
	fmt.Println("============================================")
	fmt.Println("  Google Photos Takeout Metadata Resolver")
	fmt.Println("============================================")
	fmt.Printf("  Total media files found:    %d\n", s.TotalMedia)
	fmt.Printf("  Total JSON metadata found:  %d\n", s.TotalJSON)
	fmt.Println("--------------------------------------------")
	fmt.Printf("  Successfully processed:     %d\n", s.Processed)
	fmt.Printf("  Failed:                     %d\n", s.Failed)
	fmt.Printf("  Orphan JSON (no media):     %d\n", s.OrphanJSON)
	fmt.Printf("  Orphan media (no JSON):     %d\n", s.OrphanMedia)
	if s.OrphansCopied > 0 {
		fmt.Printf("  Orphan media copied:        %d\n", s.OrphansCopied)
	}
	fmt.Println("--------------------------------------------")
	fmt.Printf("  Elapsed time:               %s\n", elapsed.Round(time.Millisecond))
	fmt.Println("============================================")
}
