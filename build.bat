@echo off
:: props build script
:: Author: Hadi Cahyadi <cumulus13@gmail.com>

echo [props] Building...
go build -ldflags "-s -w" -o props.exe .

if %ERRORLEVEL% NEQ 0 (
    echo [props] Build FAILED.
    exit /b 1
)

echo [props] Build OK  ->  props.exe
