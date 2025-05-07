// D:\fishyinhe\backend\internal\api\handler\screen_handler.go
package handler

import (
	"bytes"
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有来源的 WebSocket 连接 (在生产环境中你可能需要更严格的检查)
		return true
	},
}

// ScreenMirrorWS 处理屏幕镜像的 WebSocket 请求
func ScreenMirrorWS(c *gin.Context) {
	deviceId := c.Param("deviceId") // 从 URL 路径中获取 deviceId
	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	// 将 HTTP 连接升级到 WebSocket 连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade to websocket for device %s: %v", deviceId, err)
		// Upgrade 通常会自己处理错误响应，但如果出错，我们记录日志
		// c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upgrade to websocket"}) // 这行通常不需要
		return
	}
	defer conn.Close() // 确保连接在使用完毕后关闭

	log.Printf("WebSocket connection established for screen mirroring: %s, device: %s", conn.RemoteAddr(), deviceId)

	// 创建一个 ticker 来控制截图的频率 (例如，每秒10帧 -> 100ms 间隔)
	// 你可以根据性能调整这个值
	frameInterval := 100 * time.Millisecond // 10 FPS
	// frameInterval := 200 * time.Millisecond // 5 FPS
	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()

	// 循环处理：读取客户端消息（用于检测断开）和发送屏幕截图
	// 我们启动一个 goroutine 来读取消息，这样主循环可以专注于发送
	clientDisconnected := make(chan struct{})
	go func() {
		defer close(clientDisconnected) // 当 goroutine 退出时关闭 channel
		for {
			// 如果客户端发送任何消息或连接关闭，ReadMessage 会返回错误
			if _, _, err := conn.ReadMessage(); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Client %s (device: %s) disconnected (read error): %v", conn.RemoteAddr(), deviceId, err)
				} else {
					log.Printf("Client %s (device: %s) WebSocket read error: %v", conn.RemoteAddr(), deviceId, err)
				}
				return // 退出 goroutine，这将触发 clientDisconnected channel 的关闭
			}
		}
	}()

	for {
		select {
		case <-ticker.C: // 每当 ticker 触发时
			// 执行 adb shell screencap -p
			// -s <deviceId> 指定设备
			// exec-out 直接将输出流到标准输出，而不是保存到文件再读取
			cmd := exec.Command("adb", "-s", deviceId, "exec-out", "screencap", "-p")
			var out bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err != nil {
				log.Printf("Error running screencap for device %s: %v\nStderr: %s", deviceId, err, stderr.String())
				// 如果截图失败，可以考虑通知客户端或直接断开
				// 这里我们暂时只记录日志，并尝试下一次截图
				continue // 继续下一次 ticker 触发
			}

			pngData := out.Bytes()
			if len(pngData) == 0 {
				log.Printf("Screencap for device %s returned empty data.", deviceId)
				continue
			}

			// log.Printf("Sending frame for device %s, size: %d bytes", deviceId, len(pngData))
			// 将 PNG 二进制数据作为 BinaryMessage 发送
			if err := conn.WriteMessage(websocket.BinaryMessage, pngData); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Client %s (device: %s) disconnected (write error): %v", conn.RemoteAddr(), deviceId, err)
				} else {
					log.Printf("Error writing message to client %s (device: %s): %v", conn.RemoteAddr(), deviceId, err)
				}
				return // 写入错误，通常意味着客户端已断开，退出循环
			}

		case <-clientDisconnected: // 如果客户端断开连接的 goroutine 退出了
			log.Printf("Client %s (device: %s) has disconnected. Stopping screen mirror.", conn.RemoteAddr(), deviceId)
			return // 退出主循环
		}
	}
}
