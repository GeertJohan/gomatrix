package main

import (
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/nsf/termbox-go"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

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

func main() {
	// setup logging with logfile ~/.gomatrix-log
	logfile, err := os.OpenFile(os.Getenv("HOME")+"/.gomatrix-log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Could not open logfile. %s\n", err)
		os.Exit(1)
	}
	defer logfile.Close()
	log.SetOutput(logfile)
	log.Println("-------------")
	log.Println("Starting gomatrix. This logfile is for development/debug purposes.")

	// initialize termbox
	err = termbox.Init()
	if err != nil {
		fmt.Println("Could not start termbox for gomatrix. View ~/.gomatrix-log for error messages.")
		log.Printf("Cannot start gomatrix, termbox.Init() gave an error:\n%s\n", err)
		os.Exit(1)
	}
	defer termbox.Close()
	termbox.HideCursor()

	// start stream display manager
	newColumnSizeCh := RunStreamDisplayManager()
	columnSize, _ := termbox.Size()
	newColumnSizeCh <- columnSize

	// flusher flushes every 100 miliseconds)
	go func() {
		for {
			<-time.After(100 * time.Millisecond)
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
					// give size to SD manager over channel
					newColumnSizeCh <- event.Width

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
	fmt.Println("gomatrix closed")
}
