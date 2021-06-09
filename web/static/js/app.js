var mediaSource = new MediaSource();
var sourceBuffer;
var queue = [];
var ws;

function pushPacket(data) {
  queue.push(new Uint8Array(data));
  loadPacket();
}

function loadPacket() {
  // get package data from queue FIFO
  const data = queue.shift();

  if (!data || sourceBuffer.updating) {
    // sourceBuffer is busy or no packet data to display
    return; // nothings to load
  }

  // add to sourceBuffer for displaying
  sourceBuffer.appendBuffer(data);
}

var protocol = "ws";
if (location.protocol.indexOf("s") >= 0) {
  protocol = "wss";
}

function handleSourceOpen() {
  var inputVal = $("#suuid").val();
  var port = $("#port").val();

  ws = new WebSocket(
    protocol + "://127.0.0.1" + port + "/ws/live?suuid=" + inputVal
  );
  ws.binaryType = "arraybuffer";

  ws.onopen = function () {
    // initial sourceBuffer for format video/mp4; codecs="avc1.42C01E"
    sourceBuffer = mediaSource.addSourceBuffer(
      'video/mp4; codecs="avc1.42C01E"'
    );
    sourceBuffer.mode = "segments";

    sourceBuffer.addEventListener("updateend", () => {
      // load packet data when sourceBuffer idle
      loadPacket();
    });
  };

  ws.onmessage = function (event) {
    pushPacket(event.data);
  };
}

var livestream = document.getElementById("livestream");

function startup() {
  mediaSource.addEventListener("sourceopen", handleSourceOpen, false);
  livestream.src = window.URL.createObjectURL(mediaSource);
}

$(document).ready(function () {
  startup();
  var suuid = $("#suuid").val();
  $("#" + suuid).addClass("active");
});
