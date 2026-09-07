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

enum EP2PSend {
    k_EP2PSendUnreliable = 0,
    k_EP2PSendUnreliableNoDelay = 1,
    k_EP2PSendReliable = 2,
    k_EP2PSendReliableWithBuffering = 3,
};

enum EFriendRelationship {
    k_EFriendRelationshipNone = 0,
    k_EFriendRelationshipFriend = 3,
};

struct P2PSessionRequest_t {
    enum { k_iCallback = 1 };
    CSteamID m_steamIDRemote;
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
// steam_bridge.cpp can write BridgeCallbacks::OnP2PSessionRequest(...) below
// without a separate method declaration in the class body.
#define STEAM_CALLBACK(thisclass, func, param, var) CCallback<thisclass, param> var; void func(param *pParam)

// --- Interfaces used by steam_bridge.cpp ------------------------------------

class ISteamNetworking {
public:
    virtual bool SendP2PPacket(CSteamID steamIDRemote, const void* pubData, uint32 cubData, EP2PSend eP2PSendType, int nChannel = 0) = 0;
    virtual bool IsP2PPacketAvailable(uint32* pcubMsgSize, int nChannel = 0) = 0;
    virtual bool ReadP2PPacket(void* pubDest, uint32 cubDest, uint32* pcubMsgSize, CSteamID* psteamIDRemote, int nChannel = 0) = 0;
    virtual bool AcceptP2PSessionWithUser(CSteamID steamIDRemote) = 0;
    virtual bool CloseP2PSessionWithUser(CSteamID steamIDRemote) = 0;
    virtual ~ISteamNetworking() = default;
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

ISteamNetworking* SteamNetworking();
ISteamUser* SteamUser();
ISteamFriends* SteamFriends();
