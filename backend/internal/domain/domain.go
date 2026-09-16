// Package domain holds the core entities of the backup management system,
// independent of how they are stored (repository) or exposed (httpapi).
package domain

import "time"

// DBPasswordAAD is the crypto.Sealer column identifier used for
// Server.DBPasswordEncrypted, on both the encrypt side (handlers) and the
// decrypt side (scheduler, test-connection). It must be identical in every
// call site — a mismatch silently fails decryption (see docs/Memoria.md for
// a prior incident with the Azure SAS token AAD).
const DBPasswordAAD = "db_password_encrypted"

type ServerStatus string

const (
	ServerStatusPendingKey            ServerStatus = "pending_key"
	ServerStatusAwaitingAuthorization ServerStatus = "awaiting_authorization"
	ServerStatusReady                 ServerStatus = "ready"
	ServerStatusDisabled              ServerStatus = "disabled"
)

type BackupRunStatus string

const (
	BackupRunStatusQueued  BackupRunStatus = "queued"
	BackupRunStatusRunning BackupRunStatus = "running"
	BackupRunStatusSuccess BackupRunStatus = "success"
	BackupRunStatusFailed  BackupRunStatus = "failed"
)

type AdminUser struct {
	ID           string
	Email        string
	CPF          string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastLoginAt  *time.Time
}

type AdminSession struct {
	ID        string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
	UserAgent string
	IP        string
}

// StorageTargetType discriminates which concrete storage backend a
// StorageTarget configures.
type StorageTargetType string

const (
	StorageTargetTypeAzure      StorageTargetType = "azure"
	StorageTargetTypeS3         StorageTargetType = "s3"
	StorageTargetTypeFilesystem StorageTargetType = "filesystem"
)

// StorageTarget is a registered backup destination: Azure Blob Storage,
// an S3-compatible object store, or a local/NFS-mounted filesystem
// directory (Type discriminates which). Only the fields relevant to Type are
// populated; the others are zero/nil. Secrets (AzureSASTokenEncrypted,
// S3SecretAccessKeyEncrypted) are only ever populated in memory right after
// decryption — they are never serialized back to API responses.
type StorageTarget struct {
	ID   string
	Name string
	Type StorageTargetType

	// Azure fields (Type == StorageTargetTypeAzure).
	AzureAccountName       *string
	AzureContainerName     *string
	AzureSASTokenEncrypted []byte
	AzureSASTokenExpiresAt *time.Time

	// S3 fields (Type == StorageTargetTypeS3).
	S3Endpoint                 *string
	S3Region                   *string
	S3Bucket                   *string
	S3AccessKeyID              *string
	S3SecretAccessKeyEncrypted []byte
	S3UsePathStyle             bool

	// Filesystem fields (Type == StorageTargetTypeFilesystem).
	FSRootPath *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// DBEngine discriminates which database engine a Server's dump commands
// target. Postgres preserves today's trust/peer-auth behavior (no password
// required); MySQL and SQL Server require DBPasswordEncrypted to be set.
type DBEngine string

const (
	DBEnginePostgres  DBEngine = "postgres"
	DBEngineMySQL     DBEngine = "mysql"
	DBEngineSQLServer DBEngine = "sqlserver"
)

// DeploymentMode discriminates whether Server's database runs inside a
// Docker container (ContainerName required) or directly on the remote
// host/instance (ContainerName must be nil).
type DeploymentMode string

const (
	DeploymentModeDocker DeploymentMode = "docker"
	DeploymentModeHost   DeploymentMode = "host"
)

// Server is a registered backup target reachable over SSH.
// SSHPrivateKeyEncrypted and DBPasswordEncrypted are never serialized back
// to API responses.
type Server struct {
	ID                      string
	Name                    string
	Host                    string
	Port                    int
	SSHUser                 string
	DBEngine                DBEngine
	DeploymentMode          DeploymentMode
	ContainerName           *string // required iff DeploymentMode == DeploymentModeDocker
	DBName                  string
	DBUser                  string
	DBPasswordEncrypted     []byte // nullable; required iff DBEngine != DBEnginePostgres
	PgDumpExtraArgs         string
	MySQLDumpExtraArgs      string
	SqlCmdExtraArgs         string
	SSHPrivateKeyEncrypted  []byte
	SSHPublicKey            *string
	SSHKeyFingerprint       *string
	SSHHostKeyFingerprint   *string
	StorageTargetID         *string
	CronExpression          string
	Enabled                 bool
	Status                  ServerStatus
	LastTestConnectionAt    *time.Time
	LastTestConnectionOK    *bool
	LastTestConnectionError *string
	NextRunAt               *time.Time
	LastScheduledAt         *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// RetentionPolicy configures the GFS-style retention rule.
// ServerID nil means this is the single global default policy.
type RetentionPolicy struct {
	ID           string
	ServerID     *string
	RecentCount  int
	MonthlyCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type BackupRun struct {
	ID               string
	ServerID         string
	Status           BackupRunStatus
	StartedAt        *time.Time
	FinishedAt       *time.Time
	BlobName         *string
	BlobSizeBytes    *int64
	DumpDurationMS   *int
	UploadDurationMS *int
	ErrorMessage     *string
	LogOutput        *string
	CreatedAt        time.Time
}

// BackupRunDestinationBucket is one row of a time-bucketed (day or month),
// per-destination aggregate over backup_runs. Destination reflects the
// server's CURRENT storage_target_id, not the target in effect at the time
// of the run — see RN-BACKUP-031. StorageTargetID/StorageTargetName are nil
// when the server has no storage target configured.
type BackupRunDestinationBucket struct {
	Bucket            time.Time
	StorageTargetID   *string
	StorageTargetName *string
	RunCount          int
	TotalBytes        int64
}

type RetentionDeletion struct {
	ID          string
	ServerID    string
	BackupRunID *string
	BlobName    string
	DeletedAt   time.Time
	Reason      string
}
