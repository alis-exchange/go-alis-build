package mapper

// resetConfigForTest clears package-level config between tests.
func resetConfigForTest() {
	defaultConfig = Config{}
}
