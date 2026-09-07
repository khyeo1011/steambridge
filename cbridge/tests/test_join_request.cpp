#include <gtest/gtest.h>

#include <atomic>
#include <condition_variable>
#include <cstdint>
#include <mutex>
#include <set>
#include <thread>
#include <vector>

#include "fake_steam_control.h"
#include "steam_bridge.h"

namespace {

class JoinRequestTest : public ::testing::Test {
protected:
  void SetUp() override {
    FakeSteam_Reset();
    ASSERT_TRUE(Bridge_Init());
  }

  void TearDown() override { Bridge_Shutdown(); }
};

// g_pendingJoin (steam_bridge.cpp) is a single atomic<uint64_t> slot, not a
// queue: it's last-write-wins by design, and Bridge_GetJoinRequest() drains
// it via exchange(0). This test locks in the two things a single-slot
// exchange must still guarantee under concurrent producers/consumers:
//
//  1. No torn reads: every non-zero value handed back by
//  Bridge_GetJoinRequest()
//     is exactly one of the SteamIDs a producer actually stored — never a
//     bitwise mix of two IDs, never garbage.
//  2. No duplication: exchange(0) means each stored value can be handed to
//     at most one consumer. The same non-zero value must never come back
//     out of two different Bridge_GetJoinRequest() calls.
//
// It does NOT guarantee no loss — with a single slot, a producer can
// overwrite a value no consumer has read yet. That's expected, not a bug.
TEST_F(JoinRequestTest, ExchangeClearsPendingValue) {
  FakeSteam_TriggerJoinRequested(1234);
  Bridge_RunCallbacks();
  EXPECT_EQ(Bridge_GetJoinRequest(), 1234u);
  // Already drained by the read above.
  EXPECT_EQ(Bridge_GetJoinRequest(), 0u);
}

// Claims a fresh, globally-unique id per iteration and drives it through the
// real callback path (FakeSteam_TriggerJoinRequested + Bridge_RunCallbacks),
// exactly like a live join-request callback firing.
void Producer_Loop(int iterations, std::atomic<uint64_t> *counter,
                    std::mutex *cv_m, std::condition_variable *cv,
                    bool *start) {
  {
    std::unique_lock<std::mutex> lock(*cv_m);
    cv->wait(lock, [start] { return *start; });
  }

  for (int i = 0; i < iterations; i++) {
    uint64_t id = (*counter)++;  // atomic post-increment: old value is ours alone
    FakeSteam_TriggerJoinRequested(id);
    Bridge_RunCallbacks();
  }
}

// Polls Bridge_GetJoinRequest() and records every non-zero value it sees
// into a shared, mutex-guarded vector for the assertions below.
void Consumer_Loop(int iterations, std::mutex *cv_m,
                    std::condition_variable *cv, bool *start,
                    std::mutex *results_m, std::vector<uint64_t> *results) {
  {
    std::unique_lock<std::mutex> lock(*cv_m);
    cv->wait(lock, [start] { return *start; });
  }

  for (int i = 0; i < iterations; i++) {
    uint64_t id = Bridge_GetJoinRequest();
    if (id != 0) {
      std::lock_guard<std::mutex> lock(*results_m);
      results->push_back(id);
    }
  }
}

TEST_F(JoinRequestTest, ConcurrentJoinRequestsAreNeverTornOrDuplicated) {
  const int kNumProducers = 5;
  const int kNumConsumers = 5;
  const int kIterationsPerProducer = 200;
  const int kIterationsPerConsumer = 300;
  const uint64_t kBaseId = 1000;

  std::atomic<uint64_t> counter{kBaseId};
  std::mutex cv_m;
  std::condition_variable cv;
  bool start = false;

  std::mutex results_m;
  std::vector<uint64_t> results;

  // All threads block on the same gate so they actually race on
  // g_pendingJoin, instead of finishing serially in creation order.
  std::vector<std::thread> threads;
  for (int i = 0; i < kNumProducers; i++) {
    threads.emplace_back(Producer_Loop, kIterationsPerProducer, &counter,
                          &cv_m, &cv, &start);
  }
  for (int i = 0; i < kNumConsumers; i++) {
    threads.emplace_back(Consumer_Loop, kIterationsPerConsumer, &cv_m, &cv,
                          &start, &results_m, &results);
  }

  {
    std::lock_guard<std::mutex> lock(cv_m);
    start = true;
  }
  cv.notify_all();

  for (auto &t : threads) t.join();

  const uint64_t kMaxId = kBaseId + kNumProducers * kIterationsPerProducer;

  // Invariant 1: no torn reads. Every id a producer ever stored came from
  // this contiguous, known range — anything outside it would mean two
  // partial writes got mashed together.
  for (uint64_t id : results) {
    EXPECT_GE(id, kBaseId);
    EXPECT_LT(id, kMaxId);
  }

  // Invariant 2: no duplication. If exchange(0) ever handed the same value
  // to two consumers, the set would be smaller than the vector.
  std::set<uint64_t> unique_results(results.begin(), results.end());
  EXPECT_EQ(unique_results.size(), results.size());
}

}  // namespace
