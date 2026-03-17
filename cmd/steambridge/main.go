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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := facade.Start(ctx); err != nil { /* log fatal */
		log.Fatalf("Error starting facade: %s", err)
	}

	<-ctx.Done()

	facade.Stop()

	log.Println("\nReceived shutdown signal. Closing bridge...")

	log.Println("Shutdown complete.")
}
