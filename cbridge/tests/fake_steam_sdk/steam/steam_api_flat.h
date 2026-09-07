#pragma once
// Minimal stand-in for the real SDK's flat (C-style) API header. Only the
// one function steam_bridge.cpp uses is declared.

#include "steam_api.h"

uint64_t SteamAPI_ISteamUser_GetSteamID(ISteamUser* self);
