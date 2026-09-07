#pragma once
// Test-only control surface for fake_steam_api.cpp. Tests use these to feed
// the fake Steam layer and inspect what steam_bridge.cpp did with it.

#include <cstdint>
#include <string>
#include <vector>

struct FakeSentPacket {
    uint64_t toSteamId;
    std::vector<uint8_t> data;
    bool reliable;
};

// Resets ALL fake-Steam state, including the callback registry (see
// steam_api.h's FakeSteam_CallbackRegistry) so leaked BridgeCallbacks
// instances from a prior test's Bridge_Init() don't receive this test's
// events. Call at the start of every test, before Bridge_Init().
void FakeSteam_Reset();

// -- Local identity --
void FakeSteam_SetLocalSteamID(uint64_t steamId);

// -- P2P networking --
void FakeSteam_QueueIncomingPacket(uint64_t fromSteamId, const uint8_t* data, size_t size);
size_t FakeSteam_PendingIncomingPacketCount();
void FakeSteam_SetSendShouldFail(bool shouldFail);
const std::vector<FakeSentPacket>& FakeSteam_GetSentPackets();

// -- Friends --
void FakeSteam_SetFriendRelationship(uint64_t steamId, int relationship); // EFriendRelationship value
const std::string& FakeSteam_GetRichPresence();
bool FakeSteam_WasOverlayActivated();

// -- Callbacks: queue a fake event, delivered on the next Bridge_RunCallbacks() --
void FakeSteam_TriggerP2PSessionRequest(uint64_t remoteSteamId);
void FakeSteam_TriggerJoinRequested(uint64_t friendSteamId);
