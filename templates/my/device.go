package my

func (s Session) Label(tr func(string) string) string {
	if s.Browser != "" && s.OS != "" {
		return s.Browser + " (" + s.OS + ")"
	}
	if s.Browser != "" {
		return s.Browser
	}
	if s.OS != "" {
		return s.OS
	}
	if s.Device != "" {
		return s.Device
	}
	return tr("web.unknown_device")
}
