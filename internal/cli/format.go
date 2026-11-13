package cli

// Small formatting helpers shared by the fact commands' `key=value` renderers.

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
