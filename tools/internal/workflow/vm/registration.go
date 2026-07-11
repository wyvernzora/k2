package vm

// Registrations returns the command workflows owned by the VM bucket.
func Registrations() []Registration {
	return []Registration{vmRegistration()}
}
