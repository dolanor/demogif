// demogif allow you to capture your screen or part of as an animated GIF.
package demogif

import (
	"context"
	"image"
	"image/gif"
	"io"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/andybons/gogif"
	"github.com/vova616/screenshot"
)

// ScreenCapture captures a gif animation until the context is done.
// The video is taken from the whole main screen with the defined fps.
func ScreenCapture(ctx context.Context, w io.Writer, fps int) {
	rect, err := screenshot.ScreenRect()
	if err != nil {
		log.Println("cannot get screen dimensions, not capturing video:", err)
		return
	}
	Capture(ctx, w, rect, fps)
}

// Capture captures a gif animation until the context is done.
// The video is taken from the rect coordinates with the defined fps.
func Capture(ctx context.Context, w io.Writer, rect image.Rectangle, fps int) {
	freq := float64(time.Second) / float64(fps)
	// pre-allocate 1 minute worth of animation images
	anim := make([]image.Image, 0, fps*60)
	frameTicker := time.NewTicker(time.Duration(freq))

loop:
	for range frameTicker.C {
		select {
		case <-ctx.Done():
			break loop
		default:
		}
		sc, err := screenshot.CaptureRect(rect)
		if err != nil {
			log.Fatal(err)
		}
		anim = append(anim, sc)
	}

	out := &gif.GIF{}
	var wg sync.WaitGroup
	concurrency := runtime.NumCPU() * 2
	pool := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		pool <- struct{}{}
	}

	for _, img := range anim {
		img := img
		pImg := image.NewPaletted(img.Bounds(), nil)
		quantizer := gogif.MedianCutQuantizer{NumColor: 64}

		<-pool
		wg.Add(1)
		go func() {
			quantizer.Quantize(pImg, img.Bounds(), img, image.ZP)
			wg.Done()
			pool <- struct{}{}
		}()
		out.Image = append(out.Image, pImg)
		out.Delay = append(out.Delay, 0)
	}
	wg.Wait()

	err := gif.EncodeAll(w, out)
	if err != nil {
		log.Fatal(err)
	}
}
