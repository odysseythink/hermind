@echo off
cd /d "%~dp0\..\.."
if not exist build mkdir build
cd build
cmake .. -G "Ninja" -DCMAKE_BUILD_TYPE=Release -DCMAKE_PREFIX_PATH=C:\Qt\6.8.0\msvc2022_64
cmake --build . --config Release
windeployqt hermind-desktop.exe 2>nul
:: Copy Go backend binary to same dir
copy ..\..\bin\hermind-desktop-backend.exe . 2>nul
echo Built: hermind-desktop.exe
