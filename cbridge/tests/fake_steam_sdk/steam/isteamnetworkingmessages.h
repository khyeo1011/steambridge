#pragma once
// The fake steam_api.h already declares the ISteamNetworkingMessages subset
// steam_bridge.cpp uses. This shim only exists so the bridge's
// #include <steam/isteamnetworkingmessages.h> resolves in the test build.
#include "steam_api.h"
