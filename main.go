package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

type TransferEvents struct {
	From  common.Address
	To    common.Address
	Value *big.Int
}

type Metric struct {
	Address common.Address
	Count   int
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file")
	}
	ctx := context.Background()

	apiKey := os.Getenv("ETH_API_KEY")
	url := fmt.Sprintf("https://go.getblock.io/%s", apiKey)

	client, err := ethclient.DialContext(ctx, url)
	if err != nil {
		log.Fatal("error in dialing Ethereum client")
		return
	}

	metrics, err := currentBlock(ctx, client)
	if err != nil {
		fmt.Printf("error in currenBlock:%v", err)
	}

	for i := 0; i < 5; i++ {
		fmt.Printf("address %v used ERC20 %v times\n", metrics[i].Address, metrics[i].Count)
	}
}

func currentBlock(ctx context.Context, client *ethclient.Client) ([]Metric, error) {
	block, err := client.HeaderByNumber(ctx, nil)

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve the latest block header: %v", err)
	}

	latestBlockNumber := block.Number
	blockNumber := new(big.Int).Sub(latestBlockNumber, big.NewInt(int64(99)))

	transferEventABI := `[{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"}]`

	transferEventSignature := []byte("Transfer(address,address,uint256)")
	transferEventHash := crypto.Keccak256Hash(transferEventSignature)

	query := ethereum.FilterQuery{
		FromBlock: blockNumber,
		ToBlock:   latestBlockNumber,
		Topics:    [][]common.Hash{{transferEventHash}},
	}

	logs, err := client.FilterLogs(ctx, query)

	if err != nil {
		return nil, fmt.Errorf("failed to filter logs: %v", err)
	}

	contractAbi, err := abi.JSON(strings.NewReader(string(transferEventABI)))
	if err != nil {
		return nil, fmt.Errorf("failed in marshall abi contract: %v", err)
	}

	metric := make(map[common.Address]int)

	for _, vLog := range logs {
		if len(vLog.Topics) != 3 || len(vLog.Data) == 0 {
			continue
		}

		var transferEvent TransferEvents
		transferEvent.From = common.HexToAddress(vLog.Topics[1].Hex())
		transferEvent.To = common.HexToAddress(vLog.Topics[2].Hex())

		err := contractAbi.UnpackIntoInterface(&transferEvent, "Transfer", vLog.Data)
		if err != nil {
			fmt.Println("Error unpacking from ABI: ", err)
			continue
		}

		metric[transferEvent.From]++
		metric[transferEvent.To]++
	}

	return SortAddressesByCount(metric)
}

func SortAddressesByCount(logsMap map[common.Address]int) ([]Metric, error) {
	if len(logsMap) == 0 {
		return nil, fmt.Errorf("no logs in map to sort")
	}

	counters := make([]Metric, 0, len(logsMap))

	for address, count := range logsMap {
		if address != (common.Address{}) {
			counters = append(counters, Metric{Address: address, Count: count})
		}
	}

	// Сортировка среза  в порядке убывания.
	sort.Slice(counters, func(i, j int) bool {
		return counters[i].Count > counters[j].Count
	})

	return counters, nil
}
