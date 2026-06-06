package version

var (
	Version = "0.1.0-dev"
	Commit  = "unknown"
	BuiltAt = "unknown"
)

func Display() string {
	out := Version
	if Commit != "" && Commit != "unknown" {
		out += " (" + Commit + ")"
	}
	return out
}
