package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/canopy-network/canopy/cmd/rpc"
	"github.com/canopy-network/canopy/lib"
)

var (
	chainID    = flag.Uint64("chainID", 1, "Blockchain chain ID")
	rpcURL     = flag.String("url", "http://localhost:50002", "rpc url of the node")
	metricsURL = flag.String("metricsURL", "http://localhost:9090/metrics", "metrics url of the node")
	dbName     = flag.String("dbName", "metrics.sqlite3", "database name")
)

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	logger := lib.NewDefaultLogger()

	flag.Parse()

	metrics, err := NewMetrics(*dbName, *metricsURL)
	if err != nil {
		logger.Errorf("failed to create metrics database: %v", err)
		return
	}

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

	rcManager := rpc.NewRCManager(GatherMetrics(metrics, logger), config, logger)
	rcManager.Start()

	logger.Info("listening to new blocks")
	<-sigChan
	logger.Info("closing the program")
}

// GatherStats() collects information from the chain on each new block
func GatherMetrics(metrics *Metrics, logger lib.LoggerI) func(info *lib.RootChainInfo) {
	return func(info *lib.RootChainInfo) {
		if err := metrics.Scrap(); err != nil {
			logger.Errorf("failed to scrap metrics: %v\n", err)
		}

		blkProcessTime, err := metrics.GetMetric(BlockProcessingTime)
		if err != nil {
			logger.Errorf("failed to get block processing time: %v\n", err)
		}

		blkSize, err := metrics.GetMetric(BlockSize)
		if err != nil {
			logger.Errorf("failed to get block size: %v\n", err)
		}

		partitionTime, err := metrics.GetMetric(DBPartitionTime)
		if err != nil {
			logger.Errorf("failed to get partition time: %v\n", err)
		}

		metric := &Metric{
			Height:         info.Height,
			PartitionTime:  partitionTime,
			BlockBuildTime: blkProcessTime,
			BlockSize:      uint64(blkSize),
		}

		if err := metrics.InsertMetric(metric); err != nil {
			logger.Errorf("failed to insert metric: %v\n", err)
		}
	}
}
