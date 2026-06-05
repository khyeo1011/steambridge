package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"steambridge/internal/facade"
)

func main() {
	if runtime.GOOS == "windows" {
		exePath, _ := os.Executable()
		exeDir := filepath.Dir(exePath)

		_, filename, _, _ := runtime.Caller(0)
		rootDir := filepath.Join(filepath.Dir(filename), "..", "..")
		sourceDLL := filepath.Join(rootDir, "wintun.dll")

		destDLL := filepath.Join(exeDir, "wintun.dll")
		if sourceFile, err := os.Open(sourceDLL); err == nil {
			if destFile, err := os.Create(destDLL); err == nil {
				io.Copy(destFile, sourceFile)
				destFile.Close()
			}
			sourceFile.Close()
		}
	}
	ifaceName := flag.String("ifaceName", "steambridge0", "Name of the TAP interface to create/bind")
	ifaceID := flag.String("ifaceID", "steambridge0", "ID of the TAP interface to create/bind")
	peerID := flag.Uint64("peer", 0, "SteamID of the remote peer to bootstrap")
	enableFirewall := flag.Bool("firewall", false, "Enable firewall")
	flag.Parse()
	log.Printf("Firewall enabled: %t", *enableFirewall)
	log.Println("Starting SteamBridge...")

	config := facade.Config{
		IfaceName:       *ifaceName,
		IfaceID:         *ifaceID,
		BootstrapPeerID: *peerID,
	}

	facade := facade.NewFacade(config)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		if err := facade.Start(ctx); err != nil {
			log.Printf("facade stopped: %s", err)
			cancel()
		} else {
			facade.SetFirewall(*enableFirewall)
			facade.AddPort(80)
			facade.AddPort(443)
			facade.AddPort(3000)
			facade.AddPort(5021)
			facade.AddPort(5000)
			facade.AddPort(25565)
		}
	}()

	<-ctx.Done()

	facade.Stop()

	log.Println("\nReceived shutdown signal. Closing bridge...")

	log.Println("Shutdown complete.")
}
