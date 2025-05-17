@echo off

:: 构建可执行文件
go build -o server.exe

:: 启动三个服务器实例
start /b server.exe -port=8001
start /b server.exe -port=8002
start /b server.exe -port=8003 -api=1

:: 等待2秒确保服务器启动
timeout /t 2 /nobreak

echo ">>> start test"
:: 并发请求一个不存在的 key，这样会触发缓存重建
curl "http://localhost:9999/api?key=Sam" && echo.
curl "http://localhost:9999/api?key=Sam" && echo.
curl "http://localhost:9999/api?key=Sam" && echo.

:: 等待用户按键后关闭
pause

:: 清理进程
taskkill /f /im server.exe