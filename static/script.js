// JavaScript function to jump to a specific video ID when input field loses focus
document.getElementById("video-id-input").addEventListener("blur", function() {
    jumpToVideo();
  });


function jumpToVideo() {
    var input = document.getElementById("video-id-input").value;
    var videoId = parseInt(input);
    if (!isNaN(videoId)) {
        window.location.href = "/video/" + videoId;
    }
}

function selectAllText() {
    var input = document.getElementById("video-id-input");
    input.focus();
    input.select();
}

// handle changing the video based on dropdown selection
function changeVideo(selectElement) {
    var selectedIdx = selectElement.value;
    var videoUrl = "/video/" + selectedIdx;
    window.location.href = videoUrl;
}


