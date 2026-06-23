#include "steam_bridge.h"
#include <steam/steam_api.h>
#include <steam/steam_api_flat.h>
#include <atomic>

// SteamID of a friend whose "Join Game" we received but haven't acted on yet.
// Go drains this once per ReadLoop iteration via Bridge_GetJoinRequest().
std::atomic<uint64_t> g_pendingJoin{0};

class BridgeCallbacks {
public:
    BridgeCallbacks()
        : m_CallbackP2PSessionRequest(this, &BridgeCallbacks::OnP2PSessionRequest),
          m_CallbackJoinRequested(this, &BridgeCallbacks::OnGameRichPresenceJoinRequested) {}

    STEAM_CALLBACK(BridgeCallbacks, OnP2PSessionRequest, P2PSessionRequest_t, m_CallbackP2PSessionRequest);
    STEAM_CALLBACK(BridgeCallbacks, OnGameRichPresenceJoinRequested, GameRichPresenceJoinRequested_t, m_CallbackJoinRequested);
};

void BridgeCallbacks::OnP2PSessionRequest(P2PSessionRequest_t *pCallback) {
    SteamNetworking()->AcceptP2PSessionWithUser(pCallback->m_steamIDRemote);
}

void BridgeCallbacks::OnGameRichPresenceJoinRequested(GameRichPresenceJoinRequested_t *pCallback) {
    // m_steamIDFriend is the host whose session the local user chose to join.
    g_pendingJoin.store(pCallback->m_steamIDFriend.ConvertToUint64());
}

BridgeCallbacks* g_Callbacks = nullptr;

BRIDGE_EXPORT bool Bridge_Init() {
    if (!SteamAPI_Init()) {
        return false;
    }
    g_Callbacks = new BridgeCallbacks(); // Spin up the listener
    return true;
}

BRIDGE_EXPORT void Bridge_Shutdown() {
    SteamAPI_Shutdown();
}

BRIDGE_EXPORT bool Bridge_Send(uint64_t steamId, const uint8_t* data, int size) {
    CSteamID remoteSteamID((uint64)steamId);
    return SteamNetworking()->SendP2PPacket(remoteSteamID, data, size, k_EP2PSendUnreliable);
}

BRIDGE_EXPORT bool Bridge_SendReliable(uint64_t steamId, const uint8_t* data, int size) {
    CSteamID remoteSteamID((uint64)steamId);
    return SteamNetworking()->SendP2PPacket(remoteSteamID, data, size, k_EP2PSendReliable);
}

BRIDGE_EXPORT int Bridge_Receive(uint8_t* buffer, int bufferSize, uint64_t * outSteamIDRemote) {
    uint32_t msgSize;
    if (!SteamNetworking()->IsP2PPacketAvailable(&msgSize, 0)) {
        return 0; 
    }
    if (msgSize > bufferSize) {
        // Message is too large for the buffer
        return -1;
    }
    CSteamID remoteSteamID;
    uint32_t bytesRead;
    if (SteamNetworking()->ReadP2PPacket(buffer, bufferSize, &bytesRead, &remoteSteamID, 0)) {
        *outSteamIDRemote = remoteSteamID.ConvertToUint64();
        return bytesRead;
    } else {
        return 0;
    }
}

BRIDGE_EXPORT void Bridge_RunCallbacks() {
    SteamAPI_RunCallbacks();
}

BRIDGE_EXPORT uint64_t Bridge_GetLocalSteamID() {
    return SteamAPI_ISteamUser_GetSteamID(SteamUser());
}

BRIDGE_EXPORT uint64_t Bridge_GetJoinRequest() {
    return g_pendingJoin.exchange(0);
}

BRIDGE_EXPORT void Bridge_SetJoinable(bool joinable) {
    // A non-empty "connect" key makes "Join Game" appear in the friends list.
    SteamFriends()->SetRichPresence("connect", joinable ? "steambridge" : "");
}

BRIDGE_EXPORT void Bridge_OpenFriendsOverlay() {
    SteamFriends()->ActivateGameOverlay("friends");
}