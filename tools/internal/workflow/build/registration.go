package build

// Registrations returns the command workflows owned by the build bucket.
func Registrations() []Registration {
	return []Registration{buildRegistration()}
}
