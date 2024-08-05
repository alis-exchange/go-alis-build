package sproto

import (
	"fmt"
	"strconv"
	"strings"
)

type RowKeyConverter struct {
	// If set, the resource name will be shortened by using the first letter of each collection identifier
	// E.g. /projects/{project}/instances/{instance}/databases/{database} -> /p/{project}/i/{instance}/d/{database}
	AbbreviateCollectionIdentifiers bool
	// If set, the version in the resource id will be converted to a number such that the latest version is shown first
	// when listed lexicographically (most common default ordering in databases like Bigtable and Spanner)
	LatestVersionFirst bool
}

func (r *RowKeyConverter) GetRowKey(resource string) (string, error) {
	key := resource
	if r.AbbreviateCollectionIdentifiers {
		key = abbreviateCollectionIdentifiers(key)
	}
	if r.LatestVersionFirst {
		parts := strings.Split(key, "/")

		// extract major, minor and patch version from the resource id
		id := parts[len(parts)-1]
		versionParts := strings.Split(id, "-")
		majorVersion, err := strconv.ParseInt(versionParts[0], 10, 32)
		if err != nil {
			return "", fmt.Errorf("invalid major version")
		}
		minorVersion, err := strconv.ParseInt(versionParts[1], 10, 32)
		if err != nil {
			return "", fmt.Errorf("invalid minor version")
		}
		patchVersion, err := strconv.ParseInt(versionParts[2], 10, 32)
		if err != nil {
			return "", fmt.Errorf("invalid patch version")
		}

		// reverse the version number
		maxVersion := int32(2140000000)
		versionValue := int32(majorVersion-1)*100000000 + int32(minorVersion)*10000 + int32(patchVersion)
		reversedVersion := int32(maxVersion) - versionValue

		// create new key with id padded to 10 digits
		newParts := append(parts[:len(parts)-1], fmt.Sprintf("%010d", reversedVersion))
		key = strings.Join(newParts, "/")
	}
	return key, nil
}

func (r *RowKeyConverter) GetRowKeyPrefix(parentResource string) (string, error) {
	prefix := parentResource
	if r.AbbreviateCollectionIdentifiers {
		prefix = abbreviateCollectionIdentifiers(prefix)
	}
	return prefix, nil
}

func abbreviateCollectionIdentifiers(resource string) string {
	parts := strings.Split(resource, "/")
	for i := 0; i < len(parts); i++ {
		if i%2 == 0 {
			// use first letter of each collection identifier
			parts[i] = string(parts[i][0])
		}
	}
	return strings.Join(parts, "/")
}
