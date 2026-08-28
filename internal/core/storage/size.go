package storage

import "fmt"

var sizeUnits = [...]string{"B", "KB", "MB", "GB", "TB"}

// HumanBytes renders n the same way the shell script's awk human() did:
// base-1024, one decimal place, capped at TB.
func HumanBytes(n int64) string {
	b := float64(n)
	i := 0
	for b >= 1024 && i < len(sizeUnits)-1 {
		b /= 1024
		i++
	}
	return fmt.Sprintf("%.1f%s", b, sizeUnits[i])
}
