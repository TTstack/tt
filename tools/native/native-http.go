//go:build ignore

package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "native-http") })
	if err := http.ListenAndServe(":80", nil); err != nil {
		panic(err)
	}
}
