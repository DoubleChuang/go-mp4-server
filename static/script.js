// JavaScript function to jump to a specific video ID when input field loses focus
document.getElementById("my-video").addEventListener("blur", function() {
    jumpToVideo();
  });


function jumpToVideo() {
    var input = document.getElementById("my-video").value;
    console.log(input)
    var videoId = parseInt(input);
    if (!isNaN(videoId)) {
        window.location.href = "/video/" + videoId;
    }
}

function selectAllText() {
    var input = document.getElementById("my-video");
    input.focus();
    input.select();
}

// handle changing the video based on dropdown selection
function changeVideo(selectElement) {
    var selectedIdx = selectElement.value;
    var videoUrl = "/video/" + selectedIdx;
    window.location.href = videoUrl;
}



// Function to re-create the video element with a dynamic src
function createVideoElement(src) {
    var container = document.getElementById('video-container');
    container.innerHTML = `
    <video id="my-video" class="video-js vjs-default-skin" controls>
    <source src="${src}" type="video/mp4">
    Your browser does not support the video tag.
    </video>
`;
}

// Function to limit FOV value within a given range
function clampFOV(fov) {
    return Math.max(30, Math.min(fov, 150));  // Clamp FOV between 30 and 150
}

// Function to initialize the player with or without VR
function initializePlayer(enableVR, src) {
    // Destroy the existing player instance if it exists
    if (player) {
        player.dispose();        
    }

    // Re-create the video element in the DOM with the dynamic src
    createVideoElement(src);

    // Re-initialize the player
    window.player = videojs('my-video', {
    fluid: true,  // Let the video size adjust with the container
});

    // If VR mode is enabled, initialize the VR plugin
    if (enableVR) {
        player.vr({
            projection: '360',
            fov: 90, // Initial Field of View (FOV)            
        });

        // Zoom slider event listener
        document.getElementById('zoomRange').addEventListener('input', function (e) {
            var newFov = clampFOV(e.target.value);  // Clamp the FOV value
            player.vr().camera.fov = newFov;
            player.vr().camera.updateProjectionMatrix();
        });

        // Add mouse wheel zoom functionality
        document.getElementById('my-video').addEventListener('wheel', function (event) {
            // Prevent the default scroll action
            event.preventDefault();

            // Adjust FOV based on the direction of the mouse wheel
            var currentFov = player.vr().camera.fov;
            if (event.deltaY < 0) {
            // Scroll up, zoom in (decrease FOV)
            currentFov = clampFOV(currentFov - 2);  // Decrease and clamp FOV
            } else {
            // Scroll down, zoom out (increase FOV)
            currentFov = clampFOV(currentFov + 2);  // Increase and clamp FOV
            }

            // Update FOV and the projection matrix
            player.vr().camera.fov = currentFov;
            player.vr().camera.updateProjectionMatrix();

            // Sync the zoom range slider with the updated FOV
            document.getElementById('zoomRange').value = currentFov;
        });
    }
}

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

