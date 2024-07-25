package client

import (
	"github.com/sslwarp/internal/config"
)

type Client struct {
	//
	config *config.Config
}

func New(cfg *config.Config) (*Client, error) {
	return &Client {
		config: cfg,
	}, nil
}


func (c *Client) Run() error {
    // Implement client logic
    // - Connect to server
	
    // - Establish tunnel
    // - Forward local traffic through tunnel
    return nil
}

