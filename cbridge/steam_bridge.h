#pragma once
#include <stdbool.h>
#include <stdint.h>

#if defined(_WIN32)
#define BRIDGE_EXPORT extern "C" __declspec(dllexport)
#else
#define BRIDGE_EXPORT extern "C" __attribute__((visibility("default")))
#endif

BRIDGE_EXPORT bool Bridge_Init();

BRIDGE_EXPORT void Bridge_Shutdown();

BRIDGE_EXPORT bool Bridge_Send(uint64_t steamId, const uint8_t* data, int size);

BRIDGE_EXPORT bool Bridge_SendReliable(uint64_t steamId, const uint8_t* data,
                                       int size);

BRIDGE_EXPORT int Bridge_Receive(uint8_t* buffer, int bufferSize,
                                 uint64_t* outSteamIDRemote);

BRIDGE_EXPORT void Bridge_RunCallbacks();

BRIDGE_EXPORT uint64_t Bridge_GetLocalSteamID();

BRIDGE_EXPORT uint64_t Bridge_GetJoinRequest();

BRIDGE_EXPORT void Bridge_SetJoinable(bool joinable);

BRIDGE_EXPORT void Bridge_OpenFriendsOverlay();

BRIDGE_EXPORT uint64_t Bridge_GetSessionRequest();
BRIDGE_EXPORT bool Bridge_IsFriend(uint64_t steamId);
BRIDGE_EXPORT void Bridge_AcceptSession(uint64_t steamId);
BRIDGE_EXPORT void Bridge_RejectSession(uint64_t steamId);

// Fills in live connection quality for steamId (ping, relay vs. direct,
// instantaneous throughput). Returns false without touching the outputs if
// there's no active session with steamId.
BRIDGE_EXPORT bool Bridge_GetPeerStats(uint64_t steamId, int* outPingMs,
                                       bool* outRelayed,
                                       float* outBytesInPerSec,
                                       float* outBytesOutPerSec);