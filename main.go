package main

import (
    "log"
    "net/http"
    "godb/pkg/handlers"
    "godb/pkg/database"
)

func main() {
    err := database.InitDB()
    if err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    http.HandleFunc("/sol-route/comment", handlers.RequireAuth(handlers.PostComment))
    http.HandleFunc("/sol-route/batch-comments", handlers.RequireAuth(handlers.PostBatchComments))
    http.HandleFunc("/sol-route/like", handlers.RequireAuth(handlers.LikeMessage))
	http.HandleFunc("/sol-route/operation-status", handlers.RequireAuth(handlers.GetOperationStatus))
    log.Println("Server starting on :80")
    if err := http.ListenAndServe(":80", nil); err != nil {
        log.Fatal(err)
    }
}