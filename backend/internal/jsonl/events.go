package jsonl

// 本文件定义 --json 模式下 stdout 上的全部线上类型，与
// docs/architecture/cli-contract.md §3、§4 一一对应（字段顺序即输出顺序）。
// 修改须走"契约"PR，并保持 contract_test.go 通过。

// 事件类型（type 字段取值）。
const (
	TypeProgress = "progress"
	TypeLog      = "log"
	TypeError    = "error"
	TypeResult   = "result"
)

// 进度阶段（Progress.Phase 取值）。
const (
	PhaseScan    = "scan"
	PhasePack    = "pack"
	PhaseRestore = "restore"
)

// 日志级别（Log.Level 取值）。
const (
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// result 事件的 command 字段取值。
const (
	CommandVersion = "version"
	CommandBackup  = "backup"
	CommandDryRun  = "dry-run"
	CommandRestore = "restore"
	CommandList    = "list"
	CommandInfo    = "info"
	CommandDelete  = "delete"
	CommandConfig  = "config"
)

// 错误码（Error.Code 取值），见契约 §4。
const (
	CodeIO               = "E_IO"
	CodeNotFound         = "E_NOT_FOUND"
	CodeTargetNotEmpty   = "E_TARGET_NOT_EMPTY"
	CodeCorrupt          = "E_CORRUPT"
	CodeExists           = "E_EXISTS"
	CodeInternal         = "E_INTERNAL"
	CodeUsage            = "E_USAGE"
	CodeAuth             = "E_AUTH"
	CodePasswordRequired = "E_PASSWORD_REQUIRED"
	CodeCancelled        = "E_CANCELLED"
)

// ErrorCodes 列出全部错误码，供契约测试与 cli 的退出码映射使用。
var ErrorCodes = []string{
	CodeIO, CodeNotFound, CodeTargetNotEmpty, CodeCorrupt, CodeExists, CodeInternal,
	CodeUsage, CodeAuth, CodePasswordRequired, CodeCancelled,
}

// Progress 是进度事件。TotalFiles/TotalBytes 只在 pack、restore 阶段给出。
type Progress struct {
	Type       string `json:"type"`
	Phase      string `json:"phase"`
	Files      int64  `json:"files"`
	Bytes      int64  `json:"bytes"`
	TotalFiles *int64 `json:"totalFiles,omitempty"`
	TotalBytes *int64 `json:"totalBytes,omitempty"`
	Current    string `json:"current"`
}

// Log 是日志事件。
type Log struct {
	Type    string `json:"type"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// Error 是失败时的最后一个事件。
type Error struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// VersionResult 是 --version 的结果。
type VersionResult struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Version string `json:"version"`
}

// BackupResult 是 backup 的结果。
type BackupResult struct {
	Type        string `json:"type"`
	Command     string `json:"command"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Files       int64  `json:"files"`
	Dirs        int64  `json:"dirs"`
	Bytes       int64  `json:"bytes"`
	OutputBytes int64  `json:"outputBytes"`
	Compressed  bool   `json:"compressed"`
	Encrypted   bool   `json:"encrypted"`
	Skipped     int64  `json:"skipped"`
	Warnings    int64  `json:"warnings"`
	MS          int64  `json:"ms"`
}

// DryRunResult 是 backup --dry-run 的结果。
type DryRunResult struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Files   int64  `json:"files"`
	Dirs    int64  `json:"dirs"`
	Bytes   int64  `json:"bytes"`
	Skipped int64  `json:"skipped"`
}

// RestoreResult 是 restore 的结果。
type RestoreResult struct {
	Type     string `json:"type"`
	Command  string `json:"command"`
	Target   string `json:"target"`
	Files    int64  `json:"files"`
	Dirs     int64  `json:"dirs"`
	Bytes    int64  `json:"bytes"`
	Warnings int64  `json:"warnings"`
	MS       int64  `json:"ms"`
}

// BackupInfo 描述仓库中的一个备份，用于 list 与 info。
// CreatedAt 为 RFC 3339（秒精度，带时区偏移）。
type BackupInfo struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	OutputBytes   int64  `json:"outputBytes"`
	CreatedAt     string `json:"createdAt"`
	Source        string `json:"source"`
	Compressed    bool   `json:"compressed"`
	Encrypted     bool   `json:"encrypted"`
	Files         int64  `json:"files"`
	Dirs          int64  `json:"dirs"`
	Bytes         int64  `json:"bytes"`
	FormatVersion int    `json:"formatVersion"`
	Corrupt       bool   `json:"corrupt"`
}

// ListResult 是 list 的结果。没有备份时 Backups 必须是空切片（编码为 []），不能是 nil。
type ListResult struct {
	Type    string       `json:"type"`
	Command string       `json:"command"`
	Repo    string       `json:"repo"`
	Backups []BackupInfo `json:"backups"`
}

// InfoResult 是 info 的结果，BackupInfo 字段平铺。
type InfoResult struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	BackupInfo
}

// DeleteResult 是 delete 的结果。
type DeleteResult struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Name    string `json:"name"`
	Path    string `json:"path"`
}

// ConfigResult 是 config get/set 的结果（完整配置）。
type ConfigResult struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Repo    string `json:"repo"`
}
