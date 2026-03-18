package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"steambridge/internal/facade"
)

func main() {
	ifaceName := flag.String("ifaceName", "steambridge0", "Name of the TAP interface to create/bind")
	ifaceID := flag.String("ifaceID", "steambridge0", "ID of the TAP interface to create/bind")
	peerID := flag.Uint64("peer", 0, "SteamID of the remote peer to bootstrap")
	enableFirewall := flag.Bool("firewall", true, "Enable firewall")
	flag.Parse()
	log.Println("Starting SteamBridge...")

	config := facade.Config{
		IfaceName:       *ifaceName,
		IfaceID:         *ifaceID,
		BootstrapPeerID: *peerID,
	}

	facade, err := facade.NewFacade(config)
	if err != nil {
		log.Fatalf("Error creating facade: %s", err)
	}
	facade.SetFirewall(*enableFirewall)
	facade.AddPort(80)
	facade.AddPort(443)
	facade.AddPort(3000)
	facade.AddPort(5021)
	facade.AddPort(5000)
	facade.AddPort(25565)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		if err := facade.Start(ctx); err != nil {
			log.Printf("facade stopped: %s", err)
			cancel()
		}
	}()

	<-ctx.Done()

	facade.Stop()

	log.Println("\nReceived shutdown signal. Closing bridge...")

	log.Println("Shutdown complete.")
}
