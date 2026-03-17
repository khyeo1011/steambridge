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

	bridgeReceive      func(buffer *byte, bufferSize int, outSteamIDRemote *uint64) int32
	bridgeSend         func(steamId uint64, data *byte, size int) bool
	bridgeSendReliable func(steamId uint64, data *byte, size int) bool
)

func LoadLibrary() error {
	var target string
	switch runtime.GOOS {
	case "windows":
		target = "libsteam_bridge.dll"
	case "linux":
		target = "./libsteam_bridge.so"
	default:
		return errors.New("unsupported OS")
	}

	libc, err := openLibrary(target)

	purego.RegisterLibFunc(&bridgeInit, libc, "Bridge_Init")
	purego.RegisterLibFunc(&bridgeRunCallbacks, libc, "Bridge_RunCallbacks")
	purego.RegisterLibFunc(&bridgeShutdown, libc, "Bridge_Shutdown")
	purego.RegisterLibFunc(&bridgeReceive, libc, "Bridge_Receive")
	purego.RegisterLibFunc(&bridgeSend, libc, "Bridge_Send")
	purego.RegisterLibFunc(&bridgeSendReliable, libc, "Bridge_SendReliable")

	return err
}
