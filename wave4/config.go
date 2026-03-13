package wave4

type Config struct {
	// The root directory to watch for file changes
	// and against which all other paths and patterns
	// in the config are resolved. Set this path
	// relative to the current working directory of
	// the underlying OS process.
	RootDir string
}
