// D:\fishyinhe\backend\internal\api\handler\screen_handler.go
package handler

import (
	"bytes"
	"encoding/json" // 用于解析 JSON 消息
	"fmt"           // 用于格式化错误消息
	"log"
	"net/http"
	"os/exec"
	"strconv" // 用于将数字转换为字符串
	"strings"
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
// 确保这个结构体与前端发送的匹配
type InputMessage struct {
	Type     string `json:"type"`               // 例如 "input_tap", "input_text", "input_keyevent", "input_swipe"
	X        int    `json:"x,omitempty"`        // 用于 tap 事件
	Y        int    `json:"y,omitempty"`        // 用于 tap 事件
	Text     string `json:"text,omitempty"`     // 用于 text 事件
	Keycode  string `json:"keycode,omitempty"`  // 用于 keyevent 事件 (例如 "KEYCODE_ENTER" 或数字 "66")
	X1       int    `json:"x1,omitempty"`       // 用于 swipe 事件的起始X
	Y1       int    `json:"y1,omitempty"`       // 用于 swipe 事件的起始Y
	X2       int    `json:"x2,omitempty"`       // 用于 swipe 事件的结束X
	Y2       int    `json:"y2,omitempty"`       // 用于 swipe 事件的结束Y
	Duration int    `json:"duration,omitempty"` // 用于 swipe 事件的持续时间 (毫秒)
}

// ScreenMirrorWS 处理屏幕镜像的 WebSocket 请求，并增加输入处理
func ScreenMirrorWS(c *gin.Context) {
	deviceId := c.Param("deviceId")
	if deviceId == "" {
		log.Println("ScreenMirrorWS: Device ID is required but not provided in URL.")
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ScreenMirrorWS: Failed to upgrade to websocket for device %s: %v", deviceId, err)
		return
	}
	defer conn.Close()
	log.Printf("ScreenMirrorWS: WebSocket connection established for screen mirroring & input: %s, device: %s", conn.RemoteAddr(), deviceId)

	frameInterval := 10 * time.Millisecond
	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()

	clientDisconnected := make(chan struct{})

	// Goroutine 用于读取来自客户端的消息
	go func() {
		defer func() {
			log.Printf("ScreenMirrorWS: Read goroutine for device %s, client %s exiting.", deviceId, conn.RemoteAddr())
			close(clientDisconnected)
		}()
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
					log.Printf("ScreenMirrorWS: Client %s (device: %s) disconnected (read error): %v", conn.RemoteAddr(), deviceId, err)
				} else if err == websocket.ErrCloseSent {
					log.Printf("ScreenMirrorWS: Client %s (device: %s) WebSocket closed by server (CloseSent).", conn.RemoteAddr(), deviceId)
				} else {
					log.Printf("ScreenMirrorWS: Error reading message from client %s (device: %s): %v", conn.RemoteAddr(), deviceId, err)
				}
				return
			}

			if messageType == websocket.TextMessage {
				var msg InputMessage
				if err := json.Unmarshal(p, &msg); err != nil {
					log.Printf("ScreenMirrorWS: Error unmarshalling JSON from client %s (device: %s): %v. Message: %s", conn.RemoteAddr(), deviceId, err, string(p))
					// 可选：发送错误消息回客户端
					errMsg := fmt.Sprintf("{\"type\":\"error\", \"message\":\"Invalid JSON format: %v\"}", err)
					if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(errMsg)); writeErr != nil {
						log.Printf("ScreenMirrorWS: Error sending JSON unmarshal error to client: %v", writeErr)
					}
					continue
				}

				log.Printf("ScreenMirrorWS: Received message from client %s (device: %s): Type=%s, X=%d, Y=%d, X1=%d, Y1=%d, X2=%d, Y2=%d, Duration=%d, Text='%s', Keycode='%s'",
					conn.RemoteAddr(), deviceId, msg.Type, msg.X, msg.Y, msg.X1, msg.Y1, msg.X2, msg.Y2, msg.Duration, msg.Text, msg.Keycode)

				var adbCmd *exec.Cmd
				var actionDescription string
				var successMessage string
				var ackType string = "unknown_ack"

				switch msg.Type {
				case "input_tap":
					if msg.X < 0 || msg.Y < 0 {
						log.Printf("ScreenMirrorWS: Invalid tap coordinates received: X=%d, Y=%d", msg.X, msg.Y)
						continue
					}
					actionDescription = fmt.Sprintf("input tap at (%d,%d)", msg.X, msg.Y)
					adbCmd = exec.Command("adb", "-s", deviceId, "shell", "input", "tap", strconv.Itoa(msg.X), strconv.Itoa(msg.Y))
					successMessage = fmt.Sprintf("Successfully executed tap for device %s at (%d,%d)", deviceId, msg.X, msg.Y)
					ackType = "input_tap_ack"

				case "input_text":
					if msg.Text == "" {
						log.Printf("ScreenMirrorWS: Empty text received for input_text.")
						continue
					}
					actionDescription = fmt.Sprintf("input text '%s'", msg.Text)
					adbCmd = exec.Command("adb", "-s", deviceId, "shell", "input", "text", msg.Text)
					successMessage = fmt.Sprintf("Successfully executed input text for device %s, text '%s'", deviceId, msg.Text)
					ackType = "input_text_ack"

				case "input_keyevent":
					if msg.Keycode == "" {
						log.Printf("ScreenMirrorWS: Empty keycode received for input_keyevent.")
						continue
					}
					actionDescription = fmt.Sprintf("input keyevent %s", msg.Keycode)
					adbCmd = exec.Command("adb", "-s", deviceId, "shell", "input", "keyevent", msg.Keycode)
					successMessage = fmt.Sprintf("Successfully executed input keyevent for device %s, keycode '%s'", deviceId, msg.Keycode)
					ackType = "input_keyevent_ack"

				case "input_swipe": // 新增：处理滑动事件
					if msg.X1 < 0 || msg.Y1 < 0 || msg.X2 < 0 || msg.Y2 < 0 {
						log.Printf("ScreenMirrorWS: Invalid swipe coordinates received: (%d,%d) to (%d,%d)", msg.X1, msg.Y1, msg.X2, msg.Y2)
						continue
					}
					durationStr := "300" // 默认滑动时间 300ms
					if msg.Duration > 0 {
						durationStr = strconv.Itoa(msg.Duration)
					}
					actionDescription = fmt.Sprintf("input swipe from (%d,%d) to (%d,%d) duration %s ms", msg.X1, msg.Y1, msg.X2, msg.Y2, durationStr)
					adbCmd = exec.Command("adb", "-s", deviceId, "shell", "input", "swipe",
						strconv.Itoa(msg.X1), strconv.Itoa(msg.Y1),
						strconv.Itoa(msg.X2), strconv.Itoa(msg.Y2),
						durationStr)
					successMessage = fmt.Sprintf("Successfully executed swipe for device %s from (%d,%d) to (%d,%d)", deviceId, msg.X1, msg.Y1, msg.X2, msg.Y2)
					ackType = "input_swipe_ack"

				default:
					log.Printf("ScreenMirrorWS: Received unknown message type from client %s (device: %s): %s", conn.RemoteAddr(), deviceId, msg.Type)
					// 发送一个错误消息回客户端
					errMsg := fmt.Sprintf("{\"type\":\"error\", \"message\":\"Unknown input type: %s\"}", msg.Type)
					if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(errMsg)); writeErr != nil {
						log.Printf("ScreenMirrorWS: Error sending unknown type error to client: %v", writeErr)
					}
					continue
				}

				// 执行 ADB 命令 (如果 adbCmd 已被设置)
				if adbCmd != nil {
					var cmdStderr bytes.Buffer
					adbCmd.Stderr = &cmdStderr
					log.Printf("ScreenMirrorWS: Executing for device %s: %s", deviceId, strings.Join(adbCmd.Args, " "))
					if err := adbCmd.Run(); err != nil {
						log.Printf("ScreenMirrorWS: Error executing %s for device %s: %v. Stderr: %s", actionDescription, deviceId, err, cmdStderr.String())
						// 发送错误消息回客户端
						errMsg := fmt.Sprintf("{\"type\":\"error\", \"message\":\"Failed to execute %s: %s\"}", msg.Type, cmdStderr.String())
						if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(errMsg)); writeErr != nil {
							log.Printf("ScreenMirrorWS: Error sending command execution error to client: %v", writeErr)
						}
					} else {
						log.Println(successMessage)
						// 发送成功确认消息回客户端
						ackMsg := fmt.Sprintf("{\"type\":\"%s\", \"status\":\"success\"}", ackType)
						if msg.Type == "input_text" { // 特别地，为文本输入返回发送的文本
							ackMsg = fmt.Sprintf("{\"type\":\"%s\", \"status\":\"success\", \"text\":\"%s\"}", ackType, msg.Text)
						} else if msg.Type == "input_keyevent" {
							ackMsg = fmt.Sprintf("{\"type\":\"%s\", \"status\":\"success\", \"keycode\":\"%s\"}", ackType, msg.Keycode)
						}
						if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(ackMsg)); writeErr != nil {
							log.Printf("ScreenMirrorWS: Error sending ack message to client: %v", writeErr)
						}
					}
				}
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
				log.Printf("ScreenMirrorWS: Error running screencap for device %s: %v\nStderr: %s", deviceId, err, stderr.String())
				errMsg := fmt.Sprintf("{\"type\":\"error\", \"message\":\"Screencap failed: %s\"}", strings.TrimSpace(stderr.String()))
				if writeErr := conn.WriteMessage(websocket.TextMessage, []byte(errMsg)); writeErr != nil {
					log.Printf("ScreenMirrorWS: Error sending screencap error to client: %v", writeErr)
				}
				continue
			}
			pngData := out.Bytes()
			if len(pngData) == 0 {
				log.Printf("ScreenMirrorWS: Screencap for device %s returned empty data.", deviceId)
				continue
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, pngData); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
					log.Printf("ScreenMirrorWS: Client %s (device: %s) disconnected (write error): %v", conn.RemoteAddr(), deviceId, err)
				} else if err == websocket.ErrCloseSent {
					log.Printf("ScreenMirrorWS: Client %s (device: %s) WebSocket closed by server (write error after CloseSent).", conn.RemoteAddr(), deviceId)
				} else {
					log.Printf("ScreenMirrorWS: Error writing screen frame to client %s (device: %s): %v", conn.RemoteAddr(), deviceId, err)
				}
				return
			}
		case <-clientDisconnected:
			log.Printf("ScreenMirrorWS: Client %s (device: %s) has disconnected (signaled by read goroutine). Stopping screen mirror.", conn.RemoteAddr(), deviceId)
			return
		}
	}
}
