package crossplatform

import "runtime"

func IsMac() bool {
	return runtime.GOOS == "darwin"
}
