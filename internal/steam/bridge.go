package steam

import (
	"errors"
	"log"
	"runtime"

	"github.com/ebitengine/purego"
)

var (
	bridgeInit         func() bool
	bridgeRunCallbacks func()
	bridgeShutdown     func()

	bridgeReceive         func(buffer *byte, bufferSize int, outSteamIDRemote *uint64) int32
	bridgeSend            func(steamId uint64, data *byte, size int) bool
	bridgeSendReliable    func(steamId uint64, data *byte, size int) bool
	bridgeGetLocalSteamID func() uint64

	bridgeGetJoinRequest     func() uint64
	bridgeSetJoinable        func(joinable bool)
	bridgeOpenFriendsOverlay func()
)

// tryRegisterLibFunc registers a DLL export but logs a warning instead of panicking
// if the symbol isn't found. Used for exports added after an existing DLL was built.
func tryRegisterLibFunc(fptr interface{}, lib uintptr, name string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("bridge: optional export %s not found (rebuild cbridge to enable): %v", name, r)
		}
	}()
	purego.RegisterLibFunc(fptr, lib, name)
}

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
	if err != nil {
		return err
	}

	purego.RegisterLibFunc(&bridgeInit, libc, "Bridge_Init")
	purego.RegisterLibFunc(&bridgeRunCallbacks, libc, "Bridge_RunCallbacks")
	purego.RegisterLibFunc(&bridgeShutdown, libc, "Bridge_Shutdown")
	purego.RegisterLibFunc(&bridgeReceive, libc, "Bridge_Receive")
	purego.RegisterLibFunc(&bridgeSend, libc, "Bridge_Send")
	purego.RegisterLibFunc(&bridgeSendReliable, libc, "Bridge_SendReliable")
	purego.RegisterLibFunc(&bridgeGetLocalSteamID, libc, "Bridge_GetLocalSteamID")

	// These three exports were added in Phase 2.2. If the DLL predates that build,
	// fall back to no-ops so the app still starts — only the join features are absent.
	bridgeGetJoinRequest = func() uint64 { return 0 }
	bridgeSetJoinable = func(bool) {}
	bridgeOpenFriendsOverlay = func() {}
	tryRegisterLibFunc(&bridgeGetJoinRequest, libc, "Bridge_GetJoinRequest")
	tryRegisterLibFunc(&bridgeSetJoinable, libc, "Bridge_SetJoinable")
	tryRegisterLibFunc(&bridgeOpenFriendsOverlay, libc, "Bridge_OpenFriendsOverlay")

	return nil
}
