package manifest

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// LinuxCategories are the freedesktop.org main categories a menu entry can be
// filed under; see the Desktop Menu Specification.
var LinuxCategories = []string{
	"AudioVideo", "Audio", "Video", "Development", "Education", "Game", "Graphics",
	"Network", "Office", "Science", "Settings", "System", "Utility",
}

// DefaultLinuxCategory is the category of an application that sets none.
const DefaultLinuxCategory = "Utility"

var (
	maintainerPattern = regexp.MustCompile(`^[^<>\x00-\x1f\x7f]*[^<>\x00-\x1f\x7f\s] <[^<>\s@]+@[^<>\s@]+>$`)
	// dependsPattern accepts a Debian dependency loosely: package names, version
	// relations in parentheses, architecture and alternatives (|). dpkg has the
	// final say when the package is installed.
	dependsPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+.:()<>=~| -]*$`)
	// archDependsPattern accepts an Arch dependency: a package name and an
	// optional version relation, such as vulkan-icd-loader or libxcb>=1.16.
	archDependsPattern = regexp.MustCompile(`^[a-z0-9@._+][a-z0-9@._+-]*( ?(>=|<=|=|>|<) ?[A-Za-z0-9._+:-]+)?$`)
)

// LinuxCategory is the category the menu entry is filed under.
func (linux Linux) LinuxCategory() string {
	if linux.Category == "" {
		return DefaultLinuxCategory
	}
	return linux.Category
}

func hasControl(text string) bool {
	return strings.IndexFunc(text, func(r rune) bool { return r < ' ' || r == 0x7f }) >= 0
}

// validateLinux checks the [linux] table.
func (manifest *Manifest) validateLinux() error {
	linux := manifest.Linux
	fail := func(key, format string, arguments ...any) error {
		return fmt.Errorf("[linux] %s: %s", key, fmt.Sprintf(format, arguments...))
	}
	if linux.Category != "" && !slices.Contains(LinuxCategories, linux.Category) {
		return fail("category", "%q is not a freedesktop main category; use one of: %s", linux.Category, strings.Join(LinuxCategories, ", "))
	}
	if hasControl(linux.Comment) {
		return fail("comment", "must be a single line without control characters")
	}
	if linux.Maintainer != "" && !maintainerPattern.MatchString(linux.Maintainer) {
		return fail("maintainer", "%q must look like Name <name@example.com>", linux.Maintainer)
	}
	if linux.Homepage != "" {
		address, err := url.Parse(linux.Homepage)
		if err != nil || (address.Scheme != "http" && address.Scheme != "https") || address.Host == "" || strings.ContainsAny(linux.Homepage, " \t") || hasControl(linux.Homepage) {
			return fail("homepage", "%q must be an http or https address", linux.Homepage)
		}
	}
	for _, dependency := range linux.Depends {
		if !dependsPattern.MatchString(dependency) || strings.TrimSpace(dependency) != dependency {
			return fail("depends", "%q is not a Debian dependency such as libfoo1 (>= 1.2)", dependency)
		}
	}
	for _, dependency := range linux.ArchDepends {
		if !archDependsPattern.MatchString(dependency) || strings.TrimSpace(dependency) != dependency {
			return fail("arch_depends", "%q is not an Arch dependency such as libxcb or libxcb>=1.16", dependency)
		}
	}
	for _, keyword := range linux.Keywords {
		if keyword == "" || strings.Contains(keyword, ";") || hasControl(keyword) {
			return fail("keywords", "%q must be non-empty and cannot contain ; or control characters", keyword)
		}
	}
	return nil
}
