@echo off
setlocal enabledelayedexpansion

set GOFLAGS=-tags="fts5"

if "%~1"=="" goto :usage

if /I "%~1"=="build-frontend" goto :build-frontend
if /I "%~1"=="build-server" goto :build-server
if /I "%~1"=="dev" goto :dev
if /I "%~1"=="test" goto :test
if /I "%~1"=="lint" goto :lint

echo Unknown target: %~1
goto :usage

:build-frontend
echo === Building frontend ===
pushd frontend
call yarn install
if errorlevel 1 exit /b 1
call yarn build
if errorlevel 1 exit /b 1
popd
goto :eof

:build-server
call :build-frontend
if errorlevel 1 exit /b 1

echo === Building server ===
pushd backend
if exist cmd\server\frontend\dist (
    rmdir /S /Q cmd\server\frontend\dist
)
xcopy /E /I /Y ..\frontend\dist cmd\server\frontend\dist >nul
if errorlevel 1 exit /b 1

move /Y cmd\server\frontend\dist\_index.html cmd\server\frontend\dist\index.html >nul

go build %GOFLAGS% -o bin\hermind.exe .\cmd\server\
if errorlevel 1 exit /b 1
popd

echo === Build complete: backend\bin\hermind.exe ===
goto :eof

:dev
echo === Running dev server ===
pushd backend
go run %GOFLAGS% .\cmd\server\ -logtostderr
popd
goto :eof

:test
echo === Running tests ===
pushd backend
go test -v .\...
if errorlevel 1 exit /b 1
popd
goto :eof

:lint
echo === Running linter ===
pushd backend
golangci-lint run .\...
if errorlevel 1 exit /b 1
popd
goto :eof

:usage
echo Usage: %~nx0 [build-frontend ^| build-server ^| dev ^| test ^| lint]
exit /b 1
