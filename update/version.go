package update

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
)

// Compare compares two semantic versions and reports whether a is older than,
// equal to, or newer than b: -1, 0 or 1. A leading "v" and build metadata
// ("+build") are ignored, and pre-release identifiers order as Semantic
// Versioning 2.0.0 says ("1.0.0-rc.1" is older than "1.0.0"). It returns an
// error when either argument is not a version this package can order.
func Compare(a, b string) (int, error) {
	left, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	right, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	return left.compare(right), nil
}

// version is a parsed semantic version. The build metadata is dropped: it
// does not take part in precedence.
type version struct {
	major, minor, patch uint64
	pre                 []string
}

func parseVersion(text string) (version, error) {
	original := text
	text = strings.TrimPrefix(text, "v")
	if text == "" {
		return version{}, fmt.Errorf("update: %q is not a version", original)
	}
	// Build metadata comes last and is ignored for precedence, but its shape is
	// still checked: it is identifiers separated by dots.
	if index := strings.IndexByte(text, '+'); index >= 0 {
		if err := validateBuildMetadata(strings.Split(text[index+1:], ".")); err != nil {
			return version{}, fmt.Errorf("update: %q: %w", original, err)
		}
		text = text[:index]
	}
	var pre []string
	if index := strings.IndexByte(text, '-'); index >= 0 {
		pre = strings.Split(text[index+1:], ".")
		text = text[:index]
		if err := validatePrerelease(pre); err != nil {
			return version{}, fmt.Errorf("update: %q: %w", original, err)
		}
	}
	parts := strings.Split(text, ".")
	if len(parts) != 3 {
		return version{}, fmt.Errorf("update: %q is not a major.minor.patch version", original)
	}
	var numbers [3]uint64
	for index, part := range parts {
		number, err := parseNumber(part)
		if err != nil {
			return version{}, fmt.Errorf("update: %q: %w", original, err)
		}
		numbers[index] = number
	}
	return version{major: numbers[0], minor: numbers[1], patch: numbers[2], pre: pre}, nil
}

// parseNumber parses one dot-separated core number. A leading zero is
// refused, as Semantic Versioning requires.
func parseNumber(part string) (uint64, error) {
	if part == "" {
		return 0, fmt.Errorf("empty version number")
	}
	if len(part) > 1 && part[0] == '0' {
		return 0, fmt.Errorf("version number %q has a leading zero", part)
	}
	for index := 0; index < len(part); index++ {
		if part[index] < '0' || part[index] > '9' {
			return 0, fmt.Errorf("version number %q is not made of digits", part)
		}
	}
	number, err := strconv.ParseUint(part, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("version number %q: %w", part, err)
	}
	return number, nil
}

func validatePrerelease(identifiers []string) error {
	for _, identifier := range identifiers {
		if identifier == "" {
			return fmt.Errorf("empty pre-release identifier")
		}
		for index := 0; index < len(identifier); index++ {
			character := identifier[index]
			allowed := character >= '0' && character <= '9' ||
				character >= 'a' && character <= 'z' ||
				character >= 'A' && character <= 'Z' ||
				character == '-'
			if !allowed {
				return fmt.Errorf("pre-release identifier %q has %q", identifier, character)
			}
		}
		if isNumericIdentifier(identifier) && len(identifier) > 1 && identifier[0] == '0' {
			return fmt.Errorf("pre-release identifier %q has a leading zero", identifier)
		}
	}
	return nil
}

func isNumericIdentifier(text string) bool {
	if text == "" {
		return false
	}
	for index := 0; index < len(text); index++ {
		if text[index] < '0' || text[index] > '9' {
			return false
		}
	}
	return true
}

func (left version) compare(right version) int {
	if order := cmp.Compare(left.major, right.major); order != 0 {
		return order
	}
	if order := cmp.Compare(left.minor, right.minor); order != 0 {
		return order
	}
	if order := cmp.Compare(left.patch, right.patch); order != 0 {
		return order
	}
	return comparePrerelease(left.pre, right.pre)
}

func comparePrerelease(left, right []string) int {
	switch {
	case len(left) == 0 && len(right) == 0:
		return 0
	case len(left) == 0:
		// A release is newer than any pre-release of the same version.
		return 1
	case len(right) == 0:
		return -1
	}
	for index := 0; index < len(left) && index < len(right); index++ {
		if order := comparePrereleaseIdentifier(left[index], right[index]); order != 0 {
			return order
		}
	}
	return cmp.Compare(len(left), len(right))
}

func comparePrereleaseIdentifier(left, right string) int {
	leftNumeric := isNumericIdentifier(left)
	rightNumeric := isNumericIdentifier(right)
	switch {
	case leftNumeric && rightNumeric:
		// Neither has a leading zero, so the longer number is larger, and equal
		// lengths compare digit by digit.
		if len(left) != len(right) {
			return cmp.Compare(len(left), len(right))
		}
		return strings.Compare(left, right)
	case leftNumeric:
		// Numeric identifiers have lower precedence than alphanumeric ones.
		return -1
	case rightNumeric:
		return 1
	default:
		return strings.Compare(left, right)
	}
}

func validateBuildMetadata(identifiers []string) error {
	for _, identifier := range identifiers {
		if identifier == "" {
			return fmt.Errorf("empty build metadata identifier")
		}
		for index := 0; index < len(identifier); index++ {
			character := identifier[index]
			allowed := character >= '0' && character <= '9' ||
				character >= 'a' && character <= 'z' ||
				character >= 'A' && character <= 'Z' ||
				character == '-'
			if !allowed {
				return fmt.Errorf("build metadata identifier %q has %q", identifier, character)
			}
		}
	}
	return nil
}
