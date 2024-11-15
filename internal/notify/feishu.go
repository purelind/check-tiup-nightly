package notify

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "time"
)

const (
    EnvFeishuSuccessWebhook = "FEISHU_SUCCESS_WEBHOOK"
    EnvFeishuFailureWebhook = "FEISHU_FAILURE_WEBHOOK"
)

type Notifier struct {
    successWebhook string
    failureWebhook string
}

type Message struct {
    MsgType string `json:"msg_type"`
    Content struct {
        Text string `json:"text"`
    } `json:"content"`
}

func NewNotifier() *Notifier {
    return &Notifier{
        successWebhook: os.Getenv(EnvFeishuSuccessWebhook),
        failureWebhook: os.Getenv(EnvFeishuFailureWebhook),
    }
}

func (n *Notifier) SendSuccessNotification(platform string, version string) error {
    if n.successWebhook == "" {
        return nil
    }

    msg := Message{
        MsgType: "text",
        Content: struct {
            Text string `json:"text"`
        }{
            Text: fmt.Sprintf("✅ TiUP Nightly Check Success\nPlatform: %s\nTiUP Version: %s\nTime: %s", 
                platform,
                version, 
                time.Now().Format(time.RFC3339)),
        },
    }
    return n.send(n.successWebhook, msg)
}

func (n *Notifier) SendFailureNotification(platform string, version string, errors []ErrorDetail) error {
    if n.failureWebhook == "" {
        return nil
    }
    
    var errorText string
    for _, err := range errors {
        errorText += fmt.Sprintf("\n- [%s] %s (at %s)", 
            err.Stage, 
            err.Error,
            err.Timestamp)
    }
    
    msg := Message{
        MsgType: "text",
        Content: struct {
            Text string `json:"text"`
        }{
            Text: fmt.Sprintf("❌ TiUP Nightly Check Failed\nPlatform: %s\nTiUP Version: %s\nTime: %s\nErrors:%s", 
                platform,
                version, 
                time.Now().Format(time.RFC3339),
                errorText),
        },
    }
    return n.send(n.failureWebhook, msg)
}

func (n *Notifier) send(webhook string, msg Message) error {
    payload, err := json.Marshal(msg)
    if err != nil {
        return fmt.Errorf("marshal message failed: %v", err)
    }

    resp, err := http.Post(webhook, "application/json", bytes.NewBuffer(payload))
    if err != nil {
        return fmt.Errorf("send message failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("send message failed with status code: %d", resp.StatusCode)
    }

    return nil
}

type ErrorDetail struct {
    Stage     string
    Error     string
    Timestamp time.Time
}