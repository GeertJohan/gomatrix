package main

import (
	"github.com/nsf/termbox-go"
	"log"
	"math/rand"
	"sync"
	"time"
)

// Stream updates a StreamDisplay with new data updates
type Stream struct {
	display *StreamDisplay
	headPos int
	tailPos int
	stopCh  chan bool
}

func (s *Stream) run() {
	for {
		select {
		case <-s.stopCh:
			log.Println("Stream on SD-%d stopped.\n", s.display.column)
			return
		case <-time.After(300 * time.Millisecond):
			newRune := alphaNumerics[rand.Intn(len(alphaNumerics))]
			termbox.SetCell(s.display.column, s.headPos, newRune, termbox.ColorGreen, termbox.ColorBlack)
			// stream length is random between 6 and 12 characters, although the shorter ones will display more often
			if s.tailPos > 0 || (s.tailPos == 0 && s.headPos > 6+rand.Intn(6)) {
				termbox.SetCell(s.display.column, s.tailPos, ' ', termbox.ColorBlack, termbox.ColorBlack)
				s.tailPos++
			}
			s.headPos++
		}
	}
}

// StreamDisplay represents a horizontal line in the terminal on which `Stream`s are displayed.
// StreamDisplay also creates the Streams themselves
type StreamDisplay struct {
	column      int
	stopCh      chan bool
	streams     map[*Stream]bool
	streamsLock sync.Mutex
}

func (sd *StreamDisplay) run() {
	for {
		select {
		case <-sd.stopCh:
			// lock this SD forever
			sd.streamsLock.Lock()

			// stop streams
			for s, _ := range sd.streams {
				s.stopCh <- true
			}

			// log
			log.Printf("StreamDisplay on column %d stopped.\n", sd.column)
		case <-time.After(time.Duration(rand.Intn(3)) * time.Second):
			sd.streamsLock.Lock()
			s := &Stream{
				display: sd,
				stopCh:  make(chan bool),
			}
			sd.streams[s] = true
			go s.run()
			sd.streamsLock.Unlock()
		}
	}
}

func RunStreamDisplayManager() chan int {
	streamDisplaysByColumn := make(map[int]*StreamDisplay)

	curColumnSize := 0
	newColumnSizeCh := make(chan int)

	// start actual manager in goroutine
	go func() {
		for {
			newColumnSize := <-newColumnSizeCh
			diff := newColumnSize - curColumnSize

			if diff == 0 {
				// same column size, wait for new information
				log.Println("Got resize over channel, but diff = 0")
				continue
			}

			if diff > 0 {
				log.Printf("Starting %d new SD's\n", diff)
				for newColumn := curColumnSize; newColumn < newColumnSize; newColumn++ {
					// create stream display
					sd := &StreamDisplay{
						column:  newColumn,
						stopCh:  make(chan bool),
						streams: make(map[*Stream]bool),
					}
					streamDisplaysByColumn[newColumn] = sd

					// start StreamDisplay in goroutine
					go sd.run()
				}
				curColumnSize = newColumnSize
			}

			if diff < 0 {
				log.Printf("Closing %d SD's\n", diff)
				for closeColumn := curColumnSize - 1; closeColumn > newColumnSize; closeColumn-- {
					// get sd
					sd := streamDisplaysByColumn[closeColumn]

					// delete from map
					delete(streamDisplaysByColumn, sd.column)

					// inform sd that it's being closed
					sd.stopCh <- true
				}
				curColumnSize = newColumnSize
			}
		}
	}()

	return newColumnSizeCh
}
