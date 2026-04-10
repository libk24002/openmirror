package mirror

import "testing"

func TestTTLForPathImmutableArtifactsUseLongTTL(t *testing.T) {
	metadataTTL := 10

	tests := []struct {
		name string
		path string
	}{
		{name: "docker blob digest", path: "/v2/library/alpine/blobs/sha256:abc123"},
		{name: "npm tarball", path: "/left-pad/-/left-pad-1.3.0.tgz"},
		{name: "pypi wheel", path: "/packages/pkg-1.0.0-py3-none-any.whl"},
		{name: "pypi source tarball", path: "/packages/pkg-1.0.0.tar.gz"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TTLForPath(tc.path, metadataTTL)
			if got != immutableTTLMinutes {
				t.Fatalf("TTLForPath(%q, %d) = %d, want %d", tc.path, metadataTTL, got, immutableTTLMinutes)
			}
		})
	}
}

func TestIsLargeArtifactPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "docker blob digest", path: "/v2/library/alpine/blobs/sha256:abc123", want: true},
		{name: "npm tarball", path: "/left-pad/-/left-pad-1.3.0.tgz", want: true},
		{name: "pypi wheel", path: "/packages/pkg-1.0.0-py3-none-any.whl", want: true},
		{name: "pypi source tarball", path: "/packages/pkg-1.0.0.tar.gz", want: true},
		{name: "docker manifest", path: "/v2/library/alpine/manifests/latest", want: false},
		{name: "npm metadata", path: "/left-pad", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsLargeArtifactPath(tc.path)
			if got != tc.want {
				t.Fatalf("IsLargeArtifactPath(%q) = %t, want %t", tc.path, got, tc.want)
			}
		})
	}
}

func TestTTLForPathMetadataUsesMetadataTTL(t *testing.T) {
	metadataTTL := 17
	path := "/v2/library/alpine/manifests/latest"

	got := TTLForPath(path, metadataTTL)
	if got != metadataTTL {
		t.Fatalf("TTLForPath(%q, %d) = %d, want %d", path, metadataTTL, got, metadataTTL)
	}
}
