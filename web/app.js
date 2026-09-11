// LPUt Management Console — app.js
(function() {
    'use strict';

    const canvas = document.getElementById('screen-canvas');
    const ctx = canvas.getContext('2d', { alpha: false, desynchronized: true });
    
    const ui = {
        views: {
            dashboard: document.getElementById('dashboard-view'),
            session: document.getElementById('session-view')
        },
        devices: document.getElementById('devices-grid'),
        sysinfo: document.getElementById('sysinfo-panel'),
        files: document.getElementById('file-panel'),
        sysinfoContent: document.getElementById('sysinfo-content'),
        sessionTitle: document.getElementById('session-title'),
        serverStatus: document.getElementById('server-status'),
        toastContainer: document.getElementById('toast-container'),
        toolbar: document.getElementById('session-toolbar'),
        remoteCursor: document.getElementById('remote-cursor'),
        canvasWrapper: document.getElementById('canvas-wrapper')
    };

    const btns = {
        disconnect: document.getElementById('disconnect-btn'),
        selfDestruct: document.getElementById('self-destruct-btn'),
        refresh: document.getElementById('refresh-btn'),
        sysinfo: document.getElementById('sysinfo-btn'),
        closeSysinfo: document.getElementById('close-sysinfo'),
        files: document.getElementById('btn-files'),
        closeFiles: document.getElementById('close-files'),
        copy: document.getElementById('btn-copy'),
        paste: document.getElementById('btn-paste'),
        fullscreen: document.getElementById('fullscreen-btn'),
        pin: document.getElementById('pin-btn')
    };

    const controls = {
        quality: document.getElementById('quality-select'),
        fps: document.getElementById('fps-select')
    };

    let ws = null;
    let activeDeviceId = null;
    let isPinned = true;
    let canvasRectCache = null;

    let pc = null;
    let webrtcVideoChannel = null;
    let webrtcInputChannel = null;

    function showToast(message, icon = 'bx-info-circle', color = 'var(--accent)') {
        const toast = document.createElement('div');
        toast.className = 'toast';
        toast.innerHTML = `<i class='bx ${icon}' style="color: ${color}; font-size: 20px;"></i> <span>${escHtml(message)}</span>`;
        ui.toastContainer.appendChild(toast);
        setTimeout(() => {
            toast.style.opacity = '0';
            setTimeout(() => toast.remove(), 300);
        }, 3000);
    }

    function switchView(viewName) {
        Object.values(ui.views).forEach(v => v.classList.remove('active'));
        ui.views[viewName].classList.add('active');
    }

    function connect() {
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = `${proto}//${location.host}/ws/console`;
        ws = new WebSocket(url);
        ws.binaryType = 'blob';

        ws.onopen = () => {
            ui.serverStatus.innerHTML = `<span class="status-dot online"></span> Connected`;
            requestDeviceList();
        };
        ws.onclose = () => {
            ui.serverStatus.innerHTML = `<span class="status-dot offline"></span> Reconnecting...`;
            setTimeout(connect, 3000);
        };
        ws.onmessage = handleMessage;
    }

    function send(msg) {
        if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(msg));
    }

    function requestDeviceList() { send({ type: 'list_devices' }); }

    let pendingBlob = null;
    let isRendering = false;

    async function handleMessage(event) {
        if (event.data instanceof Blob) {
            pendingBlob = event.data;
            if (!isRendering) {
                isRendering = true;
                requestAnimationFrame(renderFrame);
            }
            return;
        }

        const msg = JSON.parse(event.data);
        switch (msg.type) {
            case 'device_list':
                renderDeviceList(msg.payload || []);
                break;
            case 'session_established':
                const sess = msg.payload;
                ui.sessionTitle.textContent = `${sess.hostname}`;
                switchView('session');
                showToast(`Connected to ${sess.hostname}`, 'bx-check-circle', 'var(--success)');
                initWebRTC();
                break;
            case 'webrtc_answer':
                if (pc) pc.setRemoteDescription(new RTCSessionDescription(msg.payload));
                break;
            case 'webrtc_ice_candidate':
                if (pc) pc.addIceCandidate(new RTCIceCandidate(msg.payload));
                break;
            case 'system_info':
                renderSystemInfo(msg.payload);
                break;
            case 'clipboard_update':
                if (msg.payload.text) {
                    try {
                        await navigator.clipboard.writeText(msg.payload.text);
                        showToast('Remote clipboard copied to local', 'bx-check-double', 'var(--success)');
                    } catch (err) {
                        showToast('Failed to write to local clipboard', 'bx-error', 'var(--danger)');
                    }
                }
                break;
            case 'cursor_position':
                updateRemoteCursor(msg.payload.x, msg.payload.y);
                break;
            case 'performance_stats':
                document.getElementById('diag-fps').textContent = msg.payload.fps.toFixed(1);
                document.getElementById('diag-capture').textContent = msg.payload.capture_latency_ms.toFixed(1);
                document.getElementById('diag-encode').textContent = msg.payload.encode_latency_ms.toFixed(1);
                document.getElementById('diag-res').textContent = msg.payload.resolution;
                break;
            case 'pong':
                const rtt = performance.now() - msg.payload.timestamp;
                document.getElementById('diag-rtt').textContent = rtt.toFixed(1);
                break;
            case 'error':
                showToast(msg.error || 'Error occurred', 'bx-error-circle', 'var(--danger)');
                break;
                break;
        }
    }

    async function initWebRTC() {
        if (pc) pc.close();
        
        pc = new RTCPeerConnection({
            iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
        });
        
        webrtcVideoChannel = pc.createDataChannel('video');
        webrtcInputChannel = pc.createDataChannel('input');
        
        webrtcVideoChannel.binaryType = 'arraybuffer';
        webrtcVideoChannel.onmessage = (e) => {
            pendingBlob = new Blob([e.data]);
            if (!isRendering) {
                isRendering = true;
                requestAnimationFrame(renderFrame);
            }
        };
        
        pc.onicecandidate = (e) => {
            if (e.candidate) {
                send({ type: 'webrtc_ice_candidate', payload: e.candidate });
            }
        };
        
        const offer = await pc.createOffer();
        await pc.setLocalDescription(offer);
        send({ type: 'webrtc_offer', payload: { type: offer.type, sdp: offer.sdp } });
    }

    async function renderFrame() {
        if (!pendingBlob) { isRendering = false; return; }
        const blob = pendingBlob;
        pendingBlob = null;
        try {
            const bmp = await createImageBitmap(blob);
            if (canvas.width !== bmp.width || canvas.height !== bmp.height) {
                canvas.width = bmp.width;
                canvas.height = bmp.height;
                canvasRectCache = null; // force recalculate layout
            }
            ctx.drawImage(bmp, 0, 0);
            bmp.close();
        } catch (e) {}
        if (pendingBlob) requestAnimationFrame(renderFrame);
        else isRendering = false;
    }

    function renderDeviceList(devices) {
        if (devices.length === 0) {
            ui.devices.innerHTML = `
                <div class="empty-state" style="grid-column: 1 / -1;">
                    <i class='bx bx-devices'></i>
                    <p>No endpoints available</p>
                </div>`;
            return;
        }
        ui.devices.innerHTML = '';
        devices.forEach(dev => {
            const card = document.createElement('div');
            card.className = `device-card ${dev.status}`;
            card.innerHTML = `
                <div style="display: flex; justify-content: space-between; align-items: flex-start;">
                    <div>
                        <h3>${escHtml(dev.hostname || 'Unknown')}</h3>
                        <p>ID: ${escHtml(dev.device_id.substring(0, 8))}</p>
                        <p>OS: ${escHtml(dev.os)}</p>
                    </div>
                    <button class="btn-tool btn-danger remove-device-btn" style="padding: 5px; margin: 0; background: rgba(255,59,48,0.2);" title="Remove Device" data-id="${dev.device_id}">
                        <i class='bx bx-trash'></i>
                    </button>
                </div>
            `;
            
            // Handle clicking the card to connect
            card.addEventListener('click', (e) => {
                // Ignore clicks on the remove button
                if (e.target.closest('.remove-device-btn')) return;
                connectToDevice(dev.device_id);
            });

            // Handle clicking the remove button
            const removeBtn = card.querySelector('.remove-device-btn');
            if (removeBtn) {
                removeBtn.addEventListener('click', (e) => {
                    e.stopPropagation();
                    if(confirm("Are you sure? This will permanently terminate the remote agent and delete it from the target computer.")) {
                        // We need to connect to it briefly to send the shutdown command
                        if (activeDeviceId !== dev.device_id) {
                            send({
                                type: 'connect_device',
                                payload: { device_id: dev.device_id, quality: 'low', fps: 15 }
                            });
                            // Wait a moment for connection before sending shutdown
                            setTimeout(() => {
                                send({ type: 'agent_shutdown', payload: null });
                                send({ type: 'disconnect' });
                                activeDeviceId = null;
                                requestDeviceList(); // Refresh list immediately
                            }, 500);
                        } else {
                            // If we're already connected to it (though we shouldn't be if we're on dashboard view)
                            send({ type: 'agent_shutdown', payload: null });
                            stopSession();
                        }
                    }
                });
            }

            ui.devices.appendChild(card);
        });
    }

    function connectToDevice(deviceId) {
        if (activeDeviceId) send({ type: 'disconnect' });
        activeDeviceId = deviceId;
        send({
            type: 'connect_device',
            payload: { device_id: deviceId, quality: controls.quality.value, fps: parseInt(controls.fps.value) }
        });
        ui.sysinfo.classList.remove('open');
        ui.files.classList.remove('open');
        ui.remoteCursor.style.display = 'none';
    }

    function renderSystemInfo(info) {
        if (!info) return;
        let html = `<table class="data-table">
            <tr><th colspan="2">System Overview</th></tr>
            <tr><td>Hostname</td><td>${escHtml(info.hostname)}</td></tr>
            <tr><td>OS</td><td>${escHtml(info.os_version)}</td></tr>
            <tr><td>CPU</td><td>${escHtml(info.cpu_model)}</td></tr>
            <tr><td>Memory</td><td>${info.memory_used_mb} MB / ${info.memory_total_mb} MB</td></tr>
        </table>`;
        ui.sysinfoContent.innerHTML = html;
    }

    // --- Input & Cursor Logic --- //
    
    function getNorm(e) {
        if (!canvas.width || !canvas.height) return null;
        const rect = canvas.getBoundingClientRect();
        const imgR = canvas.width / canvas.height;
        const boxR = rect.width / rect.height;
        let rw = rect.width, rh = rect.height, ox = 0, oy = 0;
        if (boxR > imgR) { rw = rect.height * imgR; ox = (rect.width - rw) / 2; }
        else { rh = rect.width / imgR; oy = (rect.height - rh) / 2; }
        
        let x = e.clientX - rect.left - ox;
        let y = e.clientY - rect.top - oy;
        
        if (x < 0 || x > rw || y < 0 || y > rh) return null;
        return { 
            nx: Math.max(0, Math.min(1, x / rw)), 
            ny: Math.max(0, Math.min(1, y / rh))
        };
    }

    // The fake cursor logic is disabled because the actual OS cursor is already natively composited 
    // into the image stream by ScreenCaptureKit (macOS) and GDI (Windows).
    function updateRemoteCursor(rx, ry) {
        // ui.remoteCursor.style.display = 'none';
    }

    function sendInput(payload) { 
        const msg = { type: 'input_event', payload: payload };
        if (webrtcInputChannel && webrtcInputChannel.readyState === 'open') {
            webrtcInputChannel.send(JSON.stringify(msg));
        } else {
            send(msg); 
        }
    }

    ui.canvasWrapper.addEventListener('mousemove', e => {
        const c = getNorm(e); if (!c) return;
        // Don't spam mouse moves constantly
        if (performance.now() % 50 < 16) { 
            sendInput({ type: 'mouse_move', x: c.nx, y: c.ny });
            ui.remoteCursor.style.display = 'none';
        }
    });

    ui.canvasWrapper.addEventListener('mousedown', e => {
        const c = getNorm(e); if (!c) return;
        sendInput({ type: 'mouse_down', button: e.button === 2 ? 'right' : 'left', x: c.nx, y: c.ny });
    });
    ui.canvasWrapper.addEventListener('mouseup', e => {
        const c = getNorm(e); if (!c) return;
        sendInput({ type: 'mouse_up', button: e.button === 2 ? 'right' : 'left', x: c.nx, y: c.ny });
    });
    ui.canvasWrapper.addEventListener('contextmenu', e => e.preventDefault());
    
    document.addEventListener('keydown', e => {
        if (!ui.views.session.classList.contains('active')) return;
        
        // Developer Diagnostics Toggle (Ctrl+Shift+D)
        if (e.ctrlKey && e.shiftKey && e.code === 'KeyD') {
            e.preventDefault();
            const diag = document.getElementById('diagnostics-overlay');
            diag.style.display = diag.style.display === 'none' ? 'block' : 'none';
            return;
        }

        if (['ArrowUp','ArrowDown','Space','Tab'].includes(e.code)) e.preventDefault();
        sendInput({ type: 'key_press', key: e.key, code: e.code, alt_key: e.altKey, ctrl_key: e.ctrlKey, shift_key: e.shiftKey, meta_key: e.metaKey });
    });

    // Toolbar logic
    btns.pin.addEventListener('click', () => {
        isPinned = !isPinned;
        if (isPinned) {
            ui.toolbar.classList.add('pinned');
            btns.pin.classList.add('active');
        } else {
            ui.toolbar.classList.remove('pinned');
            btns.pin.classList.remove('active');
        }
    });
    
    // Auto-show toolbar on mouse hover near top
    document.addEventListener('mousemove', e => {
        if (!ui.views.session.classList.contains('active') || isPinned) return;
        if (e.clientY < 50) {
            ui.toolbar.style.transform = 'translate(-50%, 0)';
        } else if (e.clientY > ui.toolbar.offsetHeight + 10) {
            ui.toolbar.style.transform = 'translate(-50%, -100%)';
        }
    });

    // Pinger for Network RTT
    setInterval(() => {
        if (ws && ws.readyState === WebSocket.OPEN && ui.views.session.classList.contains('active')) {
            send({ type: 'ping', payload: { timestamp: performance.now() } });
        }
    }, 1000);

    function stopSession() {
        send({ type: 'disconnect' });
        activeDeviceId = null;
        if (pc) { pc.close(); pc = null; }
        switchView('dashboard');
        const ctx = canvas.getContext('2d');
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        requestDeviceList();
        showToast('Session ended');
    }

    btns.refresh.addEventListener('click', requestDeviceList);
    btns.disconnect.addEventListener('click', stopSession);

    btns.selfDestruct.addEventListener('click', () => {
        if(confirm("Are you sure? This will permanently terminate the remote agent and delete it from the target computer.")) {
            send({ type: 'agent_shutdown', payload: null });
            stopSession();
        }
    });

    btns.sysinfo.addEventListener('click', () => {
        ui.files.classList.remove('open');
        ui.sysinfo.classList.toggle('open');
        if (ui.sysinfo.classList.contains('open')) send({ type: 'system_info_request' });
    });
    btns.files.addEventListener('click', () => {
        ui.sysinfo.classList.remove('open');
        ui.files.classList.toggle('open');
    });
    btns.closeSysinfo.addEventListener('click', () => ui.sysinfo.classList.remove('open'));
    btns.closeFiles.addEventListener('click', () => ui.files.classList.remove('open'));

    btns.copy.addEventListener('click', () => { if(activeDeviceId) send({ type: 'clipboard_request' }); });
    btns.paste.addEventListener('click', async () => {
        if(!activeDeviceId) return;
        try {
            const t = await navigator.clipboard.readText();
            if(t) send({ type: 'clipboard_update', payload: { text: t } });
        } catch(e) { showToast('Local clipboard error','bx-error','var(--danger)'); }
    });
    
    controls.quality.addEventListener('change', () => send({ type: 'quality_control', payload: { quality: controls.quality.value, fps: parseInt(controls.fps.value) }}));
    controls.fps.addEventListener('change', () => send({ type: 'quality_control', payload: { quality: controls.quality.value, fps: parseInt(controls.fps.value) }}));

    btns.fullscreen.addEventListener('click', () => {
        if (document.fullscreenElement) document.exitFullscreen();
        else document.documentElement.requestFullscreen();
    });

    // File Drop
    const uz = document.getElementById('upload-zone');
    uz.addEventListener('dragover', e => e.preventDefault());
    uz.addEventListener('drop', e => {
        e.preventDefault();
        if(!activeDeviceId) return;
        const files = e.dataTransfer.files;
        for(let i=0; i<files.length; i++) uploadFile(files[i]);
    });

    function uploadFile(file) {
        const tid = 'tx-'+Math.random().toString(36).substr(2,9);
        const chunk = 524288;
        let offset = 0;
        send({ type: 'file_transfer_start', payload: { transfer_id: tid, filename: file.name, size: file.size, direction: 'upload' }});
        const r = new FileReader();
        r.onload = e => {
            send({ type: 'file_transfer_chunk', payload: { transfer_id: tid, offset: offset, data: e.target.result.split(',')[1] }});
            offset += chunk;
            if (offset < file.size) r.readAsDataURL(file.slice(offset, offset+chunk));
            else {
                send({ type: 'file_transfer_end', payload: { transfer_id: tid, success: true }});
                showToast(`Finished: ${file.name}`, 'bx-check', 'var(--success)');
            }
        };
        r.readAsDataURL(file.slice(0, chunk));
    }

    function escHtml(s) { return s?s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;'):''; }

    setInterval(() => { if (ws && ws.readyState === WebSocket.OPEN && !activeDeviceId) requestDeviceList(); }, 5000);
    connect();
})();
