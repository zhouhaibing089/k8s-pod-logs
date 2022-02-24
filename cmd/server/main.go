package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/spf13/pflag"

	"github.com/zhouhaibing089/k8s-pod-logs/pkg/storage/s3"
)

var (
	s3CfgPath string
	port      int
)

func init() {
	pflag.StringVar(&s3CfgPath, "s3-config-path", "", "Path to s3 configuration file")
	pflag.IntVar(&port, "port", 8080, "http port to listen on")
}

func main() {
	pflag.Parse()

	if s3CfgPath == "" {
		log.Fatalf("flag --s3-config-path is not set")
	}
	storage, err := s3.New(s3CfgPath)
	if err != nil {
		log.Fatalf("failed to new s3 storage: %s", err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/{namespace}/{name}/{container}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		namespace := vars["namespace"]
		name := vars["name"]
		container := vars["container"]
		key := namespace + "/" + name + "/" + container

		exists, err := storage.Has(key)
		if err != nil {
			log.Printf("failed to check existence of %s: %s", key, err)
			http.Error(w, "failed to check existence", http.StatusInternalServerError)
			return
		}
		if !exists {
			http.NotFound(w, r)
			return
		}

		data, err := storage.Get(key)
		if err != nil {
			log.Printf("failed to fetch data of %s: %s", key, err)
			http.Error(w, "failed to fetch data", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Add("Content-Type", "text/plain")
		w.Header().Add("Content-Length", fmt.Sprintf("%d", len(data)))
		w.Write(data)
	})

	http.ListenAndServe(fmt.Sprintf(":%d", port), router)
}
