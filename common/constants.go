package common

const (
	Tcp            = "tcp4"
	Ip4Format      = "localhost:%s"
	MaxPayloadSize = 2048
	PublicKeySize  = 32
	HashSize       = 32
)

const (
	GeneratorBalance uint64 = 10_000_000_000_000_000_000
)

const (
	SyncTimeoutSeconds = 1
	// little smaller than 2tick
	FinalityTimeoutMilSec = 400
)
