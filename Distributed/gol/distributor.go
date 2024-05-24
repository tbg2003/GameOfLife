package gol

import (
	"fmt"
	"os"

	// "fmt"
	"log"
	"net/rpc"
	"time"

	// "os"
	"strconv"
	// "time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var myWorld [][]byte

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

var (
	mulch = make(chan [][]byte, 2)
)

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

func giveOutput(p Params, c distributorChannels, world [][]byte, turn int) {
	c.ioCommand <- ioOutput
	filenameOut := getFilename(p, true)
	c.ioFilename <- filenameOut
	for h := 0; h < p.ImageHeight; h++ {
		for w := 0; w < p.ImageWidth; w++ {
			c.ioOutput <- world[h][w]
		}
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- ImageOutputComplete{CompletedTurns: turn, Filename: filenameOut}
}

func getFilename(p Params, out bool) string {
	filename := strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.ImageWidth)
	if out {
		filename = filename + "x" + strconv.Itoa(p.Turns)
	}
	return filename
}

func makeCall(client *rpc.Client, world [][]byte, p Params, c distributorChannels) {

	chanrequest := stubs.ChannelRequest{Topic: "world", Buffer: 1}
	result := new(stubs.JobReport)
	status := new(stubs.StatusReport)
	client.Call(stubs.CreateChannel, chanrequest, status)
	towork := stubs.PubRequest{Topic: "world", World: world, Threads: p.Threads, Turns: p.Turns}
	client.Call(stubs.Publish, towork, result)
	pauseStatus := new(stubs.StatusReport)
	pause := false
	client.Call(stubs.GetPause, new(stubs.PubRequest), pauseStatus)
	if pauseStatus.Message == "paused" {
		pause = true
	}
	output := new(stubs.JobReport)
	outreq := new(stubs.PubRequest)
	done := make(chan *rpc.Call, 1)
	ticker := time.NewTicker(2 * time.Second)
	client.Go(stubs.GetWorld, outreq, output, done)
	running := true

	for running {
		select {
		case <-done:
			running = false
		case <-ticker.C:
			if !pause {
				ACReply := new(stubs.JobReport)
				client.Call(stubs.AliveCells, new(stubs.PubRequest), ACReply)
				c.events <- AliveCellsCount{CompletedTurns: ACReply.Turn, CellsCount: ACReply.AliveCells}
			}
		case kp := <-c.keyPresses:
			worldReq := new(stubs.PubRequest)
			worldRes := new(stubs.JobReport)
			if kp == 'p' {
				pause = !pause
				client.Call(stubs.PauseB, worldReq, worldRes)
				if pause {
					fmt.Println("Paused on turn. ", worldRes.Turn+1)
				} else {
					fmt.Println("Continuing")
				}
			} else if kp == 's' {
				client.Call(stubs.GetLiveWorld, worldReq, worldRes)
				giveOutput(p, c, worldRes.Result, worldRes.Turn)
			} else if kp == 'q' {
				client.Call(stubs.ResumeB, worldReq, worldRes)
				os.Exit(1)
			} else if kp == 'k' {
				client.Call(stubs.PauseB, worldReq, worldRes)
				client.Call(stubs.GetLiveWorld, worldReq, worldRes)
				giveOutput(p, c, worldRes.Result, worldRes.Turn)
				client.Call(stubs.Shutdown, worldReq, worldRes)
				time.Sleep(time.Second)
				os.Exit(1)
			}
		}
	}
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			world[i][j] = output.Result[i][j]
		}
	}
	//req := new(stubs.PubRequest)
	//res := new(stubs.StatusReport)
	//client.Call(stubs.DeleteChannels, req, res)
	//fmt.Println(res.Message)
}

func makeWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := 0; i < height; i++ {
		row := make([]byte, width)
		world[i] = row
	}
	return world
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// TODO: Create a 2D slice to store the world.
	world := makeWorld(p.ImageHeight, p.ImageWidth)
	// receiving input from the IO
	c.ioCommand <- ioInput
	filename := getFilename(p, false)
	c.ioFilename <- filename
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			world[i][j] = <-c.ioInput
		}
	}
	// TODO: Execute all turns of the Game of Life.
	brokerAddr := "107.22.73.254:8050"
	// creates client connected to broker
	client, err := rpc.Dial("tcp", brokerAddr)
	if err != nil {
		log.Fatal("Invalid ip address")
	}
	//function to call the client
	makeCall(client, world, p, c)

	// TODO: Report the final state using FinalTurnCompleteEvent.
	// Sends event saying final turn is completed
	AliveCells := getAliveCells(world, p)
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: AliveCells}
	giveOutput(p, c, world, p.Turns)
	// Sends event saying that execution is finished
	c.events <- StateChange{p.Turns, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
