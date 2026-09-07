#include "steam/steam_api.h"
#include "steam/steam_api_flat.h"
#include "fake_steam_control.h"

#include <cstring>
#include <deque>
#include <map>
#include <mutex>

namespace {

struct QueuedPacket {
    uint64_t fromSteamId;
    std::vector<uint8_t> data;
};

struct PendingEvent {
    int callbackId;
    std::vector<uint8_t> payload;
};

struct FakeState {
    std::mutex mutex;

    uint64_t localSteamId = 0;

    std::deque<QueuedPacket> incomingPackets;
    std::vector<FakeSentPacket> sentPackets;
    bool sendShouldFail = false;

    std::map<uint64_t, int> friendRelationships;
    std::string richPresenceValue;
    bool overlayActivated = false;

    std::deque<PendingEvent> pendingEvents;
};

FakeState& State() {
    static FakeState state;
    return state;
}

template <class T>
PendingEvent MakeEvent(int callbackId, const T& data) {
    PendingEvent ev;
    ev.callbackId = callbackId;
    ev.payload.resize(sizeof(T));
    std::memcpy(ev.payload.data(), &data, sizeof(T));
    return ev;
}

class FakeSteamNetworking : public ISteamNetworking {
public:
    bool SendP2PPacket(CSteamID steamIDRemote, const void* pubData, uint32 cubData, EP2PSend eP2PSendType, int) override {
        auto& s = State();
        std::lock_guard<std::mutex> lock(s.mutex);
        if (s.sendShouldFail) return false;
        FakeSentPacket p;
        p.toSteamId = steamIDRemote.ConvertToUint64();
        const uint8_t* bytes = static_cast<const uint8_t*>(pubData);
        p.data.assign(bytes, bytes + cubData);
        p.reliable = (eP2PSendType == k_EP2PSendReliable);
        s.sentPackets.push_back(std::move(p));
        return true;
    }

    bool IsP2PPacketAvailable(uint32* pcubMsgSize, int) override {
        auto& s = State();
        std::lock_guard<std::mutex> lock(s.mutex);
        if (s.incomingPackets.empty()) return false;
        *pcubMsgSize = static_cast<uint32>(s.incomingPackets.front().data.size());
        return true;
    }

    bool ReadP2PPacket(void* pubDest, uint32 cubDest, uint32* pcubMsgSize, CSteamID* psteamIDRemote, int) override {
        auto& s = State();
        std::lock_guard<std::mutex> lock(s.mutex);
        if (s.incomingPackets.empty()) {
            *pcubMsgSize = 0;
            return false;
        }
        QueuedPacket pkt = std::move(s.incomingPackets.front());
        s.incomingPackets.pop_front();
        uint32 n = static_cast<uint32>(std::min<size_t>(cubDest, pkt.data.size()));
        std::memcpy(pubDest, pkt.data.data(), n);
        *pcubMsgSize = n;
        *psteamIDRemote = CSteamID(pkt.fromSteamId);
        return true;
    }

    bool AcceptP2PSessionWithUser(CSteamID) override { return true; }
    bool CloseP2PSessionWithUser(CSteamID) override { return true; }
};

class FakeSteamUser : public ISteamUser {};

class FakeSteamFriends : public ISteamFriends {
public:
    bool SetRichPresence(const char*, const char* pchValue) override {
        std::lock_guard<std::mutex> lock(State().mutex);
        State().richPresenceValue = pchValue ? pchValue : "";
        return true;
    }

    void ActivateGameOverlay(const char*) override {
        std::lock_guard<std::mutex> lock(State().mutex);
        State().overlayActivated = true;
    }

    EFriendRelationship GetFriendRelationship(CSteamID steamIDFriend) override {
        auto& s = State();
        std::lock_guard<std::mutex> lock(s.mutex);
        auto it = s.friendRelationships.find(steamIDFriend.ConvertToUint64());
        return it != s.friendRelationships.end() ? static_cast<EFriendRelationship>(it->second) : k_EFriendRelationshipNone;
    }
};

FakeSteamNetworking g_networking;
FakeSteamUser g_user;
FakeSteamFriends g_friends;

}  // namespace

bool SteamAPI_Init() { return true; }

void SteamAPI_Shutdown() {}

void SteamAPI_RunCallbacks() {
    std::deque<PendingEvent> events;
    {
        std::lock_guard<std::mutex> lock(State().mutex);
        events.swap(State().pendingEvents);
    }
    for (auto& ev : events) {
        for (auto* cb : FakeSteam_CallbackRegistry()) {
            cb->Dispatch(ev.callbackId, ev.payload.data());
        }
    }
}

ISteamNetworking* SteamNetworking() { return &g_networking; }
ISteamUser* SteamUser() { return &g_user; }
ISteamFriends* SteamFriends() { return &g_friends; }

uint64_t SteamAPI_ISteamUser_GetSteamID(ISteamUser*) {
    return State().localSteamId;
}

// ---- Test control API ------------------------------------------------------

void FakeSteam_Reset() {
    auto& s = State();
    std::lock_guard<std::mutex> lock(s.mutex);
    s.localSteamId = 0;
    s.incomingPackets.clear();
    s.sentPackets.clear();
    s.sendShouldFail = false;
    s.friendRelationships.clear();
    s.richPresenceValue.clear();
    s.overlayActivated = false;
    s.pendingEvents.clear();
    FakeSteam_CallbackRegistry().clear();
}

void FakeSteam_SetLocalSteamID(uint64_t steamId) {
    std::lock_guard<std::mutex> lock(State().mutex);
    State().localSteamId = steamId;
}

void FakeSteam_QueueIncomingPacket(uint64_t fromSteamId, const uint8_t* data, size_t size) {
    std::lock_guard<std::mutex> lock(State().mutex);
    State().incomingPackets.push_back({fromSteamId, std::vector<uint8_t>(data, data + size)});
}

size_t FakeSteam_PendingIncomingPacketCount() {
    std::lock_guard<std::mutex> lock(State().mutex);
    return State().incomingPackets.size();
}

void FakeSteam_SetSendShouldFail(bool shouldFail) {
    std::lock_guard<std::mutex> lock(State().mutex);
    State().sendShouldFail = shouldFail;
}

const std::vector<FakeSentPacket>& FakeSteam_GetSentPackets() {
    return State().sentPackets;
}

void FakeSteam_SetFriendRelationship(uint64_t steamId, int relationship) {
    std::lock_guard<std::mutex> lock(State().mutex);
    State().friendRelationships[steamId] = relationship;
}

const std::string& FakeSteam_GetRichPresence() {
    return State().richPresenceValue;
}

bool FakeSteam_WasOverlayActivated() {
    std::lock_guard<std::mutex> lock(State().mutex);
    return State().overlayActivated;
}

void FakeSteam_TriggerP2PSessionRequest(uint64_t remoteSteamId) {
    P2PSessionRequest_t evData{};
    evData.m_steamIDRemote = CSteamID(remoteSteamId);
    std::lock_guard<std::mutex> lock(State().mutex);
    State().pendingEvents.push_back(MakeEvent(P2PSessionRequest_t::k_iCallback, evData));
}

void FakeSteam_TriggerJoinRequested(uint64_t friendSteamId) {
    GameRichPresenceJoinRequested_t evData{};
    evData.m_steamIDFriend = CSteamID(friendSteamId);
    std::lock_guard<std::mutex> lock(State().mutex);
    State().pendingEvents.push_back(MakeEvent(GameRichPresenceJoinRequested_t::k_iCallback, evData));
}
