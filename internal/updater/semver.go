package updater

import (
	"fmt"
	"strconv"
	"strings"
)

type semver struct {
	major, minor, patch int
}

func parseVersion(s string) (semver, error) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return semver{}, fmt.Errorf("invalid version format: %s", s)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return semver{}, fmt.Errorf("invalid major version: %s", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return semver{}, fmt.Errorf("invalid minor version: %s", parts[1])
	}
	// Patch may contain pre-release suffix like "0-2-gabcdef", take only the numeric part
	patchStr := strings.SplitN(parts[2], "-", 2)[0]
	patch, err := strconv.Atoi(patchStr)
	if err != nil {
		return semver{}, fmt.Errorf("invalid patch version: %s", patchStr)
	}

	return semver{major, minor, patch}, nil
}

// compareVersions returns 1 if a > b, -1 if a < b, 0 if equal.
func compareVersions(a, b string) (int, error) {
	va, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	vb, err := parseVersion(b)
	if err != nil {
		return 0, err
	}

	switch {
	case va.major != vb.major:
		return cmp(va.major, vb.major), nil
	case va.minor != vb.minor:
		return cmp(va.minor, vb.minor), nil
	case va.patch != vb.patch:
		return cmp(va.patch, vb.patch), nil
	default:
		return 0, nil
	}
}

func cmp(a, b int) int {
	if a > b {
		return 1
	}
	return -1
}
