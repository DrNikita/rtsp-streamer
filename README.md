# Description
This service allows to store your video and stream them as RTSP stream. Then the server display the stream on "localhost:8080/" using WebRTC. You can create multiple streams.

Plan for this service for now:
    DownloadObject func for minion to save video to tmp dir;
    mb wg into service to monitor when to delete tmp dir;
    


Command to publish vidofile on RTSP-server:
```
ffmpeg -re -stream_loop -1 -i data/newAnime.mkv -c copy -f rtsp rtsp://localhost:8554/{file_name}
```

Command to convert vidofile codec:
```
ffmpeg -i data/anime.mp4 -c:v libx254 -crf 18 converted_anime.mp4
```

Command to check videofile codec:
```
ffprobe -v error -select_streams v:0 -show_entries stream=codec_name -of default=noprint_wrappers=1:nokey=1 video.mkv
```

CHECK_CONVERSION:::: https://www.bannerbear.com/blog/converting-video-and-audio-formats-using-ffmpeg/