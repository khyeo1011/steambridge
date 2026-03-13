package steam

import (
	"errors"
	"runtime"

	"github.com/ebitengine/purego"
)

var (
	bridgeInit         func() bool
	bridgeRunCallbacks func()
	bridgeShutdown     func()

	bridgeReceive func(buffer *byte, bufferSize int, outSteamIDRemote *uint64) int32
	bridgeSend    func(steamId uint64, data *byte, size int, sendType int) bool
)

func LoadLibrary() error {
	var target string
	if runtime.GOOS == "windows" {
		target = "libsteam_bridge.dll"
	} else if runtime.GOOS == "linux" {
		target = "./libsteam_bridge.so"
	} else {
		return errors.New("unsupported OS")
	}

	libc, err := openLibrary(target)

	purego.RegisterLibFunc(&bridgeInit, libc, "Bridge_Init")
	purego.RegisterLibFunc(&bridgeRunCallbacks, libc, "Bridge_RunCallbacks")
	purego.RegisterLibFunc(&bridgeShutdown, libc, "Bridge_Shutdown")
	purego.RegisterLibFunc(&bridgeReceive, libc, "Bridge_Receive")
	purego.RegisterLibFunc(&bridgeSend, libc, "Bridge_Send")

	return err
}
