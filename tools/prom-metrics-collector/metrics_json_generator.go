package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	metricFamilies := readMetrics()

	jsonBytes, err := json.Marshal(metricFamilies)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(string(jsonBytes))
}
