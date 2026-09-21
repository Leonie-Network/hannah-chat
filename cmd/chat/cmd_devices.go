package main

func init() {
	registerCommand(localCommand{
		name:       "devices",
		help:       "browse and control Hannah's devices",
		trustLevel: deviceMenuTrustLevel,
		run:        cmdDevices,
	})
}

func cmdDevices(s *session) {
	startDeviceMenu(s)
}
