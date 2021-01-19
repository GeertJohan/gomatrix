package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"runtime/pprof"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gdamore/tcell"
	"github.com/jessevdk/go-flags"
)

var screen tcell.Screen

// command line flags variable
var opts struct {
	// display ascii only instead of ascii+kana's
	Ascii bool `short:"a" long:"ascii" description:"Use ascii/alphanumeric characters only instead of a mixture with japanese kana's."`

	// display kana's instead of ascii+kana's
	Kana bool `short:"k" long:"kana" description:"Use japanese kana's only instead of a mix of ascii/alphanumeric and kana's."`

	// enable logging
	Logging bool `short:"l" long:"log" description:"Enable logging debug messages to ~/.gomatrix-log."`

	// enable profiling
	Profile string `short:"p" long:"profile" description:"Write profile to given file path"`

	// FPS
	FPS int `long:"fps" description:"required FPS, must be somewhere between 1 and 60" default:"25"`
}

// array with half width kanas as Go runes
// source: http://en.wikipedia.org/wiki/Half-width_kana
var halfWidthKana = []rune{
	'｡', '｢', '｣', '､', '･', 'ｦ', 'ｧ', 'ｨ', 'ｩ', 'ｪ', 'ｫ', 'ｬ', 'ｭ', 'ｮ', 'ｯ',
	'ｰ', 'ｱ', 'ｲ', 'ｳ', 'ｴ', 'ｵ', 'ｶ', 'ｷ', 'ｸ', 'ｹ', 'ｺ', 'ｻ', 'ｼ', 'ｽ', 'ｾ', 'ｿ',
	'ﾀ', 'ﾁ', 'ﾂ', 'ﾃ', 'ﾄ', 'ﾅ', 'ﾆ', 'ﾇ', 'ﾈ', 'ﾉ', 'ﾊ', 'ﾋ', 'ﾌ', 'ﾍ', 'ﾎ', 'ﾏ',
	'ﾐ', 'ﾑ', 'ﾒ', 'ﾓ', 'ﾔ', 'ﾕ', 'ﾖ', 'ﾗ', 'ﾘ', 'ﾙ', 'ﾚ', 'ﾛ', 'ﾜ', 'ﾝ', 'ﾞ', 'ﾟ',
}

// just basic alphanumeric characters
var alphaNumerics = []rune{
	'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O',
	'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
	'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o',
	'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
}

var extendedChars = []rune{
	'¹', '²', '³', 'ª', 'º', '%', '$', '€', '¥', '£', '¢', '&', '*', '@', '#',
	'|', 'á', 'â', 'à', 'ä', 'å', 'ã', 'æ', 'ç', 'é', 'ê', 'è', 'ë', 'í', 'î',
	'ì', 'ï', 'ı', 'ñ', 'ó', 'ô','ò', 'ö', 'õ', 'ø', 'œ', 'š', 'ß', 'ú', 'û',
	'ù', 'ü', 'ý', 'ÿ', 'ž', 'Â', 'À', 'Ä', 'Å', 'Ã', 'Æ', 'Ç', 'É', 'Ê', 'È',
	'Ë', 'Í', 'Î', 'Ì', 'Ï', 'Ñ', 'Ó', 'Ô', 'Ò', 'Ö', 'Õ', 'Ø', 'Œ', 'Š', 'Û',
	'Ù', 'Ü', 'Ý', 'Ÿ', ',', ':', ';', '-', '–', '—', '•', '.', '…', '“', '‘',
	'’', '‘', '‚', '“', '”', '„', '‹', '›', '«', '»', '/', '\\', '?', '!',
	'¿', '¡', '(', ')', '[', ']', '{', '}', '©', '®', '§', '+', '×', '=', '_', '°',
}

// everything together, for that authentic feel
var allTheCharacters = append(halfWidthKana, alphaNumerics, extendedChars...)

// characters to be used, can be set to alphaNumerics or halfWidthKana depending on flags
// defaults to allTheCharacters
var characters []rune

// streamDisplays by column number
var streamDisplaysByColumn = make(map[int]*StreamDisplay)

// current sizes
var curSizes sizes

// channel used to notify StreamDisplayManager
var sizesUpdateCh = make(chan sizes)

// struct sizes contains terminal sizes (in amount of characters)
type sizes struct {
	width                      int
	height                     int
	curStreamsPerStreamDisplay int // current amount of streams per display allowed
}

// set the sizes and notify StreamDisplayManager
func (s *sizes) setSizes(width int, height int) {
	s.width = width
	s.height = height
	s.curStreamsPerStreamDisplay = 1 + height/10
}

func main() {
	// parse flags
	args, err := flags.Parse(&opts)
	if err != nil {
		flagError := err.(*flags.Error)
		if flagError.Type == flags.ErrHelp {
			return
		}
		if flagError.Type == flags.ErrUnknownFlag {
			fmt.Println("Use --help to view all available options.")
			return
		}
		fmt.Printf("Error parsing flags: %s\n", err)
		return
	}
	if len(args) > 0 {
		// we don't accept too much arguments..
		fmt.Printf("Unknown argument '%s'.\n", args[0])
		return
	}
	if opts.FPS < 1 || opts.FPS > 60 {
		fmt.Println("Error: option --fps not within range 1-60")
		os.Exit(1)
	}
	// Start profiling (if required)
	if len(opts.Profile) > 0 {
		f, err := os.Create(opts.Profile)
		if err != nil {
			fmt.Printf("Error opening profiling file: %s\n", err)
			os.Exit(1)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			fmt.Printf("Error start profiling : %s\n", err)
			os.Exit(1)
		}
	}
	// Use a println for fun..
	fmt.Println("Opening connection to The Matrix.. Please stand by..")
	// setup logging with logfile /dev/null or ~/.gomatrix-log
	filename := os.DevNull
	if opts.Logging {
		filename = os.Getenv("HOME") + "/log/gomatrix-log"
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
	if opts.Ascii {
		characters = alphaNumerics
	} else if opts.Kana {
		characters = halfWidthKana
	} else {
		characters = allTheCharacters
	}
	// seed the rand package with time
	rand.Seed(time.Now().UnixNano())

	// initialize tcell
	if screen, err = tcell.NewScreen(); err != nil {
		fmt.Println("Could not start tcell for gomatrix. View ~/.gomatrix-log for error messages.")
		log.Printf("Cannot alloc screen, tcell.NewScreen() gave an error:\n%s", err)
		os.Exit(1)
	}

	err = screen.Init()
	if err != nil {
		fmt.Println("Could not start tcell for gomatrix. View ~/.gomatrix-log for error messages.")
		log.Printf("Cannot start gomatrix, screen.Init() gave an error:\n%s", err)
		os.Exit(1)
	}
	screen.HideCursor()
	screen.SetStyle(tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(tcell.ColorBlack))
	screen.Clear()

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
	curSizes.setSizes(screen.Size())
	sizesUpdateCh <- curSizes

	// flusher flushes the termbox every x milliseconds
	curFPS := opts.FPS
	fpsSleepTime := time.Duration(1000000/curFPS) * time.Microsecond
	fmt.Printf("fps sleep time: %s\n", fpsSleepTime.String())
	go func() {
		for {
			time.Sleep(fpsSleepTime)
			screen.Show()
		}
	}()

	// make chan for tembox events and run poller to send events on chan
	eventChan := make(chan tcell.Event)
	go func() {
		for {
			event := screen.PollEvent()
			eventChan <- event
		}
	}()

	// register signals to channel
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, os.Kill)

	// handle tcell events and unix signals
EVENTS:
	for {
		// select for either event or signal
		select {
		case event := <-eventChan:
			log.Printf("Have event: \n%s", spew.Sdump(event))
			// switch on event type
			switch ev := event.(type) {
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyCtrlZ, tcell.KeyCtrlC:
					break EVENTS

				case tcell.KeyCtrlL:
					screen.Sync()

				case tcell.KeyRune:
					switch ev.Rune() {
					case 'q':
						break EVENTS

					case 'c':
						screen.Clear()

					case 'k':
						characters = halfWidthKana

					case 'b': // "both"
						characters = allTheCharacters

					case '+': // speed it up
						if curFPS < 60 {
							curFPS++
							fpsSleepTime = time.Duration(1000000/curFPS) * time.Microsecond
						}

					case '-': // slow it down
						if curFPS > 1 {
							curFPS--
							fpsSleepTime = time.Duration(1000000/curFPS) * time.Microsecond
						}

					case '=': // set the speed back to where it started
						curFPS = opts.FPS
						fpsSleepTime = time.Duration(1000000/curFPS) * time.Microsecond
					}

					//++ TODO: add more fun keys (slowmo? freeze? rampage?)
				}
			case *tcell.EventResize: // set sizes
				w, h := ev.Size()
				curSizes.setSizes(w, h)
				sizesUpdateCh <- curSizes
			case *tcell.EventError: // quit
				log.Fatalf("Quitting because of tcell error: %v", ev.Error())
			}

		case signal := <-sigChan:
			log.Printf("Have signal: \n%s", spew.Sdump(signal))
			break EVENTS
		}
	}

	// close down
	screen.Fini()

	log.Println("stopping gomatrix")
	fmt.Println("Thank you for connecting with Morpheus' Matrix API v4.2. Have a nice day!")

	// stop profiling (if required)
	if len(opts.Profile) > 0 {
		pprof.StopCPUProfile()
	}
}
