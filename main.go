package main

import (
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/jessevdk/go-flags"
	"github.com/nsf/termbox-go"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var opts struct {
	// display ascii instead of kana's
	Ascii bool `short:"a" long:"ascii" description:"Use ascii/alphanumeric characters instead of japanese kana's."`

	// enable logging
	Logging bool `short:"l" long:"log" description:"Enable logging debug messages to ~/.gomatrix-log."`
}

// array with half width kanas as Go runes
// source: http://en.wikipedia.org/wiki/Half-width_kana
var halfWidthKana = []rune{
	'｡', '｢', '｣', '､', '･', 'ｦ', 'ｧ', 'ｨ', 'ｩ', 'ｪ', 'ｫ', 'ｬ', 'ｭ', 'ｮ', 'ｯ',
	'ｰ', 'ｱ', 'ｲ', 'ｳ', 'ｴ', 'ｵ', 'ｶ', 'ｷ', 'ｸ', 'ｹ', 'ｺ', 'ｻ', 'ｼ', 'ｽ', 'ｾ', 'ｿ',
	'ﾀ', 'ﾁ', 'ﾂ', 'ﾃ', 'ﾄ', 'ﾅ', 'ﾆ', 'ﾇ', 'ﾈ', 'ﾉ', 'ﾊ', 'ﾋ', 'ﾌ', 'ﾍ', 'ﾎ', 'ﾏ',
	'ﾐ', 'ﾑ', 'ﾒ', 'ﾓ', 'ﾔ', 'ﾕ', 'ﾖ', 'ﾗ', 'ﾘ', 'ﾙ', 'ﾚ', 'ﾛ', 'ﾜ', 'ﾝ', 'ﾞ', 'ﾟ',
}

var alphaNumerics = []rune{
	'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o',
	'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
}

var characters []rune
var streamDisplaysByColumn = make(map[int]*StreamDisplay)

type sizes struct {
	width  int
	height int
}

var curSizes sizes
var curStreamsPerStreamDisplay = 0
var sizesUpdateCh = make(chan sizes)

func setSizes(width int, height int) {
	s := sizes{
		width:  width,
		height: height,
	}
	curSizes = s
	curStreamsPerStreamDisplay = 1 + height/10
	sizesUpdateCh <- s
}

func main() {
	// parse flags
	args, err := flags.Parse(&opts)
	if err != nil {
		if err.Error()[0:5] == "Usage" {
			fmt.Println(err)
			return
		}
		fmt.Printf("Error parsing flags: %s\nUse --help to view all available options.\n", err)
		return
	}
	if len(args) > 0 {
		fmt.Printf("Unknown argument '%s'\n", args[0])
		return
	}

	fmt.Println("Opening connection to The Matrix.. Please stand by..")

	// setup logging with logfile ~/.gomatrix-log
	filename := "/dev/null"
	if opts.Logging {
		filename = os.Getenv("HOME") + "/.gomatrix-log"
	}
	logfile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Could not open logfile. %s\n", err)
		os.Exit(1)
	}
	defer logfile.Close()
	log.SetOutput(logfile)
	log.Println("-------------")
	log.Println("Starting gomatrix. This logfile is for development/debug purposes.")

	characters = halfWidthKana
	if opts.Ascii {
		characters = alphaNumerics
	}

	// seed the rand package with time
	rand.Seed(time.Now().UnixNano())

	// initialize termbox
	err = termbox.Init()
	if err != nil {
		fmt.Println("Could not start termbox for gomatrix. View ~/.gomatrix-log for error messages.")
		log.Printf("Cannot start gomatrix, termbox.Init() gave an error:\n%s\n", err)
		os.Exit(1)
	}
	termbox.HideCursor()

	// StreamDisplay manager
	go func() {
		var lastWidth int

		for newSizes := range sizesUpdateCh {
			log.Printf("New width: %d\n", newSizes.width)
			diffWidth := newSizes.width - lastWidth

			if diffWidth == 0 {
				// same column size, wait for new information
				log.Println("Got resize over channel, but diffWidth = 0")
				continue
			}

			if diffWidth > 0 {
				log.Printf("Starting %d new SD's\n", diffWidth)
				for newColumn := lastWidth; newColumn < newSizes.width; newColumn++ {
					// create stream display
					sd := &StreamDisplay{
						column:    newColumn,
						stopCh:    make(chan bool, 1),
						streams:   make(map[*Stream]bool),
						newStream: make(chan bool, 1), // will only be filled at start and when a spawning stream has it's tail released
					}
					streamDisplaysByColumn[newColumn] = sd

					// start StreamDisplay in goroutine
					go sd.run()

					// create first new stream
					sd.newStream <- true
				}
				lastWidth = newSizes.width
			}

			if diffWidth < 0 {
				log.Printf("Closing %d SD's\n", diffWidth)
				for closeColumn := lastWidth - 1; closeColumn > newSizes.width; closeColumn-- {
					// get sd
					sd := streamDisplaysByColumn[closeColumn]

					// delete from map
					delete(streamDisplaysByColumn, closeColumn)

					// inform sd that it's being closed
					sd.stopCh <- true
				}
				lastWidth = newSizes.width
			}
		}
	}()

	// set initial sizes
	setSizes(termbox.Size())

	// flusher flushes every 100 miliseconds)
	go func() {
		for {
			<-time.After(40 * time.Millisecond)
			termbox.Flush()
		}
	}()

	// make chan for tembox events and run poller to send events on chan
	eventChan := make(chan termbox.Event)
	go func() {
		for {
			event := termbox.PollEvent()
			eventChan <- event
		}
	}()

	// register signals to channel
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGTSTP)
	signal.Notify(sigChan, syscall.SIGKILL)
	signal.Notify(sigChan, syscall.SIGQUIT)
	signal.Notify(sigChan, syscall.SIGTERM)

	// handle termbox events and unix signals
	func() {
		for {
			// select for either event or signal
			select {
			case event := <-eventChan:
				log.Printf("Have event: \n%s", spew.Sdump(event))
				// switch on event type
				switch event.Type {
				case termbox.EventKey:
					switch event.Key {
					case termbox.KeyCtrlZ, termbox.KeyCtrlC:
						return
					}

				case termbox.EventResize:
					// set sizes
					setSizes(event.Width, event.Height)

				case termbox.EventError:
					log.Fatalf("Quitting because of termbox error: \n%s\n", event.Err)
				}
			case signal := <-sigChan:
				log.Printf("Have signal: \n%s", spew.Sdump(signal))
				return
			}
		}
	}()

	// close down
	termbox.Close()
	log.Println("stopping gomatrix")
	fmt.Println("Thank you for connecting with Cypher's Matrix API v4.2. Have a nice day!")
	os.Exit(0)
}
