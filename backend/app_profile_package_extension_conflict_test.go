package backend

import (
	"testing"

	"ant-chrome/backend/internal/browser"
)

func TestDecideProfilePackageExtensionImport(t *testing.T) {
	tests := []struct {
		name     string
		existing browser.Extension
		incoming browser.Extension
		want     profilePackageExtensionImportDecision
	}{
		{
			name:     "same hash reuses",
			existing: browser.Extension{Version: "1.0.0", PackageHash: "abc"},
			incoming: browser.Extension{Version: "2.0.0", PackageHash: "ABC"},
			want:     profilePackageExtensionReuse,
		},
		{
			name:     "newer version updates",
			existing: browser.Extension{Version: "1.2.3", PackageHash: "old"},
			incoming: browser.Extension{Version: "1.3.0", PackageHash: "new"},
			want:     profilePackageExtensionUpdate,
		},
		{
			name:     "older version reuses target",
			existing: browser.Extension{Version: "2.0.0", PackageHash: "new"},
			incoming: browser.Extension{Version: "1.9.9", PackageHash: "old"},
			want:     profilePackageExtensionReuse,
		},
		{
			name:     "same version different hash conflicts",
			existing: browser.Extension{Version: "1.0.0", PackageHash: "one"},
			incoming: browser.Extension{Version: "1.0.0", PackageHash: "two"},
			want:     profilePackageExtensionConflict,
		},
		{
			name:     "same version different remote source conflicts",
			existing: browser.Extension{Version: "1.0.0", SourceURL: "https://store.example/a"},
			incoming: browser.Extension{Version: "1.0.0", SourceURL: "https://store.example/b"},
			want:     profilePackageExtensionConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := decideProfilePackageExtensionImport(tt.existing, tt.incoming)
			if got != tt.want {
				t.Fatalf("decision = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompareExtensionVersions(t *testing.T) {
	tests := []struct {
		left       string
		right      string
		want       int
		comparable bool
	}{
		{left: "1.2", right: "1.2.0", want: 0, comparable: true},
		{left: "1.2.1", right: "1.2.0", want: 1, comparable: true},
		{left: "1.1.9", right: "1.2", want: -1, comparable: true},
		{left: "1.2.beta", right: "1.2.0", comparable: false},
		{left: "1.2.3.4.5", right: "1.2.3", comparable: false},
	}
	for _, tt := range tests {
		got, comparable := compareExtensionVersions(tt.left, tt.right)
		if comparable != tt.comparable || (comparable && got != tt.want) {
			t.Fatalf("compareExtensionVersions(%q,%q) = (%d,%v), want (%d,%v)", tt.left, tt.right, got, comparable, tt.want, tt.comparable)
		}
	}
}
