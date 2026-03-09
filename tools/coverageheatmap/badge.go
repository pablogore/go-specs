package coverage

import (
	"fmt"
	"os"
	"path/filepath"
)

// badgeColor returns the hex color for the coverage badge.
// >= 95% brightgreen, >= 90% green, >= 80% yellowgreen, >= 70% yellow, >= 60% orange, < 60% red.
func badgeColor(pct float64) string {
	switch {
	case pct >= 95:
		return "#44cc11" // brightgreen
	case pct >= 90:
		return "#97ca00" // green
	case pct >= 80:
		return "#a4a61d" // yellowgreen
	case pct >= 70:
		return "#dfb317" // yellow
	case pct >= 60:
		return "#fe7d37" // orange
	default:
		return "#e05d44" // red
	}
}

// GenerateBadge writes an SVG coverage badge to docs/coverage.svg.
// Creates docs/ if needed. Overwrites the file each run.
func GenerateBadge(moduleRoot string, totalPct float64) error {
	docsDir := filepath.Join(moduleRoot, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return fmt.Errorf("create docs dir: %w", err)
	}
	outPath := filepath.Join(docsDir, "coverage.svg")
	svg := buildBadgeSVG(totalPct)
	if err := os.WriteFile(outPath, []byte(svg), 0644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

// buildBadgeSVG returns a simple shield-style SVG: [ coverage | 92% ].
// width 120, height 20. Left part gray, right part colored by coverage.
func buildBadgeSVG(pct float64) string {
	color := badgeColor(pct)
	label := "coverage"
	value := fmt.Sprintf("%.0f%%", pct)
	if pct == 100 {
		value = "100%"
	}

	// Shield: two rectangles, rounded ends. Left gray, right colored.
	// Using rounded rect for the whole badge, then a rect for the divider.
	const w, h = 120, 20
	const rx = 3

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">
  <rect width="%d" height="%d" rx="%d" fill="#555"/>
  <rect x="70" width="50" height="%d" rx="0 3 3 0" fill="%s"/>
  <rect x="68" width="2" height="%d" fill="%s"/>
  <text x="35" y="14" fill="#fff" font-size="11" font-family="Deja Vu Sans,Verdana,sans-serif" text-anchor="middle">%s</text>
  <text x="95" y="14" fill="#fff" font-size="11" font-family="Deja Vu Sans,Verdana,sans-serif" text-anchor="middle">%s</text>
</svg>`,
		w, h,
		w, h, rx,
		h, color,
		h, color,
		label, value,
	)
}
