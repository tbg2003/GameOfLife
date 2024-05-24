package gol

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	keyPresses <-chan rune
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func mod(x int, y int) int {
	a := x % y
	if a < 0 {
		a += y
	}
	return a
}

func calcLiveNeighbours(world [][]byte, x int, y int, p Params) int {
	liveNeighbours := 0

	xp := mod(x+1, p.ImageWidth)
	xm := mod(x-1, p.ImageWidth)
	yp := mod(y+1, p.ImageHeight)
	ym := mod(y-1, p.ImageHeight)
	xs := []int{x, xp, xm}
	ys := []int{y, yp, ym}
	for _, xi := range xs {
		for _, yi := range ys {
			if (world[yi][xi] == 255) && (!((xi == x) && (yi == y))) {
				liveNeighbours++
			}
		}
	}

	return liveNeighbours
}

func calculateNextState(p Params, world [][]byte, starty, endy int) [][]byte {
	var change []util.Cell
	newWorld := make([][]byte, endy-starty)
	for i := range newWorld {
		newWorld[i] = make([]byte, p.ImageWidth)
	}
	for h := starty; h < endy; h++ {
		for w := 0; w < p.ImageWidth; w++ {
			newWorld[h-starty][w] = world[h][w]
			ln := calcLiveNeighbours(world, w, h, p)
			if world[h][w] == 255 {
				if ln < 2 || ln > 3 {
					change = append(change, util.Cell{X: w, Y: h})
				}
			} else {
				if ln == 3 {
					change = append(change, util.Cell{X: w, Y: h})
				}
			}
		}
	}
	for i := 0; i < len(change); i++ {
		if world[change[i].Y][change[i].X] == 0 {
			newWorld[change[i].Y-starty][change[i].X] = 255
		} else if world[change[i].Y][change[i].X] == 255 {
			newWorld[change[i].Y-starty][change[i].X] = 0
		}
	}
	return newWorld
}

// worker function
func worker(startY, endY, startX, endX int, in [][]byte, out chan<- [][]byte, p Params) {
	output := calculateNextState(p, in, startY, endY)
	out <- output
}

func getAliveCells(world [][]byte, p Params) []util.Cell {
	aliveCells := []util.Cell{}
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if world[i][j] == 255 {
				aliveCells = append(aliveCells, util.Cell{X: j, Y: i})
			}
		}
	}
	return aliveCells
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.
	var world = make([][]byte, p.ImageHeight)
	for i := 0; i < p.ImageHeight; i++ {
		row := make([]byte, p.ImageWidth)
		world[i] = row
	}
	// var outputWorld = make([][]byte, p.ImageHeight)

	turn := 0

	// TODO: Execute all turns of the Game of Life.
	c.ioCommand <- ioInput
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth)
	c.ioFilename <- filename
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			world[i][j] = <-c.ioInput
		}
	}
	//creates an array of output channels for the workers to put their chunks in
	chanoutarray := []chan [][]byte{}
	for i := 0; i < p.Threads; i++ {
		chanout := make(chan [][]byte)
		chanoutarray = append(chanoutarray, chanout)
	}
	// To do if only one worker thread is wanted.
	ticker := time.NewTicker(2 * time.Second)
	done := make(chan bool)
	request := make(chan bool)
	pause := false
	go func() {
		for {
			if !pause {
				select {
				case <-done:
					return
				case <-ticker.C:
					request <- true
				case input := <-c.keyPresses:
					if input == 's' {
						c.ioCommand <- ioOutput
						filenameOut := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.Turns)
						c.ioFilename <- filenameOut
						for h := 0; h < p.ImageHeight; h++ {
							for w := 0; w < p.ImageWidth; w++ {
								c.ioOutput <- world[h][w]
							}
						}
						c.ioCommand <- ioCheckIdle
						<-c.ioIdle
						c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filenameOut}
					} else if input == 'p' {
						if !pause {
							fmt.Println(turn)
							pause = true
							c.events <- StateChange{turn, Paused}
						}
					} else if input == 'q' {
						c.events <- FinalTurnComplete{turn, getAliveCells(world, p)}
						c.ioCommand <- ioOutput
						filenameOut := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.Turns)
						c.ioFilename <- filenameOut
						for h := 0; h < p.ImageHeight; h++ {
							for w := 0; w < p.ImageWidth; w++ {
								c.ioOutput <- world[h][w]
							}
						}
						c.ioCommand <- ioCheckIdle
						<-c.ioIdle
						c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filenameOut}
						c.events <- StateChange{turn, Quitting}
					}
				}
			}
		}
	}()
	if p.Threads == 1 {
		for i := 0; i < p.Turns; i++ {
			go worker(0, p.ImageHeight, 0, p.ImageWidth, world, chanoutarray[0], p)
			world = <-chanoutarray[0]
			turn++
		}
	} else {
		for j := 0; j < p.Turns; j++ {
			for pause {
				select {
				case input := <-c.keyPresses:
					if input == 'p' {
						fmt.Println("Continuing")
						pause = false
						c.events <- StateChange{turn, Executing}
					}
				}
			}
			select {
			case <-request:
				// turn = turns
				aliveCells := len(getAliveCells(world, p))
				c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: aliveCells}

			default:
			}
			// starts worker goroutines with grid split into chunks
			for i := 0; i < p.Threads; i++ {
				starth := int(math.Ceil(float64(i) * (float64(p.ImageHeight) / float64(p.Threads))))
				endh := int(math.Ceil(float64(i+1) * (float64(p.ImageHeight) / float64(p.Threads))))
				go worker(starth, endh, 0, p.ImageWidth, world, chanoutarray[i], p)
			}
			// var newWorld [][]byte
			var newWorld = make([][]byte, p.ImageHeight)
			for i := range newWorld {
				newWorld[i] = make([]byte, p.ImageWidth)
			}
			for i := 0; i < p.Threads; i++ {
				out := <-chanoutarray[i]
				starth := int(math.Ceil(float64(i) * (float64(p.ImageHeight) / float64(p.Threads))))
				endh := int(math.Ceil(float64(i+1) * (float64(p.ImageHeight) / float64(p.Threads))))
				for h := 0; h < endh-starth; h++ {
					for w := 0; w < p.ImageWidth; w++ {
						newWorld[h+starth][w] = out[h][w]
					}
				}
			}
			for h := 0; h < p.ImageHeight; h++ {
				for w := 0; w < p.ImageWidth; w++ {
					if world[h][w] != newWorld[h][w] {
						c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: w, Y: h}}
					}
					world[h][w] = newWorld[h][w]
				}
			}
			c.events <- TurnComplete{CompletedTurns: turn}
			turn++
		}
	}
	// TODO: Report the final state using Finammand <- ioOutputlTurnCompleteEvent.
	// creates a slice of all the alive cells

	AliveCells := getAliveCells(world, p)

	// sends event to IO saying final turn is completed
	done <- true
	ticker.Stop()
	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: AliveCells}
	c.ioCommand <- ioOutput
	filenameOut := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.Turns)
	c.ioFilename <- filenameOut
	for h := 0; h < p.ImageHeight; h++ {
		for w := 0; w < p.ImageWidth; w++ {
			c.ioOutput <- world[h][w]
		}
	}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filenameOut}
	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
