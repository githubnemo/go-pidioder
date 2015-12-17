package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"
)

var (
	templates *template.Template
	blaster   *Blaster

	flag_R        = flag.Uint("r", 17, "Red GPIO pin")
	flag_G        = flag.Uint("g", 22, "Green GPIO pin")
	flag_B        = flag.Uint("b", 27, "Blue GPIO pin")
	flag_Cooldown = flag.Uint("cooldown", 10, "Milliseconds cooldown between requests")
)

type RGB struct {
	R uint8
	G uint8
	B uint8
}

func (c RGB) String() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func mustParseTemplates() {
	var err error
	templates, err = template.ParseGlob("templates/*.html")

	if err != nil {
		log.Fatal(err)
	}
}

func attemptPiBlasterStart() error {
	cmd := exec.Command("pi-blaster")
	return cmd.Run()
}

func mustOpenPiBlaster() *os.File {
	file, err := os.OpenFile("/dev/pi-blaster", os.O_RDWR, os.ModeNamedPipe)

	if err != nil {
		if perr, ok := err.(*os.PathError); ok && perr.Err == syscall.ENOENT {
			err = attemptPiBlasterStart()
		}
	}

	if err != nil {
		log.Fatal(err)
	}

	return file
}

type Blaster struct {
	pipe  *os.File
	Input chan RGB
	Color chan chan RGB

	r, g, b uint8
}

func NewBlaster() *Blaster {
	return &Blaster{
		Input: make(chan RGB),
		Color: make(chan chan RGB),
	}
}

func (b *Blaster) Run() {
	b.pipe = mustOpenPiBlaster()
	defer b.pipe.Close()

	for {
		select {
		case c := <-b.Input:
			b.setAll(c)
		case c := <-b.Color:
			go func(c chan RGB) {
				select {
				case c <- RGB{b.r, b.g, b.b}:
					// delivered, everything's OK
				case <-time.After(5 * time.Second):
					log.Fatal("Requested color chan blocked too long.")
				}
			}(c)
		}
	}
}

func (b *Blaster) setPin(pin uint, val float64) error {
	chanCmd := fmt.Sprintf("%d=%f\n", pin, val)

	b.pipe.Write([]byte(chanCmd))
	return nil
}

func (b *Blaster) setChannelInteger(pin uint, val uint8) error {
	switch {
	case val > 255:
		return errors.New("can't go over 255. sorry mate.")
	case val < 0:
		return errors.New("can't go below 0. sorry mate.")
	default:
		fval := float64(val) / 255.0
		return b.setPin(pin, fval)
	}
}

func (b *Blaster) setRed(val uint8) (err error) {
	if err = b.setChannelInteger(*flag_R, val); err == nil {
		b.r = val
	}
	return
}

func (b *Blaster) setGreen(val uint8) (err error) {
	if err = b.setChannelInteger(*flag_G, val); err == nil {
		b.g = val
	}
	return
}

func (b *Blaster) setBlue(val uint8) (err error) {
	if err = b.setChannelInteger(*flag_B, val); err == nil {
		b.b = val
	}
	return
}

func (_ *Blaster) correctColor(c RGB) RGB {
	gcorrection := float64(0x77) / 0xFF
	bcorrection := float64(0x33) / 0xFF

	c.G = uint8(float64(c.G) * gcorrection)
	c.B = uint8(float64(c.B) * bcorrection)

	return c
}

func (b *Blaster) setAll(c RGB) {
	c = b.correctColor(c)
	b.setRed(c.R)
	b.setGreen(c.G)
	b.setBlue(c.B)
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	if err := recover(); err != nil {
		w.WriteHeader(401)

		fmt.Fprintf(w, "Oh...:(\n\n")

		if e, ok := err.(error); ok {
			w.Write([]byte(e.Error()))
			w.Write([]byte{'\n', '\n'})
			w.Write(debug.Stack())
		} else {
			fmt.Fprintf(w, "%s\n\n", err)
		}

		log.Println(
			"panic catched:", err,
			"\nRequest data:", r)
	}
}

func parseUint8OrZero(s string) uint8 {
	i, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0
	}
	return uint8(i)
}

func withRequestedColor(f func(color RGB)) {
	c := make(chan RGB)
	defer close(c)
	blaster.Color <- c
	f(<-c)
}

func actionHandler(w http.ResponseWriter, r *http.Request) {
	defer errorHandler(w, r)

	values, err := url.ParseQuery(r.URL.RawQuery)

	if err != nil {
		panic(err)
	}

	action := values.Get("action")

	switch action {
	case "lighter":
		withRequestedColor(func(c RGB) {
			blaster.Input <- RGB{c.R + 10, c.G + 10, c.B + 10}
		})
	case "darker":
		withRequestedColor(func(c RGB) {
			blaster.Input <- RGB{c.R - 10, c.G - 10, c.B - 10}
		})
	case "off":
		blaster.Input <- RGB{0, 0, 0}
	case "set":
		r := parseUint8OrZero(values.Get("r"))
		g := parseUint8OrZero(values.Get("g"))
		b := parseUint8OrZero(values.Get("b"))

		blaster.Input <- RGB{r, g, b}
	}

	withRequestedColor(func(c RGB) {
		log.Println(c.R, c.G, c.B)
		w.Write([]byte(c.String()))
	})
}

func currentColorHandler(w http.ResponseWriter, r *http.Request) {
	defer errorHandler(w, r)
	w.Header().Set("Content-Type", "text/plain")

	withRequestedColor(func(c RGB) {
		w.Write([]byte(c.String()))
	})
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	defer errorHandler(w, r)
	templates.ExecuteTemplate(w, "index.html", nil)
}

func main() {
	flag.Parse()

	blaster = NewBlaster()
	go blaster.Run()

	blaster.Input <- RGB{}

	mustParseTemplates()

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/do", actionHandler)
	http.HandleFunc("/color", currentColorHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("templates"))))

	log.Fatal(http.ListenAndServe(":1337", nil))
}
