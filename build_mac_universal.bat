@echo on
:: 强制使用 UTF-8 编码，解决中文乱码
chcp 65001 >nul

:: 备份原始环境变量
set ORIGINAL_GOOS=%GOOS%
set ORIGINAL_GOARCH=%GOARCH%

echo ==============================================
echo 正在编译并合并 macOS 通用版
echo ==============================================

:: 1. 编译 Intel 版
set GOOS=darwin
set GOARCH=amd64
echo [1/3] 正在编译 Intel 版...
go build -ldflags="-s -w" -o mac_intel .
if %errorlevel% neq 0 goto error

:: 2. 编译 M系列 版
set GOARCH=arm64
echo [2/3] 正在编译 M系列芯片版...
go build -ldflags="-s -w" -o mac_arm .
if %errorlevel% neq 0 goto error

:: 3. 合并为通用版
echo [3/3] 正在合并为通用版...
lipo -create -output "安全助手_Mac通用版" mac_intel mac_arm

if %errorlevel% neq 0 (
    echo.
    echo ⚠️ 合并失败！请确保已安装 lipo 工具。
    goto error
) else (
    echo ✅ 编译并合并成功！
    del mac_intel
    del mac_arm
)

:: 恢复环境变量
if defined ORIGINAL_GOOS (set GOOS=%ORIGINAL_GOOS%) else (set GOOS=)
if defined ORIGINAL_GOARCH (set GOARCH=%ORIGINAL_GOARCH%) else (set GOARCH=)
chcp 936 >nul

echo.
echo 脚本执行完毕，按任意键退出...
pause
exit /b 0

:error
echo.
echo ❌ 发生错误，编译中止！请查看上方的红色报错信息。
:: 恢复环境变量
if defined ORIGINAL_GOOS (set GOOS=%ORIGINAL_GOOS%) else (set GOOS=)
if defined ORIGINAL_GOARCH (set GOARCH=%ORIGINAL_GOARCH%) else (set GOARCH=)
chcp 936 >nul
echo 按任意键退出...
pause
exit /b 1