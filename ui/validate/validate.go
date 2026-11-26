// Package validate is the opt-in set of rules a form checks a text value
// against: required, length, range, pattern, e-mail and URL, each with the
// message the field shows when the rule fails. A rule is a function from the
// value to a message, empty when the value passes, so an application's own rule
// is a plain function of the same shape. A page builds its rules once, in the
// model, and checks them when the value changes or the form is submitted:
//
//	m.nameRules = []validate.Rule{
//		validate.Required("Enter a name"),
//		validate.MaxLength(32, "At most 32 characters"),
//	}
//	// when the value changes, or on submit:
//	ok := validate.Apply(&m.nameField, m.name, m.nameRules)
//
// Check returns the message of the first failing rule, in the order given;
// Apply puts it in a FormFieldState's Error and Invalid, so the field shows the
// message under the control. Lengths count characters (runes), as the text
// field's MaxLength does, and the rules other than Required pass an empty value:
// an optional field is checked only once something is entered.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Rules are closures made once; checking a value with them, URL
// aside, does not allocate.
package validate

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/ui"
)

// Rule checks a value and returns the message to show when it fails, or the
// empty string when it passes.
type Rule func(value string) string

// Required fails a value that is empty after trimming white space.
func Required(message string) Rule {
	return func(value string) string {
		if strings.TrimSpace(value) == "" {
			return message
		}
		return ""
	}
}

// MinLength fails a non-empty value shorter than n characters.
func MinLength(n int, message string) Rule {
	return func(value string) string {
		if value != "" && utf8.RuneCountInString(value) < n {
			return message
		}
		return ""
	}
}

// MaxLength fails a value longer than n characters.
func MaxLength(n int, message string) Rule {
	return func(value string) string {
		if utf8.RuneCountInString(value) > n {
			return message
		}
		return ""
	}
}

// Min fails a non-empty value that is not a number or is less than n.
func Min(n float64, message string) Rule {
	return func(value string) string {
		if value == "" {
			return ""
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err != nil || v < n {
			return message
		}
		return ""
	}
}

// Max fails a non-empty value that is not a number or is greater than n.
func Max(n float64, message string) Rule {
	return func(value string) string {
		if value == "" {
			return ""
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err != nil || v > n {
			return message
		}
		return ""
	}
}

// Pattern fails a non-empty value that does not match re.
func Pattern(re *regexp.Regexp, message string) Rule {
	return func(value string) string {
		if value != "" && !re.MatchString(value) {
			return message
		}
		return ""
	}
}

// Email fails a non-empty value that is not a bare e-mail address: a local
// part, one "@" and a dotted host, with no white space or angle brackets.
func Email(message string) Rule {
	return func(value string) string {
		if value == "" {
			return ""
		}
		at := strings.IndexByte(value, '@')
		if at <= 0 || strings.ContainsAny(value, " \t\r\n<>,;\"") || strings.IndexByte(value[at+1:], '@') >= 0 {
			return message
		}
		host := value[at+1:]
		if !strings.Contains(host, ".") || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") || strings.Contains(host, "..") {
			return message
		}
		return ""
	}
}

// URL fails a non-empty value that is not an absolute URL with a scheme and a
// host.
func URL(message string) Rule {
	return func(value string) string {
		if value == "" {
			return ""
		}
		if u, err := url.Parse(value); err != nil || u.Scheme == "" || u.Host == "" {
			return message
		}
		return ""
	}
}

// Check returns the message of the first rule that fails value, or the empty
// string when all pass.
func Check(value string, rules []Rule) string {
	for _, rule := range rules {
		if message := rule(value); message != "" {
			return message
		}
	}
	return ""
}

// Apply checks value and puts the result in the field's Error and Invalid. It
// reports whether the value passed.
func Apply(field *ui.FormFieldState, value string, rules []Rule) bool {
	field.Error = Check(value, rules)
	field.Invalid = field.Error != ""
	return !field.Invalid
}
