let currentPath = '/';
let selectedItem = null;
let contextMenuItem = null;
let currentFiles = [];
let sortField = 'name';
let sortAsc = true;
let viewMode = localStorage.getItem('websz_viewMode') || 'list';

let searchActive = false;
let searchQuery = '';
let searchRoot = '';
let searchTruncated = false;

// Cinema mode state
let cinemaMediaFiles = [];
let cinemaIndex = 0;
let cinemaActive = false;
let cinemaShuffleMode = false;
let cinemaShuffleOrder = [];
let cinemaVideoRotation = 0;

const CINEMA_IMAGE_EXTS = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.bmp', '.svg'];
const CINEMA_VIDEO_EXTS = ['.mp4', '.webm', '.mov', '.ogg'];

document.addEventListener('DOMContentLoaded', function() {
    parseHashState();
    applyViewMode();
    loadDirectory(currentPath);
    updateSortIndicators();
    setupDragAndDrop();
    setupContextMenu();
});

window.addEventListener('popstate', function(e) {
    if (cinemaActive) {
        exitCinemaMode(true);
        return;
    }
    parseHashState();
    updateSortIndicators();
    if (currentPath) {
        loadDirectory(currentPath, true);
    }
});

function parseHashState() {
    const hash = location.hash.slice(1);
    if (!hash) { currentPath = '/'; return; }
    const qIdx = hash.indexOf('?');
    if (qIdx === -1) {
        currentPath = decodeURIComponent(hash);
    } else {
        currentPath = decodeURIComponent(hash.substring(0, qIdx));
        const params = new URLSearchParams(hash.substring(qIdx + 1));
        if (params.get('sort')) sortField = params.get('sort');
        if (params.get('order')) sortAsc = params.get('order') === 'asc';
    }
}

function buildHash() {
    let h = '#' + encodeURIComponent(currentPath);
    if (sortField !== 'name' || !sortAsc) {
        h += '?sort=' + sortField + '&order=' + (sortAsc ? 'asc' : 'desc');
    }
    return h;
}

function loadDirectory(path, skipPush) {
    currentPath = path;
    if (!skipPush) {
        history.pushState(null, '', buildHash());
    }
    if (searchActive) {
        searchActive = false;
        updateSearchBanner();
        const input = document.getElementById('findInput');
        if (input) input.value = '';
    }
    updateBreadcrumb();
    
    console.log('Loading directory:', path);
    
    fetch(`/api/list?p=${encodeURIComponent(path)}`)
        .then(response => {
            console.log('Response status:', response.status);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return response.json();
        })
        .then(data => {
            console.log('Received data:', data);
            if (data.ok) {
                currentFiles = data.data.items || [];
                renderFileList(sortFiles(currentFiles));
            } else {
                showError(data.error || 'Unknown error');
            }
        })
        .catch(error => {
            console.error('Fetch error:', error);
            showError('Failed to load directory: ' + error.message);
        });
}

function updateBreadcrumb() {
    const breadcrumb = document.getElementById('breadcrumb');
    breadcrumb.innerHTML = '';
    
    const parts = currentPath.split('/').filter(p => p !== '');
    
    const rootLink = document.createElement('a');
    rootLink.href = 'javascript:void(0)';
    rootLink.className = 'breadcrumb-item';
    rootLink.textContent = 'Root';
    rootLink.onclick = () => loadDirectory('/');
    breadcrumb.appendChild(rootLink);
    
    let path = '';
    for (let i = 0; i < parts.length; i++) {
        const separator = document.createElement('span');
        separator.className = 'breadcrumb-separator';
        separator.textContent = '/';
        breadcrumb.appendChild(separator);
        
        path += '/' + parts[i];
        const link = document.createElement('a');
        link.href = 'javascript:void(0)';
        link.className = 'breadcrumb-item';
        link.textContent = parts[i];
        const currentPathForClosure = path;
        link.onclick = () => loadDirectory(currentPathForClosure);
        breadcrumb.appendChild(link);
    }
}

function sortFiles(files) {
    const sorted = [...files];
    // Directories always first
    sorted.sort((a, b) => {
        if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
        let cmp = 0;
        if (sortField === 'name') {
            cmp = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
        } else if (sortField === 'size') {
            cmp = (a.size || 0) - (b.size || 0);
        } else if (sortField === 'mtime') {
            cmp = new Date(a.mtime) - new Date(b.mtime);
        }
        return sortAsc ? cmp : -cmp;
    });
    return sorted;
}

function sortBy(field) {
    if (sortField === field) {
        sortAsc = !sortAsc;
    } else {
        sortField = field;
        sortAsc = true;
    }
    updateSortIndicators();
    history.replaceState(null, '', buildHash());
    renderFileList(sortFiles(currentFiles));
}

function updateSortIndicators() {
    ['name', 'size', 'mtime'].forEach(f => {
        const th = document.getElementById('th-' + f);
        const label = { name: 'Name', size: 'Size', mtime: 'Modified' }[f];
        if (f === sortField) {
            th.innerHTML = label + '<span class="sort-arrow">' + (sortAsc ? '\u25B2' : '\u25BC') + '</span>';
        } else {
            th.textContent = label;
        }
    });
}

function renderFileList(files) {
    const tbody = document.getElementById('fileTableBody');
    tbody.innerHTML = '';

    // Detect media files for cinema mode
    cinemaMediaFiles = files.filter(f => {
        if (f.isDir) return false;
        const ext = (f.ext || '').toLowerCase();
        return CINEMA_IMAGE_EXTS.includes(ext) || CINEMA_VIDEO_EXTS.includes(ext);
    });
    const cinemaBtn = document.getElementById('cinemaBtn');
    cinemaBtn.style.display = cinemaMediaFiles.length > 0 ? '' : 'none';

    // Also render gallery view
    renderGalleryView(files);

    if (files.length === 0) {
        const row = tbody.insertRow();
        const cell = row.insertCell(0);
        cell.colSpan = 4;
        cell.textContent = 'No files found';
        cell.style.textAlign = 'center';
        cell.style.color = '#666';
        cell.style.padding = '40px';
        return;
    }
    
    files.forEach(file => {
        const row = tbody.insertRow();
        row.className = 'file-row';
        row.dataset.path = file.path;
        row.dataset.isDir = file.isDir;
        row.dataset.name = file.name;
        
        row.ondblclick = () => {
            if (file.isDir) {
                loadDirectory(file.path);
            } else {
                openFile(file.path);
            }
        };
        
        row.oncontextmenu = (e) => {
            e.preventDefault();
            showContextMenu(e, row);
        };
        
        row.onclick = (e) => {
            if (e.detail === 1) {
                selectRow(row);
            }
        };
        
        const nameCell = row.insertCell(0);
        let nameHTML = `
            <div class="file-name">
                <span class="file-icon">${file.isDir ? '📁' : getFileIcon(file.ext)}</span>
                ${escapeHTML(file.name)}
            </div>
        `;
        if (searchActive) {
            const parent = file.path.substring(0, file.path.lastIndexOf('/')) || '/';
            nameHTML += `<div class="file-name-path" title="${escapeAttr(parent)}">${escapeHTML(parent)}</div>`;
        }
        nameCell.innerHTML = nameHTML;
        
        const sizeCell = row.insertCell(1);
        sizeCell.textContent = file.isDir ? '' : formatFileSize(file.size);
        
        const modifiedCell = row.insertCell(2);
        modifiedCell.textContent = formatDate(file.mtime);
        
        const actionsCell = row.insertCell(3);
        let actionsHTML = '<div class="file-actions">';
        
        // Add specific actions based on file type
        if (file.isDir) {
            actionsHTML += `<button class="file-action-btn" onclick="loadDirectory('${file.path}')">Open</button>`;
        } else {
            if (isPreviewable(file.ext, file.mime)) {
                actionsHTML += `<button class="file-action-btn" onclick="openFile('${file.path}')">Preview</button>`;
            }
            actionsHTML += `<button class="file-action-btn" onclick="downloadFile('${file.path}')">Download</button>`;
        }
        
        // Add common actions for all items
        actionsHTML += `<button class="file-action-btn" onclick="showRenameModalForItem('${file.path}', '${file.name.replace(/'/g, "\\'")}')">Rename</button>`;
        actionsHTML += `<button class="file-action-btn file-action-btn-danger" onclick="deleteItemDirect('${file.path}', ${file.isDir}, '${file.name.replace(/'/g, "\\'")}')">Delete</button>`;
        actionsHTML += `<button class="file-action-btn" onclick="showItemPropertiesDirect('${file.path}')">Properties</button>`;
        
        actionsHTML += '</div>';
        actionsCell.innerHTML = actionsHTML;
    });
}

function getFileIcon(ext) {
    const iconMap = {
        '.jpg': '🖼️', '.jpeg': '🖼️', '.png': '🖼️', '.gif': '🖼️', '.webp': '🖼️',
        '.mp4': '🎥', '.mov': '🎥', '.avi': '🎥', '.webm': '🎥',
        '.mp3': '🎵', '.wav': '🎵', '.ogg': '🎵', '.aac': '🎵', '.flac': '🎵', '.m4a': '🎵', '.wma': '🎵',
        '.pdf': '📄',
        '.txt': '📝', '.md': '📝',
        '.zip': '📦', '.tar': '📦', '.gz': '📦',
        '.js': '📄', '.html': '📄', '.css': '📄', '.json': '📄',
        '.go': '📄', '.py': '📄', '.java': '📄', '.cpp': '📄'
    };
    return iconMap[ext] || '📄';
}

function isPreviewable(ext, mime) {
    const previewableExts = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.mp4', '.mov', '.avi', '.webm', '.pdf', '.txt', '.md'];
    const previewableMimes = ['image/', 'video/mp4', 'application/pdf', 'text/plain', 'text/markdown'];
    
    return previewableExts.includes(ext) || previewableMimes.some(m => mime && mime.startsWith(m));
}

function formatFileSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatDate(dateString) {
    const date = new Date(dateString);
    return date.toLocaleString();
}

function selectRow(row) {
    document.querySelectorAll('.file-row').forEach(r => r.classList.remove('selected'));
    row.classList.add('selected');
    selectedItem = row;
}

function openFile(path) {
    window.open(`/open?p=${encodeURIComponent(path)}`, '_blank');
}

function downloadFile(path) {
    window.open(`/api/download?p=${encodeURIComponent(path)}`, '_blank');
}

function refresh() {
    loadDirectory(currentPath);
}

function setupDragAndDrop() {
    const uploadArea = document.getElementById('uploadArea');
    const container = document.querySelector('.main-content');
    
    ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
        container.addEventListener(eventName, preventDefaults, false);
    });
    
    function preventDefaults(e) {
        e.preventDefault();
        e.stopPropagation();
    }
    
    ['dragenter', 'dragover'].forEach(eventName => {
        container.addEventListener(eventName, () => {
            uploadArea.style.display = 'block';
            uploadArea.classList.add('dragover');
        }, false);
    });
    
    ['dragleave', 'drop'].forEach(eventName => {
        container.addEventListener(eventName, () => {
            uploadArea.style.display = 'none';
            uploadArea.classList.remove('dragover');
        }, false);
    });
    
    container.addEventListener('drop', handleDrop, false);
}

function handleDrop(e) {
    const files = e.dataTransfer.files;
    uploadFilesArray(Array.from(files));
}

function uploadFiles() {
    const fileInput = document.getElementById('fileInput');
    uploadFilesArray(Array.from(fileInput.files));
    fileInput.value = '';
}

function uploadFilesArray(files) {
    if (files.length === 0) return;

    const formData = new FormData();
    let totalSize = 0;
    const fileNames = [];

    files.forEach(file => {
        formData.append('files', file);
        totalSize += file.size;
        fileNames.push(file.name);
    });

    // Show progress modal
    const progressModal = document.getElementById('uploadProgressModal');
    const progressFill = document.getElementById('uploadProgressFill');
    const progressPercent = document.getElementById('uploadProgressPercent');
    const progressSize = document.getElementById('uploadProgressSize');
    const progressText = document.getElementById('uploadProgressText');
    const fileName = document.getElementById('uploadFileName');

    fileName.textContent = files.length === 1 ? fileNames[0] : `${files.length} files`;
    progressFill.style.width = '0%';
    progressPercent.textContent = '0%';
    progressSize.textContent = `0 B / ${formatFileSize(totalSize)}`;
    progressText.textContent = 'Uploading...';
    showModal('uploadProgressModal');

    const xhr = new XMLHttpRequest();

    xhr.upload.addEventListener('progress', function(e) {
        if (e.lengthComputable) {
            const percent = Math.round((e.loaded / e.total) * 100);
            progressFill.style.width = percent + '%';
            progressPercent.textContent = percent + '%';
            progressSize.textContent = `${formatFileSize(e.loaded)} / ${formatFileSize(e.total)}`;

            if (percent === 100) {
                progressText.textContent = 'Processing...';
            }
        }
    });

    xhr.addEventListener('load', function() {
        hideModal('uploadProgressModal');

        if (xhr.status === 200) {
            try {
                const data = JSON.parse(xhr.responseText);
                if (data.ok) {
                    refresh();
                } else {
                    showError(data.error);
                }
            } catch (e) {
                showError('Invalid response from server');
            }
        } else {
            try {
                const data = JSON.parse(xhr.responseText);
                showError(data.error || 'Upload failed');
            } catch (e) {
                showError('Upload failed: ' + xhr.statusText);
            }
        }
    });

    xhr.addEventListener('error', function() {
        hideModal('uploadProgressModal');
        showError('Upload failed: Network error');
    });

    xhr.addEventListener('abort', function() {
        hideModal('uploadProgressModal');
        showError('Upload cancelled');
    });

    xhr.open('POST', `/api/upload?p=${encodeURIComponent(currentPath)}`);
    xhr.send(formData);
}

function setupContextMenu() {
    document.addEventListener('click', () => {
        hideContextMenu();
    });
}

function showContextMenu(e, row) {
    contextMenuItem = row;
    const menu = document.getElementById('contextMenu');
    menu.style.display = 'block';
    menu.style.left = e.pageX + 'px';
    menu.style.top = e.pageY + 'px';
}

function hideContextMenu() {
    document.getElementById('contextMenu').style.display = 'none';
}

function openItem() {
    if (!contextMenuItem) return;
    const path = contextMenuItem.dataset.path;
    const isDir = contextMenuItem.dataset.isDir === 'true';
    
    if (isDir) {
        loadDirectory(path);
    } else {
        openFile(path);
    }
    hideContextMenu();
}

function downloadItem() {
    if (!contextMenuItem) return;
    const path = contextMenuItem.dataset.path;
    const isDir = contextMenuItem.dataset.isDir === 'true';
    
    if (!isDir) {
        downloadFile(path);
    }
    hideContextMenu();
}

function showRenameModal() {
    if (!contextMenuItem) return;
    
    document.getElementById('renameInput').value = contextMenuItem.dataset.name;
    showModal('renameModal');
    hideContextMenu();
}

function showRenameModalForItem(path, name) {
    // Set up a temporary context item for the rename function
    contextMenuItem = {
        dataset: {
            path: path,
            name: name
        }
    };
    
    document.getElementById('renameInput').value = name;
    showModal('renameModal');
}

function renameItem() {
    if (!contextMenuItem) return;
    
    const newName = document.getElementById('renameInput').value.trim();
    if (!newName) return;
    
    const oldPath = contextMenuItem.dataset.path;
    const parentPath = oldPath.substring(0, oldPath.lastIndexOf('/'));
    const newPath = parentPath === '' ? '/' + newName : parentPath + '/' + newName;
    
    fetch('/api/rename', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            from: oldPath,
            to: newPath
        })
    })
    .then(response => response.json())
    .then(data => {
        if (data.ok) {
            refresh();
            hideModal('renameModal');
        } else {
            showError(data.error);
        }
    })
    .catch(error => {
        showError('Rename failed: ' + error.message);
    });
}

function deleteItem() {
    if (!contextMenuItem) return;
    
    const path = contextMenuItem.dataset.path;
    const isDir = contextMenuItem.dataset.isDir === 'true';
    const name = contextMenuItem.dataset.name;
    
    if (!confirm(`Are you sure you want to delete "${name}"?`)) {
        hideContextMenu();
        return;
    }
    
    fetch('/api/delete', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            p: path,
            recursive: isDir
        })
    })
    .then(response => response.json())
    .then(data => {
        if (data.ok) {
            refresh();
        } else {
            showError(data.error);
        }
    })
    .catch(error => {
        showError('Delete failed: ' + error.message);
    });
    
    hideContextMenu();
}

function showItemProperties() {
    if (!contextMenuItem) return;
    
    const path = contextMenuItem.dataset.path;
    
    fetch(`/api/stat?p=${encodeURIComponent(path)}`)
        .then(response => response.json())
        .then(data => {
            if (data.ok) {
                const info = data.data;
                document.getElementById('propertiesContent').innerHTML = `
                    <div class="form-group">
                        <strong>Name:</strong> ${info.name}<br>
                        <strong>Path:</strong> ${info.path}<br>
                        <strong>Type:</strong> ${info.isDir ? 'Directory' : 'File'}<br>
                        ${!info.isDir ? `<strong>Size:</strong> ${formatFileSize(info.size)}<br>` : ''}
                        <strong>Modified:</strong> ${formatDate(info.mtime)}<br>
                        ${info.mime ? `<strong>MIME Type:</strong> ${info.mime}<br>` : ''}
                        ${info.mode ? `<strong>Permissions:</strong> ${info.mode}<br>` : ''}
                        ${info.sha256 ? `<strong>SHA256:</strong> ${info.sha256}<br>` : ''}
                    </div>
                `;
                showModal('propertiesModal');
            } else {
                showError(data.error);
            }
        })
        .catch(error => {
            showError('Failed to get file properties: ' + error.message);
        });
    
    hideContextMenu();
}

function deleteItemDirect(path, isDir, name) {
    if (!confirm(`Are you sure you want to delete "${name}"?`)) {
        return;
    }
    
    fetch('/api/delete', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            p: path,
            recursive: isDir
        })
    })
    .then(response => response.json())
    .then(data => {
        if (data.ok) {
            refresh();
        } else {
            showError(data.error);
        }
    })
    .catch(error => {
        showError('Delete failed: ' + error.message);
    });
}

function showItemPropertiesDirect(path) {
    fetch(`/api/stat?p=${encodeURIComponent(path)}`)
        .then(response => response.json())
        .then(data => {
            if (data.ok) {
                const info = data.data;
                document.getElementById('propertiesContent').innerHTML = `
                    <div class="form-group">
                        <strong>Name:</strong> ${info.name}<br>
                        <strong>Path:</strong> ${info.path}<br>
                        <strong>Type:</strong> ${info.isDir ? 'Directory' : 'File'}<br>
                        ${!info.isDir ? `<strong>Size:</strong> ${formatFileSize(info.size)}<br>` : ''}
                        <strong>Modified:</strong> ${formatDate(info.mtime)}<br>
                        ${info.mime ? `<strong>MIME Type:</strong> ${info.mime}<br>` : ''}
                        ${info.mode ? `<strong>Permissions:</strong> ${info.mode}<br>` : ''}
                        ${info.sha256 ? `<strong>SHA256:</strong> ${info.sha256}<br>` : ''}
                    </div>
                `;
                showModal('propertiesModal');
            } else {
                showError(data.error);
            }
        })
        .catch(error => {
            showError('Failed to get file properties: ' + error.message);
        });
}

function showNewFolderModal() {
    document.getElementById('newFolderName').value = '';
    showModal('newFolderModal');
}

function createFolder() {
    const name = document.getElementById('newFolderName').value.trim();
    if (!name) return;
    
    fetch('/api/mkdir', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            p: currentPath,
            name: name
        })
    })
    .then(response => response.json())
    .then(data => {
        if (data.ok) {
            refresh();
            hideModal('newFolderModal');
        } else {
            showError(data.error);
        }
    })
    .catch(error => {
        showError('Failed to create folder: ' + error.message);
    });
}

function showModal(modalId) {
    document.getElementById(modalId).style.display = 'flex';
}

function hideModal(modalId) {
    document.getElementById(modalId).style.display = 'none';
}

// ---- Cinema Mode ----

function isCinemaImage(ext) {
    return CINEMA_IMAGE_EXTS.includes((ext || '').toLowerCase());
}

function buildShuffleOrder() {
    cinemaShuffleOrder = Array.from({ length: cinemaMediaFiles.length }, (_, i) => i);
    for (let i = cinemaShuffleOrder.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [cinemaShuffleOrder[i], cinemaShuffleOrder[j]] = [cinemaShuffleOrder[j], cinemaShuffleOrder[i]];
    }
}

function enterCinemaMode() {
    if (cinemaMediaFiles.length === 0) return;
    cinemaActive = true;
    cinemaIndex = 0;
    buildShuffleOrder();
    updateCinemaShuffleBtn();
    history.pushState({ cinema: true }, '');
    document.getElementById('cinemaOverlay').classList.add('active');
    renderCinemaItem();
    document.addEventListener('keydown', cinemaKeyHandler);
    document.getElementById('cinemaOverlay').addEventListener('wheel', cinemaWheelHandler, { passive: false });
}

function exitCinemaMode(fromPopstate) {
    cinemaActive = false;
    if (!fromPopstate) {
        history.back();
    }
    document.getElementById('cinemaOverlay').classList.remove('active');
    document.removeEventListener('keydown', cinemaKeyHandler);
    document.getElementById('cinemaOverlay').removeEventListener('wheel', cinemaWheelHandler);
    // Stop any playing video
    const container = document.getElementById('cinemaMediaContainer');
    const video = container.querySelector('video');
    if (video) { video.pause(); video.src = ''; }
    container.innerHTML = '';
}

function renderCinemaItem() {
    const fileIdx = cinemaShuffleMode ? cinemaShuffleOrder[cinemaIndex] : cinemaIndex;
    const file = cinemaMediaFiles[fileIdx];
    const container = document.getElementById('cinemaMediaContainer');
    const ext = (file.ext || '').toLowerCase();
    const url = `/open?p=${encodeURIComponent(file.path)}`;

    // Stop previous video if any
    const oldVideo = container.querySelector('video');
    if (oldVideo) { oldVideo.pause(); oldVideo.src = ''; }

    const rotateBtn = document.getElementById('cinemaRotateBtn');
    cinemaVideoRotation = 0;
    if (isCinemaImage(ext)) {
        container.innerHTML = `<img src="${url}" alt="${file.name}">`;
        if (rotateBtn) rotateBtn.style.display = 'none';
    } else {
        container.innerHTML = `<video src="${url}" autoplay controls loop></video>`;
        if (rotateBtn) {
            rotateBtn.style.display = '';
            updateCinemaRotateBtn();
        }
    }

    document.getElementById('cinemaFileName').textContent = file.name;
    document.getElementById('cinemaCounter').textContent = `${cinemaIndex + 1} / ${cinemaMediaFiles.length}`;
}

function cycleCinemaVideoRotation() {
    cinemaVideoRotation = (cinemaVideoRotation + 90) % 360;
    applyCinemaVideoRotation();
    updateCinemaRotateBtn();
}

function applyCinemaVideoRotation() {
    const container = document.getElementById('cinemaMediaContainer');
    const video = container.querySelector('video');
    if (!video) return;
    const r = cinemaVideoRotation;
    video.style.transform = r ? `rotate(${r}deg)` : '';
    if (r === 90 || r === 270) {
        video.style.maxWidth = container.clientHeight + 'px';
        video.style.maxHeight = container.clientWidth + 'px';
    } else {
        video.style.maxWidth = '';
        video.style.maxHeight = '';
    }
}

function updateCinemaRotateBtn() {
    const btn = document.getElementById('cinemaRotateBtn');
    if (!btn) return;
    btn.textContent = cinemaVideoRotation ? `↻ ${cinemaVideoRotation}°` : '↻ Rotate';
    btn.classList.toggle('active', cinemaVideoRotation !== 0);
}

function cinemaNext() {
    cinemaIndex = (cinemaIndex + 1) % cinemaMediaFiles.length;
    renderCinemaItem();
}

function cinemaPrev() {
    cinemaIndex = (cinemaIndex - 1 + cinemaMediaFiles.length) % cinemaMediaFiles.length;
    renderCinemaItem();
}

let cinemaWheelCooldown = false;

function cinemaKeyHandler(e) {
    if (!cinemaActive) return;
    if (e.key === 'Escape') {
        exitCinemaMode();
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowRight') {
        e.preventDefault();
        cinemaNext();
    } else if (e.key === 'ArrowUp' || e.key === 'ArrowLeft') {
        e.preventDefault();
        cinemaPrev();
    }
}

function cinemaWheelHandler(e) {
    e.preventDefault();
    if (cinemaWheelCooldown) return;
    cinemaWheelCooldown = true;
    setTimeout(() => { cinemaWheelCooldown = false; }, 300);

    if (e.deltaY > 0) {
        cinemaNext();
    } else if (e.deltaY < 0) {
        cinemaPrev();
    }
}

function toggleCinemaShuffle() {
    cinemaShuffleMode = !cinemaShuffleMode;
    if (cinemaShuffleMode) {
        // Rebuild so current file stays first in shuffled order
        const currentFileIdx = cinemaIndex;
        buildShuffleOrder();
        // Move current file to position 0 in shuffle order
        const pos = cinemaShuffleOrder.indexOf(currentFileIdx);
        if (pos > 0) {
            [cinemaShuffleOrder[0], cinemaShuffleOrder[pos]] = [cinemaShuffleOrder[pos], cinemaShuffleOrder[0]];
        }
        cinemaIndex = 0;
    }
    updateCinemaShuffleBtn();
    renderCinemaItem();
}

function updateCinemaShuffleBtn() {
    const btn = document.getElementById('cinemaShuffleBtn');
    if (!btn) return;
    btn.classList.toggle('active', cinemaShuffleMode);
}

// ---- End Cinema Mode ----

// ---- Gallery Mode ----

const GALLERY_IMAGE_EXTS = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.bmp', '.svg', '.ico'];
const GALLERY_AUDIO_EXTS = ['.mp3', '.wav', '.ogg', '.aac', '.flac', '.m4a', '.wma'];
const GALLERY_MAX_CONCURRENT = 3;
let galleryLoadQueue = [];
let galleryActiveLoads = 0;
let galleryObserver = null;

function isGalleryImage(ext) {
    return GALLERY_IMAGE_EXTS.includes((ext || '').toLowerCase());
}

function isGalleryAudio(ext) {
    return GALLERY_AUDIO_EXTS.includes((ext || '').toLowerCase());
}

function setViewMode(mode) {
    viewMode = mode;
    localStorage.setItem('websz_viewMode', mode);
    applyViewMode();
}

function applyViewMode() {
    const fileList = document.querySelector('.file-list');
    const galleryGrid = document.getElementById('galleryGrid');
    const listBtn = document.getElementById('viewListBtn');
    const galleryBtn = document.getElementById('viewGalleryBtn');

    if (viewMode === 'gallery') {
        fileList.style.display = 'none';
        galleryGrid.style.display = '';
        listBtn.classList.remove('active');
        galleryBtn.classList.add('active');
    } else {
        fileList.style.display = '';
        galleryGrid.style.display = 'none';
        listBtn.classList.add('active');
        galleryBtn.classList.remove('active');
    }
}

function renderGalleryView(files) {
    const grid = document.getElementById('galleryGrid');
    grid.innerHTML = '';

    // Clean up previous observer and queue
    if (galleryObserver) {
        galleryObserver.disconnect();
        galleryObserver = null;
    }
    galleryLoadQueue = [];
    galleryActiveLoads = 0;

    if (files.length === 0) {
        grid.innerHTML = '<div style="grid-column:1/-1;text-align:center;color:#666;padding:40px">No files found</div>';
        return;
    }

    galleryObserver = new IntersectionObserver(galleryIntersectionCallback, {
        root: grid.parentElement,
        rootMargin: '200px',
        threshold: 0
    });

    files.forEach(file => {
        const item = document.createElement('div');
        item.className = 'gallery-item';
        item.dataset.path = file.path;
        item.dataset.isDir = file.isDir;
        item.dataset.name = file.name;

        const thumb = document.createElement('div');
        thumb.className = 'gallery-thumb';

        const ext = (file.ext || '').toLowerCase();
        if (!file.isDir && isGalleryImage(ext)) {
            // Show loading spinner; actual image loaded via IntersectionObserver
            thumb.innerHTML = '<div class="gallery-thumb-loading"></div>';
            thumb.dataset.imagePath = file.path;
        } else if (!file.isDir && isGalleryAudio(ext)) {
            thumb.innerHTML = '<span class="gallery-thumb-placeholder">🎵</span>';
            thumb.dataset.audioPath = file.path;
        } else {
            thumb.innerHTML = '<span class="gallery-thumb-placeholder">' + (file.isDir ? '📁' : getFileIcon(file.ext)) + '</span>';
        }

        const label = document.createElement('div');
        label.className = 'gallery-label';
        label.textContent = file.name;
        label.title = file.name;

        item.appendChild(thumb);
        item.appendChild(label);

        item.ondblclick = () => {
            if (file.isDir) {
                loadDirectory(file.path);
            } else {
                openFile(file.path);
            }
        };

        item.onclick = (e) => {
            if (e.detail === 1) {
                document.querySelectorAll('.gallery-item').forEach(el => el.classList.remove('selected'));
                item.classList.add('selected');
            }
        };

        item.oncontextmenu = (e) => {
            e.preventDefault();
            contextMenuItem = { dataset: { path: file.path, isDir: String(file.isDir), name: file.name } };
            showContextMenu(e, contextMenuItem);
        };

        if (thumb.dataset.audioPath) {
            let hoverAudio = null;
            item.addEventListener('mouseenter', () => {
                if (hoverAudio) return;
                hoverAudio = new Audio(`/open?p=${encodeURIComponent(thumb.dataset.audioPath)}`);
                hoverAudio.loop = false;
                thumb.querySelector('.gallery-thumb-placeholder').textContent = '🔊';
                hoverAudio.addEventListener('ended', () => {
                    thumb.querySelector('.gallery-thumb-placeholder').textContent = '🎵';
                    hoverAudio = null;
                });
                hoverAudio.play().catch(() => {
                    thumb.querySelector('.gallery-thumb-placeholder').textContent = '🎵';
                    hoverAudio = null;
                });
            });
            item.addEventListener('mouseleave', () => {
                if (hoverAudio) {
                    hoverAudio.pause();
                    hoverAudio.src = '';
                    hoverAudio = null;
                }
                thumb.querySelector('.gallery-thumb-placeholder').textContent = '🎵';
            });
        }

        grid.appendChild(item);

        // Observe for lazy loading if it's an image
        if (thumb.dataset.imagePath) {
            galleryObserver.observe(thumb);
        }
    });
}

function galleryIntersectionCallback(entries) {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            const thumb = entry.target;
            galleryObserver.unobserve(thumb);
            const imagePath = thumb.dataset.imagePath;
            if (imagePath) {
                enqueueGalleryLoad(thumb, imagePath);
            }
        }
    });
}

function enqueueGalleryLoad(thumb, imagePath) {
    galleryLoadQueue.push({ thumb, imagePath });
    processGalleryQueue();
}

function processGalleryQueue() {
    while (galleryActiveLoads < GALLERY_MAX_CONCURRENT && galleryLoadQueue.length > 0) {
        const { thumb, imagePath } = galleryLoadQueue.shift();
        galleryActiveLoads++;

        const img = new Image();
        img.onload = () => {
            thumb.innerHTML = '';
            thumb.appendChild(img);
            galleryActiveLoads--;
            processGalleryQueue();
        };
        img.onerror = () => {
            thumb.innerHTML = '<span class="gallery-thumb-placeholder">🖼️</span>';
            galleryActiveLoads--;
            processGalleryQueue();
        };
        img.src = `/open?p=${encodeURIComponent(imagePath)}`;
    }
}

// ---- End Gallery Mode ----

function escapeHTML(s) {
    return String(s).replace(/[&<>"']/g, c => ({
        '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    })[c]);
}

function escapeAttr(s) {
    return escapeHTML(s);
}

function doFind() {
    const input = document.getElementById('findInput');
    const q = (input.value || '').trim();
    if (!q) return;

    searchQuery = q;
    searchRoot = currentPath;

    fetch(`/api/find?p=${encodeURIComponent(currentPath)}&q=${encodeURIComponent(q)}`)
        .then(response => response.json())
        .then(data => {
            if (!data.ok) {
                showError(data.error || 'Find failed');
                return;
            }
            searchActive = true;
            searchTruncated = !!data.data.truncated;
            currentFiles = data.data.items || [];
            updateSearchBanner();
            renderFileList(sortFiles(currentFiles));
        })
        .catch(error => {
            showError('Find failed: ' + error.message);
        });
}

function clearFind() {
    searchActive = false;
    searchQuery = '';
    document.getElementById('findInput').value = '';
    updateSearchBanner();
    loadDirectory(currentPath, true);
}

function updateSearchBanner() {
    const banner = document.getElementById('searchBanner');
    const text = document.getElementById('searchBannerText');
    if (!banner || !text) return;
    if (searchActive) {
        const count = currentFiles.length;
        const suffix = searchTruncated ? ' (truncated)' : '';
        text.textContent = `Found ${count} match${count === 1 ? '' : 'es'} for "${searchQuery}" in ${searchRoot}${suffix}`;
        banner.style.display = '';
    } else {
        banner.style.display = 'none';
    }
}

function showError(message) {
    const existing = document.querySelector('.error');
    if (existing) {
        existing.remove();
    }
    
    const errorDiv = document.createElement('div');
    errorDiv.className = 'error';
    errorDiv.textContent = message;
    document.querySelector('.container').insertBefore(errorDiv, document.querySelector('.main-content'));
    
    setTimeout(() => {
        errorDiv.remove();
    }, 5000);
}