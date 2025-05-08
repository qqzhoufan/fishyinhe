// D:\fishyinhe\backend\internal\adb\adb.go
package adb

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DeviceInfo 存储检测到的设备信息
type DeviceInfo struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// ListConnectedDevices 执行 "adb devices" 并解析输出
func ListConnectedDevices() ([]DeviceInfo, error) {
	cmd := exec.Command("adb", "devices")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("Error running adb devices: %v\nStderr: %s", err, stderr.String())
		return nil, err
	}

	var devices []DeviceInfo
	scanner := bufio.NewScanner(strings.NewReader(out.String()))

	// 跳过第一行 "List of devices attached"
	if scanner.Scan() {
		// log.Println("Skipped line:", scanner.Text())
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line) // 使用 Fields 来分割，可以处理多个制表符或空格
		if len(parts) == 2 {
			devices = append(devices, DeviceInfo{ID: parts[0], Status: parts[1]})
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error scanning adb output: %v", err)
		return nil, err
	}

	return devices, nil
}

// AdbFileItem 存储文件或目录信息
type AdbFileItem struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	// 未来可以添加更多信息，如大小、权限、修改日期等，但需要更复杂的ls参数和解析
}

// ListFiles 执行 "adb -s <deviceId> shell ls -p <path>" 并解析输出
func ListFiles(deviceId string, remotePath string) ([]AdbFileItem, error) {
	if remotePath == "" {
		remotePath = "/" // 默认为根目录，或者你可以选择 /sdcard/
	}

	// 使用 -p 参数会在目录名后附加斜杠
	// 使用 -A 参数可以包含隐藏文件 (以 . 开头的文件)，如果需要的话
	// cmd := exec.Command("adb", "-s", deviceId, "shell", "ls", "-Ap", remotePath)
	cmd := exec.Command("adb", "-s", deviceId, "shell", "ls", "-p", remotePath)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// adb shell ls 在目录为空或不存在时，有时会返回非0退出码，但stderr可能包含有用信息或为空
		// 所以我们不直接返回错误，而是先尝试解析输出，除非stderr明确指示了严重错误
		// 例如，如果路径不存在，stderr 可能会有 "No such file or directory"
		// 如果只是空目录，stdout 会是空的，但 stderr 可能也为空，命令也可能成功
		log.Printf("Warning/Error running adb ls for device %s, path %s: %v\nStdout: %s\nStderr: %s",
			deviceId, remotePath, err, out.String(), stderr.String())
		// 即使有错误，也尝试解析可能的输出，因为某些adb ls版本在空目录时可能返回错误码
	}

	// 检查 stderr 是否包含明确的错误信息，如 "No such file or directory"
	// 这是一个简单的检查，实际情况可能更复杂
	if strings.Contains(stderr.String(), "No such file or directory") {
		// 我们可以选择返回一个特定的错误类型或nil, nil让调用者知道路径无效
		// 为了简单，我们这里返回错误
		return nil, &exec.ExitError{Stderr: stderr.Bytes()} // 返回包含 stderr 的错误
	}

	var files []AdbFileItem
	scanner := bufio.NewScanner(strings.NewReader(out.String()))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "." || line == ".." { // 忽略空行和 . 及 ..
			continue
		}

		isDir := strings.HasSuffix(line, "/")
		name := line
		if isDir {
			name = strings.TrimSuffix(line, "/")
		}

		// 对于某些系统，ls -p 可能会在符号链接目录后也加 /
		// 更复杂的解析可能需要 ls -l 等来区分
		files = append(files, AdbFileItem{Name: name, IsDir: isDir})
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error scanning adb ls output for device %s, path %s: %v", deviceId, remotePath, err)
		return nil, err
	}

	return files, nil
}

func PullFile(deviceId string, remoteFilePath string, localTempBaseDir string) (string, error) {
	if deviceId == "" || remoteFilePath == "" {
		return "", fmt.Errorf("device ID and remote file path cannot be empty")
	}

	// 创建一个唯一的本地临时文件名
	// localFileName := filepath.Base(remoteFilePath) // 使用原始文件名
	// 为了避免冲突和处理特殊字符，可以生成一个唯一ID作为文件名，或确保localFileName是安全的
	// 这里我们先简单地在临时目录下创建子目录来存放，以deviceId区分

	// 确保基础临时目录存在
	if err := os.MkdirAll(localTempBaseDir, 0755); err != nil {
		log.Printf("Failed to create temp base directory %s: %v", localTempBaseDir, err)
		return "", err
	}

	// 为本次操作创建一个临时文件，避免直接使用原始文件名（可能包含非法字符）
	// 更简单的方式是让 `adb pull` 直接拉取到指定目录，它会使用原始文件名
	// 我们需要一个临时目标路径，让 adb pull 自动创建文件
	// 例如，localTempBaseDir/original_filename.ext
	// 为了确保唯一性，我们可以创建一个临时文件然后获取其名称，或者直接指定目标路径

	// 构建本地目标路径，直接使用文件名
	localFileName := filepath.Base(remoteFilePath)
	localFilePath := filepath.Join(localTempBaseDir, localFileName)

	log.Printf("Attempting to pull '%s' from device '%s' to '%s'", remoteFilePath, deviceId, localFilePath)

	// adb pull "remote_path" "local_path"
	cmd := exec.Command("adb", "-s", deviceId, "pull", remoteFilePath, localFilePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to pull file '%s' from device '%s': %v. Stderr: %s",
			remoteFilePath, deviceId, err, stderr.String())
		log.Println(errMsg)
		// 如果本地文件已部分创建，尝试删除它
		if _, statErr := os.Stat(localFilePath); statErr == nil {
			os.Remove(localFilePath)
		}
		return "", fmt.Errorf(errMsg)
	}

	log.Printf("Successfully pulled '%s' to '%s'", remoteFilePath, localFilePath)
	return localFilePath, nil
}

// PushFile 将服务器上的本地文件推送到指定设备的远程路径
func PushFile(deviceId string, localFilePath string, remoteDevicePath string) error {
	if deviceId == "" || localFilePath == "" || remoteDevicePath == "" {
		return fmt.Errorf("PushFile: deviceId, localFilePath, and remoteDevicePath cannot be empty. Got: D='%s', L='%s', R='%s'", deviceId, localFilePath, remoteDevicePath)
	}

	log.Printf("Attempting to push '%s' (local) to device '%s' at '%s'", localFilePath, deviceId, remoteDevicePath)

	// adb -s <deviceId> push "<localFilePath>" "<remoteDevicePath>"
	// 确保路径被正确引用，特别是包含空格的路径
	cmd := exec.Command("adb", "-s", deviceId, "push", localFilePath, remoteDevicePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to push file '%s' to device '%s' at '%s': %v. Stderr: %s",
			localFilePath, deviceId, remoteDevicePath, err, stderr.String())
		log.Println(errMsg)
		return fmt.Errorf(errMsg)
	}

	log.Printf("Successfully pushed '%s' to device '%s' at '%s'", localFilePath, deviceId, remoteDevicePath)
	return nil
}

func InstallAPK(deviceId string, localApkPathOnServer string) (string, error) {
	if deviceId == "" || localApkPathOnServer == "" {
		return "", fmt.Errorf("InstallAPK: deviceId and localApkPathOnServer cannot be empty")
	}

	// 检查服务器上的文件是否存在
	if _, err := os.Stat(localApkPathOnServer); os.IsNotExist(err) {
		log.Printf("InstallAPK: Error - Local APK file not found on server at path: %s", localApkPathOnServer)
		return "", fmt.Errorf("local APK file not found on server: %s", localApkPathOnServer)
	}

	// 打印将要执行的命令的准确形式
	log.Printf("InstallAPK: Preparing to execute: adb -s \"%s\" install -r -g \"%s\"", deviceId, localApkPathOnServer)

	// 构建命令: adb -s <deviceId> install -r -g <localApkPathOnServer>
	cmd := exec.Command("adb", "-s", deviceId, "install", "-r", "-g", localApkPathOnServer)
	var out bytes.Buffer // adb install 的输出通常在 stdout
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(out.String() + "\n" + stderr.String()) // 合并 stdout 和 stderr

	log.Printf("InstallAPK: 'adb install' command for device '%s', APK '%s' finished.", deviceId, filepath.Base(localApkPathOnServer))
	log.Printf("InstallAPK: Stdout: [%s]", strings.TrimSpace(out.String()))
	log.Printf("InstallAPK: Stderr: [%s]", strings.TrimSpace(stderr.String()))
	log.Printf("InstallAPK: Combined output: [%s]", output)

	if err != nil {
		errMsg := fmt.Sprintf("InstallAPK: Failed to execute adb install for APK '%s' on device '%s': %v. Full Output: %s",
			localApkPathOnServer, deviceId, err, output)
		log.Println(errMsg)
		// 注意：即使 cmd.Run() 返回错误，output 也可能包含有用的 ADB 错误信息
		return output, fmt.Errorf("adb install command execution failed: %v. Output: %s", err, output)
	}

	log.Printf("InstallAPK: APK installation command executed successfully (cmd.Run returned nil) for '%s' on device '%s'. Output: %s", filepath.Base(localApkPathOnServer), deviceId, output)
	// 检查输出是否明确包含 "Success"
	if !strings.Contains(strings.ToLower(output), "success") {
		log.Printf("InstallAPK: Warning - Output for device '%s', APK '%s' did not explicitly contain 'success'. Full Output: %s", deviceId, filepath.Base(localApkPathOnServer), output)
		// 可以考虑返回一个特定错误，如果需要前端明确知道安装是否真的成功
		// return output, fmt.Errorf("installation did not report success: %s", output)
	}

	return output, nil
}
