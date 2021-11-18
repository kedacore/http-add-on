package build

var version string

// Version returns the current git SHA of commit the binary was built from
func Version() string {
	return version
}
