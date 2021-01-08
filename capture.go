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

type sshot struct {
	img *image.RGBA
	ts  time.Time
}
type tsimg struct {
	img *image.Paletted
	ts  time.Time
}

// Capture captures a gif animation until the context is done.
// The video is taken from the rect coordinates with the defined fps.
func Capture(ctx context.Context, w io.Writer, rect image.Rectangle, fps int) {
	freq := float64(time.Second) / float64(fps)
	// pre-allocate 1 minute worth of animation images
	anim := make([]image.Image, 0, fps*60)

	screens := make(chan sshot, 100)

	paletteds := make(chan tsimg, 100)
	frameTicker := time.NewTicker(time.Duration(freq))

	go quantize(screens, paletteds)
	out := &gif.GIF{}
	done := make(chan struct{})
	go gifer(paletteds, out, done)

	i := 0
loop:
	for range frameTicker.C {
		select {
		case <-ctx.Done():
			close(screens)
			break loop
		default:
		}
		sc, err := screenshot.CaptureRect(rect)
		if err != nil {
			log.Fatal(err)
		}
		println("screen", i)

		//anim = append(anim, sc)
		screens <- sshot{
			img: sc,
			ts:  time.Now(),
		}
		i++
	}

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

	<-done
	err := gif.EncodeAll(w, out)
	if err != nil {
		log.Fatal(err)
	}
}

func quantize(screens <-chan sshot, paletteds chan<- tsimg) {
	i := 0
	concurrency := runtime.NumCPU() * 2
	pool := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		pool <- struct{}{}
	}

	var wg sync.WaitGroup
	quantizer := gogif.MedianCutQuantizer{NumColor: 64}
	for sc := range screens {
		<-pool
		sc := sc
		wg.Add(1)
		go func() {
			pImg := image.NewPaletted(sc.img.Bounds(), nil)
			quantizer.Quantize(pImg, sc.img.Bounds(), sc.img, image.ZP)
			paletteds <- tsimg{
				img: pImg,
				ts:  sc.ts,
			}
			log.Println("palette", i)
			i++
			pool <- struct{}{}
			wg.Done()
		}()
	}
	wg.Wait()
	println("closing paletteds")
	close(paletteds)
}

func gifer(imgs <-chan tsimg, out *gif.GIF, done chan<- struct{}) {
	concurrency := runtime.NumCPU() * 2
	pool := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		pool <- struct{}{}
	}

	i := 0
	for img := range imgs {
		log.Println("gif", i)
		img := img
		<-pool
		go func() {
			out.Image = append(out.Image, img.img)
			out.Delay = append(out.Delay, 0)
			i++
			pool <- struct{}{}
		}()
	}
	done <- struct{}{}
	close(done)
}
