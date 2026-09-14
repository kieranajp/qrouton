These synthetic fixtures were generated with ImageMagick 7. They contain no external artwork.

```sh
magick -size 48x32 xc:none -fill '#8aadf4' -draw 'rectangle 4,4 43,27' small.png
magick -size 960x640 gradient:'#8aadf4-#363a4f' large.png
magick small.png -background white -alpha remove sample.jpg
magick small.png sample.gif
magick small.png sample.webp
magick small.png sample.avif
```

The small PNG retains transparent margins. JPEG, GIF, WebP, and AVIF exercise browser decoding of each admitted raster format.
