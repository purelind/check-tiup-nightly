package checker

import (
    "time"
)

type PlatformInfo struct {
    OS       string `json:"os"`
    Arch     string `json:"arch"`
    Platform string `json:"platform"`
}

type ComponentVersion struct {
    FullVersion  string `json:"full_version"`
    BaseVersion  string `json:"base_version"`
    GitHash      string `json:"git_hash"`
    CommitTime   time.Time `json:"commit_time"`
}

type Versions struct {
    TiUP       string                     `json:"tiup"`
    Python     string                     `json:"python"`
    Components map[string]ComponentVersion `json:"components"`
}

type Error struct {
    Stage     string    `json:"stage"`
    Error     string    `json:"error"`
    Timestamp time.Time `json:"timestamp"`
}

type CheckReport struct {
    Timestamp time.Time    `json:"timestamp"`
    Status    string       `json:"status"`
    Platform  string       `json:"platform"`
    OS        string       `json:"os"`
    Arch      string       `json:"arch"`
    Errors    []Error      `json:"errors,omitempty"`
    Version   Versions     `json:"version"`
}

type BranchCommitInfo struct {
    Component  string    `json:"component"`
    Branch    string    `json:"branch"`
    GitHash   string    `json:"git_hash"`
    CommitTime time.Time `json:"commit_time"`
    UpdatedAt  time.Time `json:"updated_at"`
}

