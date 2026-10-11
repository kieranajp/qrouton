These synthetic fixtures were generated with ImageMagick 7 and FFmpeg. They contain no external artwork.

```sh
magick -size 48x32 xc:none -fill '#8aadf4' -draw 'rectangle 4,4 43,27' small.png
magick -size 960x640 gradient:'#8aadf4-#363a4f' large.png
magick small.png -background white -alpha remove sample.jpg
magick small.png sample.gif
magick small.png sample.webp
magick small.png sample.avif
```

The two one-second clips were generated with FFmpeg.

```sh
ffmpeg -f lavfi -i testsrc=size=160x90:rate=10 -t 1 -c:v libvpx-vp9 -an sample.webm
ffmpeg -f lavfi -i testsrc=size=160x90:rate=10 -t 1 -c:v libx264 -pix_fmt yuv420p -an sample.mp4
```

The small PNG retains transparent margins. JPEG, GIF, WebP, and AVIF exercise browser decoding of each admitted raster format.

`not-a-video.mp4` is a byte copy of `small.png`, so Chromium rejects it with `MEDIA_ERR_SRC_NOT_SUPPORTED`.
