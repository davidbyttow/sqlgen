// Package naming provides utilities for converting between naming conventions
// used in database schemas and Go code.
package naming

import (
	"strings"
	"unicode"
)

// Common initialisms that should be fully uppercased in Go names.
var commonInitialisms = map[string]bool{
	"acl":   true,
	"api":   true,
	"ascii": true,
	"cpu":   true,
	"css":   true,
	"dns":   true,
	"eof":   true,
	"guid":  true,
	"html":  true,
	"http":  true,
	"https": true,
	"id":    true,
	"ip":    true,
	"json":  true,
	"lhs":   true,
	"qps":   true,
	"ram":   true,
	"rhs":   true,
	"rpc":   true,
	"sla":   true,
	"smtp":  true,
	"sql":   true,
	"ssh":   true,
	"tcp":   true,
	"tls":   true,
	"ttl":   true,
	"udp":   true,
	"ui":    true,
	"uid":   true,
	"uri":   true,
	"url":   true,
	"utf8":  true,
	"uuid":  true,
	"vm":    true,
	"xml":   true,
	"xmpp":  true,
	"xss":   true,
}

// splitWords splits a string into words by underscores, hyphens, and camelCase boundaries.
func splitWords(s string) []string {
	// First pass: split on delimiters and camelCase boundaries.
	var raw []string
	var current strings.Builder

	flush := func() {
		if current.Len() > 0 {
			raw = append(raw, current.String())
			current.Reset()
		}
	}

	runes := []rune(s)
	for i := range len(runes) {
		r := runes[i]
		switch {
		case r == '_' || r == '-' || r == ' ':
			flush()
		case unicode.IsUpper(r):
			if current.Len() > 0 {
				if i > 0 && unicode.IsLower(runes[i-1]) {
					flush()
				}
				if i > 0 && i+1 < len(runes) && unicode.IsUpper(runes[i-1]) && unicode.IsLower(runes[i+1]) {
					flush()
				}
			}
			current.WriteRune(r)
		default:
			current.WriteRune(r)
		}
	}
	flush()

	// Second pass: split all-uppercase words into known initialisms.
	// "HTTPURL" -> "HTTP", "URL"
	var words []string
	for _, w := range raw {
		if isAllUpper(w) && len(w) > 1 {
			words = append(words, splitInitialisms(w)...)
		} else {
			words = append(words, w)
		}
	}
	return words
}

func isAllUpper(s string) bool {
	for _, r := range s {
		if !unicode.IsUpper(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// splitInitialisms greedily matches known initialisms from left to right in an all-caps string.
func splitInitialisms(s string) []string {
	var parts []string
	lower := strings.ToLower(s)
	i := 0
	for i < len(lower) {
		matched := false
		// Try longest match first (up to 5 chars).
		for l := min(len(lower)-i, 5); l >= 2; l-- {
			candidate := lower[i : i+l]
			if commonInitialisms[candidate] {
				parts = append(parts, s[i:i+l])
				i += l
				matched = true
				break
			}
		}
		if !matched {
			// No initialism found; take the rest as one word.
			parts = append(parts, s[i:])
			break
		}
	}
	return parts
}

// ToPascal converts a string to PascalCase, respecting Go initialisms.
// Examples: "user_id" -> "UserID", "created_at" -> "CreatedAt", "http_url" -> "HTTPURL"
func ToPascal(s string) string {
	words := splitWords(s)
	var result strings.Builder
	for _, w := range words {
		lower := strings.ToLower(w)
		if commonInitialisms[lower] {
			result.WriteString(strings.ToUpper(lower))
		} else {
			result.WriteString(strings.ToUpper(lower[:1]))
			result.WriteString(lower[1:])
		}
	}
	return result.String()
}

// ToCamel converts a string to camelCase, respecting Go initialisms.
// Examples: "user_id" -> "userID", "created_at" -> "createdAt"
func ToCamel(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}
	var result strings.Builder

	// First word is lowercase, but if it's an initialism, keep it lower.
	first := strings.ToLower(words[0])
	result.WriteString(first)

	for _, w := range words[1:] {
		lower := strings.ToLower(w)
		if commonInitialisms[lower] {
			result.WriteString(strings.ToUpper(lower))
		} else {
			result.WriteString(strings.ToUpper(lower[:1]))
			result.WriteString(lower[1:])
		}
	}
	return result.String()
}

// ToSnake converts a string to snake_case.
// Examples: "UserID" -> "user_id", "CreatedAt" -> "created_at"
func ToSnake(s string) string {
	words := splitWords(s)
	lower := make([]string, len(words))
	for i, w := range words {
		lower[i] = strings.ToLower(w)
	}
	return strings.Join(lower, "_")
}

// Irregular plural mappings (singular -> plural).
var irregularPlurals = map[string]string{
	"person":  "people",
	"child":   "children",
	"man":     "men",
	"woman":   "women",
	"mouse":   "mice",
	"goose":   "geese",
	"tooth":   "teeth",
	"foot":    "feet",
	"datum":   "data",
	"medium":  "media",
	"stadium": "stadiums",
	"index":   "indexes",
	"status":  "statuses",
	"alias":   "aliases",
}

// Irregular singular mappings (plural -> singular). Built from irregularPlurals.
var irregularSingulars map[string]string

func init() {
	irregularSingulars = make(map[string]string, len(irregularPlurals))
	for s, p := range irregularPlurals {
		irregularSingulars[p] = s
	}
}

// Pluralize returns the plural form of a word.
// Handles common English rules. Not perfect, but good enough for table/struct names.
func Pluralize(s string) string {
	if s == "" {
		return s
	}
	lower := strings.ToLower(s)
	if p, ok := irregularPlurals[lower]; ok {
		return matchCase(s, p)
	}

	switch {
	case strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") ||
		strings.HasSuffix(lower, "z") || strings.HasSuffix(lower, "sh") ||
		strings.HasSuffix(lower, "ch"):
		return s + "es"
	case strings.HasSuffix(lower, "y") && len(lower) > 1 && !isVowel(rune(lower[len(lower)-2])):
		return s[:len(s)-1] + "ies"
	case strings.HasSuffix(lower, "fe"):
		return s[:len(s)-2] + "ves"
	case strings.HasSuffix(lower, "f") && !strings.HasSuffix(lower, "ff"):
		return s[:len(s)-1] + "ves"
	default:
		return s + "s"
	}
}

// Singularize returns the singular form of a word.
func Singularize(s string) string {
	if s == "" {
		return s
	}
	lower := strings.ToLower(s)
	if sing, ok := irregularSingulars[lower]; ok {
		return matchCase(s, sing)
	}

	switch {
	case strings.HasSuffix(lower, "ies") && len(lower) > 3:
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(lower, "ves"):
		// Could be -f or -fe original. Default to -f.
		return s[:len(s)-3] + "f"
	case strings.HasSuffix(lower, "ses") || strings.HasSuffix(lower, "xes") ||
		strings.HasSuffix(lower, "zes") || strings.HasSuffix(lower, "shes") ||
		strings.HasSuffix(lower, "ches"):
		return s[:len(s)-2]
	case strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss"):
		return s[:len(s)-1]
	default:
		return s
	}
}

func isVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// matchCase applies the casing pattern of the original to the replacement.
func matchCase(original, replacement string) string {
	if len(original) == 0 {
		return replacement
	}
	if unicode.IsUpper(rune(original[0])) {
		return strings.ToUpper(replacement[:1]) + replacement[1:]
	}
	return replacement
}

// goReservedWords are Go keywords and predeclared identifiers that can't be used as-is.
var goReservedWords = map[string]bool{
	"break": true, "default": true, "func": true, "interface": true, "select": true,
	"case": true, "defer": true, "go": true, "map": true, "struct": true,
	"chan": true, "else": true, "goto": true, "package": true, "switch": true,
	"const": true, "fallthrough": true, "if": true, "range": true, "type": true,
	"continue": true, "for": true, "import": true, "return": true, "var": true,

	// Predeclared identifiers worth avoiding.
	"error": true, "string": true, "bool": true, "int": true, "any": true,
	"len": true, "cap": true, "make": true, "new": true, "append": true,
	"copy": true, "delete": true, "panic": true, "recover": true, "close": true,
}

// SafeGoName returns the name escaped if it's a Go reserved word.
func SafeGoName(name string) string {
	if goReservedWords[name] {
		return name + "_"
	}
	return name
}
