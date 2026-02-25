package main

import (
	"e2eapp/backend/src/router"
	"fmt"
	"net/http"
)

func main() {
	addr, handler := router.Init()
	fmt.Printf("Server starting at http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		panic(err)
	}
}
