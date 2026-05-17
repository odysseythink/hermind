@echo off
chcp 65001 >nul
setlocal

echo Building go-desktop-interface static library...

set CGO_ENABLED=1

:: Auto-detect C compiler from Qt environment
if exist "E:\Qt-install\Tools\llvm-mingw1706_64\bin\clang.exe" (
    set "CC=E:\Qt-install\Tools\llvm-mingw1706_64\bin\clang.exe"
) else if exist "E:\Qt-install\Tools\mingw1120_64\bin\gcc.exe" (
    set "CC=E:\Qt-install\Tools\mingw1120_64\bin\gcc.exe"
) else (
    echo ERROR: Cannot find MinGW/LLVM compiler for CGO.
    echo Please set CC environment variable manually.
    exit /b 1
)

:: Ensure output directory exists
if not exist "%~dp0build" mkdir "%~dp0build"

:: Build the static library
cd /d "%~dp0.."
go build -buildmode=c-archive -o "%~dp0build\libgo-desktop-interface.a" ./cmd/go-desktop-interface

if %errorlevel% neq 0 (
    echo ERROR: Go build failed.
    exit /b 1
)

echo.
echo Success: libgo-desktop-interface.a and libgo-desktop-interface.h
echo generated in desktop/build/

endlocal
