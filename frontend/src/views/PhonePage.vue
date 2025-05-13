<script setup>
import { ref, onMounted, onUnmounted, watch, computed } from 'vue';
import axios from 'axios';

// --- 设备列表与选择状态 ---
const devices = ref([]);
const isLoading = ref(false);
const errorMessage = ref('');
const selectedDeviceId = ref(null);

// --- 屏幕镜像相关状态 ---
const isMirroring = ref(false);
const screenCanvasRef = ref(null);
let socket = null;
let canvasCtx = null;
const isDraggingOnCanvas = ref(false);
const dragStartX = ref(0);
const dragStartY = ref(0);

// --- 文件浏览相关状态 ---
const currentRemotePath = ref('/sdcard/');
const fileList = ref([]);
const isFileListing = ref(false);
const fileListError = ref('');

// --- 文件上传相关状态 (普通文件上传) ---
const fileInputRef = ref(null);
const selectedFileToUpload = ref(null);
const isUploadingFile = ref(false);
const uploadProgress = ref(0);
const uploadError = ref('');
const uploadSuccessMessage = ref('');

// --- APK 安装相关状态 ---
const apkFileInputRef = ref(null);
const selectedApkToInstall = ref(null);
const isInstallingApk = ref(false);
const apkInstallMessage = ref('');
const apkInstallError = ref('');

// --- 返回主页功能的状态 ---
const isSendingGoHome = ref(false);
const goHomeMessage = ref('');

// --- 应用管理相关状态 ---
const installedApps = ref([]);
const isLoadingApps = ref(false);
const appsListError = ref('');
const appFilterOption = ref('');
const uninstallingPackage = ref(null);
const uninstallStatusMessage = ref('');
const stoppingPackage = ref(null);
const stopAppStatusMessage = ref('');

// --- 远程文本输入状态 ---
const remoteInputText = ref('');
const isSendingText = ref(false);
const remoteInputStatus = ref('');

// --- Logcat 管理相关状态 ---
const isClearingLogcat = ref(false);
const clearLogcatMessage = ref('');
const isDownloadingLogcat = ref(false);
const downloadLogcatMessage = ref('');

// --- 唤醒屏幕功能的状态 ---
const isWakingUpDevice = ref(false);
const wakeUpMessage = ref('');


// --- 获取设备列表 ---
async function fetchDevices() {
  isLoading.value = true;
  errorMessage.value = '';
  devices.value = [];
  if (isMirroring.value) stopScreenMirroring();
  selectedDeviceId.value = null;

  // 清空所有子功能状态
  fileList.value = []; currentRemotePath.value = '/sdcard/'; fileListError.value = '';
  uploadError.value = ''; uploadSuccessMessage.value = ''; selectedFileToUpload.value = null;
  selectedApkToInstall.value = null; apkInstallMessage.value = ''; apkInstallError.value = '';
  goHomeMessage.value = '';
  installedApps.value = []; isLoadingApps.value = false; appsListError.value = ''; uninstallStatusMessage.value = '';
  stoppingPackage.value = null; stopAppStatusMessage.value = '';
  remoteInputText.value = ''; remoteInputStatus.value = '';
  isClearingLogcat.value = false; clearLogcatMessage.value = '';
  isDownloadingLogcat.value = false; downloadLogcatMessage.value = '';
  isWakingUpDevice.value = false; wakeUpMessage.value = '';

  try {
    const response = await axios.get('http://localhost:5679/api/devices');
    devices.value = response.data || [];
    if (devices.value.length === 0) errorMessage.value = "未检测到已连接的设备。";
  } catch (error) {
    errorMessage.value = `加载设备列表失败: ${error.message}`;
  } finally {
    isLoading.value = false;
  }
}

// --- 选择设备 ---
function selectDevice(deviceId) {
  if (isMirroring.value) stopScreenMirroring();
  selectedDeviceId.value = deviceId;

  // 重置所有子功能状态
  fileList.value = []; currentRemotePath.value = '/sdcard/'; fileListError.value = '';
  uploadError.value = ''; uploadSuccessMessage.value = ''; selectedFileToUpload.value = null;
  selectedApkToInstall.value = null; apkInstallMessage.value = ''; apkInstallError.value = '';
  goHomeMessage.value = '';
  installedApps.value = []; isLoadingApps.value = false; appsListError.value = ''; uninstallStatusMessage.value = '';
  stoppingPackage.value = null; stopAppStatusMessage.value = '';
  remoteInputText.value = ''; remoteInputStatus.value = '';
  isClearingLogcat.value = false; clearLogcatMessage.value = '';
  isDownloadingLogcat.value = false; downloadLogcatMessage.value = '';
  isWakingUpDevice.value = false; wakeUpMessage.value = '';

  if (deviceId) fetchFileList();
}

// --- 坐标转换辅助函数 ---
function getCanvasCoordinates(event, canvasElement) {
  const rect = canvasElement.getBoundingClientRect();
  const scaleX = canvasElement.width / rect.width;
  const scaleY = canvasElement.height / rect.height;
  const x = Math.round((event.clientX - rect.left) * scaleX);
  const y = Math.round((event.clientY - rect.top) * scaleY);
  return { x, y, scaleX, scaleY, rect };
}


// --- 处理 Canvas 点击/触摸事件 ---
function handleCanvasPointerDown(event) {
  if (!isMirroring.value || !socket || socket.readyState !== WebSocket.OPEN || !screenCanvasRef.value) return;
  event.preventDefault();

  const { x, y } = getCanvasCoordinates(event.touches ? event.touches[0] : event, screenCanvasRef.value);
  isDraggingOnCanvas.value = true;
  dragStartX.value = x;
  dragStartY.value = y;
}

function handleCanvasPointerUp(event) {
  if (!isMirroring.value || !socket || socket.readyState !== WebSocket.OPEN || !screenCanvasRef.value || !isDraggingOnCanvas.value) {
    isDraggingOnCanvas.value = false;
    return;
  }
  event.preventDefault();

  const { x: endX, y: endY } = getCanvasCoordinates(event.changedTouches ? event.changedTouches[0] : event, screenCanvasRef.value);
  isDraggingOnCanvas.value = false;

  const deltaX = endX - dragStartX.value;
  const deltaY = endY - dragStartY.value;
  const distance = Math.sqrt(deltaX * deltaX + deltaY * deltaY);
  const swipeThreshold = 10;

  if (distance < swipeThreshold) {
    if (dragStartX.value >= 0 && dragStartX.value <= screenCanvasRef.value.width &&
        dragStartY.value >= 0 && dragStartY.value <= screenCanvasRef.value.height) {
      const clickData = { type: "input_tap", x: dragStartX.value, y: dragStartY.value };
      socket.send(JSON.stringify(clickData));
    }
  } else {
    if (dragStartX.value >= 0 && dragStartX.value <= screenCanvasRef.value.width &&
        dragStartY.value >= 0 && dragStartY.value <= screenCanvasRef.value.height &&
        endX >=0 && endX <= screenCanvasRef.value.width &&
        endY >=0 && endY <= screenCanvasRef.value.height
    ) {
      const swipeData = { type: "input_swipe", x1: dragStartX.value, y1: dragStartY.value, x2: endX, y2: endY, duration: 300 };
      socket.send(JSON.stringify(swipeData));
    }
  }
  dragStartX.value = 0; dragStartY.value = 0;
}

// --- 屏幕镜像方法 ---
function startScreenMirroring() {
  if (!selectedDeviceId.value) { alert("请先选择一个设备！"); return; }
  if (isMirroring.value || socket) return;
  const wsUrl = `ws://localhost:5679/api/screen/${selectedDeviceId.value}`;
  socket = new WebSocket(wsUrl);
  socket.binaryType = 'blob';
  isMirroring.value = true;
  errorMessage.value = '';
  remoteInputStatus.value = '';

  socket.onopen = () => {
    if (screenCanvasRef.value) {
      canvasCtx = screenCanvasRef.value.getContext('2d');
      screenCanvasRef.value.addEventListener('pointerdown', handleCanvasPointerDown);
      screenCanvasRef.value.addEventListener('pointerup', handleCanvasPointerUp);
    }
  };
  socket.onmessage = async (event) => {
    if (event.data instanceof Blob && canvasCtx && screenCanvasRef.value) {
      try {
        const imageBitmap = await createImageBitmap(event.data);
        if (screenCanvasRef.value.width !== imageBitmap.width || screenCanvasRef.value.height !== imageBitmap.height) {
          screenCanvasRef.value.width = imageBitmap.width;
          screenCanvasRef.value.height = imageBitmap.height;
        }
        canvasCtx.drawImage(imageBitmap, 0, 0);
        imageBitmap.close();
      } catch (e) { console.error("Canvas draw error:", e); }
    } else if (typeof event.data === 'string') {
      try {
        const msgData = JSON.parse(event.data);
        if (msgData.type === 'error') remoteInputStatus.value = `屏幕镜像错误: ${msgData.message || '未知错误'}`;
        else if (msgData.type === 'input_text_ack') remoteInputStatus.value = `文本已发送: "${msgData.text}"`;
        else if (msgData.type === 'input_keyevent_ack') remoteInputStatus.value = `按键 ${msgData.keycode} 已发送。`;
        else if (msgData.type === 'input_swipe_ack') remoteInputStatus.value = `滑动操作已发送。`;
        else if (msgData.type === 'input_tap_ack') remoteInputStatus.value = `点击操作已发送。`;
      } catch (e) { remoteInputStatus.value = `收到未知文本消息: ${event.data}`; }
    }
  };
  socket.onerror = (error) => { errorMessage.value = "屏幕镜像连接发生错误。"; remoteInputStatus.value = ''; };
  socket.onclose = (event) => {
    isMirroring.value = false;
    if (screenCanvasRef.value) {
      screenCanvasRef.value.removeEventListener('pointerdown', handleCanvasPointerDown);
      screenCanvasRef.value.removeEventListener('pointerup', handleCanvasPointerUp);
      if (canvasCtx) canvasCtx.clearRect(0, 0, screenCanvasRef.value.width, screenCanvasRef.value.height);
    }
    canvasCtx = null; socket = null;
    if (event.code !== 1000 && !event.wasClean && !errorMessage.value) errorMessage.value = `屏幕镜像连接意外断开 (Code: ${event.code})`;
    if (!errorMessage.value) remoteInputStatus.value = '屏幕镜像已断开。';
  };
}
function stopScreenMirroring() { if (socket) socket.close(1000, "User stop"); }

// --- 文件浏览方法 ---
async function fetchFileList() {
  if (!selectedDeviceId.value) { fileListError.value = "请先选择设备。"; return; }
  isFileListing.value = true;
  fileListError.value = ''; uploadSuccessMessage.value = ''; uploadError.value = '';
  apkInstallMessage.value = ''; apkInstallError.value = ''; goHomeMessage.value = '';
  appsListError.value = ''; uninstallStatusMessage.value = ''; remoteInputStatus.value = '';
  clearLogcatMessage.value = ''; downloadLogcatMessage.value = ''; stopAppStatusMessage.value = '';
  wakeUpMessage.value = '';

  try {
    const response = await axios.get(`http://localhost:5679/api/files/list/${selectedDeviceId.value}`, {
      params: { path: currentRemotePath.value }
    });
    currentRemotePath.value = response.data.path || currentRemotePath.value;
    fileList.value = response.data.files || [];
  } catch (error) {
    fileListError.value = `加载文件列表失败: ${error.response?.data?.error || error.message}`;
    fileList.value = [];
  } finally {
    isFileListing.value = false;
  }
}
function navigateTo(item) {
  if (item.isDir) {
    let newPath = currentRemotePath.value;
    if (newPath !== '/' && !newPath.endsWith('/')) newPath += '/';
    newPath += item.name;
    if (newPath !== '/' && !newPath.endsWith('/')) newPath += '/';
    currentRemotePath.value = newPath;
    fetchFileList();
  } else {
    downloadFile(item);
  }
}
async function downloadFile(item) {
  if (!selectedDeviceId.value || item.isDir) return;
  let fullRemotePath = currentRemotePath.value;
  if (fullRemotePath !== '/' && !fullRemotePath.endsWith('/')) fullRemotePath += '/';
  fullRemotePath += item.name;
  fileListError.value = `准备下载 ${item.name}...`;
  const downloadUrl = `http://localhost:5679/api/files/download/${selectedDeviceId.value}?filePath=${encodeURIComponent(fullRemotePath)}`;
  try {
    const response = await axios({ url: downloadUrl, method: 'GET', responseType: 'blob' });
    const href = URL.createObjectURL(response.data);
    const link = document.createElement('a');
    link.href = href; link.setAttribute('download', item.name);
    document.body.appendChild(link); link.click();
    document.body.removeChild(link); URL.revokeObjectURL(href);
    fileListError.value = `${item.name} 下载开始。`;
    setTimeout(() => { if (fileListError.value === `${item.name} 下载开始。`) fileListError.value = ''; }, 3000);
  } catch (error) {
    let errorMsg = `下载文件 "${item.name}" 失败: `;
    if (error.response) {
      if (error.response.data instanceof Blob && error.response.data.type === "application/json") {
        try {
          const errJsonText = await error.response.data.text();
          const errJson = JSON.parse(errJsonText);
          errorMsg += `${error.response.status} - ${errJson.error || '服务器返回未知错误'}`;
        } catch (parseError) {
          errorMsg += `${error.response.status} - ${error.response.statusText || '无法解析服务器错误响应'}`;
        }
      } else {
        errorMsg += `${error.response.status} - ${error.response.statusText || '服务器错误'}`;
      }
    } else if (error.request) { errorMsg += "网络错误或无法连接到服务器。"; }
    else { errorMsg += error.message; }
    fileListError.value = errorMsg;
  }
}
function navigateUp() {
  let path = currentRemotePath.value;
  if (path !== '/' && path.endsWith('/')) path = path.substring(0, path.length - 1);
  const lastSlashIndex = path.lastIndexOf('/');
  if (lastSlashIndex > 0) currentRemotePath.value = path.substring(0, lastSlashIndex + 1);
  else if (lastSlashIndex === 0 && path.length > 1) currentRemotePath.value = '/';
  else return;
  fetchFileList();
}
const canNavigateUp = computed(() => currentRemotePath.value && currentRemotePath.value !== '/');

// --- 文件上传方法 ---
function triggerFileInput() {
  selectedFileToUpload.value = null; uploadError.value = ''; uploadSuccessMessage.value = ''; uploadProgress.value = 0;
  if (fileInputRef.value) { fileInputRef.value.value = ''; fileInputRef.value.click(); }
}
function handleFileSelect(event) {
  const file = event.target.files[0];
  if (file) { selectedFileToUpload.value = file; uploadError.value = ''; uploadSuccessMessage.value = ''; }
  else { selectedFileToUpload.value = null; }
}
async function uploadSelectedFile() {
  if (!selectedFileToUpload.value || !selectedDeviceId.value || !currentRemotePath.value.endsWith('/')) {
    uploadError.value = !selectedFileToUpload.value ? "请选择文件。" : (!selectedDeviceId.value ? "请选择设备。" : "当前远程路径无效。");
    return;
  }
  isUploadingFile.value = true; uploadProgress.value = 0; uploadError.value = ''; uploadSuccessMessage.value = '';
  const formData = new FormData();
  formData.append('file', selectedFileToUpload.value);
  formData.append('remoteDirPath', currentRemotePath.value);
  try {
    const response = await axios.post(
        `http://localhost:5679/api/files/upload/${selectedDeviceId.value}`, formData,
        { headers: { 'Content-Type': 'multipart/form-data' },
          onUploadProgress: (e) => { if (e.total) uploadProgress.value = Math.round((e.loaded * 100) / e.total); } }
    );
    uploadSuccessMessage.value = `文件 "${response.data.filename}" 成功上传到 "${response.data.filePath}"。`;
    selectedFileToUpload.value = null; if (fileInputRef.value) fileInputRef.value.value = '';
    fetchFileList();
  } catch (error) {
    uploadError.value = `上传失败: ${error.response?.data?.error || error.message}`;
  } finally {
    isUploadingFile.value = false;
  }
}

// --- APK 安装方法 ---
function triggerApkFileInput() {
  selectedApkToInstall.value = null; apkInstallError.value = ''; apkInstallMessage.value = '';
  if (apkFileInputRef.value) { apkFileInputRef.value.value = ''; apkFileInputRef.value.click(); }
}
function handleApkFileSelect(event) {
  const file = event.target.files[0];
  if (file && file.name.toLowerCase().endsWith('.apk')) {
    selectedApkToInstall.value = file; apkInstallError.value = ''; apkInstallMessage.value = '';
  } else if (file) {
    selectedApkToInstall.value = null; apkInstallError.value = "请选择有效的 .apk 文件。";
  } else { selectedApkToInstall.value = null; }
}
async function installSelectedApk() {
  if (!selectedApkToInstall.value || !selectedDeviceId.value) {
    apkInstallError.value = !selectedApkToInstall.value ? "请选择 APK 文件。" : "请选择设备。"; return;
  }
  isInstallingApk.value = true; apkInstallMessage.value = `安装 ${selectedApkToInstall.value.name}...`; apkInstallError.value = '';
  uploadSuccessMessage.value = ''; uploadError.value = ''; fileListError.value = ''; goHomeMessage.value = ''; remoteInputStatus.value = '';
  clearLogcatMessage.value = ''; downloadLogcatMessage.value = ''; stopAppStatusMessage.value = ''; wakeUpMessage.value = '';

  const formData = new FormData();
  formData.append('apkFile', selectedApkToInstall.value);
  try {
    const response = await axios.post(
        `http://localhost:5679/api/apk/install/${selectedDeviceId.value}`, formData,
        { headers: { 'Content-Type': 'multipart/form-data' } }
    );
    apkInstallMessage.value = `APK 安装命令执行。\nADB 输出:\n${response.data.details || '无详细输出。'}`;
    if (response.data.details && response.data.details.toLowerCase().includes("success")) apkInstallMessage.value += "\n(安装成功!)";
    else apkInstallError.value = "警告：输出未明确包含 'Success'。";
    selectedApkToInstall.value = null; if (apkFileInputRef.value) apkFileInputRef.value.value = '';
  } catch (error) {
    let errorDetails = '';
    if (error.response && error.response.data) {
      errorDetails = `\n详情: ${error.response.data.details || ''}`;
      apkInstallError.value = `APK 安装失败: ${error.response.data.error || '未知错误'}${errorDetails}`;
    } else {
      apkInstallError.value = `APK 安装失败: ${error.message || '未知服务器或网络错误'}`;
    }
    apkInstallMessage.value = '';
  } finally {
    isInstallingApk.value = false;
  }
}

// --- 发送返回主页命令的方法 ---
async function sendGoHomeCommand() {
  if (!selectedDeviceId.value) { alert("请选择设备！"); return; }
  isSendingGoHome.value = true; goHomeMessage.value = '发送返回主页命令...';
  apkInstallMessage.value = ''; apkInstallError.value = ''; uploadSuccessMessage.value = ''; uploadError.value = ''; fileListError.value = ''; remoteInputStatus.value = '';
  clearLogcatMessage.value = ''; downloadLogcatMessage.value = ''; stopAppStatusMessage.value = ''; wakeUpMessage.value = '';
  try {
    const response = await axios.post(`http://localhost:5679/api/devices/${selectedDeviceId.value}/gohome`);
    goHomeMessage.value = response.data.message || "返回主页命令已发送。";
    setTimeout(() => { goHomeMessage.value = ''; }, 3000);
  } catch (error) {
    goHomeMessage.value = `错误: ${error.response?.data?.error || error.message}`;
  } finally {
    isSendingGoHome.value = false;
  }
}

// --- 发送唤醒屏幕命令的方法 ---
async function sendWakeUpCommand() {
  if (!selectedDeviceId.value) {
    alert("请先选择一个设备！");
    return;
  }
  isWakingUpDevice.value = true;
  wakeUpMessage.value = '正在发送唤醒命令...';
  apkInstallMessage.value = ''; apkInstallError.value = ''; uploadSuccessMessage.value = ''; uploadError.value = '';
  fileListError.value = ''; goHomeMessage.value = ''; remoteInputStatus.value = ''; appsListError.value = ''; uninstallStatusMessage.value = '';
  clearLogcatMessage.value = ''; downloadLogcatMessage.value = ''; stopAppStatusMessage.value = '';

  try {
    const response = await axios.post(`http://localhost:5679/api/devices/${selectedDeviceId.value}/wakeup`);
    wakeUpMessage.value = response.data.message || "唤醒命令已发送。";
    console.log("Wake up command response:", response.data);
    setTimeout(() => { wakeUpMessage.value = ''; }, 3000);
  } catch (error) {
    console.error("Error sending wake up command:", error);
    if (error.response && error.response.data && error.response.data.error) {
      wakeUpMessage.value = `错误: ${error.response.data.error} - ${error.response.data.details || ''}`;
    } else {
      wakeUpMessage.value = "发送唤醒命令失败。";
    }
  } finally {
    isWakingUpDevice.value = false;
  }
}


// --- 应用管理方法 ---
async function fetchInstalledApps() {
  if (!selectedDeviceId.value) { appsListError.value = "请先选择一个设备。"; return; }
  isLoadingApps.value = true;
  appsListError.value = ''; uninstallStatusMessage.value = ''; stopAppStatusMessage.value = '';
  installedApps.value = [];
  let params = {};
  if (appFilterOption.value && appFilterOption.value !== 'all') params.filter = appFilterOption.value;
  try {
    const response = await axios.get(`http://localhost:5679/api/apps/list/${selectedDeviceId.value}`, { params });
    installedApps.value = response.data.packages || [];
    if (installedApps.value.length === 0) appsListError.value = "未找到已安装的应用。";
  } catch (error) {
    appsListError.value = `加载应用列表失败: ${error.response?.data?.error || error.message}`;
  } finally {
    isLoadingApps.value = false;
  }
}
async function confirmAndUninstallApp(packageName) {
  if (!selectedDeviceId.value || !packageName) {
    uninstallStatusMessage.value = !selectedDeviceId.value ? "错误：未选择设备。" : "错误：未提供包名。"; return;
  }
  const confirmUninstall = confirm(`确定卸载应用 "${packageName}"？`);
  if (!confirmUninstall) {
    uninstallStatusMessage.value = "卸载已取消。"; setTimeout(() => uninstallStatusMessage.value = '', 3000); return;
  }
  uninstallingPackage.value = packageName; uninstallStatusMessage.value = `卸载 ${packageName}...`;
  stopAppStatusMessage.value = '';
  try {
    const response = await axios.post(
        `http://localhost:5679/api/apps/uninstall/${selectedDeviceId.value}`,
        { packageName: packageName, keepData: false }
    );
    uninstallStatusMessage.value = `卸载 "${packageName}" 命令执行。\nADB 输出:\n${response.data.details || '无详细输出。'}`;
    if (response.data.details && response.data.details.toLowerCase().includes("success")) {
      uninstallStatusMessage.value += "\n(卸载成功!)"; fetchInstalledApps();
    } else { uninstallStatusMessage.value += "\n警告：输出未明确包含 'Success'。"; }
  } catch (error) {
    uninstallStatusMessage.value = `卸载 "${packageName}" 失败: ${error.response?.data?.error || error.message}\n详情: ${error.response?.data?.details || ''}`;
  } finally {
    uninstallingPackage.value = null;
  }
}
async function confirmAndForceStopApp(packageName) {
  if (!selectedDeviceId.value || !packageName) {
    stopAppStatusMessage.value = !selectedDeviceId.value ? "错误：未选择设备。" : "错误：未提供包名。";
    return;
  }
  const confirmStop = confirm(`确定要强制停止应用 "${packageName}" 吗？`);
  if (!confirmStop) {
    stopAppStatusMessage.value = "停止操作已取消。";
    setTimeout(() => stopAppStatusMessage.value = '', 3000);
    return;
  }
  stoppingPackage.value = packageName;
  stopAppStatusMessage.value = `正在停止 ${packageName}...`;
  uninstallStatusMessage.value = '';
  try {
    const response = await axios.post(
        `http://localhost:5679/api/apps/stop/${selectedDeviceId.value}`,
        { packageName: packageName }
    );
    stopAppStatusMessage.value = `停止应用 "${packageName}" 命令已执行。\nADB 输出:\n${response.data.details || '通常无输出代表成功。'}`;
    if (response.data.details && (response.data.details.toLowerCase().includes("error") || response.data.details.toLowerCase().includes("failed"))) {
      stopAppStatusMessage.value += "\n(操作可能未成功，请检查ADB输出)";
    } else {
      stopAppStatusMessage.value += "\n(操作已发送)";
    }
  } catch (error) {
    stopAppStatusMessage.value = `停止应用 "${packageName}" 失败: ${error.response?.data?.error || error.message}\n详情: ${error.response?.data?.details || ''}`;
  } finally {
    stoppingPackage.value = null;
  }
}


// --- 远程文本输入方法 ---
function sendRemoteText() {
  if (!isMirroring.value || !socket || socket.readyState !== WebSocket.OPEN) {
    remoteInputStatus.value = "错误：屏幕镜像未连接。"; return;
  }
  if (remoteInputText.value.trim() === '') {
    remoteInputStatus.value = "请输入文本。"; return;
  }
  isSendingText.value = true;
  remoteInputStatus.value = `发送文本: "${remoteInputText.value}"...`;
  const textData = { type: "input_text", text: remoteInputText.value };
  socket.send(JSON.stringify(textData));
  isSendingText.value = false;
  setTimeout(() => { if(remoteInputStatus.value.startsWith('发送文本')) remoteInputStatus.value = '文本已发送'; }, 500);
  setTimeout(() => { if(remoteInputStatus.value === '文本已发送') remoteInputStatus.value = ''; }, 2000);
}
function sendRemoteEnterKey() {
  if (!isMirroring.value || !socket || socket.readyState !== WebSocket.OPEN) {
    remoteInputStatus.value = "错误：屏幕镜像未连接。"; return;
  }
  isSendingText.value = true;
  remoteInputStatus.value = '发送回车键...';
  const enterData = { type: "input_keyevent", keycode: "KEYCODE_ENTER" };
  socket.send(JSON.stringify(enterData));
  isSendingText.value = false;
  setTimeout(() => { if(remoteInputStatus.value.startsWith('发送回车键')) remoteInputStatus.value = '回车键已发送'; }, 500);
  setTimeout(() => { if(remoteInputStatus.value === '回车键已发送') remoteInputStatus.value = ''; }, 2000);
}

// --- Logcat 管理方法 ---
async function clearDeviceLogcat() {
  if (!selectedDeviceId.value) {
    clearLogcatMessage.value = "错误：请选择设备。"; return;
  }
  isClearingLogcat.value = true;
  clearLogcatMessage.value = "清除 Logcat 缓存...";
  apkInstallMessage.value = ''; apkInstallError.value = ''; uploadSuccessMessage.value = ''; uploadError.value = '';
  fileListError.value = ''; goHomeMessage.value = ''; remoteInputStatus.value = ''; appsListError.value = ''; uninstallStatusMessage.value = '';
  downloadLogcatMessage.value = ''; stopAppStatusMessage.value = ''; wakeUpMessage.value = '';
  try {
    const response = await axios.post(`http://localhost:5679/api/logcat/clear/${selectedDeviceId.value}`);
    clearLogcatMessage.value = response.data.message || "Logcat 缓存已清除。";
  } catch (error) {
    clearLogcatMessage.value = `清除 Logcat 失败: ${error.response?.data?.error || error.message}`;
  } finally {
    isClearingLogcat.value = false;
    setTimeout(() => { clearLogcatMessage.value = ''; }, 3000);
  }
}
async function downloadDeviceLogcat() {
  if (!selectedDeviceId.value) {
    downloadLogcatMessage.value = "错误：请选择设备。"; return;
  }
  isDownloadingLogcat.value = true;
  downloadLogcatMessage.value = "准备下载 Logcat 文件...";
  apkInstallMessage.value = ''; apkInstallError.value = ''; uploadSuccessMessage.value = ''; uploadError.value = '';
  fileListError.value = ''; goHomeMessage.value = ''; remoteInputStatus.value = ''; appsListError.value = ''; uninstallStatusMessage.value = '';
  clearLogcatMessage.value = ''; stopAppStatusMessage.value = ''; wakeUpMessage.value = '';
  const downloadUrl = `http://localhost:5679/api/logcat/download/${selectedDeviceId.value}`;
  try {
    const response = await axios({ url: downloadUrl, method: 'GET', responseType: 'blob' });
    let filename = `logcat_${selectedDeviceId.value.replace(/:/g, '_')}_${new Date().toISOString().slice(0,19).replace(/[-T:]/g,"")}.txt`;
    const disposition = response.headers['content-disposition'];
    if (disposition && disposition.indexOf('attachment') !== -1) {
      const filenameRegex = /filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/;
      const matches = filenameRegex.exec(disposition);
      if (matches != null && matches[1]) filename = matches[1].replace(/['"]/g, '');
    }
    const href = URL.createObjectURL(response.data);
    const link = document.createElement('a');
    link.href = href; link.setAttribute('download', filename);
    document.body.appendChild(link); link.click();
    document.body.removeChild(link); URL.revokeObjectURL(href);
    downloadLogcatMessage.value = `Logcat 文件 "${filename}" 下载开始。`;
  } catch (error) {
    let errorMsg = "下载 Logcat 文件失败: ";
    if (error.response) {
      if (error.response.data instanceof Blob && error.response.data.type === "application/json") {
        try {
          const errJsonText = await error.response.data.text();
          const errJson = JSON.parse(errJsonText);
          errorMsg += `${error.response.status} - ${errJson.error || '服务器返回未知错误'}`;
        } catch (parseError) {
          errorMsg += `${error.response.status} - ${error.response.statusText || '无法解析服务器错误响应'}`;
        }
      } else {
        errorMsg += `${error.response.status} - ${error.response.statusText || '服务器错误'}`;
      }
    } else if (error.request) { errorMsg += "网络错误。"; }
    else { errorMsg += error.message; }
    downloadLogcatMessage.value = errorMsg;
  } finally {
    isDownloadingLogcat.value = false;
    setTimeout(() => { downloadLogcatMessage.value = ''; }, 5000);
  }
}


// --- 生命周期函数 和 watch ---
onMounted(() => fetchDevices());
onUnmounted(() => { if (socket) stopScreenMirroring(); });
watch(selectedDeviceId, (newId, oldId) => {
  if (isMirroring.value && newId !== oldId && oldId !== null) {
    stopScreenMirroring();
  }
  if (newId !== oldId) {
    installedApps.value = []; appsListError.value = ''; uninstallStatusMessage.value = '';
    remoteInputText.value = ''; remoteInputStatus.value = '';
    clearLogcatMessage.value = ''; downloadLogcatMessage.value = '';
    stopAppStatusMessage.value = ''; wakeUpMessage.value = '';
  }
});
</script>

<template>
  <div style="position: fixed; top: 10px; right: 10px; background: rgba(238, 238, 238, 0.95); padding: 8px; border: 1px solid #ccc; z-index: 10000; font-size: 10px; max-width: 280px; word-break: break-all; border-radius: 4px; box-shadow: 0 2px 5px rgba(0,0,0,0.2); max-height: 90vh; overflow-y: auto;">
    <p style="margin:2px 0; font-weight: bold; border-bottom: 1px solid #ddd; padding-bottom: 3px; margin-bottom: 3px;">调试信息:</p>
    <p style="margin:2px 0;">Selected Dev: {{ selectedDeviceId || 'None' }}</p>
    <p style="margin:2px 0;">Mirroring: {{ isMirroring }}</p>
    <p style="margin:2px 0;">Remote Path: {{ currentRemotePath }}</p>
    <p style="margin:2px 0;">File Listing: {{ isFileListing }}</p>
    <p style="margin:2px 0;">Files Count: {{ fileList.length }}</p>
    <p style="margin:2px 0; color: sienna;">DevErr: {{ errorMessage || 'N' }}</p>
    <p style="margin:2px 0; color: sienna;">File List Err: {{ fileListError || 'N' }}</p>
    <p style="margin:2px 0;">Upload%: {{ uploadProgress }}</p>
    <p style="margin:2px 0; color: sienna;">UploadErr: {{ uploadError || 'N' }}</p>
    <p style="margin:2px 0; color: green;">UploadOK: {{ uploadSuccessMessage || 'N' }}</p>
    <p style="margin:2px 0; font-weight: bold;">Selected APK: {{ selectedApkToInstall?.name || 'N' }}</p>
    <p style="margin:2px 0;">InstallingAPK: {{ isInstallingApk }}</p>
    <p style="margin:2px 0; color: green;">InstallOK: {{ apkInstallMessage || 'N' }}</p>
    <p style="margin:2px 0; color: sienna;">InstallErr: {{ apkInstallError || 'N' }}</p>
    <p style="margin:2px 0; color: blue;">GoHomeMsg: {{ goHomeMessage || 'N' }}</p>
    <p style="margin:2px 0; color: darkgoldenrod;">WakeUpMsg: {{ wakeUpMessage || 'N' }}</p>
    <p style="margin:2px 0;">LoadingApps: {{ isLoadingApps }}</p>
    <p style="margin:2px 0;">AppsCount: {{ installedApps.length }}</p>
    <p style="margin:2px 0; color: sienna;">AppsErr: {{ appsListError || 'N' }}</p>
    <p style="margin:2px 0; color: darkmagenta;">UninstallMsg: {{ uninstallStatusMessage || 'N' }}</p>
    <p style="margin:2px 0; color: orangered;">StopAppMsg: {{ stopAppStatusMessage || 'N' }}</p>
    <p style="margin:2px 0; color: teal;">RemoteInputStatus: {{ remoteInputStatus || 'N' }}</p>
    <p style="margin:2px 0; color: indigo;">ClearLogcat: {{ clearLogcatMessage || 'N' }}</p>
    <p style="margin:2px 0; color: indigo;">DownloadLogcat: {{ downloadLogcatMessage || 'N' }}</p>
  </div>

  <div class="phone-page">
    <header>
      <h1>手机连接与管理</h1>
      <button @click="fetchDevices" :disabled="isLoading || isMirroring || isUploadingFile || isInstallingApk || isSendingGoHome || isWakingUpDevice || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat" class="refresh-btn">
        {{ isLoading ? '刷新中...' : '刷新设备列表' }}
      </button>
    </header>

    <section v-if="isLoading" class="loading-section"><p>正在加载设备列表...</p></section>
    <section v-if="errorMessage && !isLoading && !isMirroring" class="error-section global-error-message"><p class="error-message">{{ errorMessage }}</p></section>
    <section v-if="!isLoading && devices.length > 0" class="devices-list-section">
      <h2>选择一个设备进行操作:</h2>
      <ul class="device-list">
        <li
            v-for="device in devices" :key="device.id" class="device-item"
            :class="{ 'selected': device.id === selectedDeviceId }"
            @click="selectDevice(device.id)">
          <span>ID: {{ device.id }}</span>
          <span :class="{ 'status-device-ok': device.status === 'device' }">状态: {{ device.status }}</span>
        </li>
      </ul>
    </section>
    <section v-if="!isLoading && !errorMessage && devices.length === 0 && !selectedDeviceId" class="no-devices-section">
      <p>未检测到已连接的设备，请检查USB连接和手机USB调试模式。</p>
    </section>

    <div v-if="selectedDeviceId" class="selected-device-actions">
      <hr class="section-divider">
      <h3>当前选中设备: {{ selectedDeviceId }}</h3>

      <section class="action-section screen-mirror-section">
        <h4>屏幕镜像</h4>
        <div class="mirror-controls">
          <button @click="startScreenMirroring" :disabled="isMirroring || !selectedDeviceId || isUploadingFile || isInstallingApk || isSendingGoHome || isWakingUpDevice || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat" class="control-btn start-btn">开始屏幕镜像</button>
          <button @click="stopScreenMirroring" :disabled="!isMirroring" class="control-btn stop-btn">停止屏幕镜像</button>
        </div>
        <div v-if="isMirroring" class="remote-input-area">
          <input
              type="text"
              v-model="remoteInputText"
              placeholder="在此输入文本同步到手机"
              @keyup.enter="sendRemoteText"
              :disabled="isSendingText || !isMirroring"
          />
          <button @click="sendRemoteText" class="control-btn send-text-btn" :disabled="isSendingText || !isMirroring">发送文本</button>
          <button @click="sendRemoteEnterKey" class="control-btn send-enter-btn" :disabled="isSendingText || !isMirroring">发送回车</button>
        </div>
        <p v-if="remoteInputStatus" class="status-message remote-input-feedback" :class="{'error': remoteInputStatus.toLowerCase().includes('错误')}">{{ remoteInputStatus }}</p>
        <div v-if="isMirroring && errorMessage && !fileListError && !uploadError && !apkInstallError && !goHomeMessage && !wakeUpMessage && !appsListError && !uninstallStatusMessage && !remoteInputStatus && !clearLogcatMessage && !downloadLogcatMessage && !stopAppStatusMessage" class="error-section mirror-error">
          <p class="error-message">{{ errorMessage }}</p>
        </div>
        <div v-if="isMirroring" class="mirror-display-area">
          <canvas ref="screenCanvasRef" class="mirrored-screen-canvas" title="点击或拖拽此处可在手机上模拟操作"></canvas>
          <p v-if="!canvasCtx && isMirroring">...</p> </div>
      </section>

      <hr class="section-divider">

      <section class="action-section device-controls-section">
        <h4>设备控制</h4>
        <button
            @click="sendGoHomeCommand"
            class="control-btn go-home-btn"
            :disabled="isSendingGoHome || !selectedDeviceId || isUploadingFile || isInstallingApk || isWakingUpDevice || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat"
        >
          {{ isSendingGoHome ? '处理中...' : '返回手机主页' }}
        </button>
        <button
            @click="sendWakeUpCommand"
            class="control-btn wakeup-btn"
            :disabled="isWakingUpDevice || !selectedDeviceId || isUploadingFile || isInstallingApk || isSendingGoHome || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat"
        >
          {{ isWakingUpDevice ? '唤醒中...' : '唤醒屏幕' }}
        </button>
        <p v-if="goHomeMessage" class="status-message" :class="{'error': goHomeMessage.toLowerCase().includes('错误') || goHomeMessage.toLowerCase().includes('失败')}">
          {{ goHomeMessage }}
        </p>
        <p v-if="wakeUpMessage" class="status-message" :class="{'error': wakeUpMessage.toLowerCase().includes('错误') || wakeUpMessage.toLowerCase().includes('失败')}">
          {{ wakeUpMessage }}
        </p>
      </section>

      <hr class="section-divider">

      <section class="action-section file-browser-section">
        <h4>文件浏览器 & 文件上传</h4>
        <div class="path-navigation">
          <input type="text" v-model="currentRemotePath" @keyup.enter="fetchFileList" placeholder="输入设备路径" :disabled="isFileListing || isUploadingFile || isInstallingApk || isSendingGoHome || isWakingUpDevice || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat || !selectedDeviceId" />
          <button @click="fetchFileList" :disabled="isFileListing || isUploadingFile || isInstallingApk || isSendingGoHome || isWakingUpDevice || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat || !selectedDeviceId" class="control-btn">
            {{ isFileListing ? '加载中...' : '转到路径' }}
          </button>
          <button @click="navigateUp" :disabled="!canNavigateUp || isFileListing || isUploadingFile || isInstallingApk || isSendingGoHome || isWakingUpDevice || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat || !selectedDeviceId" class="control-btn up-btn">
            返回上一级
          </button>
        </div>

        <div class="file-upload-area">
          <input
              type="file"
              @change="handleFileSelect"
              ref="fileInputRef"
              style="display: none;"
              :disabled="isUploadingFile || !selectedDeviceId || isInstallingApk || isSendingGoHome || isWakingUpDevice || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat"
          />
          <button @click="triggerFileInput" class="control-btn choose-file-btn" :disabled="isUploadingFile || !selectedDeviceId || isInstallingApk || isSendingGoHome || isWakingUpDevice || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat">
            选择文件上传
          </button>
          <span v-if="selectedFileToUpload" class="selected-file-name">
            已选: {{ selectedFileToUpload.name }} ({{ (selectedFileToUpload.size / 1024).toFixed(2) }} KB)
          </span>
          <button
              v-if="selectedFileToUpload"
              @click="uploadSelectedFile"
              class="control-btn upload-btn"
              :disabled="isUploadingFile || !selectedDeviceId || isInstallingApk || isSendingGoHome || isWakingUpDevice || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat"
          >
            {{ isUploadingFile ? `上传中... ${uploadProgress}%` : '上传到当前目录' }}
          </button>
        </div>
        <div v-if="isUploadingFile" class="upload-progress-bar-container">
          <div class="upload-progress-bar" :style="{ width: uploadProgress + '%' }">
            {{ uploadProgress }}%
          </div>
        </div>
        <div v-if="uploadSuccessMessage" class="success-message upload-status-message">
          {{ uploadSuccessMessage }}
        </div>
        <div v-if="uploadError" class="error-message upload-status-message">
          {{ uploadError }}
        </div>

        <div v-if="isFileListing" class="loading-section"><p>正在加载文件列表...</p></div>
        <div v-if="fileListError && !isFileListing" class="error-section"><p class="error-message">{{ fileListError }}</p></div>

        <div v-if="!isFileListing && !fileListError && fileList.length > 0" class="file-list-container">
          <ul>
            <li
                v-for="item in fileList"
                :key="item.name + (item.isDir ? '/' : '')"  class="file-item"
                :class="{ 'is-dir': item.isDir }"
                @click="navigateTo(item)"
                :title="item.isDir ? `进入目录: ${item.name}` : `文件: ${item.name} (点击下载)`"
            >
              <span class="file-icon">{{ item.isDir ? '📁' : '📄' }}</span>
              <span class="file-name">{{ item.name }}</span>
            </li>
          </ul>
        </div>
        <div v-if="!isFileListing && !fileListError && fileList.length === 0 && currentRemotePath && selectedDeviceId && !errorMessage" class="no-files-section">
          <p>目录 “{{ currentRemotePath }}” 为空或无法访问。</p>
        </div>
      </section>

      <hr class="section-divider">

      <section class="action-section apk-installer-section">
        <h4>APK 安装器</h4>
        <div class="apk-install-area">
          <input
              type="file"
              @change="handleApkFileSelect"
              ref="apkFileInputRef"
              style="display: none;"
              accept=".apk"
              :disabled="isInstallingApk || !selectedDeviceId || isUploadingFile || isSendingGoHome || isWakingUpDevice || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat"
          />
          <button @click="triggerApkFileInput" class="control-btn choose-file-btn" :disabled="isInstallingApk || !selectedDeviceId || isUploadingFile || isSendingGoHome || isWakingUpDevice || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat">
            选择本地 APK 文件
          </button>
          <span v-if="selectedApkToInstall" class="selected-file-name">
                已选: {{ selectedApkToInstall.name }} ({{ (selectedApkToInstall.size / 1024 / 1024).toFixed(2) }} MB)
             </span>
          <button
              v-if="selectedApkToInstall"
              @click="installSelectedApk"
              class="control-btn install-apk-btn"
              :disabled="isInstallingApk || !selectedDeviceId || isUploadingFile || isSendingGoHome || isWakingUpDevice || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat"
          >
            {{ isInstallingApk ? '正在安装...' : '安装选中的 APK' }}
          </button>
        </div>
        <div v-if="apkInstallMessage" class="success-message install-status-message">
          <pre>{{ apkInstallMessage }}</pre>
        </div>
        <div v-if="apkInstallError" class="error-message install-status-message">
          <pre>{{ apkInstallError }}</pre>
        </div>
      </section>

      <hr class="section-divider">

      <section class="action-section app-management-section">
        <h4>应用管理</h4>
        <div class="app-list-controls">
          <label for="appFilter">过滤应用: </label>
          <select id="appFilter" v-model="appFilterOption" @change="fetchInstalledApps" :disabled="isLoadingApps || !selectedDeviceId || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat">
            <option value="">所有应用</option>
            <option value="third_party">仅第三方应用</option>
          </select>
          <button @click="fetchInstalledApps" class="control-btn" :disabled="isLoadingApps || !selectedDeviceId || uninstallingPackage || stoppingPackage || isSendingText || isClearingLogcat || isDownloadingLogcat">
            {{ isLoadingApps ? '加载中...' : '加载应用列表' }}
          </button>
        </div>

        <div v-if="isLoadingApps" class="loading-section"><p>正在加载应用列表...</p></div>
        <div v-if="appsListError && !isLoadingApps" class="error-section"><p class="error-message">{{ appsListError }}</p></div>

        <div v-if="uninstallStatusMessage || stopAppStatusMessage"
             class="status-message operation-feedback"
             :class="{'error': (uninstallStatusMessage && (uninstallStatusMessage.toLowerCase().includes('失败') || uninstallStatusMessage.toLowerCase().includes('错误'))) || (stopAppStatusMessage && (stopAppStatusMessage.toLowerCase().includes('失败') || stopAppStatusMessage.toLowerCase().includes('错误')))}">
          <pre v-if="uninstallStatusMessage">{{ uninstallStatusMessage }}</pre>
          <pre v-if="stopAppStatusMessage">{{ stopAppStatusMessage }}</pre>
        </div>


        <div v-if="!isLoadingApps && !appsListError && installedApps.length > 0" class="installed-apps-container">
          <h5>已安装应用包名 ({{ installedApps.length }}):</h5>
          <ul>
            <li v-for="pkg in installedApps" :key="pkg" class="app-item">
              <span class="app-package-name">{{ pkg }}</span>
              <div class="app-item-actions">
                <button
                    @click="confirmAndForceStopApp(pkg)"
                    class="control-btn stop-app-btn"
                    :disabled="stoppingPackage === pkg || uninstallingPackage || !selectedDeviceId || isSendingText || isClearingLogcat || isDownloadingLogcat"
                >
                  {{ stoppingPackage === pkg ? '停止中...' : '停止' }}
                </button>
                <button
                    @click="confirmAndUninstallApp(pkg)"
                    class="control-btn uninstall-app-btn"
                    :disabled="uninstallingPackage === pkg || stoppingPackage || !selectedDeviceId || isSendingText || isClearingLogcat || isDownloadingLogcat"
                >
                  {{ uninstallingPackage === pkg ? '卸载中...' : '卸载' }}
                </button>
              </div>
            </li>
          </ul>
        </div>
        <div v-if="!isLoadingApps && !appsListError && installedApps.length === 0 && selectedDeviceId" class="no-apps-section">
          <p>未找到已安装的应用 (或符合当前过滤条件的应用)。</p>
        </div>
      </section>

      <hr class="section-divider">

      <section class="action-section logcat-management-section">
        <h4>Logcat 管理</h4>
        <div class="logcat-controls">
          <button
              @click="clearDeviceLogcat"
              class="control-btn clear-logcat-btn"
              :disabled="isClearingLogcat || !selectedDeviceId || isDownloadingLogcat || isUploadingFile || isInstallingApk || isSendingGoHome || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText"
          >
            {{ isClearingLogcat ? '清除中...' : '清除 Logcat 缓存' }}
          </button>
          <button
              @click="downloadDeviceLogcat"
              class="control-btn download-logcat-btn"
              :disabled="isDownloadingLogcat || !selectedDeviceId || isClearingLogcat || isUploadingFile || isInstallingApk || isSendingGoHome || isLoadingApps || uninstallingPackage || stoppingPackage || isSendingText"
          >
            {{ isDownloadingLogcat ? '准备下载...' : '下载当前 Logcat' }}
          </button>
        </div>
        <div v-if="clearLogcatMessage" class="status-message logcat-status-message" :class="{'error': clearLogcatMessage.toLowerCase().includes('失败') || clearLogcatMessage.toLowerCase().includes('错误')}">
          {{ clearLogcatMessage }}
        </div>
        <div v-if="downloadLogcatMessage" class="status-message logcat-status-message" :class="{'error': downloadLogcatMessage.toLowerCase().includes('失败') || downloadLogcatMessage.toLowerCase().includes('错误')}">
          {{ downloadLogcatMessage }}
        </div>
      </section>
    </div>

    <nav class="navigation">
      <router-link to="/">返回主页</router-link>
    </nav>
  </div>
</template>

<style scoped>
/* 基本页面和头部 */
.phone-page { max-width: 850px; margin: 20px auto; padding: 25px; background-color: #fcfdff; border-radius: 10px; box-shadow: 0 4px 15px rgba(0,0,0,0.08); font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; }
.phone-page header { display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid #e9ecef; padding-bottom: 18px; margin-bottom: 25px; }
.phone-page header h1 { margin: 0; font-size: 2em; color: #2c3e50; }
.refresh-btn { padding: 9px 18px; background-color: #007bff; color: white; border: none; border-radius: 5px; cursor: pointer; transition: background-color 0.2s; font-size: 0.95em; }
.refresh-btn:disabled { background-color: #ced4da; cursor: not-allowed; }
.refresh-btn:not(:disabled):hover { background-color: #0056b3; }

/* 通用状态显示 */
.loading-section p { text-align: center; padding: 25px; font-size: 1.1em; color: #6c757d; }
.error-section.global-error-message { margin-bottom: 20px; }
.error-section p.error-message,
.mirror-error p.error-message,
.upload-status-message.error-message,
.install-status-message.error-message,
.status-message.error,
.uninstall-feedback.error,
.remote-input-feedback.error,
.logcat-status-message.error,
.operation-feedback.error {
  color: #721c24; background-color: #f8d7da; border: 1px solid #f5c6cb;
  padding: 12px 15px; border-radius: 5px; text-align: left; margin-top: 12px; word-break: break-word; font-size: 0.95em;
}
.success-message.upload-status-message,
.success-message.install-status-message,
.status-message:not(.error),
.uninstall-feedback:not(.error),
.remote-input-feedback:not(.error),
.logcat-status-message:not(.error),
.operation-feedback:not(.error) {
  color: #155724; background-color: #d4edda; border: 1px solid #c3e6cb;
  padding: 12px 15px; border-radius: 5px; text-align: left; margin-top: 12px; word-break: break-word; font-size: 0.95em;
}
.status-message:not(.error) {
  color: #004085;
  background-color: #cce5ff;
  border: 1px solid #b8daff;
}
.install-status-message pre, .uninstall-feedback pre, .operation-feedback pre {
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.9em;
  max-height: 150px;
  overflow-y: auto;
  background-color: rgba(0,0,0,0.03);
  padding: 8px;
  margin-top: 5px;
  border-radius: 3px;
}


/* 设备列表 */
.devices-list-section h2 { margin-top: 0; margin-bottom: 12px; color: #495057; font-size: 1.3em; text-align: left; }
.no-devices-section p { text-align: center; padding: 18px; background-color: #fff3cd; border: 1px solid #ffeeba; color: #856404; border-radius: 5px; }
.device-list { list-style-type: none; padding: 0; margin-bottom: 25px; }
.device-item { background-color: #f8f9fa; border: 1px solid #dee2e6; padding: 14px 18px; margin-bottom: 10px; border-radius: 5px; display: flex; justify-content: space-between; align-items: center; cursor: pointer; transition: background-color 0.2s, border-color 0.2s; }
.device-item:hover { background-color: #e9ecef; border-color: #ced4da; }
.device-item.selected { background-color: #cce5ff; border-left: 5px solid #007bff; font-weight: bold; }
.device-item span { font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, Courier, monospace; color: #212529; }
.device-item span.status-device-ok { color: #28a745; font-weight: bold; }

/* 选中设备后的操作区 */
.selected-device-actions { margin-top: 25px; padding-top: 25px; }
.selected-device-actions > h3 { text-align: center; color: #2c3e50; margin-bottom: 25px; font-size: 1.5em; }
.section-divider { border: none; border-top: 1px dashed #ced4da; margin: 30px 0; }
.action-section { margin-bottom: 30px; padding: 22px; border: 1px solid #e9ecef; border-radius: 8px; background-color: #ffffff; box-shadow: 0 1px 3px rgba(0,0,0,0.05); }
.action-section h4 { margin-top: 0; margin-bottom: 18px; color: #007bff; border-bottom: 1px solid #f1f3f5; padding-bottom: 12px; font-size: 1.4em; text-align: left; }

/* 通用控制按钮 */
.control-btn { padding: 9px 16px; margin: 5px; border: none; border-radius: 5px; cursor: pointer; font-size: 0.95em; transition: background-color 0.2s, opacity 0.2s; line-height: 1.5; }
.control-btn:disabled { opacity: 0.65; cursor: not-allowed; }

/* 设备控制区域 */
.device-controls-section { text-align: center; }
.device-controls-section .control-btn { margin: 5px 8px;} /* 调整设备控制按钮间距 */
.go-home-btn { background-color: #f0ad4e; color: white; }
.go-home-btn:not(:disabled):hover { background-color: #ec971f; }
.wakeup-btn { background-color: #5bc0de; color: white; }
.wakeup-btn:not(:disabled):hover { background-color: #31b0d5; }
.status-message { margin-top: 10px; font-size: 0.9em; padding: 8px; border-radius: 4px; }


/* 屏幕镜像 */
.screen-mirror-section .mirror-controls { margin-bottom: 10px; text-align: center; }
.screen-mirror-section .start-btn { background-color: #28a745; color: white; }
.screen-mirror-section .start-btn:not(:disabled):hover { background-color: #218838; }
.screen-mirror-section .stop-btn { background-color: #dc3545; color: white; }
.screen-mirror-section .stop-btn:not(:disabled):hover { background-color: #c82333; }

/* 远程文本输入区域样式 */
.remote-input-area {
  display: flex;
  gap: 10px;
  margin-top: 10px;
  margin-bottom: 10px;
  align-items: center;
}
.remote-input-area input[type="text"] {
  flex-grow: 1;
  padding: 8px 10px;
  border: 1px solid #ced4da;
  border-radius: 4px;
  font-size: 0.95em;
}
.remote-input-area .send-text-btn {
  background-color: #17a2b8;
  color: white;
  padding: 8px 12px;
  font-size: 0.9em;
}
.remote-input-area .send-text-btn:not(:disabled):hover {
  background-color: #138496;
}
.remote-input-area .send-enter-btn {
  background-color: #6c757d;
  color: white;
  padding: 8px 12px;
  font-size: 0.9em;
}
.remote-input-area .send-enter-btn:not(:disabled):hover {
  background-color: #5a6268;
}
.remote-input-feedback {
  font-size: 0.85em;
  margin-top: 5px;
  text-align: center;
}


.screen-mirror-section .mirror-display-area { margin-top: 15px; padding: 10px; border: 1px dashed #ced4da; min-height: 220px; display: flex; justify-content: center; align-items: center; background-color: #f8f9fa; }
.screen-mirror-section .mirrored-screen-canvas { max-width: 100%; max-height: 480px; border: 1px solid #dee2e6; display: block; margin: auto; background-color: #000; cursor: pointer; }
.screen-mirror-section .mirror-display-area p { color: #6c757d; }

/* 文件浏览 */
.file-browser-section .path-navigation { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; margin-bottom: 18px; }
.file-browser-section .path-navigation input[type="text"] { flex-grow: 1; min-width: 220px; padding: 9px 12px; border: 1px solid #ced4da; border-radius: 5px; font-size: 0.95em; }
.file-browser-section .path-navigation .control-btn { padding: 9px 14px; font-size: 0.9em; white-space: nowrap; }
.file-browser-section .path-navigation .up-btn { background-color: #6c757d; color:white; }
.file-browser-section .path-navigation .up-btn:not(:disabled):hover { background-color: #5a6268; }

/* 文件上传 */
.file-upload-area { margin-top: 18px; margin-bottom: 18px; padding: 12px; background-color: #f0f3f5; border: 1px dashed #ced4da; border-radius: 5px; display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.file-upload-area .choose-file-btn { background-color: #17a2b8; color: white; }
.file-upload-area .choose-file-btn:not(:disabled):hover { background-color: #138496; }
.file-upload-area .upload-btn { background-color: #28a745; color: white; }
.file-upload-area .upload-btn:not(:disabled):hover { background-color: #218838; }
.selected-file-name { font-style: italic; color: #495057; font-size: 0.9em; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 220px; flex-shrink: 1; }
.upload-progress-bar-container { width: 100%; background-color: #e9ecef; border-radius: 5px; margin-bottom: 12px; height: 22px; overflow: hidden; }
.upload-progress-bar { width: 0%; height: 100%; background-color: #007bff; color: white; text-align: center; line-height: 22px; font-size: 0.85em; transition: width 0.3s ease-out; }

/* APK 安装 */
.apk-installer-section { margin-top: 20px; }
.apk-install-area {
  margin-bottom: 15px;
  padding: 12px;
  background-color: #fff9e6;
  border: 1px dashed #ffecb3;
  border-radius: 5px;
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.apk-install-area .choose-file-btn {
  background-color: #fd7e14;
  color: white;
}
.apk-install-area .choose-file-btn:not(:disabled):hover {
  background-color: #e67e22;
}
.apk-install-area .install-apk-btn {
  background-color: #ffc107;
  color: #212529;
}
.apk-install-area .install-apk-btn:not(:disabled):hover {
  background-color: #e0a800;
}

/* 应用管理 */
.app-management-section .app-list-controls {
  display: flex;
  align-items: center;
  gap: 15px;
  margin-bottom: 15px;
  flex-wrap: wrap;
}
.app-management-section .app-list-controls label {
  font-weight: 500;
  color: #495057;
}
.app-management-section .app-list-controls select {
  padding: 8px 10px;
  border-radius: 4px;
  border: 1px solid #ced4da;
  background-color: white;
  font-size: 0.9em;
  color: #212529;
}
.app-management-section .app-list-controls .control-btn {
  background-color: #6f42c1;
  color: white;
}
.app-management-section .app-list-controls .control-btn:not(:disabled):hover {
  background-color: #5a2d9e;
}
.installed-apps-container {
  margin-top: 15px;
}
.installed-apps-container h5 {
  margin-bottom: 10px;
  color: #333;
  font-size: 1.1em;
}
.installed-apps-container ul {
  list-style-type: none;
  padding: 0;
  max-height: 300px;
  overflow-y: auto;
  border: 1px solid #dee2e6;
  border-radius: 5px;
}
.app-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px solid #f1f3f5;
  font-size: 0.9em;
}
.app-item:last-child {
  border-bottom: none;
}
.app-item-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}
.app-package-name {
  color: #212529;
  word-break: break-all;
  margin-right: 10px;
  flex-grow: 1;
}
.stop-app-btn {
  background-color: #ffc107;
  color: #212529;
  padding: 6px 10px;
  font-size: 0.85em;
}
.stop-app-btn:not(:disabled):hover {
  background-color: #e0a800;
}
.uninstall-app-btn {
  background-color: #e74c3c;
  color: white;
  padding: 6px 10px;
  font-size: 0.85em;
}
.uninstall-app-btn:not(:disabled):hover {
  background-color: #c0392b;
}
.operation-feedback {
  margin-top: 10px;
}
.no-apps-section p {
  text-align: center;
  padding: 15px;
  background-color: #f8f9fa;
  border: 1px solid #dee2e6;
  color: #495057;
  border-radius: 5px;
  margin-top: 10px;
}

/* Logcat 管理部分样式 */
.logcat-management-section {}
.logcat-controls {
  display: flex;
  gap: 10px;
  margin-bottom: 10px;
  flex-wrap: wrap;
  justify-content: center;
}
.clear-logcat-btn {
  background-color: #6c757d;
  color: white;
}
.clear-logcat-btn:not(:disabled):hover {
  background-color: #5a6268;
}
.download-logcat-btn {
  background-color: #17a2b8;
  color: white;
}
.download-logcat-btn:not(:disabled):hover {
  background-color: #138496;
}
.logcat-status-message {
  margin-top: 10px;
  text-align: center;
}


/* 文件列表容器 */
.file-browser-section .file-list-container ul { list-style-type: none; padding: 0; max-height: 380px; overflow-y: auto; border: 1px solid #dee2e6; border-radius: 5px; background-color: #ffffff; margin-top: 15px; }
.file-browser-section .file-item { display: flex; align-items: center; padding: 11px 14px; border-bottom: 1px solid #f1f3f5; cursor: pointer; transition: background-color 0.2s; }
.file-browser-section .file-item:last-child { border-bottom: none; }
.file-browser-section .file-item:hover { background-color: #e9f5ff; }
.file-browser-section .file-item.is-dir .file-name { font-weight: 600; color: #0056b3; }
.file-browser-section .file-icon { margin-right: 12px; font-size: 1.25em; color: #6c757d; }
.file-browser-section .file-item.is-dir .file-icon { color: #007bff; }
.file-browser-section .file-name { word-break: break-all; color: #212529; font-size: 0.95em; }
.file-browser-section .no-files-section p { text-align: center; padding: 18px; background-color: #f8f9fa; border: 1px solid #dee2e6; color: #495057; border-radius: 5px; margin-top: 12px; }

/* 底部导航 */
.navigation { margin-top: 35px; text-align: center; padding-top: 22px; border-top: 1px solid #e9ecef; }
.navigation a { color: #007bff; text-decoration: none; font-size: 1.05em; }
.navigation a:hover { text-decoration: underline; }
</style>