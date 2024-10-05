package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Endpoints  []Endpoint `yaml:"endpoints"`
	Interval   int        `yaml:"interval"`
	Method     string     `yaml:"method"`
	Prometheus struct {
		Address string `yaml:"address"`
	} `yaml:"prometheus"`
}

type Endpoint struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type RPCClient interface {
	CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error
	Close()
}

type EthRPCClient struct {
	client *rpc.Client
}

func (e *EthRPCClient) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	return e.client.CallContext(ctx, result, method, args...)
}

func (e *EthRPCClient) Close() {
	e.client.Close()
}

var (
	rpcHealthy = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "blockchain_rpc_healthy",
		Help: "Indicates if the blockchain RPC endpoint is healthy (1 for healthy, 0 for unhealthy).",
	}, []string{"endpoint"})
	blockNumber = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "blockchain_block_number",
		Help: "The current block number of the blockchain.",
	}, []string{"endpoint"})
	rpcDial = dialRPC
)

func init() {
	prometheus.MustRegister(rpcHealthy)
	prometheus.MustRegister(blockNumber)
}

func main() {
	log.Println("🚀 Starting Blockchain RPC Checker...")

	config := loadConfigFile("config.yaml")
	log.Printf("📁 Loaded configuration: %+v\n", config)

	ticker := time.NewTicker(time.Duration(config.Interval) * time.Minute)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			for _, endpoint := range config.Endpoints {
				checkBlockchainRPC(endpoint, config.Method)
			}
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("📊 Starting Prometheus HTTP server on %s\n", config.Prometheus.Address)
	log.Fatal(http.ListenAndServe(config.Prometheus.Address, nil))
}

func loadConfigFile(filename string) Config {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("❌ Error reading config file: %v", err)
	}
	return loadConfig(data)
}

func loadConfig(data []byte) Config {
	var config Config
	err := yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("❌ Error parsing config file: %v", err)
	}
	return config
}

func dialRPC(ctx context.Context, endpoint string) (RPCClient, error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		Timeout: 30 * time.Second, // Set a timeout for the entire request
	}

	client, err := rpc.DialHTTPWithClient(endpoint, httpClient)
	if err != nil {
		return nil, err
	}
	return &EthRPCClient{client}, nil
}

func checkBlockchainRPC(endpoint Endpoint, method string) {
	log.Printf("🔍 Checking blockchain RPC endpoint: %s with method: %s\n", endpoint.URL, method)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := rpcDial(ctx, endpoint.URL)
	if err != nil {
		log.Printf("❌ Error connecting to blockchain RPC endpoint: %v", err)
		rpcHealthy.WithLabelValues(endpoint.Name).Set(0)
		return
	}
	defer client.Close()

	var result string
	err = client.CallContext(ctx, &result, method)
	if err != nil {
		log.Printf("❌ Error calling %s: %v", method, err)
		rpcHealthy.WithLabelValues(endpoint.Name).Set(0)
		return
	}

	log.Printf("📡 Raw result from %s: %s\n", endpoint.URL, result)
	blockNum, err := hexToInt(result)
	if err != nil {
		log.Printf("❌ Error converting hex to int from %s: %v", endpoint.URL, err)
		rpcHealthy.WithLabelValues(endpoint.Name).Set(0)
		return
	}

	rpcHealthy.WithLabelValues(endpoint.Name).Set(1)
	blockNumber.WithLabelValues(endpoint.Name).Set(float64(blockNum))
	log.Printf("✅ Block Number from %s: %d\n", endpoint.URL, blockNum)
}

func hexToInt(hexStr string) (int64, error) {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	return strconv.ParseInt(hexStr, 16, 64)
}
