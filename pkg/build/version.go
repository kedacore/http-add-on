package build

var (
	version   = "main"
	gitCommit string
)

// Version returns the current git SHA of commit the binary was built from
func Version() string {
	return version
}

// GitCommit stores the current commit hash
func GitCommit() string {
	return gitCommit
}
