package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
	"log/slog"
)

type RADb struct {
	target string
	dialer net.Dialer
}

const RADBServer = "whois.radb.net:43"

func NewRADb() (*RADb, error) {

	host, port, err := net.SplitHostPort(RADBServer)
	if err != nil {
		return nil, err
	}

	return &RADb{
		target: fmt.Sprintf("%s:%s", host, port),
		dialer: net.Dialer{
			Timeout: 5 * time.Second,
		},
	}, nil
}

var whoisRouteRegex = regexp.MustCompile("(?P<prefix>(([0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+)|([0-9a-f:]+::))/[0-9]+)")

func (db *RADb) query(fn func(n int, record []byte) error, queries ...string) error {

	conn, err := db.dialer.Dial("tcp", db.target)
	if err != nil {
		return err
	}
	defer conn.Close()

	if len(queries) > 1 {
		// enable persistent conn
		_ = conn.SetDeadline(time.Now().Add(time.Second * 5))
		_, err = conn.Write([]byte("!!\n"))
		if err != nil {
			return err
		}
	}

	scanner := bufio.NewScanner(conn)
	scanner.Split(bufio.ScanLines)
	// 16 MiB lines
	const bufferSize = 1024 * 1024 * 16
	scanner.Buffer(make([]byte, 0, bufferSize), bufferSize)

	for _, q := range queries {

		_ = conn.SetDeadline(time.Now().Add(time.Second * 5))
		_, err = conn.Write([]byte(strings.TrimSpace(q) + "\n"))
		if err != nil {
			return err
		}

		n := 0

		for scanner.Scan() {
			buf := bytes.Trim(scanner.Bytes(), "\r\n")
			if bytes.HasPrefix(buf, []byte("%")) || bytes.Equal(buf, []byte("C")) {
				// end of record
				break
			}
			err = fn(n, buf)
			if err != nil {
				return err
			}
			n++
		}

		if scanner.Err() != nil {
			return scanner.Err()
		}
	}

	if len(queries) > 1 {
		// exit
		_ = conn.SetDeadline(time.Now().Add(time.Second * 5))
		_, err = conn.Write([]byte("q\n"))
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *RADb) FetchIPInfo(ip net.IP) (result []string, err error) {
	var ipNet net.IPNet
	if ip4 := ip.To4(); ip4 != nil {
		ipNet = net.IPNet{
			IP: ip4,
			// single ip
			Mask: net.CIDRMask(len(ip4)*8, len(ip4)*8),
		}
	} else {
		ipNet = net.IPNet{
			IP: ip,
			// single ip
			Mask: net.CIDRMask(len(ip)*8, len(ip)*8),
		}
	}

	err = db.query(func(n int, record []byte) error {
		result = append(result, string(record))
		return nil
	}, fmt.Sprintf("!r%s,l", ipNet.String()))

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (db *RADb) FetchASNets(asn int) (result []net.IPNet, err error) {

	ix := whoisRouteRegex.SubexpIndex("prefix")
	if ix == -1 {
		panic("invalid regex prefix")
	}

	var prefixes []net.IPNet

	// Query IPv4 separately
	data4 := []byte{}
	err4 := db.query(func(n int, record []byte) error {
		if n == 0 {
			// skip ASN number reply
			return nil
		}
		data4 = append(data4, record...)
		return nil
	}, fmt.Sprintf("!gas%d", asn))

	if err4 == nil {
		matches := whoisRouteRegex.FindAllSubmatch(data4, -1)
		for _, match := range matches {
			_, ipNet, parseErr := net.ParseCIDR(string(match[ix]))
			if parseErr != nil {
				return nil, fmt.Errorf("invalid IPv4 CIDR %s: %w", string(match[ix]), parseErr)
			}
			prefixes = append(prefixes, *ipNet)
		}
	}

	// Query IPv6 separately
	data6 := []byte{}
	err6 := db.query(func(n int, record []byte) error {
		if n == 0 {
			// skip ASN number reply
			return nil
		}
		data6 = append(data6, record...)
		return nil
	}, fmt.Sprintf("!6as%d", asn))

	if err6 == nil {
		matches := whoisRouteRegex.FindAllSubmatch(data6, -1)
		for _, match := range matches {
			_, ipNet, parseErr := net.ParseCIDR(string(match[ix]))
			if parseErr != nil {
				return nil, fmt.Errorf("invalid IPv6 CIDR %s: %w", string(match[ix]), parseErr)
			}
			prefixes = append(prefixes, *ipNet)
		}
	}

	// Decide on error
	if err4 != nil && err6 != nil {
		return nil, fmt.Errorf("both IPv4 and IPv6 queries failed for ASN %d: %v, %v", asn, err4, err6)
	} else if err4 != nil {
		slog.Warn("IPv4 query failed for ASN, falling back to IPv6 only", "asn", asn, "err", err4, "prefixes", len(prefixes))
	} else if err6 != nil {
		slog.Warn("IPv6 query failed for ASN, falling back to IPv4 only", "asn", asn, "err", err6, "prefixes", len(prefixes))
	}

	return prefixes, nil
}
