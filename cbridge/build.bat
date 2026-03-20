@echo off
REM Create the build directory and generate build files
echo Generating build system...
cmake -B ./build

REM Check if the previous command succeeded
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] CMake configuration failed.
    exit /b %ERRORLEVEL%
)

REM Build the default target (auto-resolves ALL_BUILD vs all)
echo.
echo Building target...
cmake --build ./build/ --config Release

REM Final status check
if %ERRORLEVEL% EQU 0 (
    echo.
    echo [SUCCESS] Build completed successfully.
) else (
    echo.
    echo [ERROR] Build failed.
)

exit /b %ERRORLEVEL%