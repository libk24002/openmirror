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

func TestTTLForPathMetadataUsesMetadataTTL(t *testing.T) {
	metadataTTL := 17
	path := "/v2/library/alpine/manifests/latest"

	got := TTLForPath(path, metadataTTL)
	if got != metadataTTL {
		t.Fatalf("TTLForPath(%q, %d) = %d, want %d", path, metadataTTL, got, metadataTTL)
	}
}
