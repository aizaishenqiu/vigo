@echo off
echo Building Vigo CLI Tool...
mkdir bin
go build -o bin\vigo.exe ./framework/cli
if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo Build successful! 
    echo ========================================
    echo.
    echo CLI tool created: bin\vigo.exe
    echo.
    echo Usage:
    echo   .\bin\vigo.exe version
    echo   .\bin\vigo.exe route list
    echo   .\bin\vigo.exe make controller User
    echo.
    echo Optional: Add to PATH
    echo   Copy bin\vigo.exe to a folder in your PATH
    echo   or add %CD%\bin to your PATH environment variable
    echo.
) else (
    echo Build failed!
)
