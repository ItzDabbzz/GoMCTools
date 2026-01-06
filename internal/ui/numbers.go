package ui

func clampInt(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// Exported wrapper
func ClampInt(val, min, max int) int { return clampInt(val, min, max) }
