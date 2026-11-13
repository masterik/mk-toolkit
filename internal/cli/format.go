package cli

// Small formatting helpers shared by the commands' renderers.

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
