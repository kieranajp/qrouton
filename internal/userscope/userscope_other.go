//go:build !linux

package userscope

const Supported = false

func Available() bool { return false }

func Wrap(argv []string, _ string, _ ...string) []string { return argv }

func SessionUnit(string) string { return "" }

func TabUnit() string { return "" }

func WorkbenchUnit() string { return "" }
