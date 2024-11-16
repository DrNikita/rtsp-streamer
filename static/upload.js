// Открытие селектора файла для загрузки видео
function openFileSelector() {
    document.getElementById("videoFileInput").click();
  }
  
  // Функция для загрузки видеофайла на сервер
  function uploadVideoFile() {
    const videoFileInput = document.getElementById("videoFileInput");
    const file = videoFileInput.files[0];
    
    if (!file) return;
  
    const formData = new FormData();
    formData.append("video", file);
  
    fetch("http://localhost:8080/upload", {
      method: "POST",
      body: formData
    })
    .then(response => {
      if (!response.ok) {
        throw new Error("Ошибка загрузки видео");
      }
      // Если загрузка успешна, обновляем список видеофайлов
      updateVideoList();
    })
    .catch(error => {
      console.error("Ошибка при загрузке видео:", error);
    })
    .finally(() => {
      // Очищаем input после загрузки
      videoFileInput.value = "";
    });
  }
  