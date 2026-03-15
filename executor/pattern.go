package executor

// matchLike implements SQL LIKE pattern matching
// % matches any sequence of characters
// _ matches any single character
func matchLike(s, pattern string) bool {
	return matchPattern(s, pattern, '%', '_', false)
}

// matchGlob implements SQL GLOB pattern matching
// * matches any sequence of characters
// ? matches any single character
// GLOB is case-sensitive
func matchGlob(s, pattern string) bool {
	return matchPattern(s, pattern, '*', '?', true)
}

// matchPattern performs pattern matching with configurable wildcards
func matchPattern(s, pattern string, anyChar, oneChar rune, caseSensitive bool) bool {
	if !caseSensitive {
		s = toLower(s)
		pattern = toLower(pattern)
	}

	sp := 0 // string position
	pp := 0 // pattern position
	starIdx := -1
	match := 0

	patternRunes := []rune(pattern)
	strRunes := []rune(s)

	for sp < len(strRunes) {
		if pp < len(patternRunes) && patternRunes[pp] == anyChar {
			// '*' or '%' - save position
			starIdx = pp
			match = sp
			pp++
		} else if pp < len(patternRunes) && (patternRunes[pp] == oneChar || patternRunes[pp] == strRunes[sp]) {
			// '?' or '_' matches single char, or exact match
			sp++
			pp++
		} else if starIdx != -1 {
			// No match, but we have a previous star - backtrack
			pp = starIdx + 1
			match++
			sp = match
		} else {
			return false
		}
	}

	// Check remaining pattern characters (should all be anyChar)
	for pp < len(patternRunes) && patternRunes[pp] == anyChar {
		pp++
	}

	return pp == len(patternRunes)
}

// toLower converts string to lowercase (simple ASCII)
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}
