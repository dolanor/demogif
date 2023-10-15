// demogif allow you to capture your screen or part of as an animated GIF.
package demogif

import (
	"context"
	"fmt"
	"image"
	"image/gif"
	"io"
	"log"
	"runtime"
	"sort"
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
	img image.RGBA
	ts  time.Time
}
type tsimg struct {
	img image.Paletted
	ts  time.Time
}

type tsimgSorter struct {
	imgs []tsimg
}

func (s *tsimgSorter) Len() int {
	return len(s.imgs)
}
func (s *tsimgSorter) Swap(i, j int) {
	s.imgs[i], s.imgs[j] = s.imgs[j], s.imgs[i]
}
func (s *tsimgSorter) Less(i, j int) bool {
	cond := s.imgs[i].ts.Before(s.imgs[j].ts)
	fmt.Printf("%s < %s : %t\n", s.imgs[i].ts, s.imgs[j].ts, cond)
	return cond
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
	var wg sync.WaitGroup
loop:
	for t := range frameTicker.C {
		select {
		case <-ctx.Done():
			break loop
		default:
		}
		wg.Add(1)
		go func(i int, t time.Time) {
			sc, err := screenshot.CaptureRect(rect)
			if err != nil {
				log.Fatal(err)
			}
			println("screen", i)

			//anim = append(anim, sc)
			screens <- sshot{
				img: *sc,
				ts:  t.UTC(),
			}
			wg.Done()
		}(i, t)
		i++
	}
	wg.Wait()
	close(screens)

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
			rect := image.Rectangle{
				Min: image.Point{
					X: 0,
					Y: 0,
				},
				Max: image.Point{
					X: 800, //bounds.max.x
					Y: 600,
				},
			}
			_ = rect
			pImg := image.NewPaletted(sc.img.Bounds(), nil)
			quantizer.Quantize(pImg, sc.img.Bounds(), &sc.img, image.ZP)
			paletteds <- tsimg{
				img: *pImg,
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

	unordered := []tsimg{}
	println("creating gif")
	for img := range imgs {
		fmt.Println("t:", img.ts)
		unordered = append(unordered, img)
	}

	println("sort")
	sort.Sort(&tsimgSorter{imgs: unordered})
	for _, img := range unordered {
		img := img

		fmt.Println("s:", img.ts)
		out.Image = append(out.Image, &img.img)
		out.Delay = append(out.Delay, 0)
	}
	done <- struct{}{}
	close(done)
}
