package network

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"
)

const (
	ifaceLength       = 7
	retryGenerate     = 5
	randomBytesLength = 32
	prefix            = "fl"
	tapPrefix         = "tap"
	// It's vtap only to save space.
	macvtapPrefix = "vtap"
)

type TapDetails struct {
	VmIp  net.IP
	TapIp net.IP
	Mask  net.IP
}

func GetTapDetails(index int) TapDetails {
	return TapDetails{
		VmIp:  net.IPv4(169, 254, byte(((4*index)+1)/256), byte(((4*index)+1)%256)),
		TapIp: net.IPv4(169, 254, byte(((4*index)+2)/256), byte(((4*index)+2)%256)),
		Mask:  net.IPv4(255, 255, 255, 252),
	}
}

func NewIfaceName() (string, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return "", interfaceErrorf("failed to enumerate links: %s", err)
	}

	highestLink := -1

	// Get the next highest link available
	for _, link := range links {
		if strings.HasPrefix(link.Attrs().Name, "hypercore-") {
			idxStr := strings.ReplaceAll(link.Attrs().Name, "hypercore-", "")
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return "", interfaceErrorf("got invalid link %s: %s", link.Attrs().Name, err)
			}

			if idx > highestLink {
				highestLink = idx
			}
		}
	}

	return "hypercore-" + strconv.Itoa(highestLink+1), nil
}

func generateRandomName(prefix string) (string, error) {
	id := make([]byte, randomBytesLength)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		return "", interfaceErrorf("random generator error: %s", err.Error())
	}

	return prefix + hex.EncodeToString(id)[:ifaceLength], nil
}
