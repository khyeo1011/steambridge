#pragma once
#include <stdint.h>
#include <stdbool.h>

#if defined(_WIN32)
    #define BRIDGE_EXPORT extern "C" __declspec(dllexport)
#else
    #define BRIDGE_EXPORT extern "C" __attribute__((visibility("default")))
#endif

BRIDGE_EXPORT bool Bridge_Init();

BRIDGE_EXPORT void Bridge_Shutdown();

BRIDGE_EXPORT bool Bridge_Send(uint64_t steamId, const uint8_t* data, int size);

BRIDGE_EXPORT bool Bridge_SendReliable(uint64_t steamId, const uint8_t* data, int size);

BRIDGE_EXPORT int Bridge_Receive(uint8_t* buffer, int bufferSize, uint64_t * outSteamIDRemote);

BRIDGE_EXPORT void Bridge_RunCallbacks();