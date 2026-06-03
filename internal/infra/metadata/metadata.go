package metadata

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/denisbrodbeck/machineid"
	"github.com/package-register/mocode/internal/version"
)

// SystemInfo contains system metadata information
type SystemInfo struct {
	GOOS      string
	GOARCH    string
	TERM      string
	SHELL     string
	Version   string
	GoVersion string
	MachineID string
}

// GetSystemInfo returns system metadata information
func GetSystemInfo() SystemInfo {
	return SystemInfo{
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		TERM:      os.Getenv("TERM"),
		SHELL:     filepath.Base(os.Getenv("SHELL")),
		Version:   version.Version,
		GoVersion: runtime.Version(),
		MachineID: getMachineID(),
	}
}

// getMachineID returns a unique machine identifier
func getMachineID() string {
	// Primary method: use machineid library with our own key
	if id, err := machineid.ProtectedID("mocode"); err == nil {
		return id
	}

	// Fallback 1: MAC address
	if macAddr, err := getMacAddr(); err == nil {
		return hashString(macAddr)
	}

	// Fallback 2: unknown
	return "unknown"
}

// getMacAddr returns the MAC address of the first active network interface
func getMacAddr() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 && len(iface.HardwareAddr) > 0 {
			if addrs, err := iface.Addrs(); err == nil && len(addrs) > 0 {
				return iface.HardwareAddr.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no active interface with mac address found")
}

// hashString hashes a string using HMAC-SHA256
func hashString(str string) string {
	hash := hmac.New(sha256.New, []byte("mocode"))
	hash.Write([]byte(str))
	return hex.EncodeToString(hash.Sum(nil))
}
