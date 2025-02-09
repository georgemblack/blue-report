package display

import (
	"fmt"
	"math"
)

// FormatCount formats a number for display, i.e. "1,500" -> "1.5k"
func FormatCount(count int) string {
	// If the number is under 1,000, no changes
	if count < 1000 {
		return fmt.Sprintf("%d", count)
	}

	// If the number is between 1,000 and 10,000, format as thousands with one decimal place.
	// Examples:
	//  - 1,000 -> 1k
	//	- 1,500 -> 1.5k
	//	- 9,999 -> 9.9k
	if count < 10000 {
		return fmt.Sprintf("%.1fk", math.Floor(float64(count)/100)/10)
	}

	// If the number is greater than 10,000, format as thousands with no decimal places.
	// Examples:
	//  - 10,000 -> 10k
	//	- 15,000 -> 15k
	//	- 99,999 -> 99k
	return fmt.Sprintf("%dk", int(math.Floor(float64(count)/1000)))
}
