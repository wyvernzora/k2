package vm

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

type qgaResponse struct {
	Return json.RawMessage `json:"return"`
	Error  *struct {
		Class string `json:"class"`
		Desc  string `json:"desc"`
	} `json:"error"`
}

type qgaInterface struct {
	Name        string `json:"name"`
	IPAddresses []struct {
		IPAddress     string `json:"ip-address"`
		IPAddressType string `json:"ip-address-type"`
		Prefix        int    `json:"prefix"`
	} `json:"ip-addresses"`
}

type GuestIPv4 struct {
	Interface string
	Address   string
	Prefix    int
}

func firstGuestIPv4(meta Metadata) (string, error) {
	ip, err := BestGuestIPv4(meta)
	if err != nil {
		return "", err
	}
	return ip.Address, nil
}

func BestGuestIPv4(meta Metadata) (GuestIPv4, error) {
	return bestGuestIPv4(meta, qgaDefaultAttempts)
}

func bestGuestIPv4(meta Metadata, attempts int) (GuestIPv4, error) {
	ips, err := guestIPv4Candidates(meta, attempts)
	if err != nil {
		return GuestIPv4{}, err
	}
	if len(ips) == 0 {
		return GuestIPv4{}, fmt.Errorf("no guest IPv4 address reported by qemu guest agent")
	}
	sort.Slice(ips, func(i, j int) bool {
		leftPhysical := isPhysicalGuestInterface(ips[i].Interface)
		rightPhysical := isPhysicalGuestInterface(ips[j].Interface)
		if leftPhysical != rightPhysical {
			return leftPhysical
		}
		leftPrimary := isPrimaryGuestAddress(ips[i])
		rightPrimary := isPrimaryGuestAddress(ips[j])
		if leftPrimary != rightPrimary {
			return leftPrimary
		}
		if ips[i].Interface != ips[j].Interface {
			return ips[i].Interface < ips[j].Interface
		}
		return ips[i].Address < ips[j].Address
	})
	return ips[0], nil
}

// guestIPv4s feeds status/list displays: one QGA attempt only. Burning
// the full retry backoff (~25s) per VM turns `vm list` into a hang when
// any guest agent is unresponsive.
func guestIPv4s(meta Metadata) ([]string, error) {
	candidates, err := guestIPv4Candidates(meta, 1)
	if err != nil {
		return nil, err
	}
	var ips []string
	for _, candidate := range candidates {
		ips = append(ips, candidate.Address)
	}
	sort.Strings(ips)
	return ips, nil
}

func guestIPv4Candidates(meta Metadata, attempts int) ([]GuestIPv4, error) {
	if meta.QGAPort == 0 {
		return nil, fmt.Errorf("qemu guest agent port is not configured")
	}
	var interfaces []qgaInterface
	if err := qgaCommand(meta, "guest-network-get-interfaces", nil, &interfaces, attempts); err != nil {
		return nil, err
	}
	var ips []GuestIPv4
	for _, iface := range interfaces {
		for _, addr := range iface.IPAddresses {
			if addr.IPAddressType != "ipv4" || addr.IPAddress == "" || strings.HasPrefix(addr.IPAddress, "127.") {
				continue
			}
			ips = append(ips, GuestIPv4{Interface: iface.Name, Address: addr.IPAddress, Prefix: addr.Prefix})
		}
	}
	return ips, nil
}

func isPrimaryGuestAddress(ip GuestIPv4) bool {
	return ip.Prefix > 0 && ip.Prefix < 32
}

func isPhysicalGuestInterface(name string) bool {
	return strings.HasPrefix(name, "en") || strings.HasPrefix(name, "eth") || strings.HasPrefix(name, "wl")
}

const qgaDefaultAttempts = 8

// qgaCommand retries the whole dial+sync+command exchange: the qemu chardev
// socket races on rapid reconnects (the previous client may not be detached
// yet, silently eating the request), and the agent itself can be briefly
// unresponsive. Every serious QGA client retries. Status views pass
// attempts=1 to stay snappy.
func qgaCommand(meta Metadata, execute string, arguments map[string]any, out any, attempts int) error {
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			// Growing backoff: the chardev wedge after a rapid reconnect can
			// take several seconds to clear (observed >2s in e2e runs).
			time.Sleep(time.Duration(attempt) * 750 * time.Millisecond)
		}
		if err = qgaCommandOnce(meta, execute, arguments, out); err == nil {
			return nil
		}
	}
	return err
}

func qgaCommandOnce(meta Metadata, execute string, arguments map[string]any, out any) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(meta.QGAPort)), time.Second)
	if err != nil {
		return fmt.Errorf("connect qemu guest agent: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)
	syncID := time.Now().UnixNano()
	if err := encoder.Encode(map[string]any{
		"execute":   "guest-sync",
		"arguments": map[string]any{"id": syncID},
	}); err != nil {
		return err
	}
	// The guest-agent channel is a stream that outlives TCP connections:
	// replies a previous client never read are still queued and would be
	// misattributed to our command. Drain everything (stale returns AND
	// stale errors) until the response carrying our sync token.
	for {
		var response qgaResponse
		if err := decoder.Decode(&response); err != nil {
			return fmt.Errorf("guest-sync: %w", err)
		}
		if response.Return == nil {
			continue
		}
		var id int64
		if err := json.Unmarshal(response.Return, &id); err == nil && id == syncID {
			break
		}
	}
	request := map[string]any{"execute": execute}
	if arguments != nil {
		request["arguments"] = arguments
	}
	if err := encoder.Encode(request); err != nil {
		return err
	}
	response, err := readQGAResponse(decoder)
	if err != nil {
		return err
	}
	if out != nil {
		if err := json.Unmarshal(response.Return, out); err != nil {
			return err
		}
	}
	return nil
}

func readQGAResponse(decoder *json.Decoder) (qgaResponse, error) {
	for {
		var response qgaResponse
		if err := decoder.Decode(&response); err != nil {
			return qgaResponse{}, err
		}
		if response.Error != nil {
			return qgaResponse{}, fmt.Errorf("qemu guest agent error %s: %s", response.Error.Class, response.Error.Desc)
		}
		if response.Return != nil {
			return response, nil
		}
	}
}
