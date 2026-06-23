@echo off
pushd "%~dp0.."

echo Building cbridge DLL...
pushd cbridge
CALL build.bat
popd
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] cbridge build failed.
    popd
    exit /b %ERRORLEVEL%
)

echo Building Wails app...
wails build --clean --platform windows/amd64
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Wails build failed.
    popd
    exit /b %ERRORLEVEL%
)

echo Copying runtime DLLs to build\bin...
copy /Y "libsteam_bridge.dll" "build\bin\libsteam_bridge.dll"
copy /Y "steam_api64.dll" "build\bin\steam_api64.dll"

echo Done.
popd
