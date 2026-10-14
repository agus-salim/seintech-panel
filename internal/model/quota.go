package model

type Quota struct {
	UserID          string `json:"user_id"`
	MaxDiskMB       int64  `json:"max_disk_mb"`       // 0 = unlimited
	UsedDiskMB      int64  `json:"used_disk_mb"`
	MaxBandwidthMB  int64  `json:"max_bandwidth_mb"`  // 0 = unlimited
	UsedBandwidthMB int64  `json:"used_bandwidth_mb"`
	MaxDomains      int    `json:"max_domains"`       // 0 = unlimited
	UsedDomains     int    `json:"used_domains"`
	MaxDatabases    int    `json:"max_databases"`     // 0 = unlimited
	UsedDatabases   int    `json:"used_databases"`
	MaxFTPAccounts  int    `json:"max_ftp_accounts"`  // 0 = unlimited
	UsedFTPAccounts int    `json:"used_ftp_accounts"`
}
