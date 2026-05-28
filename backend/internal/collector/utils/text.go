package utils

import (
	"os"
)

// IsTextType determines whether the file at filepath is parseable as text.
// It reads the first 1KB and returns true if null/control characters are < 10%.
func IsTextType(filepath string) bool {
	f, err := os.Open(filepath)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false
	}
	buf = buf[:n]

	nullCount := 0
	controlCount := 0
	for _, b := range buf {
		if b == 0 {
			nullCount++
			continue
		}
		if b >= 0x00 && b <= 0x08 {
			controlCount++
		} else if b == 0x0B || b == 0x0C {
			controlCount++
		} else if b >= 0x0E && b <= 0x1F {
			controlCount++
		}
	}

	threshold := float64(n) * 0.1
	return float64(nullCount+controlCount) < threshold
}
