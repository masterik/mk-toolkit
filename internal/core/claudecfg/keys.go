package claudecfg

// The closed table of setting keys under the sections mkit reads. A key in a
// user's file that is missing here is reported as unknown: Claude Code ignores a
// misspelled key without a word. It lists keys only, not meanings, and it can
// lag the documentation — the sandbox-audit skill checks names against the
// current docs, so a miss here is a prompt to look, not a verdict.
//
// Keys under `sandbox` are keyed by their parent: "" for the top level,
// "network" and "filesystem" for the two nested objects.
var knownSandbox = map[string]map[string]bool{
	"": set("enabled", "autoAllowBashIfSandboxed", "allowUnsandboxedCommands", "excludedCommands",
		"network", "filesystem", "ignoreViolations", "enableWeakerNestedSandbox",
		"enableWeakerNetworkIsolation", "ripgrep"),
	"network": set("allowedDomains", "deniedDomains", "allowUnixSockets", "allowAllUnixSockets",
		"allowLocalBinding", "allowMachLookup", "httpProxyPort", "socksProxyPort"),
	"filesystem": set("allowWrite", "denyWrite", "allowRead", "denyRead", "disabled"),
}

var knownPermissions = set("allow", "ask", "deny", "additionalDirectories", "defaultMode",
	"disableBypassPermissionsMode")

var knownAutoMode = set("allow", "soft_deny", "environment")

func set(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}
