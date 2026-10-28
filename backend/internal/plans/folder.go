package plans

import (
	"regexp"
	"strings"
)

var notSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug is the plan's name as it appears in object keys: lower case ASCII letters, digits and
// dashes, and "plan" for a name with none of those.
func Slug(name string) string {
	slug := strings.Trim(notSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		return "plan"
	}
	return slug
}

// Folder is where the plan's archives go in each of its storages.
func Folder(name string) string { return Slug(name) + "/" }
