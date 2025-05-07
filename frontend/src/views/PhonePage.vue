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

// --- 文件浏览相关状态 ---
const currentRemotePath = ref('/sdcard/');
const fileList = ref([]);
const isFileListing = ref(false);
const fileListError = ref('');

// --- 文件上传相关状态 ---
const fileInputRef = ref(null);
const selectedFileToUpload = ref(null);
const isUploadingFile = ref(false);
const uploadProgress = ref(0);
const uploadError = ref('');
const uploadSuccessMessage = ref('');

// --- 获取设备列表 ---
async function fetchDevices() {
  console.log('[fetchDevices] Fetching device list...');
  isLoading.value = true;
  errorMessage.value = '';
  devices.value = [];
  if (isMirroring.value) stopScreenMirroring();
  selectedDeviceId.value = null;

  fileList.value = [];
  currentRemotePath.value = '/sdcard/';
  fileListError.value = '';
  uploadError.value = '';
  uploadSuccessMessage.value = '';

  try {
    const response = await axios.get('http://localhost:5679/api/devices');
    devices.value = response.data || [];
    if (devices.value.length === 0) {
      errorMessage.value = "未检测到已连接的设备。";
    }
  } catch (error) {
    console.error("[fetchDevices] Error:", error);
    errorMessage.value = `加载设备列表失败: ${error.message}`;
  } finally {
    isLoading.value = false;
  }
}

// --- 选择设备 ---
function selectDevice(deviceId) {
  console.log('[selectDevice] Called with:', deviceId);
  if (isMirroring.value) stopScreenMirroring();

  selectedDeviceId.value = deviceId;

  fileList.value = [];
  currentRemotePath.value = '/sdcard/';
  fileListError.value = '';
  uploadError.value = '';
  uploadSuccessMessage.value = '';

  if (deviceId) {
    fetchFileList();
  }
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

  socket.onopen = () => {
    if (screenCanvasRef.value) canvasCtx = screenCanvasRef.value.getContext('2d');
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
    }
  };
  socket.onerror = (error) => {
    console.error("Mirror WebSocket error:", error);
    errorMessage.value = "屏幕镜像连接发生错误。";
  };
  socket.onclose = (event) => {
    isMirroring.value = false;
    if (canvasCtx && screenCanvasRef.value) canvasCtx.clearRect(0, 0, screenCanvasRef.value.width, screenCanvasRef.value.height);
    canvasCtx = null;
    socket = null;
    if (event.code !== 1000 && !event.wasClean && !errorMessage.value) {
      errorMessage.value = `屏幕镜像连接意外断开 (Code: ${event.code})`;
    }
  };
}
function stopScreenMirroring() { if (socket) socket.close(1000, "User stop"); }

// --- 文件浏览方法 ---
async function fetchFileList() {
  if (!selectedDeviceId.value) { fileListError.value = "请先选择设备。"; return; }
  isFileListing.value = true;
  fileListError.value = '';
  uploadSuccessMessage.value = '';
  uploadError.value = '';

  try {
    const response = await axios.get(`http://localhost:5679/api/devices/${selectedDeviceId.value}/files`, {
      params: { path: currentRemotePath.value }
    });
    currentRemotePath.value = response.data.path || currentRemotePath.value;
    fileList.value = response.data.files || [];
  } catch (error) {
    console.error("Fetch file list error:", error);
    fileListError.value = `加载文件列表失败 (${currentRemotePath.value}): ${error.response?.data?.error || error.message}`;
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
  if (!selectedDeviceId.value) { alert("错误：没有选中的设备。"); return; }
  if (item.isDir) { alert("错误：不能下载文件夹。"); return; }

  let fullRemotePath = currentRemotePath.value;
  if (fullRemotePath !== '/' && !fullRemotePath.endsWith('/')) fullRemotePath += '/';
  fullRemotePath += item.name;

  fileListError.value = `正在准备下载 ${item.name}...`;
  const downloadUrl = `http://localhost:5679/api/devices/${selectedDeviceId.value}/files/download?filePath=${encodeURIComponent(fullRemotePath)}`;

  try {
    const response = await axios({ url: downloadUrl, method: 'GET', responseType: 'blob' });
    const href = URL.createObjectURL(response.data);
    const link = document.createElement('a');
    link.href = href;
    link.setAttribute('download', item.name);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(href);
    fileListError.value = `${item.name} 下载已开始。`;
    setTimeout(() => { if (fileListError.value === `${item.name} 下载已开始。`) fileListError.value = ''; }, 3000);
  } catch (error) {
    console.error("Download error:", error);
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
    } else if (error.request) {
      errorMsg += "网络错误或无法连接到服务器。";
    } else {
      errorMsg += error.message;
    }
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
  selectedFileToUpload.value = null;
  uploadError.value = '';
  uploadSuccessMessage.value = '';
  uploadProgress.value = 0;
  if (fileInputRef.value) {
    fileInputRef.value.value = '';
    fileInputRef.value.click();
  }
}

function handleFileSelect(event) {
  const file = event.target.files[0];
  if (file) {
    selectedFileToUpload.value = file;
    uploadError.value = '';
    uploadSuccessMessage.value = '';
  } else {
    selectedFileToUpload.value = null;
  }
}

async function uploadSelectedFile() {
  if (!selectedFileToUpload.value) {
    uploadError.value = "请先选择文件。";
    return;
  }
  if (!selectedDeviceId.value) {
    uploadError.value = "请先选择设备。";
    return;
  }
  if (!currentRemotePath.value || !currentRemotePath.value.endsWith('/')) {
    uploadError.value = "当前远程路径无效。";
    return;
  }

  isUploadingFile.value = true;
  uploadProgress.value = 0;
  uploadError.value = '';
  uploadSuccessMessage.value = '';

  const formData = new FormData();
  formData.append('file', selectedFileToUpload.value);
  formData.append('remoteDirPath', currentRemotePath.value);

  try {
    const response = await axios.post(
        `http://localhost:5679/api/devices/${selectedDeviceId.value}/files/upload`,
        formData,
        {
          headers: {'Content-Type': 'multipart/form-data'},
          onUploadProgress: (e) => {
            if (e.total) uploadProgress.value = Math.round((e.loaded * 100) / e.total);
          },
        }
    );
    uploadSuccessMessage.value = `文件 "${response.data.filename}" 成功上传到 "${response.data.filePath}"。`;
    // 移除了设置 lastUploadedApkInfo 的逻辑
    selectedFileToUpload.value = null;
    if (fileInputRef.value) fileInputRef.value.value = '';
    fetchFileList();
  } catch (error) {
    console.error("Upload error:", error);
    uploadError.value = `上传失败: ${error.response?.data?.error || error.message}`;
  } finally {
    isUploadingFile.value = false;
  }
}

// --- 生命周期函数 和 watch ---
onMounted(() => fetchDevices());
onUnmounted(() => {
  if (socket) stopScreenMirroring();
});
watch(selectedDeviceId, (newId, oldId) => {
  if (isMirroring.value && newId !== oldId && oldId !== null) {
    stopScreenMirroring();
  }
});

</script>

<template>
  <div
      style="position: fixed; top: 10px; right: 10px; background: rgba(238, 238, 238, 0.95); padding: 8px; border: 1px solid #ccc; z-index: 10000; font-size: 10px; max-width: 280px; word-break: break-all; border-radius: 4px; box-shadow: 0 2px 5px rgba(0,0,0,0.2); max-height: 90vh; overflow-y: auto;">
    <p style="margin:2px 0; font-weight: bold; border-bottom: 1px solid #ddd; padding-bottom: 3px; margin-bottom: 3px;">
      调试信息:</p>
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
  </div>

  <div class="phone-page">
    <header>
      <h1>手机连接与管理</h1>
      <button @click="fetchDevices" :disabled="isLoading || isMirroring || isUploadingFile" class="refresh-btn">
        {{ isLoading ? '刷新中...' : '刷新设备列表' }}
      </button>
    </header>

    <section v-if="isLoading" class="loading-section"><p>正在加载设备列表...</p></section>
    <section v-if="errorMessage && !isLoading && !isMirroring" class="error-section global-error-message"><p
        class="error-message">{{ errorMessage }}</p></section>
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
          <button @click="startScreenMirroring" :disabled="isMirroring || !selectedDeviceId || isUploadingFile"
                  class="control-btn start-btn">开始屏幕镜像
          </button>
          <button @click="stopScreenMirroring" :disabled="!isMirroring" class="control-btn stop-btn">停止屏幕镜像
          </button>
        </div>
        <div v-if="isMirroring && errorMessage && !fileListError && !uploadError" class="error-section mirror-error">
          <p class="error-message">{{ errorMessage }}</p>
        </div>
        <div v-if="isMirroring" class="mirror-display-area">
          <canvas ref="screenCanvasRef" class="mirrored-screen-canvas"></canvas>
          <p v-if="!canvasCtx && isMirroring">正在初始化画布...</p>
        </div>
      </section>

      <hr class="section-divider">

      <section class="action-section file-browser-section">
        <h4>文件浏览器</h4>
        <div class="path-navigation">
          <input type="text" v-model="currentRemotePath" @keyup.enter="fetchFileList" placeholder="输入设备路径"
                 :disabled="isFileListing || isUploadingFile || !selectedDeviceId"/>
          <button @click="fetchFileList" :disabled="isFileListing || isUploadingFile || !selectedDeviceId"
                  class="control-btn">
            {{ isFileListing ? '加载中...' : '转到路径' }}
          </button>
          <button @click="navigateUp"
                  :disabled="!canNavigateUp || isFileListing || isUploadingFile || !selectedDeviceId"
                  class="control-btn up-btn">
            返回上一级
          </button>
        </div>

        <div class="file-upload-area">
          <input
              type="file"
              @change="handleFileSelect"
              ref="fileInputRef"
              style="display: none;"
              :disabled="isUploadingFile || !selectedDeviceId"
          />
          <button @click="triggerFileInput" class="control-btn choose-file-btn"
                  :disabled="isUploadingFile || !selectedDeviceId">
            选择文件
          </button>
          <span v-if="selectedFileToUpload" class="selected-file-name">
            已选: {{ selectedFileToUpload.name }} ({{ (selectedFileToUpload.size / 1024).toFixed(2) }} KB)
          </span>
          <button
              v-if="selectedFileToUpload"
              @click="uploadSelectedFile"
              class="control-btn upload-btn"
              :disabled="isUploadingFile || !selectedDeviceId"
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
        <div v-if="fileListError && !isFileListing" class="error-section"><p class="error-message">{{
            fileListError
          }}</p></div>

        <div v-if="!isFileListing && !fileListError && fileList.length > 0" class="file-list-container">
          <ul>
            <li
                v-for="item in fileList"
                :key="item.name + (item.isDir ? '/' : '')" class="file-item"
                :class="{ 'is-dir': item.isDir }"
                @click="navigateTo(item)"
                :title="item.isDir ? `进入目录: ${item.name}` : `文件: ${item.name} (点击下载)`"
            >
              <span class="file-icon">{{ item.isDir ? '📁' : '📄' }}</span>
              <span class="file-name">{{ item.name }}</span>
            </li>
          </ul>
        </div>
        <div
            v-if="!isFileListing && !fileListError && fileList.length === 0 && currentRemotePath && selectedDeviceId && !errorMessage"
            class="no-files-section">
          <p>目录 “{{ currentRemotePath }}” 为空或无法访问。</p>
        </div>
      </section>
    </div>

    <nav class="navigation">
      <router-link to="/">返回主页</router-link>
    </nav>
  </div>
</template>

<style scoped>
/* 保持之前提供的完整样式列表，除了移除APK安装相关的特定样式 */
/* 基本页面和头部 */
.phone-page {
  max-width: 850px;
  margin: 20px auto;
  padding: 25px;
  background-color: #fcfdff;
  border-radius: 10px;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.08);
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
}

.phone-page header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid #e9ecef;
  padding-bottom: 18px;
  margin-bottom: 25px;
}

.phone-page header h1 {
  margin: 0;
  font-size: 2em;
  color: #2c3e50;
}

.refresh-btn {
  padding: 9px 18px;
  background-color: #007bff;
  color: white;
  border: none;
  border-radius: 5px;
  cursor: pointer;
  transition: background-color 0.2s;
  font-size: 0.95em;
}

.refresh-btn:disabled {
  background-color: #ced4da;
  cursor: not-allowed;
}

.refresh-btn:not(:disabled):hover {
  background-color: #0056b3;
}

/* 通用状态显示 */
.loading-section p {
  text-align: center;
  padding: 25px;
  font-size: 1.1em;
  color: #6c757d;
}

.error-section.global-error-message {
  margin-bottom: 20px;
}

.error-section p.error-message,
.mirror-error p.error-message,
.upload-status-message.error-message {
  color: #721c24;
  background-color: #f8d7da;
  border: 1px solid #f5c6cb;
  padding: 12px 15px;
  border-radius: 5px;
  text-align: left;
  margin-top: 12px;
  word-break: break-word;
  font-size: 0.95em;
}

.success-message.upload-status-message {
  color: #155724;
  background-color: #d4edda;
  border: 1px solid #c3e6cb;
  padding: 12px 15px;
  border-radius: 5px;
  text-align: left;
  margin-top: 12px;
  word-break: break-word;
  font-size: 0.95em;
}

/* 移除了 .success-message pre, .error-message pre 相关的APK安装输出样式 */

/* 设备列表 */
.devices-list-section h2 {
  margin-top: 0;
  margin-bottom: 12px;
  color: #495057;
  font-size: 1.3em;
  text-align: left;
}

.no-devices-section p {
  text-align: center;
  padding: 18px;
  background-color: #fff3cd;
  border: 1px solid #ffeeba;
  color: #856404;
  border-radius: 5px;
}

.device-list {
  list-style-type: none;
  padding: 0;
  margin-bottom: 25px;
}

.device-item {
  background-color: #f8f9fa;
  border: 1px solid #dee2e6;
  padding: 14px 18px;
  margin-bottom: 10px;
  border-radius: 5px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  cursor: pointer;
  transition: background-color 0.2s, border-color 0.2s;
}

.device-item:hover {
  background-color: #e9ecef;
  border-color: #ced4da;
}

.device-item.selected {
  background-color: #cce5ff;
  border-left: 5px solid #007bff;
  font-weight: bold;
}

.device-item span {
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, Courier, monospace;
  color: #212529;
}

.device-item span.status-device-ok {
  color: #28a745;
  font-weight: bold;
}

/* 选中设备后的操作区 */
.selected-device-actions {
  margin-top: 25px;
  padding-top: 25px;
}

.selected-device-actions > h3 {
  text-align: center;
  color: #2c3e50;
  margin-bottom: 25px;
  font-size: 1.5em;
}

.section-divider {
  border: none;
  border-top: 1px dashed #ced4da;
  margin: 30px 0;
}

.action-section {
  margin-bottom: 30px;
  padding: 22px;
  border: 1px solid #e9ecef;
  border-radius: 8px;
  background-color: #ffffff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.action-section h4 {
  margin-top: 0;
  margin-bottom: 18px;
  color: #007bff;
  border-bottom: 1px solid #f1f3f5;
  padding-bottom: 12px;
  font-size: 1.4em;
  text-align: left;
}

/* 通用控制按钮 */
.control-btn {
  padding: 9px 16px;
  margin: 5px;
  border: none;
  border-radius: 5px;
  cursor: pointer;
  font-size: 0.95em;
  transition: background-color 0.2s, opacity 0.2s;
  line-height: 1.5;
}

.control-btn:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

/* 屏幕镜像 */
.screen-mirror-section .mirror-controls {
  margin-bottom: 20px;
  text-align: center;
}

.screen-mirror-section .start-btn {
  background-color: #28a745;
  color: white;
}

.screen-mirror-section .start-btn:not(:disabled):hover {
  background-color: #218838;
}

.screen-mirror-section .stop-btn {
  background-color: #dc3545;
  color: white;
}

.screen-mirror-section .stop-btn:not(:disabled):hover {
  background-color: #c82333;
}

.screen-mirror-section .mirror-display-area {
  margin-top: 15px;
  padding: 10px;
  border: 1px dashed #ced4da;
  min-height: 220px;
  display: flex;
  justify-content: center;
  align-items: center;
  background-color: #f8f9fa;
}

.screen-mirror-section .mirrored-screen-canvas {
  max-width: 100%;
  max-height: 480px;
  border: 1px solid #dee2e6;
  display: block;
  margin: auto;
  background-color: #000;
}

.screen-mirror-section .mirror-display-area p {
  color: #6c757d;
}

/* 文件浏览 */
.file-browser-section .path-navigation {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-bottom: 18px;
}

.file-browser-section .path-navigation input[type="text"] {
  flex-grow: 1;
  min-width: 220px;
  padding: 9px 12px;
  border: 1px solid #ced4da;
  border-radius: 5px;
  font-size: 0.95em;
}

.file-browser-section .path-navigation .control-btn {
  padding: 9px 14px;
  font-size: 0.9em;
  white-space: nowrap;
}

.file-browser-section .path-navigation .up-btn {
  background-color: #6c757d;
  color: white;
}

.file-browser-section .path-navigation .up-btn:not(:disabled):hover {
  background-color: #5a6268;
}

/* 文件上传 */
.file-upload-area {
  margin-top: 18px;
  margin-bottom: 18px;
  padding: 12px;
  background-color: #f0f3f5;
  border: 1px dashed #ced4da;
  border-radius: 5px;
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.file-upload-area .choose-file-btn {
  background-color: #17a2b8;
  color: white;
}

.file-upload-area .choose-file-btn:not(:disabled):hover {
  background-color: #138496;
}

.file-upload-area .upload-btn {
  background-color: #28a745;
  color: white;
}

.file-upload-area .upload-btn:not(:disabled):hover {
  background-color: #218838;
}

.selected-file-name {
  font-style: italic;
  color: #495057;
  font-size: 0.9em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 220px;
  flex-shrink: 1;
}

.upload-progress-bar-container {
  width: 100%;
  background-color: #e9ecef;
  border-radius: 5px;
  margin-bottom: 12px;
  height: 22px;
  overflow: hidden;
}

.upload-progress-bar {
  width: 0%;
  height: 100%;
  background-color: #007bff;
  color: white;
  text-align: center;
  line-height: 22px;
  font-size: 0.85em;
  transition: width 0.3s ease-out;
}

/* APK 安装相关样式已被移除 */

/* 文件列表容器 */
.file-browser-section .file-list-container ul {
  list-style-type: none;
  padding: 0;
  max-height: 380px;
  overflow-y: auto;
  border: 1px solid #dee2e6;
  border-radius: 5px;
  background-color: #ffffff;
}

.file-browser-section .file-item {
  display: flex;
  align-items: center;
  padding: 11px 14px;
  border-bottom: 1px solid #f1f3f5;
  cursor: pointer;
  transition: background-color 0.2s;
}

.file-browser-section .file-item:last-child {
  border-bottom: none;
}

.file-browser-section .file-item:hover {
  background-color: #e9f5ff;
}

.file-browser-section .file-item.is-dir .file-name {
  font-weight: 600;
  color: #0056b3;
}

.file-browser-section .file-icon {
  margin-right: 12px;
  font-size: 1.25em;
  color: #6c757d;
}

.file-browser-section .file-item.is-dir .file-icon {
  color: #007bff;
}

.file-browser-section .file-name {
  word-break: break-all;
  color: #212529;
  font-size: 0.95em;
}

.file-browser-section .no-files-section p {
  text-align: center;
  padding: 18px;
  background-color: #f8f9fa;
  border: 1px solid #dee2e6;
  color: #495057;
  border-radius: 5px;
  margin-top: 12px;
}

/* 底部导航 */
.navigation {
  margin-top: 35px;
  text-align: center;
  padding-top: 22px;
  border-top: 1px solid #e9ecef;
}

.navigation a {
  color: #007bff;
  text-decoration: none;
  font-size: 1.05em;
}

.navigation a:hover {
  text-decoration: underline;
}

</style>