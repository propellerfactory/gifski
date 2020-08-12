package gifski

import (
	"bytes"
	"image"
	_ "image/jpeg"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleGIF(t *testing.T) {
	assert := assert.New(t)

	data, err := ioutil.ReadFile("test_images/kitten.jpg")
	assert.NoError(err)
	i, _, err := image.Decode(bytes.NewReader(data))
	assert.NoError(err)
	assert.NotNil(i)

	width := i.Bounds().Max.X
	height := i.Bounds().Max.Y
	aspectRatio := float32(width) / float32(height)
	t.Logf("found image of %d x %d (aspect ratio %g)", width, height, aspectRatio)

	outputWidth := 512
	outputHeight := int(float32(outputWidth) / aspectRatio)
	t.Logf("creating GIF of %d x %d", outputWidth, outputHeight)

	outputGIF, err := os.Create("test_images/kitten.gif")
	assert.NoError(err)
	assert.NotNil(outputGIF)

	g, err := NewGifski(&Settings{
		Width:          uint(outputWidth),
		Height:         uint(outputHeight),
		Quality:        80,
		Fast:           false,
		Once:           false,
		ReportProgress: true,
	}, outputGIF)
	assert.NoError(err)
	assert.NotNil(g)

	go func() {
		for {
			frameNumber, done := <-g.Progress()
			if !done {
				break
			}
			t.Logf("computing frame %d", frameNumber)
		}
		t.Logf("done!")
	}()

	pixels := make([]byte, width*height*4)
	index := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := i.At(x, y).RGBA()
			pixels[index+0] = byte(r >> 8)
			pixels[index+1] = byte(g >> 8)
			pixels[index+2] = byte(b >> 8)
			pixels[index+3] = byte(a >> 8)
			index = index + 4
		}
	}
	err = g.AddFrame(0, uint(width), uint(height), pixels, 0.0)
	assert.NoError(err)
	err = g.AddFrame(1, uint(width), uint(height), pixels, 1.0)
	assert.NoError(err)
	t.Logf("finishing")
	err = g.Finish()
	assert.NoError(err)

	t.Logf("closing file")
	err = outputGIF.Close()
	assert.NoError(err)
}
