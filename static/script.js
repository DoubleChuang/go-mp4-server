// handle changing the video based on dropdown selection
function changeVideo(selectElement) {
    var selectedIdx = selectElement.value;
    window.location.href = "/video/" + selectedIdx;
}

// --- Player state ---
var art = null;       // Artplayer instance
var vjsPlayer = null; // Video.js instance (360 mode)

function clampFOV(fov) {
    return Math.max(30, Math.min(fov, 150));
}

// Destroy whichever player is currently active
function destroyCurrentPlayer() {
    if (vjsPlayer) {
        vjsPlayer.dispose();
        vjsPlayer = null;
    }
    if (art) {
        art.destroy();
        art = null;
    }
}

// Reset the player container to a fresh div
function resetContainer(id) {
    var container = document.getElementById('video-container');
    container.innerHTML = '<div id="' + id + '"></div>';
    return document.getElementById(id);
}

// --- Normal mode: Artplayer ---
function initArtplayer(src) {
    destroyCurrentPlayer();
    resetContainer('artplayer-app');

    art = new Artplayer({
        container: '#artplayer-app',
        url: src,
        volume: 0.8,
        autoplay: false,
        pip: true,
        setting: true,
        playbackRate: true,
        aspectRatio: true,
        fullscreen: true,
        fullscreenWeb: true,
        miniProgressBar: true,
        mutex: true,
    });
}

// --- 360 mode: Video.js + videojs-vr ---
function init360Player(src) {
    destroyCurrentPlayer();

    // Video.js requires an actual <video> element
    var container = document.getElementById('video-container');
    container.innerHTML = '<video id="vjs-360" class="video-js vjs-default-skin" controls style="width:100%;aspect-ratio:16/9"><source src="' + src + '" type="video/mp4"></video>';

    vjsPlayer = videojs('vjs-360', { fluid: true });

    vjsPlayer.ready(function () {
        vjsPlayer.vr({ projection: '360' });
    });

    // Zoom slider → FOV
    document.getElementById('zoomRange').addEventListener('input', function (e) {
        var fov = clampFOV(parseInt(e.target.value));
        if (vjsPlayer.vr && vjsPlayer.vr().camera) {
            vjsPlayer.vr().camera.fov = fov;
            vjsPlayer.vr().camera.updateProjectionMatrix();
        }
    });

    // Mouse wheel → FOV
    container.addEventListener('wheel', function (e) {
        e.preventDefault();
        if (!vjsPlayer.vr || !vjsPlayer.vr().camera) return;
        var slider = document.getElementById('zoomRange');
        var newFov = clampFOV(parseInt(slider.value) + (e.deltaY < 0 ? -2 : 2));
        slider.value = newFov;
        vjsPlayer.vr().camera.fov = newFov;
        vjsPlayer.vr().camera.updateProjectionMatrix();
    }, { passive: false });
}

document.addEventListener('DOMContentLoaded', function () {
    var appEl = document.getElementById('artplayer-app');
    if (!appEl) return;

    var src = appEl.dataset.src;
    var toggle360 = document.getElementById('toggle360');
    var zoomRange = document.getElementById('zoomRange');

    // Start in normal mode
    initArtplayer(src);
    zoomRange.disabled = true;

    toggle360.addEventListener('change', function () {
        if (this.checked) {
            init360Player(src);
            zoomRange.disabled = false;
        } else {
            initArtplayer(src);
            zoomRange.disabled = true;
            zoomRange.value = 90;
        }
    });
});

// // Initialize the player with the dynamic video source without VR initially
var videoSrc = document.getElementById("my-video").querySelector('source').getAttribute('src');
window.player = videojs('my-video', {
    fluid: true,  // Let the video size adjust with the container
});

// Handle checkbox toggle to enable/disable 360 mode
document.getElementById('toggle360').addEventListener('change', function (e) {
    var videoSrc = document.getElementById("my-video").querySelector('source').getAttribute('src');
    console.log("videoSrc: "+videoSrc)
    if (e.target.checked) {
    initializePlayer(true, videoSrc);  // Enable 360 mode with dynamic src
    } else {
    initializePlayer(false, videoSrc); // Disable 360 mode with dynamic src
    }
});

