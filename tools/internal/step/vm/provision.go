package vm

import "fmt"

type ProvisionTarget struct {
	Host      string
	Port      int
	GuestIPv4 GuestIPv4
}

func ResolveProvisionTarget(repoRoot string, id string) (ProvisionTarget, error) {
	meta, err := loadMetadata(repoRoot, id)
	if err != nil {
		return ProvisionTarget{}, err
	}

	guestIP, guestErr := BestGuestIPv4(meta)
	if meta.SSHPort != 0 {
		return ProvisionTarget{
			Host:      "127.0.0.1",
			Port:      meta.SSHPort,
			GuestIPv4: guestIP,
		}, nil
	}
	if guestErr != nil {
		return ProvisionTarget{}, fmt.Errorf("resolve guest SSH address for VM %s: %w", id, guestErr)
	}
	return ProvisionTarget{
		Host:      guestIP.Address,
		Port:      22,
		GuestIPv4: guestIP,
	}, nil
}
