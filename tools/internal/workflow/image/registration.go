package image

// Registrations returns the command workflows owned by the image bucket.
func Registrations() []Registration {
	return []Registration{
		flashRegistration(),
		imageRegistration(),
	}
}
