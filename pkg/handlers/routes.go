package handlers

import (
    "encoding/json"
    "net/http"
    "godb/pkg/pump"
    "time"
    "godb/pkg/database"
    "log"
    "math/rand"
    "fmt"
)

type CommentRequest struct {
    Mint    string   `json:"mint"`
    Message string   `json:"message"`
    Link    string   `json:"link,omitempty"`
}

type BatchCommentRequest struct {
    Mint     string   `json:"mint"`
    Variable []int    `json:"variable"`
    Comments []string `json:"comments"`
}

type LikeRequest struct {
    MessageID string `json:"message_id"`
}

type ErrorResponse struct {
    Error       string `json:"error"`
    Field       string `json:"field,omitempty"`
    ExpectedType string `json:"expected_type,omitempty"`
    ReceivedType string `json:"received_type,omitempty"`
}

func sendJSONError(w http.ResponseWriter, status int, err ErrorResponse) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(err)
}

func PostComment(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    var req CommentRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    client, err := pump.NewPumpClient("")
    if err != nil {
        http.Error(w, "Failed to create client", http.StatusInternalServerError)
        return
    }
    err = client.Start()
    if err != nil {
        http.Error(w, "Failed to start client", http.StatusInternalServerError)
        return
    }
    _, err = client.ForThread()
    if err != nil {
        http.Error(w, "Failed to get thread token", http.StatusInternalServerError)
        return
    }
    err = client.PostComment(req.Mint, req.Message, req.Link)
    if err != nil {
        http.Error(w, "Failed to post comment", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "Comment posted successfully",
    })
}

func PostBatchComments(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        sendJSONError(w, http.StatusMethodNotAllowed, ErrorResponse{
            Error: "Method not allowed",
        })
        return
    }
    var rawData map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&rawData); err != nil {
        sendJSONError(w, http.StatusBadRequest, ErrorResponse{
            Error: "Invalid JSON format",
        })
        return
    }
    if mint, ok := rawData["mint"]; ok {
        if _, isString := mint.(string); !isString {
            sendJSONError(w, http.StatusBadRequest, ErrorResponse{
                Error:        "Invalid data type for field",
                Field:        "mint",
                ExpectedType: "string",
                ReceivedType: fmt.Sprintf("%T", mint),
            })
            return
        }
    }
    if variable, ok := rawData["variable"]; ok {
        arr, isArray := variable.([]interface{})
        if !isArray {
            sendJSONError(w, http.StatusBadRequest, ErrorResponse{
                Error:        "Invalid data type for field",
                Field:        "variable",
                ExpectedType: "array of integers",
                ReceivedType: fmt.Sprintf("%T", variable),
            })
            return
        }
        for i, v := range arr {
            if _, isFloat := v.(float64); !isFloat {
                sendJSONError(w, http.StatusBadRequest, ErrorResponse{
                    Error:        fmt.Sprintf("Invalid data type in variable array at index %d", i),
                    Field:        "variable",
                    ExpectedType: "integer",
                    ReceivedType: fmt.Sprintf("%T", v),
                })
                return
            }
        }
    }
    if comments, ok := rawData["comments"]; ok {
        arr, isArray := comments.([]interface{})
        if !isArray {
            sendJSONError(w, http.StatusBadRequest, ErrorResponse{
                Error:        "Invalid data type for field",
                Field:        "comments",
                ExpectedType: "array of strings",
                ReceivedType: fmt.Sprintf("%T", comments),
            })
            return
        }
        for i, comment := range arr {
            if _, isString := comment.(string); !isString {
                sendJSONError(w, http.StatusBadRequest, ErrorResponse{
                    Error:        fmt.Sprintf("Invalid data type in comments array at index %d", i),
                    Field:        "comments",
                    ExpectedType: "string",
                    ReceivedType: fmt.Sprintf("%T", comment),
                })
                return
            }
        }
    }
    var req BatchCommentRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        sendJSONError(w, http.StatusBadRequest, ErrorResponse{
            Error: "Failed to parse request body",
        })
        return
    }
    if req.Mint == "" {
        sendJSONError(w, http.StatusBadRequest, ErrorResponse{
            Error: "Mint address is required",
            Field: "mint",
        })
        return
    }
    authCookie, err := r.Cookie("Authorization")
    if err != nil {
        log.Printf("No auth cookie found: %v", err)
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    userID, err := database.GetUserID(authCookie.Value)
    if err != nil {
        log.Printf("Error getting user ID: %v", err)
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    operationID, err := database.CreateCommentOperation(userID, req.Mint)
    if err != nil {
        log.Printf("Error creating operation: %v", err)
        http.Error(w, "Failed to create operation", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(map[string]string{
        "operation_id": operationID,
    })
    go processBatchComments(operationID, req)
}

func GetOperationStatus(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    operationID := r.URL.Query().Get("operationid")
    if operationID == "" {
        http.Error(w, "Missing operation ID", http.StatusBadRequest)
        return
    }
    status, err := database.GetOperationStatus(operationID)
    if err != nil {
        http.Error(w, "Failed to get operation status", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "operation_id": operationID,
        "is_active": status,
    })
}

func processBatchComments(operationID string, req BatchCommentRequest) {
    minDelay := req.Variable[0]
    maxDelay := req.Variable[1]
    for _, comment := range req.Comments {
        shouldContinue, err := database.GetOperationStatus(operationID)
        if err != nil {
            log.Printf("Error checking operation status: %v", err)
            return
        }
        if !shouldContinue {
            return
        }
        client, err := pump.NewPumpClient("")
        if err != nil {
            log.Printf("Error creating client: %v", err)
            continue
        }
        err = client.Start()
        if err != nil {
            log.Printf("Error starting client: %v", err)
            continue
        }
        _, err = client.ForThread()
        if err != nil {
            log.Printf("Error getting thread token: %v", err)
            continue
        }
        delay := time.Duration(rand.Intn(maxDelay-minDelay+1)+minDelay) * time.Second
        time.Sleep(delay)
        err = client.PostComment(req.Mint, comment, "")
        if err != nil {
            log.Printf("Error posting comment: %v", err)
            continue
        }
    }
    database.FinishOperation(operationID)
}

func LikeMessage(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    var req LikeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    client, err := pump.NewPumpClient("")
    if err != nil {
        http.Error(w, "Failed to create client", http.StatusInternalServerError)
        return
    }
    err = client.Start()
    if err != nil {
        http.Error(w, "Failed to start client", http.StatusInternalServerError)
        return
    }
    err = client.LikeMessage(req.MessageID)
    if err != nil {
        http.Error(w, "Failed to like message", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "Message liked successfully",
    })
}