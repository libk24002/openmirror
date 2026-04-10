package mirror

import "strings"

const immutableTTLMinutes = 7 * 24 * 60

func TTLForPath(path string, metadataTTLMinutes int) int {
	if strings.Contains(path, "/blobs/sha256:") {
		return immutableTTLMinutes
	}

	if strings.HasSuffix(path, ".tgz") || strings.HasSuffix(path, ".whl") || strings.HasSuffix(path, ".tar.gz") {
		return immutableTTLMinutes
	}

	return metadataTTLMinutes
}
