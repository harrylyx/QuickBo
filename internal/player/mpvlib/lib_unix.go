//go:build darwin || linux || freebsd

package mpvlib

import "github.com/ebitengine/purego"

func openLib(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

func symLib(lib uintptr, name string) (uintptr, error) {
	return purego.Dlsym(lib, name)
}
