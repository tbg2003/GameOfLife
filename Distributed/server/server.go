package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var turn int
var world [][]byte
var mu sync.Mutex
var brokerAddr string

//var pause bool = false
//var resume bool = false
var (
	mulch = make(chan [][]byte, 1)
)

func mod(x int, y int) int {
	a := x % y
	if a < 0 {
		a += y
	}
	return a
}

func getOutboundIP() string {
	conn, _ := net.Dial("udp", "8.8.8.8:80")
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr).IP.String()
	return localAddr
}

func calcLiveNeighbours(world [][]byte, x int, y int) int {
	liveNeighbours := 0
	height := len(world)
	width := len(world[0])
	xp := mod(x+1, width)
	xm := mod(x-1, width)
	yp := mod(y+1, height)
	ym := mod(y-1, height)
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

func calculateNextState(height, width int, world [][]byte, starty, endy int) [][]byte {
	var change []util.Cell
	newWorld := make([][]byte, endy-starty)
	for i := range newWorld {
		newWorld[i] = make([]byte, width)
	}
	for h := starty; h < endy; h++ {
		for w := 0; w < width; w++ {
			newWorld[h-starty][w] = world[h][w]
			ln := calcLiveNeighbours(world, w, h)
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
func worker(h, w, startY, endY int, in [][]byte, out chan<- [][]byte) {
	output := calculateNextState(h, w, in, startY, endY)
	out <- output
}

func copyWorld(world [][]byte) [][]byte {
	worldCopy := makeWorld(len(world), len(world[0]))
	for i := range world {
		for j := range world[i] {
			worldCopy[i][j] = world[i][j]
		}
	}
	return worldCopy
}

func makeWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := 0; i < height; i++ {
		row := make([]byte, width)
		world[i] = row
	}
	return world
}

//func makenewboard(ch chan [][]byte, client *rpc.Client) {
//	for {
//		res := <-ch
//		towork := stubs.PubRequest{Topic: "world", World: res, Turns: turn, Threads: 1}
//		status := stubs.StatusReport{Message: "Complete"}
//		client.Call(stubs.Publish, towork, status)
//		//err := client.Call(stubs.Publish, towork, status)
//		//if err != nil {
//		//	fmt.Println("RPC client returned error:")
//		//	fmt.Println(err)
//		//	fmt.Println("Dropping world.")
//		//}
//	}
//}

type BoardOperations struct{}

func (s *BoardOperations) GetBrokerIp(req stubs.ConnectRequest, res *stubs.StatusReport) (err error) {
	brokerAddr = req.IpAddress
	return
}

func (s *BoardOperations) Shutdown(req stubs.PubRequest, res *stubs.JobReport) (err error) {
	os.Exit(1)
	return
}

func (s *BoardOperations) CalculateNextBoard(req stubs.PubRequest, res *stubs.JobReport) (err error) {
	if req.Topic == "shutdown" {
		res.Message = "off"
		os.Exit(1)
	}
	height := len(req.World)
	width := len(req.World[0])
	//if !resume {
	//	turn = 0
	//}
	//resume = false
	//pause = false
	newWorld := copyWorld(req.World)
	//creates an array of output channels for the workers to put their chunks in
	chanoutarray := []chan [][]byte{}
	for i := 0; i < req.Threads; i++ {
		chanout := make(chan [][]byte)
		chanoutarray = append(chanoutarray, chanout)
	}
	//for i := turn; i < req.Turns; i++ {
	for j := 0; j < req.Threads; j++ {
		starth := int(math.Ceil(float64(j) * (float64(height) / float64(req.Threads))))
		endh := int(math.Ceil(float64(j+1) * (float64(height) / float64(req.Threads))))
		go worker(height, width, starth, endh, newWorld, chanoutarray[j])
	}
	collectWorld := makeWorld(height, width)
	for j := 0; j < req.Threads; j++ {
		out := <-chanoutarray[j]
		starth := int(math.Ceil(float64(j) * (float64(height) / float64(req.Threads))))
		endh := int(math.Ceil(float64(j+1) * (float64(height) / float64(req.Threads))))
		for h := 0; h < endh-starth; h++ {
			for w := 0; w < width; w++ {
				collectWorld[h+starth][w] = out[h][w]
			}
		}
	}
	newWorld = copyWorld(collectWorld)
	mu.Lock()
	turn += 1
	world = copyWorld(newWorld)
	mu.Unlock()
	res.Result = world
	res.Turn = turn
	res.AliveCells = getAliveCells(world)

	//mulch <- res.Result
	if turn == req.Turns {
		turn = 0
	}
	return
}

func getAliveCells(world [][]byte) int {
	aliveCount := 0
	for i := range world {
		for j := 0; j < len(world[0]); j++ {
			if world[i][j] == 255 {
				aliveCount++
			}
		}
	}
	return aliveCount
}

//func (s *BoardOperations) ResumeState(req stubs.Request, res *stubs.Response) (err error) {
//	resume = !resume
//	return
//}
//
//func (s *BoardOperations) GetCurrentWorld(req stubs.Request, res *stubs.Response) (err error) {
//	mu.Lock()
//	res.World = world
//	res.Turn = turn
//	mu.Unlock()
//	return
//}
//
//func (s *BoardOperations) ShutdownServer(req stubs.Request, res *stubs.Response) (err error) {
//	mu.Lock()
//	res.Turn = turn
//	mu.Unlock()
//	fmt.Println("Quitting!")
//	os.Exit(1)
//	return
//}
//
//func (s *BoardOperations) PauseServer(req stubs.Request, res *stubs.Response) (err error) {
//	pause = !pause
//	mu.Lock()
//	res.Turn = turn
//	mu.Unlock()
//	return
//}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	//brokerAddr := flag.String("broker", "127.0.0.1:8050", "Address of broker instance")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&BoardOperations{})
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		log.Fatal(err)
	} else {
		fmt.Println("Listener Accepted")
	}
	fmt.Println(brokerAddr)

	rpc.Accept(listener)
	defer listener.Close()
	//client, err := rpc.Dial("tcp", *brokerAddr)
	//if err != nil {
	//	log.Fatal("dialing:", err)
	//}
	//status := new(stubs.StatusReport)
	//request := stubs.ChannelRequest{Topic: "world", Buffer: 1}
	//client.Call(stubs.CreateChannel, request, status)
	//rpc.Register(&BoardOperations{})
	//fmt.Println(*pAddr)
	//listener, err := net.Listen("tcp", ":"+*pAddr)
	//if err != nil {
	//	log.Fatal("dialing:", err)
	//}
	//client.Call(stubs.Subscribe, stubs.Subscription{Topic: "world", FactoryAddress: getOutboundIP() + *pAddr, Callback: "BoardOperations.CalculateNextBoard"}, status)
	//fmt.Println(getOutboundIP())
	//defer listener.Close()
	////go makenewboard(mulch, client)
	////fmt.Println("makenewboard goroutine started")
	//rpc.Accept(listener)
}
