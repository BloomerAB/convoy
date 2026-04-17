package app

// Command represents a parsed command from the command bar.
type Command struct {
	Name string
	Args []string
}

// ParseCommand parses a command string like "cluster prod" into a Command.
func ParseCommand(input string) Command {
	parts := splitArgs(input)
	if len(parts) == 0 {
		return Command{}
	}
	return Command{
		Name: parts[0],
		Args: parts[1:],
	}
}

func splitArgs(s string) []string {
	var parts []string
	var current []byte
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			if len(current) > 0 {
				parts = append(parts, string(current))
				current = current[:0]
			}
		} else {
			current = append(current, s[i])
		}
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}
