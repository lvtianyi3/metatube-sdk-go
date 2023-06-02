package main

import (
	"github.com/metatube-community/metatube-sdk-go/constant"
	"github.com/metatube-community/metatube-sdk-go/imageutil"
	"image/jpeg"

	"image"
	"os"
)

func main() {
	imgFile, err := os.Open("C:/Users/Administrator/Desktop/105bd00d6e7d1693.jpg")
	if err != nil {
		panic(err)
	}
	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)
	if err != nil {
		panic(err)
	}

	img = imageutil.CropImagePosition(img, constant.PrimaryImageRatio, 1)

	// 将image对象保存为jpg格式图片
	file, err := os.Create("image.jpg")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	if err = jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		panic(err)
	}
}
