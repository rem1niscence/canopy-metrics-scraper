package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/canopy-network/canopy/cmd/rpc"
	"github.com/canopy-network/canopy/lib"
)

var (
	chainID = flag.Uint64("chainID", 1, "Blockchain chain ID")
	rpcURL  = flag.String("url", "http://localhost:50002", "rpc url of the node")
)

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	logger := lib.NewDefaultLogger()

	flag.Parse()

	config := lib.Config{
		MainConfig: lib.MainConfig{
			ChainId: *chainID,
			RootChain: []lib.RootChain{
				{
					ChainId: *chainID,
					Url:     *rpcURL,
				},
			},
		},
	}

	rcManager := rpc.NewRCManager(GatherStats, config, logger)
	rcManager.Start()

	// wait for the root chain to be ready as the connection attempt is asynchronous
	time.Sleep(3 * time.Second)

	logger.Info("listening to new blocks")
	<-sigChan
	logger.Info("closing the program")
}

// GatherStats() collects information from the chain on each new block
func GatherStats(info *lib.RootChainInfo) {
	fmt.Println("----- New block ----")
	fmt.Printf("%+v\n", info)
}
