package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 1. Parse CLI Flags
	targetSteamID := flag.Uint64("steamid", 0, "Target SteamID to connect to (0 for lobby/host mode)")
	lobbyID := flag.Uint64("lobby", 0, "Target LobbyID to join")
	ifaceID := flag.String("iface", "steambridge0", "Name of the TAP interface to create")
	flag.Parse()

	log.Printf("Starting steambridge on interface: %s\n", *ifaceID)
	if *targetSteamID != 0 {
		log.Printf("Targeting SteamID: %d\n", *targetSteamID)
	} else if *lobbyID != 0 {
		log.Printf("Targeting LobbyID: %d\n", *lobbyID)
	} else {
		log.Println("No target specified, running in host/listen mode.")
	}

	// 2. Setup Context for Graceful Shutdown
	// This context will be passed down to your TAP and Steam workers.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for OS signals (Ctrl+C, SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\nShutdown signal received. Initiating graceful cleanup...")
		cancel() // This triggers ctx.Done() across all your goroutines
	}()

	// 3. Application Initialization (Placeholders for upcoming phases)
	// TODO: Initialize Steamworks (AppID 480)
	// TODO: Initialize TAP interface with MTU 1280
	// TODO: Initialize Switchboard/MAC Table
	// TODO: Start Egress/Ingress routing Goroutines

	// 4. Block until context is cancelled
	<-ctx.Done()

	// TODO: Execute final cleanup calls (Close TAP, SteamAPI_Shutdown)
	log.Println("Graceful shutdown complete.")
}
