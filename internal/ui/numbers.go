package ui

// clampInt returns val clamped to [min, max].
func clampInt(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// ClampInt is the exported wrapper around clampInt, available to pages packages.
func ClampInt(val, min, max int) int { return clampInt(val, min, max) }
