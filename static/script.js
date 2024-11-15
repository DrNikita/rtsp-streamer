// Устанавливаем WebSocket-соединение
let ws = new WebSocket("{{.}}");

function init(){
  // Получаем список видео по эндпоинту и добавляем его в боковую панель
  fetch("http://localhost:8080/video-list")
    .then(response => response.json())
    .then(videoList => {
      let videoListContainer = document.getElementById("videoList");
      videoList.forEach(video => {
        let li = document.createElement("li");
      
        // Название видеопотока с обрезанием текста
        let videoTitle = document.createElement("span");
        videoTitle.textContent = video;
        videoTitle.classList.add("video-title");
        videoTitle.onclick = () => startVideoStream(video);
      
        // Разделительная линия
        let separator = document.createElement("div");
        separator.classList.add("separator");
      
        // Зона удаления
        let deleteArea = document.createElement("div");
        deleteArea.classList.add("delete-area");
        deleteArea.onclick = (e) => {
          e.stopPropagation();
          deleteVideoStream(video);
        };
      
        // Кнопка удаления (крестик)
        let deleteBtn = document.createElement("button");
        deleteBtn.innerHTML = "&times;";
        deleteBtn.classList.add("delete-btn");
        deleteArea.appendChild(deleteBtn);
      
        li.appendChild(videoTitle);
        li.appendChild(separator);
        li.appendChild(deleteArea);
        videoListContainer.appendChild(li);
      });
    })
    .catch(error => console.error("Error fetching video list:", error));

  let pc = new RTCPeerConnection();

  pc.ontrack = function (event) {
    if (event.track.kind === 'audio') {
      return;
    }

    let el = document.createElement(event.track.kind);
    el.srcObject = event.streams[0];
    el.autoplay = true;
    el.controls = true;

    document.getElementById('remoteVideos').appendChild(el);

    event.track.onmute = function(event) {
      el.play();
    };

    event.streams[0].onremovetrack = ({ track }) => {
      if (el.parentNode) {
        el.parentNode.removeChild(el);
      }
    };
  };

  ws.onclose = function(evt) {
    window.alert("WebSocket has closed");
  };

  ws.onmessage = function(evt) {
    let msg = JSON.parse(evt.data);
    if (!msg) {
      return console.log('failed to parse msg');
    }

    switch (msg.event) {
      case 'offer':
        let offer = JSON.parse(msg.data);
        if (!offer) {
          return console.log('failed to parse answer');
        }
        pc.setRemoteDescription(offer);
        pc.createAnswer().then(answer => {
          pc.setLocalDescription(answer);
          ws.send(JSON.stringify({ event: 'answer', data: JSON.stringify(answer) })); 
        });
        return;
      
      case 'candidate':
        let candidate = JSON.parse(msg.data);
        if (!candidate) {
          return console.log('failed to parse candidate');
        }
      
        pc.addIceCandidate(candidate);
    }
  };

  ws.onerror = function(evt) {
    console.log("ERROR: " + evt.data);
  };
}

// Функция для отправки сообщения "publish" при выборе видео
function startVideoStream(video) {
  console.log("Selected video:", video);
  ws.send(JSON.stringify({ event: 'publish', data: JSON.stringify(video) }));
}

// Функция для удаления видеопотока
function deleteVideoStream(video) {
  fetch(`http://localhost:8080/delete-video?name=${encodeURIComponent(video)}`, {
    method: "DELETE"
  })
    .then(response => {
      if (response.ok) {
        console.log("Deleted video:", video);
        document.getElementById("videoList").innerHTML = "";
        fetch("http://localhost:8080/video-list")
          .then(response => response.json())
          .then(newVideoList => {
            newVideoList.forEach(newVideo => {
              let li = document.createElement("li");

              let videoTitle = document.createElement("span");
              videoTitle.textContent = newVideo;
              videoTitle.classList.add("video-title");
              videoTitle.onclick = () => startVideoStream(newVideo);

              let separator = document.createElement("div");
              separator.classList.add("separator");

              let deleteArea = document.createElement("div");
              deleteArea.classList.add("delete-area");
              deleteArea.onclick = (e) => {
                e.stopPropagation();
                deleteVideoStream(newVideo);
              };

              let deleteBtn = document.createElement("button");
              deleteBtn.innerHTML = "&times;";
              deleteBtn.classList.add("delete-btn");
              deleteArea.appendChild(deleteBtn);

              li.appendChild(videoTitle);
              li.appendChild(separator);
              li.appendChild(deleteArea);
              document.getElementById("videoList").appendChild(li);
            });
          });
      }
    })
    .catch(error => console.error("Error deleting video:", error));
}