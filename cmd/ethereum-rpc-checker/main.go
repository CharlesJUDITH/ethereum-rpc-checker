package main

import (
	"context"
    "crypto/tls"
    "crypto/x509"
	"flag"
	"fmt"
    "gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
    Endpoints  []Endpoint `yaml:"endpoints"`
    Interval   int        `yaml:"interval"`
    Method     string     `yaml:"method"`
    Debug      bool       `yaml:"debug"`
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
    debugMode  = flag.Bool("debug", false, "Enable debug mode")
    configFile = flag.String("config", "config.yaml", "Path to configuration file")
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
    helpFlag := flag.Bool("help", false, "Display help information")
    flag.Parse()

    if *helpFlag {
        printHelp()
        os.Exit(0)
    }

    log.Println("🚀 Starting Blockchain RPC Checker...")
    config, err := loadConfigFile(*configFile)
    if err != nil {
        log.Fatalf("❌ Failed to load configuration: %v", err)
    }

    
    // Set debug mode in config
    config.Debug = *debugMode

    // Log configuration
    log.Printf("📁 Loaded configuration:\n%s", safePrettyPrintConfig(config))
    
    ticker := time.NewTicker(time.Duration(config.Interval) * time.Minute)
    defer ticker.Stop()
    go func() {
        for range ticker.C {
            for _, endpoint := range config.Endpoints {
                checkBlockchainRPC(endpoint, config.Method, config.Debug)
            }
        }
    }()
    
    http.Handle("/metrics", promhttp.Handler())
    log.Printf("📊 Starting Prometheus HTTP server on %s\n", config.Prometheus.Address)
    log.Fatal(http.ListenAndServe(config.Prometheus.Address, nil))
}

func printHelp() {
    fmt.Println("Blockchain RPC Checker")
    fmt.Println("Usage: ethereum-rpc-checker [options]")
    fmt.Println("\nOptions:")
    fmt.Println("  -help\t\t\tDisplay this help message")
    fmt.Println("  -config string\tPath to configuration file (default \"config.yaml\")")
    fmt.Println("  -debug\t\tEnable debug mode for verbose output")
    fmt.Println("\nDescription:")
    fmt.Println("  This tool checks the health of blockchain RPC endpoints and exposes metrics for Prometheus.")
    fmt.Println("  It reads configuration from a YAML file and periodically checks the specified endpoints.")
    fmt.Println("\nConfiguration File Format:")
    fmt.Println("  endpoints:")
    fmt.Println("    - name: endpoint1")
    fmt.Println("      url: http://example1.com")
    fmt.Println("    - name: endpoint2")
    fmt.Println("      url: http://example2.com")
    fmt.Println("  interval: 5  # Check interval in minutes")
    fmt.Println("  method: eth_blockNumber  # RPC method to call")
    fmt.Println("  debug: false  # Set to true to enable debug mode (can also be set via command line)")
    fmt.Println("  prometheus:")
    fmt.Println("    address: :8080  # Address to expose Prometheus metrics")
}

const maxConfigDepth = 10

func loadConfigFile(filename string) (Config, error) {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        return Config{}, fmt.Errorf("❌ error reading config file: %v", err)
    }
    return loadConfig(data)
}

func loadConfig(data []byte) (Config, error) {
    var config Config
    dec := yaml.NewDecoder(strings.NewReader(string(data)))
    dec.KnownFields(true)
    
    err := dec.Decode(&config)
    if err != nil {
        return Config{}, fmt.Errorf("❌ error parsing config file: %v", err)
    }

    if err := validateConfig(&config, 0); err != nil {
        return Config{}, err
    }

    return config, nil
}

func validateConfig(config *Config, depth int) error {
    if depth > maxConfigDepth {
        return fmt.Errorf("config nesting too deep")
    }

    for _, endpoint := range config.Endpoints {
        if err := validateEndpoint(&endpoint, depth+1); err != nil {
            return err
        }
    }

    return nil
}

func validateEndpoint(endpoint *Endpoint, depth int) error {
    if depth > maxConfigDepth {
        return fmt.Errorf("endpoint nesting too deep")
    }

    if endpoint.Name == "" {
        return fmt.Errorf("endpoint name cannot be empty")
    }

    if endpoint.URL == "" {
        return fmt.Errorf("endpoint URL cannot be empty")
    }

    return nil
}

func maskSensitiveInfo(urlString string) string {
    parsedURL, err := url.Parse(urlString)
    if err != nil {
        return "invalid URL"
    }

    // Mask query parameters
    query := parsedURL.Query()
    for key := range query {
        if strings.Contains(strings.ToLower(key), "key") || 
           strings.Contains(strings.ToLower(key), "secret") || 
           strings.Contains(strings.ToLower(key), "token") {
            query.Set(key, "********")
        }
    }
    parsedURL.RawQuery = query.Encode()

    // Mask username and password in the URL
    if parsedURL.User != nil {
        username := parsedURL.User.Username()
        if username != "" {
            parsedURL.User = url.UserPassword("********", "********")
        }
    }

    return parsedURL.String()
}

func safePrettyPrintConfig(config Config) string {
    var sb strings.Builder

    if config.Debug {
        sb.WriteString("Configuration (Debug Mode):\n")
        sb.WriteString(fmt.Sprintf("  Interval: %d minutes\n", config.Interval))
        sb.WriteString(fmt.Sprintf("  Method: %s\n", config.Method))
        sb.WriteString(fmt.Sprintf("  Debug: %v\n", config.Debug))
        sb.WriteString(fmt.Sprintf("  Prometheus Address: %s\n", config.Prometheus.Address))
        sb.WriteString("  Endpoints:\n")
        for _, endpoint := range config.Endpoints {
            sb.WriteString(fmt.Sprintf("    - Name: %s\n", endpoint.Name))
            sb.WriteString(fmt.Sprintf("      URL: %s\n", maskSensitiveInfo(endpoint.URL)))
        }
        return sb.String()
    } else {
        return fmt.Sprintf("Interval: %d minutes\nMethod: %s\nNumber of Endpoints: %d",
            config.Interval, config.Method, len(config.Endpoints))
    }
}

func dialRPC(ctx context.Context, endpoint string) (RPCClient, error) {
    // Create a custom dialer
    dialer := &net.Dialer{
        Timeout:   30 * time.Second,
        KeepAlive: 30 * time.Second,
    }

    // Create a custom TLS configuration
    tlsConfig := &tls.Config{
        MinVersion: tls.VersionTLS12,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
            tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
        },
        InsecureSkipVerify: false,
        VerifyConnection: func(cs tls.ConnectionState) error {
            opts := x509.VerifyOptions{
                DNSName:       cs.ServerName,
                Intermediates: x509.NewCertPool(),
            }
            for _, cert := range cs.PeerCertificates[1:] {
                opts.Intermediates.AddCert(cert)
            }
            _, err := cs.PeerCertificates[0].Verify(opts)
            return err
        },
    }

    // Create a custom transport
    transport := &http.Transport{
        DialContext:           dialer.DialContext,
        TLSClientConfig:       tlsConfig,
        MaxIdleConnsPerHost:   100,
        IdleConnTimeout:       90 * time.Second,
        TLSHandshakeTimeout:   10 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
        ForceAttemptHTTP2:     true,
    }

    // Create a custom client with the new transport
    httpClient := &http.Client{
        Transport: transport,
        Timeout:   30 * time.Second,
    }

    client, err := rpc.DialHTTPWithClient(endpoint, httpClient)
    if err != nil {
        return nil, err
    }
    return &EthRPCClient{client}, nil
}

func checkBlockchainRPC(endpoint Endpoint, method string, debug bool) {
    logEndpoint := endpoint.Name
    if debug {
        logEndpoint = fmt.Sprintf("%s (%s)", endpoint.Name, maskSensitiveInfo(endpoint.URL))
    }
    log.Printf("🔍 Checking blockchain RPC endpoint: %s with method: %s\n", logEndpoint, method)
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    client, err := rpcDial(ctx, endpoint.URL)
    if err != nil {
        log.Printf("❌ Error connecting to blockchain RPC endpoint %s: %v", logEndpoint, err)
        rpcHealthy.WithLabelValues(endpoint.Name).Set(0)
        return
    }
    defer client.Close()

    var result string
    err = client.CallContext(ctx, &result, method)
    if err != nil {
        log.Printf("❌ Error calling %s on %s: %v", method, logEndpoint, err)
        rpcHealthy.WithLabelValues(endpoint.Name).Set(0)
        return
    }

    if debug {
        log.Printf("📡 Raw result from %s: %s\n", logEndpoint, result)
    }
    
    blockNum, err := hexToInt(result)
    if err != nil {
        log.Printf("❌ Error converting hex to int from %s: %v", logEndpoint, err)
        rpcHealthy.WithLabelValues(endpoint.Name).Set(0)
        return
    }

    rpcHealthy.WithLabelValues(endpoint.Name).Set(1)
    blockNumber.WithLabelValues(endpoint.Name).Set(float64(blockNum))
    log.Printf("✅ Block Number from %s: %d\n", logEndpoint, blockNum)
}

func hexToInt(hexStr string) (int64, error) {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	return strconv.ParseInt(hexStr, 16, 64)
}
