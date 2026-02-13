package tooling

func resolveExistingPathKeyFromProbeLookup(
	pathKeyByProbe map[string]string,
	probeKey string,
) string {
	if pathKeyByProbe == nil || probeKey == "" {
		return ""
	}
	return pathKeyByProbe[probeKey]
}

func recordPathKeyForProbeLookupIfAbsent(
	pathKeyByProbe map[string]string,
	probeKey string,
	pathKey string,
) {
	if pathKeyByProbe == nil || probeKey == "" || pathKey == "" {
		return
	}
	if _, exists := pathKeyByProbe[probeKey]; exists {
		return
	}
	pathKeyByProbe[probeKey] = pathKey
}
