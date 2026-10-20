package main

func isCLIHelp(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if args[0] == "help" {
		return true
	}
	for _, arg := range args[1:] {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}
