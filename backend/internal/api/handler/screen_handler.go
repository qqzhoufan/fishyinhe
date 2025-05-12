// D:\fishyinhe\backend\internal\api\handler\screen_handler.go
package handler

import (
	"bytes"
	"encoding/json" // 用于解析 JSON 消息
	"log"
	"net/http"
	"os/exec"
	"strconv" // 用于将数字转换为字符串
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// upgrader (保持不变)
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// InputMessage 定义从前端接收的输入事件的结构
type InputMessage struct {
	Type    string `json:"type"`              // 例如 "input_tap", "input_text", "input_keyevent"
	X       int    `json:"x,omitempty"`       // 用于 tap 事件
	Y       int    `json:"y,omitempty"`       // 用于 tap 事件
	Text    string `json:"text,omitempty"`    // 用于 text 事件
	Keycode string `json:"keycode,omitempty"` // 用于 keyevent 事件 (例如 "KEYCODE_ENTER" 或数字 "66")
}

// ScreenMirrorWS 处理屏幕镜像的 WebSocket 请求，并增加输入处理
func ScreenMirrorWS(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		// 通常 Upgrade 会处理错误，但以防万一
		log.Println("ScreenMirrorWS: Device ID is required but not provided in URL.")
		// Gin 在 WebSocket 升级失败时可能不会让我们有机会写 JSON 响应，
		// 但如果升级前检查，可以这样做。
		// c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ScreenMirrorWS: Failed to upgrade to websocket for device %s: %v", deviceId, err)
		return
	}
	defer conn.Close()
	log.Printf("ScreenMirrorWS: WebSocket connection established for screen mirroring & input: %s, device: %s", conn.RemoteAddr(), deviceId)

	frameInterval := 200 * time.Millisecond // 屏幕帧率 (5 FPS)
	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()

	clientDisconnected := make(chan struct{}) // 用于明确的断开信号

	// Goroutine 用于读取来自客户端的消息 (例如点击、文本、按键事件)
	go func() {
		defer func() {
			log.Printf("ScreenMirrorWS: Read goroutine for device %s, client %s exiting.", deviceId, conn.RemoteAddr())
			close(clientDisconnected) // 通知主循环客户端已断开
		}()
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				// 检查是否是预期的关闭错误
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
					log.Printf("ScreenMirrorWS: Client %s (device: %s) disconnected (read error): %v", conn.RemoteAddr(), deviceId, err)
				} else if err == websocket.ErrCloseSent {
					log.Printf("ScreenMirrorWS: Client %s (device: %s) WebSocket closed by server (CloseSent).", conn.RemoteAddr(), deviceId)
				} else {
					// 其他读取错误
					log.Printf("ScreenMirrorWS: Error reading message from client %s (device: %s): %v", conn.RemoteAddr(), deviceId, err)
				}
				return // 发生错误或连接关闭，退出 goroutine
			}

			if messageType == websocket.TextMessage {
				var msg InputMessage
				if err := json.Unmarshal(p, &msg); err != nil {
					log.Printf("ScreenMirrorWS: Error unmarshalling JSON from client %s (device: %s): %v. Message: %s", conn.RemoteAddr(), deviceId, err, string(p))
					// 可以选择发送错误消息回客户端
					// errMsg := fmt.Sprintf("{\"type\":\"error\", \"message\":\"Invalid JSON format: %v\"}", err)
					// conn.WriteMessage(websocket.TextMessage, []byte(errMsg))
					continue // 继续读取下一条消息
				}

				log.Printf("ScreenMirrorWS: Received message from client %s (device: %s): Type=%s, X=%d, Y=%d, Text='%s', Keycode='%s'",
					conn.RemoteAddr(), deviceId, msg.Type, msg.X, msg.Y, msg.Text, msg.Keycode)

				switch msg.Type {
				case "input_tap":
					if msg.X < 0 || msg.Y < 0 { // 基本的坐标有效性检查
						log.Printf("ScreenMirrorWS: Invalid tap coordinates received from client: X=%d, Y=%d", msg.X, msg.Y)
						continue
					}
					tapCmd := exec.Command("adb", "-s", deviceId, "shell", "input", "tap", strconv.Itoa(msg.X), strconv.Itoa(msg.Y))
					var tapStderr bytes.Buffer
					tapCmd.Stderr = &tapStderr
					log.Printf("ScreenMirrorWS: Executing for device %s: adb shell input tap %d %d", deviceId, msg.X, msg.Y)
					if err := tapCmd.Run(); err != nil {
						log.Printf("ScreenMirrorWS: Error executing input tap for device %s at (%d,%d): %v. Stderr: %s", deviceId, msg.X, msg.Y, err, tapStderr.String())
						// 可选：将错误反馈给客户端
						// errMsg := fmt.Sprintf("{\"type\":\"error\", \"message\":\"Tap failed: %s\"}", tapStderr.String())
						// conn.WriteMessage(websocket.TextMessage, []byte(errMsg))
					} else {
						log.Printf("ScreenMirrorWS: Successfully executed input tap for device %s at (%d,%d)", deviceId, msg.X, msg.Y)
					}

				case "input_text":
					if msg.Text == "" { // 检查文本是否为空
						log.Printf("ScreenMirrorWS: Empty text received for input_text from client.")
						continue
					}
					// ADB 'input text' 命令会将文本中的空格替换为 %s，所以通常直接传递即可。
					// 如果文本包含特殊shell字符，可能需要额外处理，但 exec.Command 会处理参数分隔。
					// 对于复杂的文本或包含引号的文本，可能需要更复杂的转义或使用其他 input 方法。
					textCmd := exec.Command("adb", "-s", deviceId, "shell", "input", "text", msg.Text)
					var textStderr bytes.Buffer
					textCmd.Stderr = &textStderr
					log.Printf("ScreenMirrorWS: Executing for device %s: adb shell input text \"%s\"", deviceId, msg.Text)
					if err := textCmd.Run(); err != nil {
						log.Printf("ScreenMirrorWS: Error executing input text for device %s, text '%s': %v. Stderr: %s", deviceId, msg.Text, err, textStderr.String())
						// 可选：反馈错误给客户端
						// errMsg := fmt.Sprintf("{\"type\":\"error\", \"message\":\"Send text failed: %s\"}", textStderr.String())
						// conn.WriteMessage(websocket.TextMessage, []byte(errMsg))
					} else {
						log.Printf("ScreenMirrorWS: Successfully executed input text for device %s, text '%s'", deviceId, msg.Text)
						// 可选：发送确认消息回客户端
						// ackMsg := fmt.Sprintf("{\"type\":\"input_text_ack\", \"text\":\"%s\"}", msg.Text)
						// conn.WriteMessage(websocket.TextMessage, []byte(ackMsg))
					}

				case "input_keyevent":
					if msg.Keycode == "" {
						log.Printf("ScreenMirrorWS: Empty keycode received for input_keyevent from client.")
						continue
					}
					keyeventCmd := exec.Command("adb", "-s", deviceId, "shell", "input", "keyevent", msg.Keycode)
					var keyeventStderr bytes.Buffer
					keyeventCmd.Stderr = &keyeventStderr
					log.Printf("ScreenMirrorWS: Executing for device %s: adb shell input keyevent %s", deviceId, msg.Keycode)
					if err := keyeventCmd.Run(); err != nil {
						log.Printf("ScreenMirrorWS: Error executing input keyevent for device %s, keycode '%s': %v. Stderr: %s", deviceId, msg.Keycode, err, keyeventStderr.String())
					} else {
						log.Printf("ScreenMirrorWS: Successfully executed input keyevent for device %s, keycode '%s'", deviceId, msg.Keycode)
					}

				default:
					log.Printf("ScreenMirrorWS: Received unknown message type from client %s (device: %s): %s", conn.RemoteAddr(), deviceId, msg.Type)
				}
			}
		}
	}()

	// 主循环，用于发送屏幕截图
	for {
		select {
		case <-ticker.C: // 定期捕获屏幕
			cmd := exec.Command("adb", "-s", deviceId, "exec-out", "screencap", "-p")
			var out bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			err := cmd.Run()
			if err != nil {
				log.Printf("ScreenMirrorWS: Error running screencap for device %s: %v\nStderr: %s", deviceId, err, stderr.String())
				// 可选：发送错误消息到客户端
				// errMsg := fmt.Sprintf("{\"type\":\"error\", \"message\":\"Screencap failed: %s\"}", stderr.String())
				// if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(errMsg)); writeErr != nil {
				//    log.Printf("ScreenMirrorWS: Error sending screencap error to client: %v", writeErr)
				// }
				continue
			}
			pngData := out.Bytes()
			if len(pngData) == 0 {
				log.Printf("ScreenMirrorWS: Screencap for device %s returned empty data.", deviceId)
				continue
			}
			// 将 PNG 二进制数据作为 BinaryMessage 发送
			if err := conn.WriteMessage(websocket.BinaryMessage, pngData); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
					log.Printf("ScreenMirrorWS: Client %s (device: %s) disconnected (write error): %v", conn.RemoteAddr(), deviceId, err)
				} else if err == websocket.ErrCloseSent {
					log.Printf("ScreenMirrorWS: Client %s (device: %s) WebSocket closed by server (write error after CloseSent).", conn.RemoteAddr(), deviceId)
				} else {
					log.Printf("ScreenMirrorWS: Error writing screen frame to client %s (device: %s): %v", conn.RemoteAddr(), deviceId, err)
				}
				return // 发送失败，通常意味着客户端已断开
			}
		case <-clientDisconnected: // 如果读取 goroutine 检测到断开
			log.Printf("ScreenMirrorWS: Client %s (device: %s) has disconnected (signaled by read goroutine). Stopping screen mirror.", conn.RemoteAddr(), deviceId)
			return // 退出主循环
		}
	}
}
