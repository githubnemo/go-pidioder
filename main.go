package main

import (
	"bufio"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

const (
	REDCHAN   = "0"
	BLUECHAN  = "1"
	GREENCHAN = "2"
)

var (
	templates *template.Template
	CURRENT_R int = 0
	CURRENT_G int = 0
	CURRENT_B int = 0
)

func FloatToString(input_num float64) string {
	return strconv.FormatFloat(input_num, 'f', 6, 64)
}

func parseTemplates() (err error) {
	templates, err = template.ParseGlob("templates/*.html")

	return err
}

func setchan(channel string, val float64) error {

	chancmd := channel + "=" + FloatToString(val) + "\n"

	file, err := os.OpenFile("/dev/pi-blaster", os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		panic(err)
	}

	defer file.Close()

	stream := bufio.NewWriter(file)

	_, err = stream.WriteString(chancmd)
	if err != nil {
		panic(err)
	}

	stream.Flush()

	return nil
}

func setChannelInteger(val int, channel string) error {
	if val > 255 {
		return errors.New("can't go over 255. sorry mate.")
	}

	if val < 0 {
		return errors.New("can't go below 0. sorry mate.")
	}

	floatval := float64(val) / 255.0
	setchan(channel, float64(floatval))
	return nil
}

func setRed(val int) error {
	return setChannelInteger(val, REDCHAN)
}

func setGreen(val int) error {
	return setChannelInteger(val, GREENCHAN)
}

func setBlue(val int) error {
	return setChannelInteger(val, BLUECHAN)
}

func setAll(r, g, b int) {

	step := 1

	for CURRENT_R != r ||
		CURRENT_G != g ||
		CURRENT_B != b {

		if CURRENT_R < r {
			CURRENT_R += step
			setRed(CURRENT_R)
		}

		if CURRENT_R > r {
			CURRENT_R -= step
			setRed(CURRENT_R)
		}

		if CURRENT_G < g {
			CURRENT_G += step
			setGreen(CURRENT_G)
		}

		if CURRENT_G > g {
			CURRENT_G -= step
			setGreen(CURRENT_G)
		}

		if CURRENT_B < b {
			CURRENT_B += step
			setBlue(CURRENT_B)
		}

		if CURRENT_B > b {
			CURRENT_B -= step
			setBlue(CURRENT_B)
		}
	}
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	if err := recover(); err != nil {
		w.WriteHeader(401)

		fmt.Fprintf(w, "Oh...:(\n\n")

		if e, ok := err.(error); ok {
			w.Write([]byte(e.Error()))
			w.Write([]byte{'\n', '\n'})
		} else {
			fmt.Fprintf(w, "%s\n\n", err)
		}

		log.Println(
			"panic catched:", err,
			"\nRequest data:", r)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	defer errorHandler(w, r)

	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		panic(err)
	}

	color := values.Get("targetcolor")
	manual := values.Get("manual")

	if manual == "set" {
		man_r, _ := strconv.Atoi(values.Get("manual_r"))
		man_g, _ := strconv.Atoi(values.Get("manual_g"))
		man_b, _ := strconv.Atoi(values.Get("manual_b"))

		fmt.Println("r: ", man_r, " - g: ", man_g, " - b:", man_b)

		setAll(man_r, man_g, man_b)

	} else {
		switch color {
		case "Red":
			setAll(255, 0, 0)
			break
		case "Green":
			setAll(0, 255, 0)
			break
		case "Blue":
			setAll(0, 0, 255)
			break
		case "Specific Preset 1":
			setAll(205, 55, 0)
			break
		case "Specific Preset 2":
			setAll(255, 200, 0)
			break
		case "Specific Preset 3":
			setAll(255, 0, 255)
			break
		case "Brighten":
			setAll(CURRENT_R+10, CURRENT_G+10, CURRENT_B+10)
			break
		case "Dim":
			setAll(CURRENT_R-10, CURRENT_G-10, CURRENT_B-10)
			break
		case "Turn off":
			setAll(0, 0, 0)
			break
		}
	}
	templates.ExecuteTemplate(w, "index.html", nil)
}

func main() {
	setAll(CURRENT_R, CURRENT_G, CURRENT_B)
	parseTemplates()

	http.HandleFunc("/", indexHandler)
	log.Fatal(http.ListenAndServe(":1337", nil))
}
