package mirror

import "strings"

const immutableTTLMinutes = 7 * 24 * 60

func TTLForPath(path string, metadataTTLMinutes int) int {
	if IsLargeArtifactPath(path) {
		return immutableTTLMinutes
	}

	return metadataTTLMinutes
}

func IsLargeArtifactPath(path string) bool {
	if strings.Contains(path, "/blobs/sha256:") {
		return true
	}

	return strings.HasSuffix(path, ".tgz") || strings.HasSuffix(path, ".whl") || strings.HasSuffix(path, ".tar.gz")
}
