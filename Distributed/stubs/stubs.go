package stubs

var GetBrokerIp = "BoardOperations.GetBrokerIp"
var ShutdownServer = "BoardOperations.Shutdown"
var Shutdown = "Broker.Shutdown"
var GetPause = "Broker.GetPause"
var PauseB = "Broker.PauseBroker"
var AliveCells = "Broker.GetAliveCells"
var GetLiveWorld = "Broker.LiveWorldGetter"
var ResumeB = "Broker.ResumeBroker"
var CreateChannel = "Broker.CreateChannel"
var DeleteChannels = "Broker.DeleteChannels"
var Publish = "Broker.Publish"
var Subscribe = "Broker.Subscribe"
var GetWorld = "Broker.WorldGetter"

// var PremiumReverseHandler = "SecretStringOperations.FastReverse"

type ConnectRequest struct {
	IpAddress string
}

type PubRequest struct {
	Topic   string
	World   [][]byte
	Threads int
	Turns   int
}

type ChannelRequest struct {
	Topic  string
	Buffer int
}

type Subscription struct {
	Topic          string
	FactoryAddress string
	Callback       string
}

type JobReport struct {
	Result     [][]byte
	AliveCells int
	Turn       int
	Message    string
}

type StatusReport struct {
	Message string
}
