@echo off
setlocal

set "SCRIPT_DIR=%~dp0"

:: Resolve the tools repository (holds make.py) and the mbp binary.
:: Works both from the monster root and from inside tools\mbp.
if exist "%SCRIPT_DIR%tools\make.py" (
    set "TOOLS_DIR=%SCRIPT_DIR%tools"
    set "ROOT_DIR=%SCRIPT_DIR%"
) else (
    set "TOOLS_DIR=%SCRIPT_DIR%.."
    set "ROOT_DIR=%SCRIPT_DIR%..\..\"
)
set "MBP_EXE=%TOOLS_DIR%\mbp\bin\mbp.exe"

:: Set MONSTER_ROOT if not already set
if "%MONSTER_ROOT%"=="" (
    for %%I in ("%ROOT_DIR%") do set "MONSTER_ROOT=%%~fI"
)

:: Build mbp if the binary is missing
if not exist "%MBP_EXE%" (
    echo [mbp] Binary not found at "%MBP_EXE%". Building mbp...
    if not exist "%TOOLS_DIR%\make.py" (
        echo [mbp] Error: "%TOOLS_DIR%\make.py" not found.
        exit /b 1
    )
    set "PYTHON="
    where py >nul 2>nul && set "PYTHON=py -3"
    if not defined PYTHON where python >nul 2>nul && set "PYTHON=python"
    if not defined PYTHON (
        echo [mbp] Error: Python 3 not found. Install it from https://www.python.org/ or the py launcher.
        exit /b 1
    )
    call %%PYTHON%% "%TOOLS_DIR%\make.py" build --mbp --native
    if errorlevel 1 (
        echo [mbp] Build failed.
        exit /b 1
    )
)

if not exist "%MBP_EXE%" (
    echo [mbp] Error: Executable was not created at "%MBP_EXE%".
    exit /b 1
)

:: Run mbp with all passed arguments
"%MBP_EXE%" %*
exit /b %ERRORLEVEL%
