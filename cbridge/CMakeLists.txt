cmake_minimum_required(VERSION 3.10)
project(SteamBridge CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

add_library(steam_bridge SHARED
    ${CMAKE_CURRENT_SOURCE_DIR}/steam_bridge.cpp
)

set_target_properties(steam_bridge PROPERTIES
    OUTPUT_NAME "steam_bridge"
    PREFIX "lib")

target_include_directories(steam_bridge
    PRIVATE
    "${CMAKE_CURRENT_SOURCE_DIR}/sdk/public"
)

if (WIN32)
  target_link_directories(steam_bridge PRIVATE
        "${CMAKE_CURRENT_SOURCE_DIR}/sdk/redistributable_bin/win64"
    )
  target_link_libraries(steam_bridge PRIVATE steam_api64)

  add_custom_command(TARGET steam_bridge POST_BUILD
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
            "${CMAKE_CURRENT_SOURCE_DIR}/sdk/redistributable_bin/win64/steam_api64.dll"
            "${CMAKE_CURRENT_SOURCE_DIR}/.."
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
            $<TARGET_FILE:steam_bridge>
            "${CMAKE_CURRENT_SOURCE_DIR}/.."
        COMMAND ${CMAKE_COMMAND} -E make_directory
            "${CMAKE_CURRENT_SOURCE_DIR}/../build/bin"
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
            "${CMAKE_CURRENT_SOURCE_DIR}/sdk/redistributable_bin/win64/steam_api64.dll"
            "${CMAKE_CURRENT_SOURCE_DIR}/../build/bin"
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
            $<TARGET_FILE:steam_bridge>
            "${CMAKE_CURRENT_SOURCE_DIR}/../build/bin"
    )
else()
  target_link_directories(steam_bridge PRIVATE
        "${CMAKE_CURRENT_SOURCE_DIR}/sdk/redistributable_bin/linux64"
    )
  target_link_libraries(steam_bridge PRIVATE steam_api)

  set_target_properties(steam_bridge PROPERTIES INSTALL_RPATH "$ORIGIN")
  set_target_properties(steam_bridge PROPERTIES BUILD_WITH_INSTALL_RPATH TRUE)

  add_custom_command(TARGET steam_bridge POST_BUILD
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
            "${CMAKE_CURRENT_SOURCE_DIR}/sdk/redistributable_bin/linux64/libsteam_api.so"
            "${CMAKE_CURRENT_SOURCE_DIR}/.."
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
            $<TARGET_FILE:steam_bridge>
            "${CMAKE_CURRENT_SOURCE_DIR}/.."
        COMMAND ${CMAKE_COMMAND} -E make_directory
            "${CMAKE_CURRENT_SOURCE_DIR}/../build/bin"
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
            "${CMAKE_CURRENT_SOURCE_DIR}/sdk/redistributable_bin/linux64/libsteam_api.so"
            "${CMAKE_CURRENT_SOURCE_DIR}/../build/bin"
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
            $<TARGET_FILE:steam_bridge>
            "${CMAKE_CURRENT_SOURCE_DIR}/../build/bin"
    )
endif()

# Unit tests. Off by default so `build.sh`/`build.bat` (which need the real,
# license-gated SDK under sdk/) are unaffected. Enable with:
#   cmake -B build -DSTEAMBRIDGE_BUILD_TESTS=ON
#   cmake --build build --target steam_bridge_tests
#   ctest --test-dir build --output-on-failure
#
# steam_bridge.cpp is compiled a second time here against
# tests/fake_steam_sdk, a hand-written stand-in for the handful of Steam API
# declarations the bridge uses (see steambridge#43). That fake is linked in
# instead of the real libsteam_api, so these tests need neither the
# decrypted SDK nor a live Steam client.
option(STEAMBRIDGE_BUILD_TESTS "Build C++ bridge unit tests against a fake Steam SDK" OFF)

if (STEAMBRIDGE_BUILD_TESTS)
  enable_testing()
  include(FetchContent)
  FetchContent_Declare(
        googletest
        GIT_REPOSITORY https://github.com/google/googletest.git
        GIT_TAG        v1.15.2
    )
  set(gtest_force_shared_crt ON CACHE BOOL "" FORCE)
  FetchContent_MakeAvailable(googletest)

  add_executable(steam_bridge_tests
        steam_bridge.cpp
        tests/fake_steam_api.cpp
        tests/test_bridge.cpp
        tests/test_join_request.cpp
    )

  target_include_directories(steam_bridge_tests PRIVATE
        "${CMAKE_CURRENT_SOURCE_DIR}"
        "${CMAKE_CURRENT_SOURCE_DIR}/tests"
        "${CMAKE_CURRENT_SOURCE_DIR}/tests/fake_steam_sdk"
    )

  target_link_libraries(steam_bridge_tests PRIVATE GTest::gtest_main)

  include(GoogleTest)
  gtest_discover_tests(steam_bridge_tests)
endif()
