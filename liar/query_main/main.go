package main

import (
	"encoding/json"
	"fmt"
	"liar/query"
	"os"
)

func main() {
	cfg := query.QueryConfig{
		ServicePort:  9011,
		DomainID:     os.Getenv("MG_WORKSPACE_ID"),
		ChannelID:    os.Getenv("MG_CHANNEL_ID"),
		Offset:       0,
		Limit:        10,
		ClientSecret: os.Getenv("MG_DEVICE_SECRET"),
		Timeout:      10,
	}
	result := query.FetchMessages(cfg)
	data, _ := json.MarshalIndent(result, "", "    ")
	fmt.Println(string(data))
}
