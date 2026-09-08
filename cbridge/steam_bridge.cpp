#include "steam_bridge.h"
#include <steam/steam_api.h>
#include <steam/steam_api_flat.h>
#include <steam/isteamnetworkingmessages.h>
#include <atomic>
#include <cstring>
#include <mutex>
#include <queue>

// All bridge traffic rides a single ISteamNetworkingMessages channel. The
// protocol carries its own packet-type byte, so we don't need Steam's channel
// routing.
static constexpr int kBridgeChannel = 0;

static SteamNetworkingIdentity IdentityFromSteamID(uint64_t steamId) {
    SteamNetworkingIdentity id;
    id.SetSteamID64(steamId);
    return id;
}

// SteamID of a friend whose "Join Game" we received but haven't acted on yet.
// Go drains this once per ReadLoop iteration via Bridge_GetJoinRequest().
std::atomic<uint64_t> g_pendingJoin{0};

// Queue of incoming session requests. Go drains this and decides
// accept/reject; C++ never auto-accepts so Go owns the security policy.
std::queue<uint64_t> g_sessionRequests;
std::mutex g_sessionMutex;

class BridgeCallbacks {
public:
    BridgeCallbacks()
        : m_CallbackSessionRequest(this, &BridgeCallbacks::OnSessionRequest),
          m_CallbackJoinRequested(this, &BridgeCallbacks::OnGameRichPresenceJoinRequested) {}

    STEAM_CALLBACK(BridgeCallbacks, OnSessionRequest, SteamNetworkingMessagesSessionRequest_t, m_CallbackSessionRequest);
    STEAM_CALLBACK(BridgeCallbacks, OnGameRichPresenceJoinRequested, GameRichPresenceJoinRequested_t, m_CallbackJoinRequested);
};

void BridgeCallbacks::OnSessionRequest(SteamNetworkingMessagesSessionRequest_t *pCallback) {
    // Do NOT auto-accept. Queue the request; Go will call Bridge_AcceptSession
    // or Bridge_RejectSession after applying the friend-gate policy. Steam
    // re-posts this callback periodically while the peer keeps trying, so the
    // same SteamID may be enqueued more than once — Go's peer map dedupes.
    std::lock_guard<std::mutex> lock(g_sessionMutex);
    g_sessionRequests.push(pCallback->m_identityRemote.GetSteamID64());
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
    // Clear queued session requests so stale entries don't survive a bridge restart
    // and bypass the confirmation gate on the next Start().
    {
        std::lock_guard<std::mutex> lock(g_sessionMutex);
        while (!g_sessionRequests.empty()) g_sessionRequests.pop();
    }
    g_pendingJoin.store(0);
    SteamAPI_Shutdown();
}

BRIDGE_EXPORT bool Bridge_Send(uint64_t steamId, const uint8_t* data, int size) {
    SteamNetworkingIdentity id = IdentityFromSteamID(steamId);
    EResult res = SteamNetworkingMessages()->SendMessageToUser(
        id, data, (uint32)size, k_nSteamNetworkingSend_Unreliable, kBridgeChannel);
    return res == k_EResultOK;
}

BRIDGE_EXPORT bool Bridge_SendReliable(uint64_t steamId, const uint8_t* data, int size) {
    SteamNetworkingIdentity id = IdentityFromSteamID(steamId);
    EResult res = SteamNetworkingMessages()->SendMessageToUser(
        id, data, (uint32)size, k_nSteamNetworkingSend_Reliable, kBridgeChannel);
    return res == k_EResultOK;
}

BRIDGE_EXPORT int Bridge_Receive(uint8_t* buffer, int bufferSize, uint64_t * outSteamIDRemote) {
    SteamNetworkingMessage_t* msg = nullptr;
    int received = SteamNetworkingMessages()->ReceiveMessagesOnChannel(kBridgeChannel, &msg, 1);
    if (received <= 0) {
        return 0;
    }

    int msgSize = msg->m_cbSize;
    if (msgSize > bufferSize) {
        // Message is too large for the caller's buffer. Drop it and report 0
        // (no packet delivered) rather than truncating or tearing down the
        // bridge — matches the old ISteamNetworking behaviour. The new API
        // still allows messages far larger than our Ethernet-sized buffer.
        msg->Release();
        return 0;
    }

    std::memcpy(buffer, msg->m_pData, msgSize);
    *outSteamIDRemote = msg->m_identityPeer.GetSteamID64();
    msg->Release();
    return msgSize;
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

// Pops one pending session request SteamID; returns 0 if none queued.
BRIDGE_EXPORT uint64_t Bridge_GetSessionRequest() {
    std::lock_guard<std::mutex> lock(g_sessionMutex);
    if (g_sessionRequests.empty()) return 0;
    uint64_t id = g_sessionRequests.front();
    g_sessionRequests.pop();
    return id;
}

// Returns true if steamId is a Steam friend (k_EFriendRelationshipFriend).
BRIDGE_EXPORT bool Bridge_IsFriend(uint64_t steamId) {
    CSteamID id((uint64)steamId);
    return SteamFriends()->GetFriendRelationship(id) == k_EFriendRelationshipFriend;
}

// Accepts a pending session from steamId (call after host approves).
BRIDGE_EXPORT void Bridge_AcceptSession(uint64_t steamId) {
    SteamNetworkingIdentity id = IdentityFromSteamID(steamId);
    SteamNetworkingMessages()->AcceptSessionWithUser(id);
}

// Rejects a pending session from steamId (call after host denies).
BRIDGE_EXPORT void Bridge_RejectSession(uint64_t steamId) {
    SteamNetworkingIdentity id = IdentityFromSteamID(steamId);
    SteamNetworkingMessages()->CloseSessionWithUser(id);
}
