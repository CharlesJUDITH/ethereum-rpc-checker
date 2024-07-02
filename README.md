# ethereum-rpc-checker
![GitHub release (latest by date)](https://img.shields.io/github/v/release/charlesjudith/ethereum-rpc-checker)
![GitHub Workflow Status](https://img.shields.io/github/workflow/status/charlesjudith/ethereum-rpc-checker/Release)
![Docker Image Size (latest by date)](https://img.shields.io/docker/image-size/ghcr.io/charlesjudith/ethereum-rpc-checker/latest)

## Table of Contents

- [Introduction](#introduction)
- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration)

## Introduction

`ethereum-rpc-checker` is a Go application designed to check ETH RPC endpoints and the latest block on an RPC endpoint. It supports deployment in Docker containers and integrates with Prometheus for metrics collection.

## Features

- **Dockerized**: Easily deployable using Docker.
- **Prometheus Integration**: Built-in metrics for monitoring.
- **Configurable**: Flexible configuration via `config.yaml`.

## Installation

### Prerequisites

- Go 1.22 or later
- Docker

### Clone the repository

```sh
git clone https://github.com/charlesjudith/ethereum-rpc-checker.git
cd ethereum-rpc-checker
```
### Build the application

```sh
go build -o ethereum-rpc-checker ./cmd/ethereum-rpc-checker
```

### Run the application

```sh
./ethereum-rpc-checker
```

## Using Docker

### Build Docker image

```sh
docker build -t ghcr.io/charlesjudith/ethereum-rpc-checker:latest .
```

### Run Docker container

```sh
docker run -d -p 8080:8080 ghcr.io/charlesjudith/ethereum-rpc-checker:latest
```
## Usage

To run the application, ensure you have the config.yaml file in the same directory as the binary. You can customize the configuration to fit your needs.

```yaml
endpoints:
  - "http://localhost:8545"
  - "http://another-node:8545"
interval: 5
method: "eth_blockNumber"
prometheus:
  address: ":9090"
```

## Access Metrics
Once the application is running, you can access the Prometheus metrics at http://localhost:9090/metrics.

## Configuration

The application can be configured using a config.yaml file. Below is an example configuration:

```
endpoints:
  - "http://localhost:8545"
  - "http://another-node:8545"
interval: 5
method: "eth_blockNumber"
prometheus:
  address: ":9090"
```

**endpoints**: List of RPC endpoints to monitor.

**interval**: Time interval (in minutes) between checks.

**method**: RPC method to call.

**prometheus.address**: Address to expose Prometheus metrics.
