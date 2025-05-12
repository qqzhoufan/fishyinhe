package handler

import (
	"bytes"
	"encoding/json" // 新增：用于解析 JSON 消息
	"log"
	"net/http"
	"os/exec"
	"strconv" // 新增：用于将数字转换为字符串 (如果需要)
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// upgrader (保持不变)
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// InputMessage 定义从前端接收的输入事件的结构
type InputMessage struct {
	Type string `json:"type"` // 例如 "input_tap"
	X    int    `json:"x"`
	Y    int    `json:"y"`
	// 未来可以扩展其他类型，如 swipe, keyevent 等
	// KeyCode int `json:"keyCode,omitempty"`
	// Text string `json:"text,omitempty"`
}

// ScreenMirrorWS 处理屏幕镜像的 WebSocket 请求，并增加输入处理
func ScreenMirrorWS(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade to websocket for device %s: %v", deviceId, err)
		return
	}
	defer conn.Close()
	log.Printf("WebSocket connection established for screen mirroring & input: %s, device: %s", conn.RemoteAddr(), deviceId)

	frameInterval := 200 * time.Millisecond // 保持之前的帧率设置
	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()

	// 用于从读取 goroutine 发送错误或断开信号
	errChan := make(chan error, 1)
	clientDisconnected := make(chan struct{}) // 用于明确的断开信号

	// Goroutine 用于读取来自客户端的消息 (例如点击事件)
	go func() {
		defer func() {
			log.Printf("Read goroutine for device %s, client %s exiting.", deviceId, conn.RemoteAddr())
			close(clientDisconnected) // 通知主循环客户端已断开
		}()
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
					log.Printf("Client %s (device: %s) disconnected (read error): %v", conn.RemoteAddr(), deviceId, err)
				} else if err == websocket.ErrCloseSent {
					log.Printf("Client %s (device: %s) WebSocket closed by server (CloseSent).", conn.RemoteAddr(), deviceId)
				} else {
					log.Printf("Error reading message from client %s (device: %s): %v", conn.RemoteAddr(), deviceId, err)
				}
				// 将错误发送到主循环，如果它不是预期的关闭错误
				// if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				//  errChan <- err // 可能会阻塞，如果主循环已退出
				// }
				return // 退出 goroutine
			}

			if messageType == websocket.TextMessage {
				var msg InputMessage
				if err := json.Unmarshal(p, &msg); err != nil {
					log.Printf("Error unmarshalling JSON from client %s (device: %s): %v. Message: %s", conn.RemoteAddr(), deviceId, err, string(p))
					continue // 继续读取下一条消息
				}

				log.Printf("Received message from client %s (device: %s): Type=%s, X=%d, Y=%d", conn.RemoteAddr(), deviceId, msg.Type, msg.X, msg.Y)

				if msg.Type == "input_tap" {
					// 执行 ADB 点击命令
					tapCmd := exec.Command("adb", "-s", deviceId, "shell", "input", "tap", strconv.Itoa(msg.X), strconv.Itoa(msg.Y))
					var tapStderr bytes.Buffer
					tapCmd.Stderr = &tapStderr

					log.Printf("Executing for device %s: adb shell input tap %d %d", deviceId, msg.X, msg.Y)
					if err := tapCmd.Run(); err != nil {
						log.Printf("Error executing input tap for device %s at (%d,%d): %v. Stderr: %s", deviceId, msg.X, msg.Y, err, tapStderr.String())
						// 可以选择通过 WebSocket 将错误反馈给客户端
						// errMsg := fmt.Sprintf("{\"type\":\"error\", \"message\":\"Failed to execute tap: %s\"}", tapStderr.String())
						// if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(errMsg)); writeErr != nil {
						//     log.Printf("Error sending tap error to client: %v", writeErr)
						// }
					} else {
						log.Printf("Successfully executed input tap for device %s at (%d,%d)", deviceId, msg.X, msg.Y)
					}
				}
				//未来可以处理其他类型的输入事件
			} else if messageType == websocket.BinaryMessage {
				log.Printf("Received unexpected binary message from client %s (device: %s)", conn.RemoteAddr(), deviceId)
			}
		}
	}()

	// 主循环，用于发送屏幕截图
	for {
		select {
		case <-ticker.C:
			cmd := exec.Command("adb", "-s", deviceId, "exec-out", "screencap", "-p")
			var out bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			err := cmd.Run()
			if err != nil {
				log.Printf("Error running screencap for device %s: %v\nStderr: %s", deviceId, err, stderr.String())
				// 如果截图失败，可以考虑通知客户端或断开
				// errMsg := fmt.Sprintf("{\"type\":\"error\", \"message\":\"Screencap failed: %s\"}", stderr.String())
				// if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(errMsg)); writeErr != nil {
				//    log.Printf("Error sending screencap error to client: %v", writeErr)
				//    // 如果发送错误也失败，可能客户端已断开
				//    return
				// }
				continue
			}
			pngData := out.Bytes()
			if len(pngData) == 0 {
				log.Printf("Screencap for device %s returned empty data.", deviceId)
				continue
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, pngData); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
					log.Printf("Client %s (device: %s) disconnected (write error): %v", conn.RemoteAddr(), deviceId, err)
				} else if err == websocket.ErrCloseSent {
					log.Printf("Client %s (device: %s) WebSocket closed by server (write error after CloseSent).", conn.RemoteAddr(), deviceId)
				} else {
					log.Printf("Error writing screen frame to client %s (device: %s): %v", conn.RemoteAddr(), deviceId, err)
				}
				return
			}
		case <-clientDisconnected: // 如果读取 goroutine 检测到断开
			log.Printf("Client %s (device: %s) has disconnected (signaled by read goroutine). Stopping screen mirror.", conn.RemoteAddr(), deviceId)
			return
		case err := <-errChan: // 如果读取 goroutine 报告了错误（虽然目前没用这个）
			log.Printf("Error from read goroutine for client %s (device: %s): %v. Closing connection.", conn.RemoteAddr(), deviceId, err)
			return
		}
	}
}
