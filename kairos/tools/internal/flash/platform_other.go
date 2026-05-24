//go:build !darwin

package flash

// On non-darwin builds the flasher refuses to start. The CLI still
// compiles (so the binary works for everything else) but the runner's
// first call will surface the unsupported-platform error.
func currentPlatform() (Platform, error) {
	return nil, ErrUnsupportedPlatform
}
