package cli

func contains(items []string, s string) bool {
	for _, v := range items {
		if v == s {
			return true
		}
	}
	return false
}
