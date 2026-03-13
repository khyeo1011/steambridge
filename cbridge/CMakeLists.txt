cmake_minimum_required(VERSION 3.10)
project(SteamBridge CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

add_library(steam_bridge SHARED
    ${CMAKE_CURRENT_SOURCE_DIR}/steam_bridge.cpp
)

target_include_directories(steam_bridge 
    PRIVATE 
    "${CMAKE_CURRENT_SOURCE_DIR}/../sdk/public"
)

if (WIN32)
    target_link_directories(steam_bridge PRIVATE 
        "${CMAKE_CURRENT_SOURCE_DIR}/../sdk/redistributable_bin/win64"
    )
    target_link_libraries(steam_bridge PRIVATE steam_api64)
    
    add_custom_command(TARGET steam_bridge POST_BUILD
        # 1. Copy Valve's official DLL to the project root
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
        "${CMAKE_CURRENT_SOURCE_DIR}/../sdk/redistributable_bin/win64/steam_api64.dll"
        "${CMAKE_CURRENT_SOURCE_DIR}/.."
        
        # 2. Copy YOUR newly built DLL to the project root
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
        $<TARGET_FILE:steam_bridge>
        "${CMAKE_CURRENT_SOURCE_DIR}/.."
    )
else()
    target_link_directories(steam_bridge PRIVATE 
        "${CMAKE_CURRENT_SOURCE_DIR}/../sdk/redistributable_bin/linux64"
    )
    target_link_libraries(steam_bridge PRIVATE steam_api)
    
    set_target_properties(steam_bridge PROPERTIES INSTALL_RPATH "$ORIGIN")
    set_target_properties(steam_bridge PROPERTIES BUILD_WITH_INSTALL_RPATH TRUE)

    add_custom_command(TARGET steam_bridge POST_BUILD
        # 1. Copy Valve's official .so to the project root
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
        "${CMAKE_CURRENT_SOURCE_DIR}/../sdk/redistributable_bin/linux64/libsteam_api.so"
        "${CMAKE_CURRENT_SOURCE_DIR}/.."
        
        # 2. Copy YOUR newly built .so to the project root
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
        $<TARGET_FILE:steam_bridge>
        "${CMAKE_CURRENT_SOURCE_DIR}/.."
    )
endif()