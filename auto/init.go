// auto use init() func to automatically save 1 mn worth of screen into an
// animated GIF.
package auto

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/dolanor/demogif"
)

func init() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		f, err := os.Create("demo.gif")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		demogif.ScreenCapture(ctx, f, 10)
	}()
}
