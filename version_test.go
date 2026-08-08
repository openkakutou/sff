package sff

import (
	"regexp"
	"testing"
)

func TestVersion_IsNonEmptySemanticVersionString(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
	matched, err := regexp.MatchString(`^\d+\.\d+\.\d+$`, Version)
	if err != nil {
		t.Fatalf("regexp error: %v", err)
	}
	if !matched {
		t.Fatalf("Version %q is not a semantic version string", Version)
	}
}
