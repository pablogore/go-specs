package plan

// buildPath joins parts with '/' using a zero-allocation builder.
func buildPath(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	total := len(parts) - 1
	for _, part := range parts {
		total += len(part)
	}
	buf := make([]byte, 0, total)
	for i, part := range parts {
		if i > 0 {
			buf = append(buf, '/')
		}
		buf = append(buf, part...)
	}
	return string(buf)
}
