@echo off
REM Create the build directory and generate build files
echo Generating build system...
cmake -B ../build

REM Check if the previous command succeeded
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] CMake configuration failed.
    pause
    exit /b %ERRORLEVEL%
)

REM Build the "all" target
echo.
echo Building target 'all'...
cmake --build ../build/ --target all

REM Final status check
if %ERRORLEVEL% EQU 0 (
    echo.
    echo [SUCCESS] Build completed successfully.
) else (
    echo.
    echo [ERROR] Build failed.
)

pause
exit /b %ERRORLEVEL%