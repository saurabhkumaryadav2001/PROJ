package main

import "fmt"

func main() {
    appName := "RecallAPI"
    port := 8080
    environment := "development"
    debug := true

    fmt.Printf("%s is running on :%d\n", appName, port)
    fmt.Printf("environment=%s debug=%t\n", environment, debug)
}
