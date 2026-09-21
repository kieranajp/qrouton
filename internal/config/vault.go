package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kieranajp/qrouton/internal/atomicfile"
)

type VaultStateInfo struct {
	InstallationID string
	Directory      string
}

func VaultState() (VaultStateInfo, error) {
	directory := filepath.Join(xdgDir(stateHomeEnvVar, stateHomeFallback), vaultStateDirectory)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return VaultStateInfo{}, fmt.Errorf("%w: %v", ErrVaultState, err)
	}
	result := VaultStateInfo{Directory: directory}
	err := atomicfile.WithLock(filepath.Join(directory, vaultInstallationLock), 0600, func() error {
		path := filepath.Join(directory, vaultInstallationFile)
		b, err := os.ReadFile(path)
		if err == nil {
			id := strings.TrimSpace(string(b))
			decoded, err := hex.DecodeString(id)
			if err != nil || len(decoded) != 16 {
				return ErrVaultState
			}
			result.InstallationID = id
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
		b = make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		result.InstallationID = hex.EncodeToString(b)
		return atomicfile.Replace(path, []byte(result.InstallationID+"\n"), 0600)
	})
	if err != nil {
		return VaultStateInfo{}, fmt.Errorf("%w: %v", ErrVaultState, err)
	}
	return result, nil
}
