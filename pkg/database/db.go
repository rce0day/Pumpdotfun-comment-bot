package database

import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
    "github.com/google/uuid"
    "fmt"
)

var DB *sql.DB

type CommentOperation struct {
    UserID      int    `json:"userid"`
    Mint        string `json:"mint"`
    Status      string `json:"status"`
    OperationID string `json:"operationid"`
}

func CreateCommentOperation(userID int, mint string) (string, error) {
    operationID := uuid.New().String()
    _, err := DB.Exec(`
        INSERT INTO comment_operations (userid, mint, status, operationid) 
        VALUES (?, ?, 'ongoing', ?)`,
        userID, mint, operationID)
    if err != nil {
        return "", err
    }
    return operationID, nil
}

func GetOperationStatus(operationID string) (bool, error) {
    var status string
    err := DB.QueryRow(`
        SELECT status 
        FROM comment_operations 
        WHERE operationid = ?`,
        operationID).Scan(&status)
    if err == sql.ErrNoRows {
        return false, nil
    }
    if err != nil {
        return false, err
    }
    return status == "ongoing", nil
}

func FinishOperation(operationID string) error {
    _, err := DB.Exec(`
        UPDATE comment_operations 
        SET status = 'finished' 
        WHERE operationid = ? 
        AND status = 'ongoing'`,
        operationID)
    return err
}

func InitDB() error {
    var err error
    DB, err = sql.Open("mysql", "root:root@tcp(localhost:3306)/auth_bot")
    if err != nil {
        return err
    }
    err = DB.Ping()
    if err != nil {
        return err
    }
    return nil
}

func CheckAuthToken(token string) (bool, error) {
    var exists bool
    err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM auth WHERE cookie = ?)", token).Scan(&exists)
    if err != nil {
        return false, err
    }
    return exists, nil
}

func GetUserID(token string) (int, error) {
    var userID int
    err := DB.QueryRow("SELECT userid FROM auth WHERE cookie = ?", token).Scan(&userID)
    if err == sql.ErrNoRows {
        return 0, fmt.Errorf("no user found with provided token")
    }
    if err != nil {
        return 0, err
    }
    return userID, nil
}

