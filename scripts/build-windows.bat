@echo off

echo Start running the script...
cd ../

cd ./cbridge
CALL build.bat
cd ../

echo Start building the app for windows platform...
wails build --clean --platform windows/amd64

echo End running the script!
