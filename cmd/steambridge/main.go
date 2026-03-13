package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"steambridge/internal/steam"
	"steambridge/internal/switchboard"
	"steambridge/internal/tap"
)

func main() {
	ifaceName := flag.String("iface", "steambridge0", "Name of the TAP interface to create/bind")
	peerID := flag.Uint64("peer", 0, "SteamID of the remote peer to bootstrap")
	flag.Parse()
	log.Println("Starting SteamBridge...")

	table := switchboard.NewTable()

	log.Printf("Setting up TAP interface: %s\n", *ifaceName)
	tapDev, err := tap.NewDevice(*ifaceName)
	if err != nil {
		log.Fatalf("Fatal: Could not create TAP device: %v", err)
	}
	defer tapDev.Close()

	router := switchboard.NewRouter(tapDev, nil, table)

	log.Println("Initializing Steamworks API...")
	client := steam.NewClient(router)

	if *peerID != 0 {
		client.AddPeer(*peerID)
	}

	router.SetSteamSender(client)

	log.Println("Starting Egress and Ingress routing loops...")
	go router.StartEgress()
	go client.ReadLoop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	log.Printf("SteamBridge is live on interface '%s'! Press Ctrl+C to exit.\n", *ifaceName)
	<-sigCh

	log.Println("\nReceived shutdown signal. Closing bridge...")

	client.Close()

	log.Println("Shutdown complete.")
}
