#include <gtest/gtest.h>

#include "fake_steam_control.h"
#include "steam/steam_api.h"  // for k_EFriendRelationshipFriend
#include "steam_bridge.h"

namespace {

class BridgeTest : public ::testing::Test {
protected:
    void SetUp() override {
        FakeSteam_Reset();
        ASSERT_TRUE(Bridge_Init());
    }

    void TearDown() override { Bridge_Shutdown(); }
};

// -- Bridge_Receive: buffer-too-small path -----------------------------------

TEST_F(BridgeTest, ReceiveDiscardsOversizedPacketAndReturnsZero) {
    uint8_t big[16] = {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16};
    FakeSteam_QueueIncomingPacket(/*fromSteamId=*/42, big, sizeof(big));

    uint8_t smallBuf[4];
    uint64_t remote = 0;
    int n = Bridge_Receive(smallBuf, sizeof(smallBuf), &remote);

    EXPECT_EQ(n, 0);
    // The oversized packet must be drained, not left stuck at the queue head.
    EXPECT_EQ(FakeSteam_PendingIncomingPacketCount(), 0u);
}

TEST_F(BridgeTest, ReceiveDeliversPacketThatFits) {
    uint8_t data[3] = {9, 8, 7};
    FakeSteam_QueueIncomingPacket(/*fromSteamId=*/99, data, sizeof(data));

    uint8_t buf[8];
    uint64_t remote = 0;
    int n = Bridge_Receive(buf, sizeof(buf), &remote);

    ASSERT_EQ(n, 3);
    EXPECT_EQ(remote, 99u);
    EXPECT_EQ(buf[0], 9);
    EXPECT_EQ(buf[1], 8);
    EXPECT_EQ(buf[2], 7);
}

TEST_F(BridgeTest, ReceiveWithNoPacketReturnsZero) {
    uint8_t buf[8];
    uint64_t remote = 0;
    EXPECT_EQ(Bridge_Receive(buf, sizeof(buf), &remote), 0);
}

// An oversized packet ahead of a good one must not wedge the queue: the
// bad packet is discarded and the next Receive() call gets the good one.
TEST_F(BridgeTest, ReceiveRecoversAfterOversizedPacket) {
    uint8_t big[16] = {0};
    uint8_t small[2] = {5, 6};
    FakeSteam_QueueIncomingPacket(1, big, sizeof(big));
    FakeSteam_QueueIncomingPacket(2, small, sizeof(small));

    uint8_t buf[4];
    uint64_t remote = 0;
    EXPECT_EQ(Bridge_Receive(buf, sizeof(buf), &remote), 0);  // discards big

    int n = Bridge_Receive(buf, sizeof(buf), &remote);
    ASSERT_EQ(n, 2);
    EXPECT_EQ(remote, 2u);
}

// -- Bridge_Send / Bridge_SendReliable ---------------------------------------

TEST_F(BridgeTest, SendReportsFailureWithoutCrashing) {
    FakeSteam_SetSendShouldFail(true);
    uint8_t data[1] = {1};
    EXPECT_FALSE(Bridge_Send(123, data, sizeof(data)));
    EXPECT_TRUE(FakeSteam_GetSentPackets().empty());
}

TEST_F(BridgeTest, SendReliableMarksPacketReliable) {
    uint8_t data[2] = {1, 2};
    ASSERT_TRUE(Bridge_SendReliable(123, data, sizeof(data)));
    ASSERT_EQ(FakeSteam_GetSentPackets().size(), 1u);
    EXPECT_TRUE(FakeSteam_GetSentPackets()[0].reliable);
    EXPECT_EQ(FakeSteam_GetSentPackets()[0].toSteamId, 123u);
}

// -- Local identity / friends / rich presence --------------------------------

TEST_F(BridgeTest, GetLocalSteamIDReflectsFakeIdentity) {
    FakeSteam_SetLocalSteamID(555);
    EXPECT_EQ(Bridge_GetLocalSteamID(), 555u);
}

TEST_F(BridgeTest, IsFriendReflectsFakeRelationship) {
    FakeSteam_SetFriendRelationship(7, k_EFriendRelationshipFriend);
    EXPECT_TRUE(Bridge_IsFriend(7));
    EXPECT_FALSE(Bridge_IsFriend(8));
}

TEST_F(BridgeTest, SetJoinableTogglesRichPresence) {
    Bridge_SetJoinable(true);
    EXPECT_EQ(FakeSteam_GetRichPresence(), "steambridge");

    Bridge_SetJoinable(false);
    EXPECT_EQ(FakeSteam_GetRichPresence(), "");
}

TEST_F(BridgeTest, OpenFriendsOverlayActivatesOverlay) {
    EXPECT_FALSE(FakeSteam_WasOverlayActivated());
    Bridge_OpenFriendsOverlay();
    EXPECT_TRUE(FakeSteam_WasOverlayActivated());
}

// -- P2P session requests ----------------------------------------------------

TEST_F(BridgeTest, SessionRequestIsQueuedThenDrainedOnce) {
    EXPECT_EQ(Bridge_GetSessionRequest(), 0u);

    FakeSteam_TriggerP2PSessionRequest(321);
    Bridge_RunCallbacks();

    EXPECT_EQ(Bridge_GetSessionRequest(), 321u);
    // Drained: a second call sees nothing left.
    EXPECT_EQ(Bridge_GetSessionRequest(), 0u);
}

TEST_F(BridgeTest, AcceptAndRejectSessionDoNotCrash) {
    Bridge_AcceptSession(1);
    Bridge_RejectSession(2);
}

// -- Shutdown clears queued state so it can't leak into the next session ----

TEST_F(BridgeTest, ShutdownClearsPendingJoinAndSessionRequests) {
    FakeSteam_TriggerJoinRequested(11);
    FakeSteam_TriggerP2PSessionRequest(22);
    Bridge_RunCallbacks();

    Bridge_Shutdown();
    ASSERT_TRUE(Bridge_Init());  // re-open for TearDown's Bridge_Shutdown()

    EXPECT_EQ(Bridge_GetJoinRequest(), 0u);
    EXPECT_EQ(Bridge_GetSessionRequest(), 0u);
}

}  // namespace
