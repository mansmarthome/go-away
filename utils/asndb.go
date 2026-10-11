package utils

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

const asnDatabaseMaxAge = 7 * 24 * time.Hour

type ASNRecord struct {
	ASN     uint32
	Country string
}

type ASNLookup interface {
	Lookup(ip net.IP) (ASNRecord, bool, error)
	Close() error
}

type asnDatabase struct {
	reader *maxminddb.Reader
}

type asnRecord struct {
	ASN     uint32 `maxminddb:"autonomous_system_number"`
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

func OpenASNDatabase(path string) (ASNLookup, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("ASN database path is a directory: %s", path)
	}
	if time.Since(stat.ModTime()) > asnDatabaseMaxAge {
		slog.Warn("ASN database is older than 7 days", "path", path, "modified", stat.ModTime())
	}

	reader, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	slog.Warn("opened ASN database", "path", path, "type", reader.Metadata.DatabaseType, "build_epoch", reader.Metadata.BuildEpoch)

	db := &asnDatabase{reader: reader}
	if err := db.sanity(); err != nil {
		_ = reader.Close()
		return nil, err
	}
	return db, nil
}

func (db *asnDatabase) Lookup(ip net.IP) (ASNRecord, bool, error) {
	if ip == nil {
		return ASNRecord{}, false, fmt.Errorf("nil IP")
	}
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	var record asnRecord
	_, ok, err := db.reader.LookupNetwork(ip, &record)
	if err != nil {
		return ASNRecord{}, false, err
	}
	if !ok {
		return ASNRecord{}, false, nil
	}
	return ASNRecord{ASN: record.ASN, Country: normalizeCountry(record.Country.ISOCode)}, true, nil
}

func normalizeCountry(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" || code == "NONE" {
		return ""
	}
	return code
}

func (db *asnDatabase) Close() error {
	if db == nil || db.reader == nil {
		return nil
	}
	return db.reader.Close()
}

func (db *asnDatabase) sanity() error {
	var sawASN, sawCountry bool
	for _, raw := range []string{"1.1.1.1", "8.8.8.8"} {
		rec, ok, err := db.Lookup(net.ParseIP(raw))
		if err != nil {
			return fmt.Errorf("ASN database lookup %s: %w", raw, err)
		}
		if ok && rec.ASN != 0 {
			sawASN = true
		}
		if rec.Country != "" {
			sawCountry = true
		}
	}
	if !sawASN {
		return fmt.Errorf("ASN database did not return an ASN for 1.1.1.1 or 8.8.8.8; expected autonomous_system_number")
	}
	if !sawCountry {
		slog.Warn("ASN database has no country codes; country rules will not match")
	}
	return nil
}
