package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"
	"uk.ac.bris.cs/gameoflife/stubs"
)

var state stateSave

type stateSave struct {
	world          [][]byte
	turnsCompleted int
}

func saveState(cw [][]byte, tc int) stateSave {
	state = stateSave{cw, tc}
	return state
}

var awsnodes = []string{"107.22.73.254", "18.210.236.102", "3.209.240.36", "44.212.248.16"}
var shutdown = make(chan bool, 1)
var servers []string
var mu sync.Mutex
var aliveCells int
var outputworld [][]byte
var outputdone = make(chan bool, 1)
var resume = false
var turn int
var pause = false

var (
	topics  = make(map[string]chan stubs.PubRequest)
	topicmx sync.RWMutex
)

//Create a new topic as a buffered channel.
func createTopic(topic string, buflen int) {
	topicmx.Lock()
	defer topicmx.Unlock()
	if _, ok := topics[topic]; !ok {
		topics[topic] = make(chan stubs.PubRequest, buflen)
		fmt.Println("Created channel #", topic)
	}
}

func deleteTopic(topic string) {
	topicmx.Lock()
	defer topicmx.Unlock()
	fmt.Println("Deleting time")
	notempty := true
	for notempty {
		j, more := <-topics[topic]
		if more {
			fmt.Println("Deleted", j)
		} else {
			notempty = false
		}
	}
	fmt.Println("Deleted channels")
}

//The Pair is published to the topic.
func publish(topic string, req stubs.PubRequest) (err error) {
	topicmx.RLock()
	defer topicmx.RUnlock()
	if ch, ok := topics[topic]; ok {
		ch <- req
		fmt.Println("request published :", req.Turns)
	} else {
		return errors.New("No such topic.")
	}
	return
}

//The subscriber loops run asynchronously, reading from the topic and sending the err
//'job' pairs to their associated subscriber.
func subscriber_loop(topic chan stubs.PubRequest, client *rpc.Client, callback string, res *stubs.JobReport) {
	for {
		turn = 0
		fmt.Println("In subscriber loop")
		job := <-topic
		outputworld = job.World
		if resume {
			job.World = state.world
			turn = state.turnsCompleted
		} else if !resume {
			turn = 0
		}

		for i := 0; i < job.Turns; i++ {
			if !pause {
				request := stubs.PubRequest{
					World:   job.World,
					Threads: job.Threads,
					Turns:   job.Turns,
					Topic:   job.Topic,
				}
				err := client.Call(callback, request, res)
				if err != nil {
					fmt.Println("Error")
					fmt.Println(err)
					fmt.Println("Closing subscriber thread.")
					//Place the unfulfilled job back on the topic channel.
					//topic <- job
					break
				}
				mu.Lock()
				job.World = res.Result
				outputworld = res.Result
				aliveCells = res.AliveCells
				turn++
				mu.Unlock()
			} else {
				i--
				select {
				case <-shutdown:
					fmt.Println(servers)
					for i := range servers {
						client, _ := rpc.Dial("tcp", servers[i])
						client.Call(stubs.ShutdownServer, new(stubs.PubRequest), new(stubs.StatusReport))
						os.Exit(1)
					}
				default:

				}
			}
		}
		outputdone <- true
		fmt.Println("output done sent")
	}
}

//The subscribe function registers a worker to the topic, creating an RPC client,
//and will use the given callback string as the callback function whenever work
//is available.
func subscribe(topic string, factoryAddress string, callback string, res *stubs.JobReport) (err error) {
	fmt.Println("Subscription request from ", factoryAddress, " to ", callback)
	topicmx.RLock()
	ch := topics[topic]
	topicmx.RUnlock()
	servers = append(servers, factoryAddress)
	client, err := rpc.Dial("tcp", factoryAddress)
	if err == nil {
		go subscriber_loop(ch, client, callback, res)
	} else {
		fmt.Println("Error subscribing ", factoryAddress)
		fmt.Println(err)
		return err
	}
	return
}

type Broker struct{}

//func (b *Broker) GetWorld(req stubs.ChannelRequest, res *stubs.StatusReport) (err error) {
//	input := <-topics[req.Topic]
//	world := input.World
//	res.Result = world
//	return
//}

func (b *Broker) PauseBroker(req stubs.ChannelRequest, res *stubs.JobReport) (err error) {
	pause = !pause
	mu.Lock()
	res.Turn = turn
	mu.Unlock()
	return
}

func (b *Broker) Shutdown(req stubs.ChannelRequest, res *stubs.JobReport) (err error) {
	fmt.Println("Shutting down", servers)
	res.Message = "off"
	shutdown <- true
	<-shutdown
	return
}

func (b *Broker) GetPause(req stubs.ChannelRequest, res *stubs.JobReport) (err error) {
	if pause {
		res.Message = "paused"
	} else {
		res.Message = "unpaused"
	}
	return
}

func (b *Broker) GetAliveCells(req stubs.ChannelRequest, res *stubs.JobReport) (err error) {
	res.AliveCells = aliveCells
	res.Turn = turn
	return
}

func (b *Broker) CreateChannel(req stubs.ChannelRequest, res *stubs.StatusReport) (err error) {
	createTopic(req.Topic, req.Buffer)
	res.Message = "Completed"
	return
}

func (b *Broker) DeleteChannels(req stubs.ChannelRequest, res *stubs.StatusReport) (err error) {
	fmt.Println("Deleting channels")
	deleteTopic(req.Topic)
	res.Message = "Completed"
	return
}

func (b *Broker) LiveWorldGetter(req stubs.ChannelRequest, res *stubs.JobReport) (err error) {
	mu.Lock()
	res.Turn = turn
	res.Result = outputworld
	mu.Unlock()
	return
}

func (b *Broker) WorldGetter(req stubs.ChannelRequest, res *stubs.JobReport) (err error) {
	fmt.Println("Waiting for output")
	<-outputdone
	fmt.Println("giving output work to distributor")
	res.Result = outputworld
	return
}

func (b *Broker) Subscribe(req stubs.Subscription, res *stubs.JobReport) (err error) {
	err = subscribe(req.Topic, req.FactoryAddress, req.Callback, res)
	if err != nil {
		res.Message = "Error during subscription"
	} else {
		fmt.Println("Subscribed  to ", req.Topic)
		res.Message = "Subscribed!"
	}
	return
}

func (b *Broker) Publish(req stubs.PubRequest, res *stubs.StatusReport) (err error) {
	err = publish(req.Topic, req)
	return err
}

func (b *Broker) ResumeBroker(req stubs.ChannelRequest, res *stubs.JobReport) (err error) {
	resume = !resume
	state = saveState(outputworld, turn)
	return
}

func getOutboundIP() string {
	conn, _ := net.Dial("udp", "8.8.8.8:80")
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr).IP.String()
	return localAddr
}

func main() {
	for i := range awsnodes {
		client, _ := rpc.Dial("tcp", awsnodes[i]+":8030")
		fmt.Println("Connecting to:", awsnodes[i]+":8030")
		ipAddress := getOutboundIP()
		serverReq := stubs.ConnectRequest{IpAddress: ipAddress}
		client.Call(stubs.GetBrokerIp, serverReq, new(stubs.StatusReport))
		client.Close()
	}

	pAddr := flag.String("port", "8050", "Port to listen on")

	flag.Parse()
	rpc.Register(&Broker{})
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Println("invalid address")
	}
	defer listener.Close()
	rpc.Accept(listener)
}
