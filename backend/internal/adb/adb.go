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
	"time"
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

	if scanner.Scan() {
		// 跳过第一行 "List of devices attached"
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
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
}

// ListFiles 执行 "adb -s <deviceId> shell ls -p <path>" 并解析输出
func ListFiles(deviceId string, remotePath string) ([]AdbFileItem, error) {
	if remotePath == "" {
		remotePath = "/"
	}
	cmd := exec.Command("adb", "-s", deviceId, "shell", "ls", "-p", remotePath)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("Warning/Error running adb ls for device %s, path %s: %v\nStdout: %s\nStderr: %s",
			deviceId, remotePath, err, out.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "No such file or directory") || strings.Contains(stderr.String(), "Not a directory") {
		return nil, fmt.Errorf("path not found or not a directory: %s", stderr.String())
	}

	var files []AdbFileItem
	scanner := bufio.NewScanner(strings.NewReader(out.String()))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "." || line == ".." {
			continue
		}
		isDir := strings.HasSuffix(line, "/")
		name := line
		if isDir {
			name = strings.TrimSuffix(line, "/")
		}
		files = append(files, AdbFileItem{Name: name, IsDir: isDir})
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error scanning adb ls output for device %s, path %s: %v", deviceId, remotePath, err)
		return nil, err
	}
	return files, nil
}

// PullFile 将指定设备上的远程文件拉取到服务器的本地临时目录
func PullFile(deviceId string, remoteFilePath string, localTempBaseDir string) (string, error) {
	if deviceId == "" || remoteFilePath == "" {
		return "", fmt.Errorf("device ID and remote file path cannot be empty")
	}
	if err := os.MkdirAll(localTempBaseDir, 0755); err != nil {
		log.Printf("Failed to create temp base directory %s: %v", localTempBaseDir, err)
		return "", err
	}
	localFileName := filepath.Base(remoteFilePath)
	localFilePath := filepath.Join(localTempBaseDir, localFileName)

	log.Printf("Attempting to pull '%s' from device '%s' to '%s'", remoteFilePath, deviceId, localFilePath)
	cmd := exec.Command("adb", "-s", deviceId, "pull", remoteFilePath, localFilePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to pull file '%s' from device '%s': %v. Stderr: %s",
			remoteFilePath, deviceId, err, stderr.String())
		log.Println(errMsg)
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
	log.Printf("PushFile: Attempting to push local file '%s' to device '%s' at remote path '%s'", localFilePath, deviceId, remoteDevicePath)
	cmd := exec.Command("adb", "-s", deviceId, "push", localFilePath, remoteDevicePath)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	log.Printf("PushFile: 'adb push' command for device '%s' finished.", deviceId)
	log.Printf("PushFile: Stdout: [%s]", strings.TrimSpace(out.String()))
	log.Printf("PushFile: Stderr: [%s]", strings.TrimSpace(stderr.String()))
	if err != nil {
		errMsg := fmt.Sprintf("PushFile: Failed to push file '%s' to device '%s' at '%s'. Error: %v. Combined Output (Stdout+Stderr): [%s %s]",
			localFilePath, deviceId, remoteDevicePath, err, strings.TrimSpace(out.String()), strings.TrimSpace(stderr.String()))
		log.Println(errMsg)
		return fmt.Errorf("adb push command failed. Stderr: %s", strings.TrimSpace(stderr.String()))
	}
	log.Printf("PushFile: Successfully pushed '%s' to device '%s' at '%s'", localFilePath, deviceId, remoteDevicePath)
	return nil
}

// GoToHomeScreen 在指定设备上模拟按下 Home 键
func GoToHomeScreen(deviceId string) error {
	if deviceId == "" {
		return fmt.Errorf("GoToHomeScreen: deviceId cannot be empty")
	}
	log.Printf("GoToHomeScreen: Attempting to send HOME keyevent to device '%s'", deviceId)
	cmd := exec.Command("adb", "-s", deviceId, "shell", "input", "keyevent", "KEYCODE_HOME")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		errMsg := fmt.Sprintf("GoToHomeScreen: Failed for device '%s': %v. Stderr: %s",
			deviceId, err, stderr.String())
		log.Println(errMsg)
		return fmt.Errorf(errMsg)
	}
	log.Printf("GoToHomeScreen: Successfully sent HOME keyevent to device '%s'", deviceId)
	return nil
}

// ListInstalledPackages 获取设备上已安装的应用包名列表
func ListInstalledPackages(deviceId string, options string) ([]string, error) {
	if deviceId == "" {
		return nil, fmt.Errorf("ListInstalledPackages: deviceId cannot be empty")
	}
	log.Printf("ListInstalledPackages: Attempting to list packages on device '%s' with options '%s'", deviceId, options)
	args := []string{"-s", deviceId, "shell", "pm", "list", "packages"}
	if options != "" {
		args = append(args, options)
	}
	cmd := exec.Command("adb", args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		errMsg := fmt.Sprintf("ListInstalledPackages: Failed for device '%s': %v. Stderr: %s",
			deviceId, err, stderr.String())
		log.Println(errMsg)
		return nil, fmt.Errorf(errMsg)
	}
	var packages []string
	scanner := bufio.NewScanner(strings.NewReader(out.String()))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "package:") {
			packageName := strings.TrimPrefix(line, "package:")
			packages = append(packages, packageName)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("ListInstalledPackages: Error scanning pm list packages output: %v", err)
		return nil, err
	}
	log.Printf("ListInstalledPackages: Found %d packages on device '%s'", len(packages), deviceId)
	return packages, nil
}

// InstallAPK 使用 adb install 命令从服务器本地路径安装 APK 到指定设备
// localApkPathOnServer 是 APK 文件在运行 Go 后端的服务器上的完整路径
// 返回 ADB 命令的输出
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

	// *** 修正：使用 adb install <local_path> 而不是 adb shell pm install <local_path> ***
	cmd := exec.Command("adb", "-s", deviceId, "install", "-r", "-g", localApkPathOnServer)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(out.String() + "\n" + stderr.String()) // 合并 stdout 和 stderr

	log.Printf("InstallAPK: 'adb install' command for device '%s', APK from server path '%s' finished.", deviceId, localApkPathOnServer)
	log.Printf("InstallAPK: Stdout: [%s]", strings.TrimSpace(out.String()))
	log.Printf("InstallAPK: Stderr: [%s]", strings.TrimSpace(stderr.String()))
	log.Printf("InstallAPK: Combined output: [%s]", output)

	if err != nil {
		errMsg := fmt.Sprintf("InstallAPK: Failed to execute adb install for APK from server path '%s' on device '%s': %v. Full Output: %s",
			localApkPathOnServer, deviceId, err, output)
		log.Println(errMsg)
		// 即使 cmd.Run() 返回错误，output 也可能包含有用的 ADB 错误信息
		// 这里的错误 "failed to write; ... (No such file or directory)" 是 adb.exe 自己的错误，表明它找不到本地文件
		return output, fmt.Errorf("adb install command execution failed: %v. Output: %s", err, output)
	}

	log.Printf("InstallAPK: APK installation command executed successfully (cmd.Run returned nil) for server APK '%s' on device '%s'. Output: %s", filepath.Base(localApkPathOnServer), deviceId, output)
	// 检查输出是否明确包含 "Success"
	if !strings.Contains(strings.ToLower(output), "success") {
		log.Printf("InstallAPK: Warning - Output for device '%s', server APK '%s' did not explicitly contain 'success'. Full Output: %s", deviceId, filepath.Base(localApkPathOnServer), output)
	}

	return output, nil
}
func UninstallPackage(deviceId string, packageName string, options string) (string, error) {
	if deviceId == "" || packageName == "" {
		return "", fmt.Errorf("UninstallPackage: deviceId and packageName cannot be empty")
	}

	log.Printf("UninstallPackage: Attempting to uninstall package '%s' from device '%s' with options '%s'", packageName, deviceId, options)

	args := []string{"-s", deviceId, "shell", "pm", "uninstall"}
	if options != "" {
		args = append(args, options)
	}
	args = append(args, packageName)

	cmd := exec.Command("adb", args...)
	var out bytes.Buffer // pm uninstall 的输出通常在 stdout
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	// pm uninstall 成功时返回退出码 0，输出通常包含 "Success"
	// 失败时，输出和 stderr 中会有错误信息
	output := strings.TrimSpace(out.String() + "\n" + stderr.String()) // 合并 stdout 和 stderr

	log.Printf("UninstallPackage: 'adb shell pm uninstall' command for device '%s', package '%s' finished.", deviceId, packageName)
	log.Printf("UninstallPackage: Stdout: [%s]", strings.TrimSpace(out.String()))
	log.Printf("UninstallPackage: Stderr: [%s]", strings.TrimSpace(stderr.String()))
	log.Printf("UninstallPackage: Combined output: [%s]", output)

	if err != nil {
		errMsg := fmt.Sprintf("UninstallPackage: Failed to execute for package '%s' on device '%s': %v. Full Output: %s",
			packageName, deviceId, err, output)
		log.Println(errMsg)
		return output, fmt.Errorf("adb uninstall command execution failed: %v. Output: %s", err, output)
	}

	log.Printf("UninstallPackage: Uninstallation command executed successfully (cmd.Run returned nil) for package '%s' on device '%s'. Output: %s", packageName, deviceId, output)
	if !strings.Contains(strings.ToLower(output), "success") {
		log.Printf("UninstallPackage: Warning - Output for device '%s', package '%s' did not explicitly contain 'success'. Full Output: %s", deviceId, packageName, output)
		// 可以考虑返回一个特定错误，如果需要前端明确知道卸载是否真的成功
		// return output, fmt.Errorf("uninstallation did not explicitly report success: %s", output)
	}

	return output, nil
}

// ClearLogcatBuffer 清除指定设备上的 logcat 缓存
func ClearLogcatBuffer(deviceId string) error {
	if deviceId == "" {
		return fmt.Errorf("ClearLogcatBuffer: deviceId cannot be empty")
	}
	log.Printf("ClearLogcatBuffer: Attempting to clear logcat buffer for device '%s'", deviceId)

	cmd := exec.Command("adb", "-s", deviceId, "logcat", "-c")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errMsg := fmt.Sprintf("ClearLogcatBuffer: Failed for device '%s': %v. Stderr: %s",
			deviceId, err, stderr.String())
		log.Println(errMsg)
		return fmt.Errorf(errMsg)
	}

	log.Printf("ClearLogcatBuffer: Successfully cleared logcat buffer for device '%s'", deviceId)
	return nil
}

// DumpLogcatToFile 将指定设备的 logcat -d 输出保存到服务器的临时文件
// 返回临时文件的路径和错误
func DumpLogcatToFile(deviceId string, tempDir string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("DumpLogcatToFile: deviceId cannot be empty")
	}

	log.Printf("DumpLogcatToFile: Attempting to dump logcat for device '%s'", deviceId)

	// 创建临时文件目录 (如果不存在)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Printf("DumpLogcatToFile: Failed to create temp directory %s: %v", tempDir, err)
		return "", err
	}

	// 创建一个唯一的临时文件名
	tempFileName := fmt.Sprintf("logcat_%s_%d.txt", strings.ReplaceAll(deviceId, ":", "_"), time.Now().UnixNano())
	localTempFilePath := filepath.Join(tempDir, tempFileName)

	log.Printf("DumpLogcatToFile: Saving logcat to server temporary file: %s", localTempFilePath)

	// 执行 adb logcat -d
	cmd := exec.Command("adb", "-s", deviceId, "logcat", "-d") // -d dumps the log and exits
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	// logcat -d 即使成功也可能返回非0退出码（如果缓冲区为空），所以主要关注 stderr
	if err != nil && stderr.Len() > 0 { // 只有当 stderr 有内容时才认为是真正的执行错误
		errMsg := fmt.Sprintf("DumpLogcatToFile: Failed to execute adb logcat -d for device '%s': %v. Stderr: %s",
			deviceId, err, stderr.String())
		log.Println(errMsg)
		return "", fmt.Errorf(errMsg)
	}
	if stderr.Len() > 0 {
		log.Printf("DumpLogcatToFile: Stderr from adb logcat -d for device '%s': %s", deviceId, stderr.String())
		// 这可能是一些警告或非致命错误，我们仍然尝试保存 stdout
	}

	// 将 stdout 的内容写入临时文件
	err = os.WriteFile(localTempFilePath, stdout.Bytes(), 0644)
	if err != nil {
		log.Printf("DumpLogcatToFile: Failed to write logcat output to temporary file %s: %v", localTempFilePath, err)
		return "", err
	}

	log.Printf("DumpLogcatToFile: Successfully dumped logcat for device '%s' to '%s'", deviceId, localTempFilePath)
	return localTempFilePath, nil
}
func ForceStopPackage(deviceId string, packageName string) (string, error) {
	if deviceId == "" || packageName == "" {
		return "", fmt.Errorf("ForceStopPackage: deviceId and packageName cannot be empty")
	}

	log.Printf("ForceStopPackage: Attempting to force stop package '%s' on device '%s'", packageName, deviceId)

	// adb -s <deviceId> shell am force-stop <packageName>
	cmd := exec.Command("adb", "-s", deviceId, "shell", "am", "force-stop", packageName)
	var out bytes.Buffer // am force-stop 通常没有太多 stdout 输出
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	// am force-stop 成功时返回退出码 0
	// 失败时，stderr 中会有错误信息
	output := strings.TrimSpace(out.String() + "\n" + stderr.String()) // 合并 stdout 和 stderr

	log.Printf("ForceStopPackage: 'adb shell am force-stop' command for device '%s', package '%s' finished.", deviceId, packageName)
	log.Printf("ForceStopPackage: Stdout: [%s]", strings.TrimSpace(out.String()))
	log.Printf("ForceStopPackage: Stderr: [%s]", strings.TrimSpace(stderr.String()))
	log.Printf("ForceStopPackage: Combined output: [%s]", output)

	if err != nil {
		errMsg := fmt.Sprintf("ForceStopPackage: Failed to execute for package '%s' on device '%s': %v. Full Output: %s",
			packageName, deviceId, err, output)
		log.Println(errMsg)
		return output, fmt.Errorf("adb force-stop command execution failed: %v. Output: %s", err, output)
	}

	// force-stop 成功通常没有 "Success" 字样，只要没有错误即可认为是成功
	log.Printf("ForceStopPackage: Force-stop command executed (cmd.Run returned nil) for package '%s' on device '%s'. Output: %s", packageName, deviceId, output)

	return output, nil
}

func WakeUpDevice(deviceId string) error {
	if deviceId == "" {
		return fmt.Errorf("WakeUpDevice: deviceId cannot be empty")
	}

	log.Printf("WakeUpDevice: Attempting to send WAKEUP keyevent to device '%s'", deviceId)
	// adb -s <deviceId> shell input keyevent KEYCODE_WAKEUP
	// KEYCODE_POWER (26) 也可以，但 WAKEUP (224) 更直接
	cmd := exec.Command("adb", "-s", deviceId, "shell", "input", "keyevent", "KEYCODE_WAKEUP")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errMsg := fmt.Sprintf("WakeUpDevice: Failed for device '%s': %v. Stderr: %s",
			deviceId, err, stderr.String())
		log.Println(errMsg)
		return fmt.Errorf(errMsg)
	}

	log.Printf("WakeUpDevice: Successfully sent WAKEUP keyevent to device '%s'", deviceId)
	return nil
}
