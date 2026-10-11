package utils

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

func TestASNDatabaseLookup(t *testing.T) {
	path := writeASNDatabase(t, map[string]uint32{
		"1.1.1.1/32":    13335,
		"8.8.8.8/32":    15169,
		"45.45.45.0/24": 64500,
		"45.45.46.0/24": 0,
	})

	db, err := OpenASNDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	asn, ok, err := db.Lookup(net.ParseIP("45.45.45.10"))
	if err != nil || !ok || asn != 64500 {
		t.Fatalf("hit: asn=%d ok=%v err=%v", asn, ok, err)
	}

	mapped := net.ParseIP("45.45.45.10").To4().To16()
	asn, ok, err = db.Lookup(mapped)
	if err != nil || !ok || asn != 64500 {
		t.Fatalf("mapped: asn=%d ok=%v err=%v", asn, ok, err)
	}

	_, ok, err = db.Lookup(net.ParseIP("9.9.9.9"))
	if err != nil || ok {
		t.Fatalf("miss: ok=%v err=%v", ok, err)
	}

	asn, ok, err = db.Lookup(net.ParseIP("45.45.46.10"))
	if err != nil || !ok || asn != 0 {
		t.Fatalf("asn 0: asn=%d ok=%v err=%v", asn, ok, err)
	}
}

func TestOpenASNDatabaseRejectsMissingField(t *testing.T) {
	path := writeASNDatabase(t, map[string]uint32{
		"45.45.45.0/24": 64500,
	})
	if _, err := OpenASNDatabase(path); err == nil {
		t.Fatal("expected sanity check failure")
	}
}

func writeASNDatabase(t *testing.T, records map[string]uint32) string {
	t.Helper()
	writer, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            "iptoasn",
		RecordSize:              24,
		IPVersion:               6,
		DisableIPv4Aliasing:     true,
		IncludeReservedNetworks: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for cidr, asn := range records {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatal(err)
		}
		err = writer.Insert(network, mmdbtype.Map{
			"autonomous_system_number": mmdbtype.Uint32(asn),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "iptoasn.mmdb")
	fh, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	if _, err := writer.WriteTo(fh); err != nil {
		t.Fatal(err)
	}
	return path
}
