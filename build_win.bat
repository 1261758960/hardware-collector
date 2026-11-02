@echo off
:: 强制使用 UTF-8 编码，解决中文乱码
chcp 65001 >nul
setlocal
title 一键编译 - 安全助手

echo ==============================================
echo [1/4] 正在清理旧进程...
taskkill /f /im 安全助手.exe >nul 2>&1

echo [2/4] 正在清理旧文件...
if exist "安全助手.exe" (
    del "安全助手.exe"
)

echo [3/4] 正在启动 Go 编译...
go build -ldflags="-s -w -H windowsgui" -o 安全助手_windows.exe .

if %errorlevel% neq 0 (
    echo.
    echo ❌ 编译失败！请检查 main.go 是否有语法错误。
    goto :end
)

echo [4/4] 编译成功！
echo.
echo ✅ 已生成最新版: 安全助手.exe
echo 请双击运行测试。

:end
echo.
pause
endlocal