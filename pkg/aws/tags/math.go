package tags

// Diff returns a map with key-value pairs exist in t1, but do not exist in
// t2.
func Diff(t1 map[string]string, t2 map[string]string) map[string]string {
	if t2 == nil {
		return t1
	}
	diff := map[string]string{}

	for key, value1 := range t1 {
		if value2, ok := t2[key]; ok && value1 == value2 {
			// t1 tag found in t2
			continue
		}

		// t1 has a tag that is not found in t2
		diff[key] = value1
	}

	return diff
}
