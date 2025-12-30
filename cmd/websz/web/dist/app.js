let currentPath = '/';
let selectedItem = null;
let contextMenuItem = null;

document.addEventListener('DOMContentLoaded', function() {
    loadDirectory('/');
    setupDragAndDrop();
    setupContextMenu();
});

function loadDirectory(path) {
    currentPath = path;
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
                renderFileList(data.data.items || []);
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
    rootLink.href = '#';
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
        link.href = '#';
        link.className = 'breadcrumb-item';
        link.textContent = parts[i];
        const currentPathForClosure = path;
        link.onclick = () => loadDirectory(currentPathForClosure);
        breadcrumb.appendChild(link);
    }
}

function renderFileList(files) {
    const tbody = document.getElementById('fileTableBody');
    tbody.innerHTML = '';
    
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
        nameCell.innerHTML = `
            <div class="file-name">
                <span class="file-icon">${file.isDir ? '📁' : getFileIcon(file.ext)}</span>
                ${file.name}
            </div>
        `;
        
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
    files.forEach(file => {
        formData.append('files', file);
    });
    
    fetch(`/api/upload?p=${encodeURIComponent(currentPath)}`, {
        method: 'POST',
        body: formData
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
        showError('Upload failed: ' + error.message);
    });
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