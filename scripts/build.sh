#! /bin/bash

echo -e "Start running the script..."
cd ../

echo -e "Start building the app..."
wails build --clean -tags webkit2_41

echo -e "End running the script!"
