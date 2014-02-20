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
	Color chan func(RGB)
	shake chan struct{}

	r, g, b uint8
}

func NewBlaster() *Blaster {
	return &Blaster{
		Input: make(chan RGB),
		Color: make(chan func(RGB)),
		shake: make(chan struct{}),
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
			c(RGB{b.r, b.g, b.b})
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

func (b *Blaster) setRed(val uint8) error {
	return b.setChannelInteger(*flag_R, val)
}

func (b *Blaster) setGreen(val uint8) error {
	return b.setChannelInteger(*flag_G, val)
}

func (b *Blaster) setBlue(val uint8) error {
	return b.setChannelInteger(*flag_B, val)
}

type setterFunc func(v uint8) error

func (b *Blaster) setAll(c RGB) {
	decrementer := func(cur, target uint8, f setterFunc) {
		for i := cur; i > target; i-- {
			f(i)
		}
	}

	incrementer := func(cur, target uint8, f setterFunc) {
		for i := cur; i < target; i++ {
			f(i)
		}
	}

	setter := func(cur, target uint8, f setterFunc) {
		if cur < target {
			incrementer(cur, target, f)
		} else {
			decrementer(cur, target, f)
		}
	}

	setter(b.r, c.R, b.setRed)
	setter(b.g, c.G, b.setGreen)
	setter(b.b, c.B, b.setBlue)
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

func actionHandler(w http.ResponseWriter, r *http.Request) {
	defer errorHandler(w, r)

	values, err := url.ParseQuery(r.URL.RawQuery)

	if err != nil {
		panic(err)
	}

	action := values.Get("action")

	switch action {
	case "lighter":
		blaster.Color <- func(c RGB) {
			blaster.Input <- RGB{c.R + 10, c.G + 10, c.B + 10}
		}
	case "darker":
		blaster.Color <- func(c RGB) {
			blaster.Input <- RGB{c.R - 10, c.G - 10, c.B - 10}
		}
	case "off":
		blaster.Input <- RGB{0, 0, 0}
	case "set":
		r := parseUint8OrZero(values.Get("r"))
		g := parseUint8OrZero(values.Get("g"))
		b := parseUint8OrZero(values.Get("b"))

		blaster.Input <- RGB{r, g, b}
	}
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
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("templates"))))

	log.Fatal(http.ListenAndServe(":1337", nil))
}
