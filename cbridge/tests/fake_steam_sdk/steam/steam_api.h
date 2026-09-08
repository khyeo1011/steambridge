#pragma once
// Minimal stand-in for the real Steamworks SDK header of the same name.
//
// This declares only the subset of the Steam API surface that
// steam_bridge.cpp actually uses. It is the "fake steam_api link target"
// from steambridge#43: tests compile steam_bridge.cpp against this header
// instead of the real SDK, and link it against fake_steam_api.cpp instead
// of libsteam_api. No production code or ABI changes.
//
// The callback plumbing (CCallback / STEAM_CALLBACK) mirrors the real SDK's
// self-registering callback pattern closely enough that BridgeCallbacks
// compiles unmodified and dispatches through SteamAPI_RunCallbacks().

#include <algorithm>
#include <cstdint>
#include <cstring>
#include <vector>

typedef uint32_t uint32;
typedef uint64_t uint64;

class CSteamID {
public:
    CSteamID() : m_id(0) {}
    explicit CSteamID(uint64 id) : m_id(id) {}
    uint64 ConvertToUint64() const { return m_id; }
private:
    uint64 m_id;
};

enum EFriendRelationship {
    k_EFriendRelationshipNone = 0,
    k_EFriendRelationshipFriend = 3,
};

// --- ISteamNetworkingMessages types --------------------------------------
//
// Only the members steam_bridge.cpp touches are modelled.

typedef int EResult;
const EResult k_EResultOK = 1;

// Bitmask flags for SendMessageToUser (real SDK values).
const int k_nSteamNetworkingSend_Unreliable = 0;
const int k_nSteamNetworkingSend_Reliable = 8;

class SteamNetworkingIdentity {
public:
    void SetSteamID64(uint64 steamId) { m_steamId = steamId; }
    uint64 GetSteamID64() const { return m_steamId; }
private:
    uint64 m_steamId = 0;
};

struct SteamNetworkingMessage_t {
    void* m_pData = nullptr;
    int m_cbSize = 0;
    SteamNetworkingIdentity m_identityPeer;
    int m_nChannel = 0;
    void (*m_pfnRelease)(SteamNetworkingMessage_t*) = nullptr;

    void Release() {
        if (m_pfnRelease) m_pfnRelease(this);
    }
};

struct SteamNetworkingMessagesSessionRequest_t {
    enum { k_iCallback = 1251 };
    SteamNetworkingIdentity m_identityRemote;
};

struct GameRichPresenceJoinRequested_t {
    enum { k_iCallback = 2 };
    CSteamID m_steamIDFriend;
    char m_rgchConnect[256];
};

// --- Callback dispatch registry, mirroring the real SDK's CCallback --------
//
// Every live CCallback<T, P> instance registers itself here on construction
// and deregisters on destruction. SteamAPI_RunCallbacks() (in
// fake_steam_api.cpp) walks pending fake-triggered events and hands each to
// every registered callback whose P::k_iCallback matches.

class FakeCallbackBase {
public:
    virtual void Dispatch(int callbackId, void* pData) = 0;
    virtual ~FakeCallbackBase() = default;
};

inline std::vector<FakeCallbackBase*>& FakeSteam_CallbackRegistry() {
    static std::vector<FakeCallbackBase*> registry;
    return registry;
}

template <class T, class P>
class CCallback : public FakeCallbackBase {
public:
    typedef void (T::*func_t)(P*);

    CCallback(T* obj, func_t func) : m_pObj(obj), m_Func(func) {
        FakeSteam_CallbackRegistry().push_back(this);
    }

    ~CCallback() override {
        auto& reg = FakeSteam_CallbackRegistry();
        reg.erase(std::remove(reg.begin(), reg.end(), this), reg.end());
    }

    void Dispatch(int callbackId, void* pData) override {
        if (callbackId == P::k_iCallback) {
            (m_pObj->*m_Func)(static_cast<P*>(pData));
        }
    }

private:
    T* m_pObj;
    func_t m_Func;
};

// Matches the real macro's shape: it declares both the CCallback member
// AND the handler method (defined out-of-line by the caller), which is why
// steam_bridge.cpp can write BridgeCallbacks::OnSessionRequest(...) below
// without a separate method declaration in the class body.
#define STEAM_CALLBACK(thisclass, func, param, var) CCallback<thisclass, param> var; void func(param *pParam)

// --- Interfaces used by steam_bridge.cpp ------------------------------------

class ISteamNetworkingMessages {
public:
    virtual EResult SendMessageToUser(const SteamNetworkingIdentity& identityRemote, const void* pubData, uint32 cubData, int nSendFlags, int nRemoteChannel) = 0;
    virtual int ReceiveMessagesOnChannel(int nLocalChannel, SteamNetworkingMessage_t** ppOutMessages, int nMaxMessages) = 0;
    virtual bool AcceptSessionWithUser(const SteamNetworkingIdentity& identityRemote) = 0;
    virtual bool CloseSessionWithUser(const SteamNetworkingIdentity& identityRemote) = 0;
    virtual ~ISteamNetworkingMessages() = default;
};

class ISteamUser {
public:
    virtual ~ISteamUser() = default;
};

class ISteamFriends {
public:
    virtual bool SetRichPresence(const char* pchKey, const char* pchValue) = 0;
    virtual void ActivateGameOverlay(const char* pchDialog) = 0;
    virtual EFriendRelationship GetFriendRelationship(CSteamID steamIDFriend) = 0;
    virtual ~ISteamFriends() = default;
};

bool SteamAPI_Init();
void SteamAPI_Shutdown();
void SteamAPI_RunCallbacks();

ISteamNetworkingMessages* SteamNetworkingMessages();
ISteamUser* SteamUser();
ISteamFriends* SteamFriends();
