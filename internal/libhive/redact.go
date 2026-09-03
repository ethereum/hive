package libhive

import "strings"

// redactedValue replaces the value of a sensitive build argument wherever hive
// records or serves its own invocation: the run metadata in the results JSON
// and the /hive endpoint of the simulation API.
const redactedValue = "<redacted>"

// isSensitiveBuildArg reports whether the value of a build argument must not be
// recorded. The comparison is case-insensitive because client files and
// --sim.buildarg conventionally use lowercase keys (github_token) while
// excludedBuildArgs lists them in uppercase.
func isSensitiveBuildArg(key string) bool {
	return excludedBuildArgs[strings.ToUpper(key)]
}

// redactCommandArgs returns a copy of a hive command line in which the values of
// sensitive build arguments are replaced by redactedValue. It recognises both the
// "--flag KEY=VALUE" and the "--flag=KEY=VALUE" spelling of any flag whose name
// ends in ".buildarg" (currently --sim.buildarg). All other arguments are copied
// unchanged, and the input slice is not modified.
func redactCommandArgs(args []string) []string {
	out := make([]string, len(args))
	copy(out, args)
	for i := 0; i < len(out); i++ {
		flag := strings.TrimLeft(out[i], "-")
		if flag == out[i] {
			continue // not a flag
		}
		name, inline, hasInline := strings.Cut(flag, "=")
		if !strings.HasSuffix(name, ".buildarg") {
			continue
		}
		if hasInline {
			// inline is a suffix of out[i]: keep the "--name=" prefix as written.
			out[i] = out[i][:len(out[i])-len(inline)] + redactBuildArg(inline)
		} else if i+1 < len(out) {
			i++
			out[i] = redactBuildArg(out[i])
		}
	}
	return out
}

// redactBuildArg redacts the value of a KEY=VALUE build argument when KEY is
// sensitive. Anything else is returned as is.
func redactBuildArg(kv string) string {
	key, _, ok := strings.Cut(kv, "=")
	if !ok || !isSensitiveBuildArg(key) {
		return kv
	}
	return key + "=" + redactedValue
}
