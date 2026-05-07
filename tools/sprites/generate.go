package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

const (
	frameCount = 8
	size       = 32
)

var (
	outline = color.NRGBA{28, 31, 42, 255}
	shine   = color.NRGBA{232, 253, 255, 255}
	eye     = color.NRGBA{12, 15, 24, 255}
	cheek   = color.NRGBA{255, 79, 134, 255}
	palette = [][3]color.NRGBA{
		{{18, 82, 150, 255}, {58, 166, 255, 255}, {138, 231, 255, 255}},
		{{28, 120, 96, 255}, {70, 211, 140, 255}, {179, 255, 190, 255}},
		{{122, 64, 174, 255}, {187, 104, 255, 255}, {244, 184, 255, 255}},
		{{172, 84, 32, 255}, {255, 169, 64, 255}, {255, 230, 128, 255}},
	}
)

func main() {
	outDir := filepath.Join("assets", "sprites")
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		panic(err)
	}
	for frame := 0; frame < frameCount; frame++ {
		img := image.NewNRGBA(image.Rect(0, 0, size, size))
		colors := palette[frame%len(palette)]
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				img.SetNRGBA(x, y, colors[0])
			}
		}
		drawFrame(img, frame, colors)
		path := filepath.Join(outDir, "bench_blob_"+string(rune('0'+frame))+".png")
		file, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		if err := png.Encode(file, img); err != nil {
			file.Close()
			panic(err)
		}
		if err := file.Close(); err != nil {
			panic(err)
		}
	}
}

func drawFrame(img *image.NRGBA, frame int, colors [3]color.NRGBA) {
	bob := []int{0, -1, -2, -1, 0, 1, 2, 1}[frame]
	squash := 0
	if frame == 2 || frame == 6 {
		squash = 1
	}
	left := 5
	right := 26
	top := 6 + bob + squash
	bottom := 27 + bob - squash

	fillRect(img, left, top, right, bottom, colors[1])
	fillRect(img, left+2, top+2, right-2, bottom-2, colors[2])
	fillRect(img, left+2, bottom-7, right-2, bottom-2, colors[0])
	for stripe := -32 + frame*4; stripe < 32; stripe += 8 {
		for i := 0; i < 9; i++ {
			set(img, left+3+stripe+i, top+3+i, colors[1])
			set(img, left+4+stripe+i, top+3+i, colors[1])
		}
	}
	drawBorder(img, left, top, right, bottom, outline)

	shineX := 9 + frame%4
	fillRect(img, shineX, top+4, shineX+4, top+5, shine)
	fillRect(img, shineX, top+6, shineX+2, top+7, shine)

	eyeOffset := []int{0, 0, 1, 1, 0, -1, -1, 0}[frame]
	fillRect(img, 11+eyeOffset, top+10, 13+eyeOffset, top+13, eye)
	fillRect(img, 20+eyeOffset, top+10, 22+eyeOffset, top+13, eye)
	set(img, 12+eyeOffset, top+10, shine)
	set(img, 21+eyeOffset, top+10, shine)

	mouthY := top + 17
	if frame == 3 || frame == 4 {
		fillRect(img, 14, mouthY, 18, mouthY+1, eye)
	} else {
		fillRect(img, 14, mouthY, 19, mouthY, eye)
	}
	set(img, 9, top+15, cheek)
	set(img, 23, top+15, cheek)

	set(img, left+1+frame%5, bottom-1, shine)
	set(img, right-1-frame%5, top+1, colors[0])
}

func fillRect(img *image.NRGBA, left, top, right, bottom int, c color.NRGBA) {
	for y := top; y <= bottom; y++ {
		for x := left; x <= right; x++ {
			set(img, x, y, c)
		}
	}
}

func drawBorder(img *image.NRGBA, left, top, right, bottom int, c color.NRGBA) {
	for x := left; x <= right; x++ {
		set(img, x, top, c)
		set(img, x, bottom, c)
	}
	for y := top; y <= bottom; y++ {
		set(img, left, y, c)
		set(img, right, y, c)
	}
}

func set(img *image.NRGBA, x, y int, c color.NRGBA) {
	if x < 0 || y < 0 || x >= size || y >= size {
		return
	}
	img.SetNRGBA(x, y, c)
}
