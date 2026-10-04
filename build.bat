@echo off
go build -ldflags="-s -w" -o filelist.exe .
if errorlevel 1 exit /b 1
echo Built filelist.exe (native)
set GOOS=windows
set GOARCH=386
go build -ldflags="-s -w" -o filelist32.exe .
if errorlevel 1 exit /b 1
echo Built filelist32.exe (win32)

if not exist releases mkdir releases
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=386
go build -trimpath -ldflags="-s -w" -o releases\filelist-win32.exe .
if errorlevel 1 exit /b 1
set GOOS=linux
set GOARCH=386
go build -trimpath -ldflags="-s -w" -o releases\filelist-linux-386 .
if errorlevel 1 exit /b 1
set GOARCH=amd64
go build -trimpath -ldflags="-s -w" -o releases\filelist-linux-amd64 .
if errorlevel 1 exit /b 1
set GOARCH=arm64
go build -trimpath -ldflags="-s -w" -o releases\filelist-linux-arm64 .
if errorlevel 1 exit /b 1
echo Built releases\ (win32, linux 386, amd64, arm64)
